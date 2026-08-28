package qualification

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	platformdb "github.com/frankford824/ZBT/backend/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type SyncCounts struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

type SyncResult struct {
	Certificates SyncCounts `json:"certificates"`
	Personnel    SyncCounts `json:"personnel"`
	// PendingReview 是本次同步后仍待人工确认的总数，前端用它提示工作量。
	PendingReview int      `json:"pending_review"`
	Warnings      []string `json:"warnings"`
}

// certificateCategories 是要同步到证书表的 zizhi 分类。
// 顺序固定，让同步日志与结果可复现。
var certificateCategories = []string{
	"license", "safety_permit", "construction_grade_2", "construction_grade_3",
	"highway_maintenance_grade_a", "labor_qualify", "iso", "credit",
	"honor", "bank_permit", "taxpayer", "equipment",
}

// SyncFromZizhi 从 zizhi-api 拉取资质并写入本租户档案。
//
// 同步是幂等的：以 source_ref 作唯一键，重复执行不会产生重复记录。
// 关键约束是「不覆盖人工结论」——只有仍处于 pending_review 的记录才会被后续同步刷新，
// 用户确认过或拒绝过的记录不再被机器改写，否则每小时一次的 zizhi 增量维护
// 会把人工修正反复冲掉。
//
// 业绩（performance 239 条）和审计报告（audit 315 条）暂不同步：zizhi 对这两类
// 只做了分类和全文，没有抽出合同金额、业主、财务指标这些关键字段，
// 导进来只会是一堆空壳记录，需要先接 LLM 抽取再落库。
func (s *Store) SyncFromZizhi(ctx context.Context, tenantID string, client *ZizhiClient) (*SyncResult, error) {
	if !client.Configured() {
		return nil, ErrZizhiNotConfigured
	}
	result := &SyncResult{Warnings: []string{}}

	for _, category := range certificateCategories {
		files, err := client.SearchByCategory(ctx, category)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("分类 %s 拉取失败: %v", category, err))
			continue
		}
		mapping, ok := MapCategory(category)
		if !ok || mapping.Target != "certificate" {
			continue
		}
		for _, f := range files {
			counts, err := s.upsertCertificate(ctx, tenantID, f, mapping)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("证书 %s 写入失败: %v", SourceRefFile(f.ID), err))
				continue
			}
			result.Certificates.Inserted += counts.Inserted
			result.Certificates.Updated += counts.Updated
			result.Certificates.Skipped += counts.Skipped
		}
	}

	people, err := client.People(ctx)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("人员拉取失败: %v", err))
	} else {
		for _, p := range people {
			counts, err := s.upsertPerson(ctx, tenantID, p)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("人员 %s 写入失败: %v", SourceRefPerson(p.ID), err))
				continue
			}
			result.Personnel.Inserted += counts.Inserted
			result.Personnel.Updated += counts.Updated
			result.Personnel.Skipped += counts.Skipped
		}
	}

	pending, err := s.countPendingReview(ctx, tenantID)
	if err == nil {
		result.PendingReview = pending
	}
	return result, nil
}

// certName 决定证书显示名。zizhi 抽出的 cert_kind 最准确（如「安全生产许可证」），
// 其次是分类标签，最后才退回文件名——文件名常带日期和版本号（「安许2029.2.24.pdf」），
// 直接当名称展示可读性差。
func certName(f ZizhiFile, mapping CategoryMapping) string {
	if k := strings.TrimSpace(f.CertKind); k != "" {
		return k
	}
	if l := strings.TrimSpace(f.CategoryLabel); l != "" {
		return l
	}
	if c := strings.TrimSpace(mapping.CertCategory); c != "" {
		return c
	}
	name := f.Name
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i]
	}
	return name
}

func (s *Store) upsertCertificate(ctx context.Context, tenantID string, f ZizhiFile, mapping CategoryMapping) (SyncCounts, error) {
	var counts SyncCounts

	level, rank := ResolveLevel(f.CertKind+" "+f.Name+" "+f.Snippet, mapping)

	suspicious := LooksLikeNonCertificate(f.Name)
	confidence := Confidence(f.CertNo, f.ValidTo, f.Issuer, f.Holder, suspicious)

	evidence := map[string]any{
		"source":        "zizhi",
		"zizhi_file_id": f.ID,
		"path":          f.Path,
		"unc_path":      f.UNCPath,
		"category":      f.Category,
		"content_state": f.ContentState,
		"snippet":       truncate(f.Snippet, 1000),
	}
	if suspicious {
		// 让审核界面能直接告诉用户「为什么这条排在前面需要看」
		evidence["warning"] = "文件名疑似申请表/模板/汇总表，不像证书原件，请核对原文"
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return counts, err
	}

	metadata := map[string]any{
		"holder":   f.Holder,
		"company":  f.Company,
		"expired":  f.Expired,
		"file_ext": f.Ext,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return counts, err
	}

	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var action string
		row := tx.QueryRow(ctx, `
insert into company_certificates (
    tenant_id, cert_category, cert_name, cert_level, cert_level_rank,
    cert_no, issuer, issued_at, expires_at, metadata,
    source_ref, verify_status, extracted_by, extract_confidence, extract_evidence
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'pending_review', 'zizhi', $12, $13)
on conflict (tenant_id, source_ref) where source_ref <> ''
do update set
    cert_category = excluded.cert_category,
    cert_name = excluded.cert_name,
    cert_level = excluded.cert_level,
    cert_level_rank = excluded.cert_level_rank,
    cert_no = excluded.cert_no,
    issuer = excluded.issuer,
    issued_at = excluded.issued_at,
    expires_at = excluded.expires_at,
    metadata = excluded.metadata,
    extract_confidence = excluded.extract_confidence,
    extract_evidence = excluded.extract_evidence,
    updated_at = now()
where company_certificates.verify_status = 'pending_review'
returning case when xmax = 0 then 'inserted' else 'updated' end`,
			tenantID, mapping.CertCategory, certName(f, mapping), level, rank,
			f.CertNo, f.Issuer, parseDate(f.ValidFrom), parseDate(f.ValidTo), metadataJSON,
			SourceRefFile(f.ID), confidence, evidenceJSON,
		)
		if scanErr := row.Scan(&action); scanErr != nil {
			if scanErr == pgx.ErrNoRows {
				// where 子句挡住了：这条已被人工确认或拒绝，保持人工结论不动。
				counts.Skipped++
				return nil
			}
			return scanErr
		}
		if action == "inserted" {
			counts.Inserted++
		} else {
			counts.Updated++
		}
		return nil
	})
	return counts, err
}

