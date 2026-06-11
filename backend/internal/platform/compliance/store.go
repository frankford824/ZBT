package compliance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("compliance resource not found")
	ErrInvalidRequest = errors.New("invalid compliance request")
)

type Store struct {
	pool *pgxpool.Pool
}

type Check struct {
	ID            string         `json:"id"`
	BidDocumentID *string        `json:"bid_document_id"`
	BidTitle      string         `json:"bid_title"`
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	ResultStatus  string         `json:"result_status"`
	Score         int            `json:"score"`
	Config        map[string]any `json:"config"`
	TaskID        *string        `json:"task_id"`
	IssueCount    int            `json:"issue_count"`
	StartedAt     *time.Time     `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type Issue struct {
	ID         string         `json:"id"`
	CheckID    string         `json:"check_id"`
	RuleID     *string        `json:"rule_id"`
	Category   string         `json:"category"`
	Severity   string         `json:"severity"`
	Status     string         `json:"status"`
	Title      string         `json:"title"`
	Evidence   string         `json:"evidence"`
	Suggestion string         `json:"suggestion"`
	Location   map[string]any `json:"location"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type Rule struct {
	ID          string         `json:"id"`
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Level       string         `json:"level"`
	Severity    string         `json:"severity"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Report struct {
	ID        string         `json:"id"`
	CheckID   string         `json:"check_id"`
	Status    string         `json:"status"`
	Summary   string         `json:"summary"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type CheckSnapshot struct {
	Check     Check     `json:"check"`
	Issues    []Issue   `json:"issues"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCheckRequest struct {
	Name          string   `json:"name"`
	BidDocumentID string   `json:"bid_document_id"`
	Levels        []string `json:"levels"`
}

type CreateRuleRequest struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Level       string         `json:"level"`
	Severity    string         `json:"severity"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Metadata    map[string]any `json:"metadata"`
}

type UpdateRuleRequest = CreateRuleRequest

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListChecks(ctx context.Context, tenantID string) ([]Check, error) {
	checks := []Check{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, checkSelectSQL()+`
			where cc.tenant_id = $1
			order by cc.created_at desc
			limit 100
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			check, err := scanCheck(rows)
			if err != nil {
				return err
			}
			checks = append(checks, check)
		}
		return rows.Err()
	})
	return checks, err
}

func (s *Store) CreateCheck(ctx context.Context, tenantID string, req CreateCheckRequest) (CheckSnapshot, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "合规检查"
	}
	levels := normalizeLevels(req.Levels)
	config := map[string]any{"levels": levels}
	configRaw, _ := json.Marshal(config)
	taskID := "compliance-check-" + uuid.NewString()
	var checkID string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		bidID, err := nullableUUID(req.BidDocumentID)
		if err != nil {
			return ErrInvalidRequest
		}
		if bidID != nil {
			var exists bool
			if err := tx.QueryRow(ctx, `select exists(select 1 from bid_documents where tenant_id = $1 and id = $2)`, tenantID, bidID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return pgx.ErrNoRows
			}
		}
		if err := tx.QueryRow(ctx, `
			insert into compliance_checks (tenant_id, bid_document_id, name, status, result_status, score, config, task_id, started_at, completed_at)
			values ($1, $2, $3, 'running', 'pass', 100, $4, $5, now(), null)
			returning id::text
		`, tenantID, bidID, name, configRaw, taskID).Scan(&checkID); err != nil {
			return err
		}
		bidDocumentID, _ := bidID.(string)
		if err := s.generateIssues(ctx, tx, tenantID, checkID, bidDocumentID, levels); err != nil {
			return err
		}
		resultStatus, score, err := summarizeIssues(ctx, tx, tenantID, checkID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			update compliance_checks
			set status = 'done', result_status = $3, score = $4, completed_at = now(), updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, checkID, resultStatus, score)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CheckSnapshot{}, ErrNotFound
	}
	if err != nil {
		return CheckSnapshot{}, err
	}
	return s.Snapshot(ctx, tenantID, checkID)
}

func (s *Store) GetCheck(ctx context.Context, tenantID, id string) (Check, error) {
	if err := validateUUID(id); err != nil {
		return Check{}, err
	}
	var check Check
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanCheck(tx.QueryRow(ctx, checkSelectSQL()+` where cc.tenant_id = $1 and cc.id = $2`, tenantID, id))
		if err != nil {
			return err
		}
		check = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Check{}, ErrNotFound
	}
	return check, err
}

func (s *Store) ListIssues(ctx context.Context, tenantID, checkID string) ([]Issue, error) {
	if err := validateUUID(checkID); err != nil {
		return nil, err
	}
	issues := []Issue{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, check_id::text, rule_id::text, category, severity, status, title, evidence, suggestion, location, created_at, updated_at
			from compliance_issues
			where tenant_id = $1 and check_id = $2
			order by case severity when 'fail' then 0 when 'fail_candidate' then 1 when 'warn' then 2 else 3 end, created_at
		`, tenantID, checkID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			issue, err := scanIssue(rows)
			if err != nil {
				return err
			}
			issues = append(issues, issue)
		}
		return rows.Err()
	})
	return issues, err
}

