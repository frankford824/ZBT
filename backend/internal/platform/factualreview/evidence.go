package factualreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Finding struct {
	ChapterID     string `json:"chapter_id,omitempty"`
	RequirementID string `json:"requirement_id,omitempty"`
	Title         string `json:"title"`
	Evidence      string `json:"evidence"`
}

func textHash(text string) string {
	sum := sha256.Sum256([]byte(strings.Join(strings.Fields(text), "")))
	return hex.EncodeToString(sum[:])
}

func AuditProblem(text string, audit map[string]any, revision time.Time) string {
	if len(audit) == 0 {
		return "当前正文没有独立事实复核，请在编辑器执行自检"
	}
	if audit["content_sha256"] != textHash(text) {
		return "正文已变化，旧事实复核失效，请重新自检"
	}
	sourceRevision, _ := audit["source_revision"].(string)
	checked, err := time.Parse(time.RFC3339Nano, sourceRevision)
	if err != nil || !checked.Equal(revision) {
		return "招标文件版本已变化，请针对当前原文重新自检"
	}
	if audit["status"] != "pass" {
		return "独立事实复核未通过，请核对段落依据和遗漏要求"
	}
	rows, ok := audit["paragraphs"].([]any)
	if !ok || len(rows) == 0 {
		return "事实复核缺少逐段证据，不能视为通过"
	}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || row["status"] != "supported" {
			return "存在无依据或矛盾段落，请核对原文"
		}
	}
	return ""
}

// Findings checks current saved bodies, never the writer's self-reported score.
// Latest audit is selected independently of accepted/manual-edit versions, then
// exact content digest and current source revision decide whether it is reusable.
func Findings(ctx context.Context, tx pgx.Tx, tenantID, bidID string) ([]Finding, error) {
	findings := []Finding{}
	rows, err := tx.Query(ctx, `
        select c.id::text, c.plain_text, coalesce(v.model_metadata->'self_check'->'evidence_audit','{}'),
               p.updated_at, coalesce(p.status,'')
        from bid_chapters c
        left join bid_parse_results p on p.tenant_id=c.tenant_id and p.bid_document_id=c.bid_document_id
        left join lateral (
            select model_metadata from bid_chapter_versions
            where tenant_id=c.tenant_id and chapter_id=c.id
              and model_metadata->'self_check' ? 'evidence_audit'
            order by version_no desc limit 1
        ) v on true
        where c.tenant_id=$1 and c.bid_document_id=$2 order by c.sort_order,c.id
    `, tenantID, bidID)
	if err != nil {
		return nil, err
	}
	covered := map[string]bool{}
	type knowledgeCheck struct {
		chapter, chunk, hash string
		characters           int
	}
	knowledgeChecks := []knowledgeCheck{}
	count := 0
	for rows.Next() {
		var id, text, status string
		var raw []byte
		var revision *time.Time
		if err := rows.Scan(&id, &text, &raw, &revision, &status); err != nil {
			rows.Close()
			return nil, err
		}
		count++
		var audit map[string]any
		if err := json.Unmarshal(raw, &audit); err != nil {
			rows.Close()
			return nil, err
		}
		problem := "招标文件尚未确认，无法核对正文事实"
		if status == "confirmed" && revision != nil {
			problem = AuditProblem(text, audit, *revision)
		}
		if CheckText(text) != nil {
			problem = "正文仍有待核实事实或占位内容"
		}
		if problem != "" {
			findings = append(findings, Finding{ChapterID: id, Title: problem, Evidence: "请打开本章事实依据，修订后重新执行自检。"})
			continue
		}
		coverage, _ := audit["requirement_coverage"].([]any)
		sources, _ := audit["knowledge_sources"].(map[string]any)
		for chunk, raw := range sources {
			source, _ := raw.(map[string]any)
			hash, _ := source["sha256"].(string)
			characters, _ := source["characters"].(float64)
			knowledgeChecks = append(knowledgeChecks, knowledgeCheck{id, chunk, hash, int(characters)})
		}
		for _, raw := range coverage {
			item, _ := raw.(map[string]any)
			key, _ := item["requirement_id"].(string)
			evidence, _ := item["evidence"].(string)
			compact := strings.Join(strings.Fields(evidence), "")
			if item["status"] == "covered" && compact != "" && strings.Contains(strings.Join(strings.Fields(text), ""), compact) {
				covered[key] = true
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		findings = append(findings, Finding{Title: "没有可复核的章节正文"})
	}
	for _, check := range knowledgeChecks {
		var content string
		err := tx.QueryRow(ctx, `select kc.content from knowledge_chunks kc
            join knowledge_documents d on d.tenant_id=kc.tenant_id and d.id=kc.document_id
            where kc.tenant_id=$1 and kc.id::text=$2 and d.parse_status='processed'`, tenantID, check.chunk).Scan(&content)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		runes := []rune(strings.TrimSpace(content))
		valid := err == nil && check.characters > 0 && check.characters <= 2400 && len(runes) >= check.characters
		if valid {
			valid = textHash(string(runes[:check.characters])) == check.hash
		}
		if !valid {
			findings = append(findings, Finding{ChapterID: check.chapter, Title: "企业资料来源已变化或不可用，请重新自检", Evidence: check.chunk})
		}
	}
	// Critical requirements must be covered somewhere in the current document,
	// not declared satisfied by stale DB coverage flags or per-chapter N/A.
	requirements, err := tx.Query(ctx, `select id::text,external_id,requirement,needs_review
        from bid_requirement_items where tenant_id=$1 and bid_document_id=$2
        and (mandatory or score > 0) order by sort_order,id`, tenantID, bidID)
	if err != nil {
		return nil, err
	}
	defer requirements.Close()
	for requirements.Next() {
		var id, key, text string
		var review bool
		if err := requirements.Scan(&id, &key, &text, &review); err != nil {
			return nil, err
		}
		if review || (!covered[key] && !covered[id]) {
			findings = append(findings, Finding{RequirementID: id, Title: "强制或评分要求缺少当前正文响应证据", Evidence: text})
		}
	}
	return findings, requirements.Err()
}

func RequireChapter(ctx context.Context, tx pgx.Tx, tenantID, bidID, chapterID string) error {
	findings, err := Findings(ctx, tx, tenantID, bidID)
	if err != nil {
		return err
	}
	for _, finding := range findings {
		if finding.ChapterID == chapterID {
			return ErrRequired
		}
	}
	return nil
}