// upsertPerson 按「人」而不是按「文件」建档。
// zizhi 的 personnel 分类下有 973 个文件，但聚合后只有 155 个人；
// 投标要的是「有几个一级建造师」，按文件建档会把同一个人的身份证、合同、
// 证书拆成几十条，完全没法用。
func (s *Store) upsertPerson(ctx context.Context, tenantID string, p ZizhiPerson) (SyncCounts, error) {
	var counts SyncCounts

	kinds := splitCertKinds(p.CertKinds)
	certType := strings.Join(kinds, " / ")
	level, _ := NormalizeLevel(p.CertKinds)

	evidence := map[string]any{
		"source":          "zizhi",
		"zizhi_person_id": p.ID,
		"file_count":      p.FileCount,
		"cert_kinds":      kinds,
		"expired_count":   p.ExpiredCount,
	}
	if p.ExpiredCount > 0 {
		evidence["warning"] = fmt.Sprintf("该人员名下有 %d 份证件已过期，投标前需核实", p.ExpiredCount)
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return counts, err
	}
	metadataJSON, err := json.Marshal(map[string]any{
		"company":       p.Company,
		"file_count":    p.FileCount,
		"expired_count": p.ExpiredCount,
	})
	if err != nil {
		return counts, err
	}

	// 人员档案由 zizhi 聚合而来，字段完整度整体高于单张证书扫描件，
	// 但仍要人工确认在职状态——离职人员的证书还留在 NAS 上是常态。
	confidence := 0.6
	if p.LatestValidTo != "" {
		confidence = 0.75
	}
	if LooksLikeNonPersonName(p.Name) {
		// 姓名本身就不可信时，后面的证件信息挂在谁头上也就无从谈起，
		// 直接压到队首让人工先判断这条到底对应哪个人。
		confidence = 0.05
		evidence["warning"] = "姓名疑似 OCR 噪声或目录名误判，请核对原文确认对应人员"
		evidenceJSON, err = json.Marshal(evidence)
		if err != nil {
			return counts, err
		}
	}

	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var action string
		row := tx.QueryRow(ctx, `
insert into company_personnel (
    tenant_id, person_name, cert_type, cert_level, reg_no, expires_at,
    in_service, metadata, source_ref, verify_status, extracted_by,
    extract_confidence, extract_evidence
) values ($1, $2, $3, $4, '', $5, true, $6, $7, 'pending_review', 'zizhi', $8, $9)
on conflict (tenant_id, source_ref) where source_ref <> ''
do update set
    person_name = excluded.person_name,
    cert_type = excluded.cert_type,
    cert_level = excluded.cert_level,
    expires_at = excluded.expires_at,
    metadata = excluded.metadata,
    extract_confidence = excluded.extract_confidence,
    extract_evidence = excluded.extract_evidence,
    updated_at = now()
where company_personnel.verify_status = 'pending_review'
returning case when xmax = 0 then 'inserted' else 'updated' end`,
			tenantID, p.Name, certType, level, parseDate(p.LatestValidTo),
			metadataJSON, SourceRefPerson(p.ID), confidence, evidenceJSON,
		)
		if scanErr := row.Scan(&action); scanErr != nil {
			if scanErr == pgx.ErrNoRows {
				counts.Skipped++
				return nil
			}
			return scanErr
		}
		if action == "inserted" {
			counts.Inserted++
		} else {
			counts.Updated++
		}
		return nil
	})
	return counts, err
}

func (s *Store) countPendingReview(ctx context.Context, tenantID string) (int, error) {
	var total int
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
select (select count(*) from company_certificates where verify_status = 'pending_review')
     + (select count(*) from company_personnel where verify_status = 'pending_review')`).Scan(&total)
	})
	return total, err
}

// splitCertKinds 把 zizhi 的空格分隔证件种类拆开去重。
// 形如 "专职安全生产管理人员 劳动合同 单位权益单 技工证 身份证 项目负责人"。
func splitCertKinds(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Fields(s) {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func (s *Store) withTenant(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := platformdb.WithTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