func (s *Store) Snapshot(ctx context.Context, tenantID, checkID string) (CheckSnapshot, error) {
	check, err := s.GetCheck(ctx, tenantID, checkID)
	if err != nil {
		return CheckSnapshot{}, err
	}
	issues, err := s.ListIssues(ctx, tenantID, checkID)
	if err != nil {
		return CheckSnapshot{}, err
	}
	return CheckSnapshot{Check: check, Issues: issues, UpdatedAt: time.Now()}, nil
}

func (s *Store) AutofixIssue(ctx context.Context, tenantID, issueID string) (Issue, error) {
	return s.updateIssueStatus(ctx, tenantID, issueID, "fixed", "autofix", "规则化修复动作已记录，可通过问题定位跳转到编辑器复核。")
}

func (s *Store) IgnoreIssue(ctx context.Context, tenantID, issueID string) (Issue, error) {
	return s.updateIssueStatus(ctx, tenantID, issueID, "ignored", "ignore", "问题已忽略。")
}

func (s *Store) ConfirmFailIssue(ctx context.Context, tenantID, issueID string) (Issue, error) {
	if err := validateUUID(issueID); err != nil {
		return Issue{}, err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update compliance_issues
			set severity = 'fail', status = 'confirmed_fail', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, issueID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		_, err = tx.Exec(ctx, `
			insert into compliance_fix_logs (tenant_id, issue_id, action, status, message)
			values ($1, $2, 'confirm_fail', 'done', '人工确认 fail_candidate 为 fail')
		`, tenantID, issueID)
		if err != nil {
			return err
		}
		return refreshCheckSummary(ctx, tx, tenantID, issueID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Issue{}, ErrNotFound
	}
	if err != nil {
		return Issue{}, err
	}
	return s.GetIssue(ctx, tenantID, issueID)
}

func (s *Store) CreateReport(ctx context.Context, tenantID, checkID string) (Report, error) {
	snapshot, err := s.Snapshot(ctx, tenantID, checkID)
	if err != nil {
		return Report{}, err
	}
	metadata, _ := json.Marshal(map[string]any{
		"score":         snapshot.Check.Score,
		"result_status": snapshot.Check.ResultStatus,
		"issue_count":   len(snapshot.Issues),
	})
	summary := "合规得分 " + itoa(snapshot.Check.Score) + "，结果 " + snapshot.Check.ResultStatus + "，问题 " + itoa(len(snapshot.Issues)) + " 项"
	var report Report
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `
			insert into compliance_reports (tenant_id, check_id, status, summary, metadata)
			values ($1, $2, 'generated', $3, $4)
			returning id::text, check_id::text, status, summary, metadata, created_at, updated_at
		`, tenantID, checkID, summary, metadata).Scan(&report.ID, &report.CheckID, &report.Status, &report.Summary, &raw, &report.CreatedAt, &report.UpdatedAt); err != nil {
			return err
		}
		report.Metadata = map[string]any{}
		_ = json.Unmarshal(raw, &report.Metadata)
		return nil
	})
	return report, err
}

func (s *Store) ListRules(ctx context.Context, tenantID string) ([]Rule, error) {
	rules := []Rule{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, code, name, category, level, severity, description, enabled, metadata, created_at, updated_at
			from compliance_rules
			where tenant_id = $1
			order by level, category, code
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rule, err := scanRule(rows)
			if err != nil {
				return err
			}
			rules = append(rules, rule)
		}
		return rows.Err()
	})
	return rules, err
}

func (s *Store) CreateRule(ctx context.Context, tenantID string, req CreateRuleRequest) (Rule, error) {
	normalized, err := normalizeRule(req)
	if err != nil {
		return Rule{}, err
	}
	metadata, _ := json.Marshal(normalized.Metadata)
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			insert into compliance_rules (tenant_id, code, name, category, level, severity, description, enabled, metadata)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			returning id::text
		`, tenantID, normalized.Code, normalized.Name, normalized.Category, normalized.Level, normalized.Severity, normalized.Description, normalized.Enabled, metadata).Scan(&id)
	})
	if err != nil {
		return Rule{}, err
	}
	return s.GetRule(ctx, tenantID, id)
}

func (s *Store) UpdateRule(ctx context.Context, tenantID, id string, req UpdateRuleRequest) (Rule, error) {
	if err := validateUUID(id); err != nil {
		return Rule{}, err
	}
	normalized, err := normalizeRule(req)
	if err != nil {
		return Rule{}, err
	}
	metadata, _ := json.Marshal(normalized.Metadata)
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update compliance_rules
			set code = $3, name = $4, category = $5, level = $6, severity = $7, description = $8, enabled = $9, metadata = $10, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, normalized.Code, normalized.Name, normalized.Category, normalized.Level, normalized.Severity, normalized.Description, normalized.Enabled, metadata)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, ErrNotFound
	}
	if err != nil {
		return Rule{}, err
	}
	return s.GetRule(ctx, tenantID, id)
}

func (s *Store) DeleteRule(ctx context.Context, tenantID, id string) error {
	if err := validateUUID(id); err != nil {
		return err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `delete from compliance_rules where tenant_id = $1 and id = $2`, tenantID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) GetIssue(ctx context.Context, tenantID, issueID string) (Issue, error) {
	var issue Issue
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanIssue(tx.QueryRow(ctx, `
			select id::text, check_id::text, rule_id::text, category, severity, status, title, evidence, suggestion, location, created_at, updated_at
			from compliance_issues
			where tenant_id = $1 and id = $2
		`, tenantID, issueID))
		if err != nil {
			return err
		}
		issue = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Issue{}, ErrNotFound
	}
	return issue, err
}

func (s *Store) GetRule(ctx context.Context, tenantID, id string) (Rule, error) {
	var rule Rule
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanRule(tx.QueryRow(ctx, `
			select id::text, code, name, category, level, severity, description, enabled, metadata, created_at, updated_at
			from compliance_rules
			where tenant_id = $1 and id = $2
		`, tenantID, id))
		if err != nil {
			return err
		}
		rule = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, ErrNotFound
	}
	return rule, err
}

func (s *Store) updateIssueStatus(ctx context.Context, tenantID, issueID, status, action, message string) (Issue, error) {
	if err := validateUUID(issueID); err != nil {
		return Issue{}, err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `update compliance_issues set status = $3, updated_at = now() where tenant_id = $1 and id = $2`, tenantID, issueID, status)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		_, err = tx.Exec(ctx, `
			insert into compliance_fix_logs (tenant_id, issue_id, action, status, message)
			values ($1, $2, $3, 'done', $4)
		`, tenantID, issueID, action, message)
		if err != nil {
			return err
		}
		return refreshCheckSummary(ctx, tx, tenantID, issueID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Issue{}, ErrNotFound
	}
	if err != nil {
		return Issue{}, err
	}
	return s.GetIssue(ctx, tenantID, issueID)
}

func (s *Store) generateIssues(ctx context.Context, tx pgx.Tx, tenantID, checkID, bidID string, levels []string) error {
	selectedLevels := map[string]bool{}
	for _, level := range levels {
		selectedLevels[level] = true
	}
	rows, err := tx.Query(ctx, `
		select id::text, code, name, category, level, severity, description, enabled, metadata, created_at, updated_at
		from compliance_rules
		where tenant_id = $1 and enabled
		order by level, code
	`, tenantID)
	if err != nil {
		return err
	}
	rules := []Rule{}
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			rows.Close()
			return err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, rule := range rules {
		if rule.Severity == "pass" {
			continue
		}
		if !selectedLevels[rule.Level] {
			continue
		}
		evidence, suggestion := evidenceForRule(rule)
		locationMap, err := buildIssueLocation(ctx, tx, tenantID, bidID, rule)
		if err != nil {
			return err
		}
		location, _ := json.Marshal(locationMap)
		if _, err := tx.Exec(ctx, `
			insert into compliance_issues (tenant_id, check_id, rule_id, category, severity, status, title, evidence, suggestion, location)
			values ($1, $2, $3, $4, $5, 'open', $6, $7, $8, $9)
		`, tenantID, checkID, rule.ID, rule.Category, rule.Severity, rule.Name, evidence, suggestion, location); err != nil {
			return err
		}
	}
	return nil
}

func buildIssueLocation(ctx context.Context, tx pgx.Tx, tenantID, bidID string, rule Rule) (map[string]any, error) {
	location := map[string]any{
		"module": "bid_editor",
		"anchor": rule.Code,
	}
	if bidID == "" {
		return location, nil
	}
	location["bid_document_id"] = bidID

	chapterID, partCode, err := firstBidChapterLocation(ctx, tx, tenantID, bidID)
	if err != nil {
		return nil, err
	}
	if chapterID != "" {
		location["chapter_id"] = chapterID
	}
	if partCode != "" {
		location["part_code"] = partCode
	}

	params := url.Values{}
	if partCode != "" {
		params.Set("part", partCode)
	}
	if chapterID != "" {
		params.Set("chapter", chapterID)
	}
	path := "/bids/" + bidID + "/editor"
	if query := params.Encode(); query != "" {
		path += "?" + query
	}
	location["path"] = path
	return location, nil
}

func firstBidChapterLocation(ctx context.Context, tx pgx.Tx, tenantID, bidID string) (string, string, error) {
	var chapterID, partCode string
	err := tx.QueryRow(ctx, `
		select bc.id::text, bp.code
		from bid_chapters bc
		join bid_parts bp on bp.tenant_id = bc.tenant_id and bp.id = bc.bid_part_id
		where bc.tenant_id = $1 and bc.bid_document_id = $2
		order by bp.sort_order, bc.sort_order, bc.created_at
		limit 1
	`, tenantID, bidID).Scan(&chapterID, &partCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return chapterID, partCode, err
}

func (s *Store) withTenant(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type scanner interface {
	Scan(dest ...any) error
}

func checkSelectSQL() string {
	return `
		select cc.id::text, cc.bid_document_id::text, coalesce(bd.title, ''), cc.name, cc.status,
			cc.result_status, cc.score, cc.config, cc.task_id, coalesce(issue_counts.issue_count, 0)::int,
			cc.started_at, cc.completed_at, cc.created_at, cc.updated_at
		from compliance_checks cc
		left join bid_documents bd on bd.id = cc.bid_document_id and bd.tenant_id = cc.tenant_id
		left join lateral (
			select count(*) as issue_count
			from compliance_issues ci
			where ci.tenant_id = cc.tenant_id and ci.check_id = cc.id
		) issue_counts on true
	`
}

func scanCheck(row scanner) (Check, error) {
	var check Check
	var bidID, taskID sql.NullString
	var configRaw []byte
	var startedAt, completedAt sql.NullTime
	err := row.Scan(
		&check.ID, &bidID, &check.BidTitle, &check.Name, &check.Status, &check.ResultStatus, &check.Score,
		&configRaw, &taskID, &check.IssueCount, &startedAt, &completedAt, &check.CreatedAt, &check.UpdatedAt,
	)
	if bidID.Valid {
		check.BidDocumentID = &bidID.String
	}
	if taskID.Valid {
		check.TaskID = &taskID.String
	}
	if startedAt.Valid {
		check.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		check.CompletedAt = &completedAt.Time
	}
	check.Config = map[string]any{}
	_ = json.Unmarshal(configRaw, &check.Config)
	return check, err
}

func scanIssue(row scanner) (Issue, error) {
	var issue Issue
	var ruleID sql.NullString
	var locationRaw []byte
	err := row.Scan(
		&issue.ID, &issue.CheckID, &ruleID, &issue.Category, &issue.Severity, &issue.Status,
		&issue.Title, &issue.Evidence, &issue.Suggestion, &locationRaw, &issue.CreatedAt, &issue.UpdatedAt,
	)
	if ruleID.Valid {
		issue.RuleID = &ruleID.String
	}
	issue.Location = map[string]any{}
	_ = json.Unmarshal(locationRaw, &issue.Location)
	return issue, err
}

func scanRule(row scanner) (Rule, error) {
	var rule Rule
	var metadataRaw []byte
	err := row.Scan(&rule.ID, &rule.Code, &rule.Name, &rule.Category, &rule.Level, &rule.Severity, &rule.Description, &rule.Enabled, &metadataRaw, &rule.CreatedAt, &rule.UpdatedAt)
	rule.Metadata = map[string]any{}
	_ = json.Unmarshal(metadataRaw, &rule.Metadata)
	return rule, err
}

func refreshCheckSummary(ctx context.Context, tx pgx.Tx, tenantID, issueID string) error {
	var checkID string
	if err := tx.QueryRow(ctx, `select check_id::text from compliance_issues where tenant_id = $1 and id = $2`, tenantID, issueID).Scan(&checkID); err != nil {
		return err
	}
	resultStatus, score, err := summarizeIssues(ctx, tx, tenantID, checkID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		update compliance_checks
		set result_status = $3, score = $4, updated_at = now()
		where tenant_id = $1 and id = $2
	`, tenantID, checkID, resultStatus, score)
	return err
}

func summarizeIssues(ctx context.Context, tx pgx.Tx, tenantID, checkID string) (string, int, error) {
	rows, err := tx.Query(ctx, `
		select severity, count(*)
		from compliance_issues
		where tenant_id = $1 and check_id = $2 and status in ('open', 'confirmed_fail')
		group by severity
	`, tenantID, checkID)
	if err != nil {
		return "pass", 100, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var severity string
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			return "pass", 100, err
		}
		counts[severity] = count
	}
	result := "pass"
	if counts["warn"] > 0 {
		result = "warn"
	}
	if counts["fail_candidate"] > 0 {
		result = "fail_candidate"
	}
	if counts["fail"] > 0 {
		result = "fail"
	}
	score := 100 - counts["warn"]*5 - counts["fail_candidate"]*12 - counts["fail"]*25
	if score < 0 {
		score = 0
	}
	return result, score, rows.Err()
}

func normalizeLevels(levels []string) []string {
	if len(levels) == 0 {
		return []string{"L1", "L2", "L3"}
	}
	allowed := map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true}
	result := []string{}
	for _, level := range levels {
		level = strings.ToUpper(strings.TrimSpace(level))
		if allowed[level] {
			result = append(result, level)
		}
	}
	if len(result) == 0 {
		return []string{"L1", "L2", "L3"}
	}
	return result
}

func normalizeRule(req CreateRuleRequest) (CreateRuleRequest, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	req.Level = strings.ToUpper(strings.TrimSpace(req.Level))
	req.Severity = strings.ToLower(strings.TrimSpace(req.Severity))
	req.Description = strings.TrimSpace(req.Description)
	if req.Code == "" || req.Name == "" || req.Category == "" {
		return req, ErrInvalidRequest
	}
	if !map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true}[req.Level] {
		req.Level = "L1"
	}
	if !map[string]bool{"pass": true, "warn": true, "fail_candidate": true, "fail": true}[req.Severity] {
		req.Severity = "warn"
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	return req, nil
}

func validateUUID(value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func nullableUUID(value string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if _, err := uuid.Parse(value); err != nil {
		return nil, err
	}
	return value, nil
}

func evidenceForRule(rule Rule) (string, string) {
	switch rule.Code {
	case "signature_required":
		return "系统未检测到完整签章确认记录。", "补充法定代表人签章、单位盖章并重新导出。"
	case "validity_days":
		return "投标有效期需不少于 90 天。", "将投标有效期调整为 90 天以上，并检查商务响应表。"
	case "response_time_semantic":
		return "招标要求服务响应时间 2 小时内，当前响应承诺证据不足。", "补充 2 小时响应承诺和保障机制，人工确认后可降级风险。"
	default:
		return rule.Description, "根据规则库建议补充或修正文档内容。"
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
