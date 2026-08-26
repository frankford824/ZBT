// Package tenderpool 访问平台级公开标讯池。
//
// 与 platform/tender 的区别：tender 操作的是租户内的强 RLS 表，每次查询都要
// set_config('app.tenant_id', ...) 激活策略；本包操作的 platform_tenders /
// platform_collector_runs 只存公开政府采购公告，故意不启用 RLS（见迁移
// 00036_platform_tender_pool.sql 顶部注释），因此直接使用连接池，不设置租户上下文。
package tenderpool

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidRequest = errors.New("invalid platform tender request")

const (
	maxIngestTenders          = 200
	maxExternalIDRunes        = 255
	maxTitleRunes             = 512
	maxShortTextRunes         = 255
	maxURLRunes               = 2048
	maxRunMessageRunes        = 2000
	maxRawContentBytes        = 200 * 1024
	maxTenderJSONBytes        = 64 * 1024
	maxRiskFlagItems          = 50
	maxRiskFlagItemRunes      = 500
	maxBudgetAmount           = 999_999_999_999.99
	maxFetchedCount           = 1_000_000
	defaultListLimit          = 50
	maxListLimit              = 200
	maxListOffset             = 100_000
	defaultRunsLimit          = 50
	maxRunsLimit              = 200
	rawContentPreviewRunes    = 2000
	maxRunStartedAtSkewFuture = 24 * time.Hour
)

type Store struct {
	pool *pgxpool.Pool
}

// PlatformTender 是公共池里的一条标讯。列表接口不返回 raw_content 全文（单条可达
// 200KB），只给出 RawContentPreview 供人工核对。
type PlatformTender struct {
	ID                string         `json:"id"`
	ExternalSource    string         `json:"external_source"`
	ExternalID        string         `json:"external_id"`
	Title             string         `json:"title"`
	Purchaser         string         `json:"purchaser"`
	Region            string         `json:"region"`
	NoticeTypeName    string         `json:"notice_type_name"`
	PublishDate       *time.Time     `json:"publish_date"`
	Deadline          *time.Time     `json:"deadline"`
	SourceURL         string         `json:"source_url"`
	BudgetText        string         `json:"budget_text"`
	BudgetAmount      *float64       `json:"budget_amount"`
	RawContentPreview string         `json:"raw_content_preview"`
	RequirementDims   map[string]any `json:"requirement_dims"`
	Timeline          map[string]any `json:"timeline"`
	Attachments       map[string]any `json:"attachments"`
	ReviewResult      map[string]any `json:"review_result"`
	RiskFlags         []string       `json:"risk_flags"`
	Status            string         `json:"status"`
	CollectedAt       time.Time      `json:"collected_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type CollectorRun struct {
	ID             string    `json:"id"`
	ExternalSource string    `json:"external_source"`
	Status         string    `json:"status"`
	FetchedCount   int       `json:"fetched_count"`
	IngestedCount  int       `json:"ingested_count"`
	Message        string    `json:"message"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
}

type IngestRequest struct {
	Run     CollectorRunInput `json:"run"`
	Tenders []TenderInput     `json:"tenders"`
}

type CollectorRunInput struct {
	ExternalSource string `json:"external_source"`
	Status         string `json:"status"`
	FetchedCount   int    `json:"fetched_count"`
	Message        string `json:"message"`
	StartedAt      string `json:"started_at"`
}

type TenderInput struct {
	ExternalID      string         `json:"external_id"`
	Title           string         `json:"title"`
	Purchaser       string         `json:"purchaser"`
	Region          string         `json:"region"`
	NoticeTypeName  string         `json:"notice_type_name"`
	PublishDate     string         `json:"publish_date"`
	Deadline        string         `json:"deadline"`
	SourceURL       string         `json:"source_url"`
	BudgetText      string         `json:"budget_text"`
	BudgetAmount    *float64       `json:"budget_amount"`
	RawContent      string         `json:"raw_content"`
	RequirementDims map[string]any `json:"requirement_dims"`
	Timeline        map[string]any `json:"timeline"`
	Attachments     map[string]any `json:"attachments"`
	ReviewResult    map[string]any `json:"review_result"`
	RiskFlags       []string       `json:"risk_flags"`
	Status          string         `json:"status"`
}

// IngestResult 中 Skipped 统计的是同一批推送里 external_id 重复而被丢弃的条数。
// 同 external_id 以批内最后一条为准（后到的覆盖先到的），被覆盖掉的先前条目计入 Skipped。
type IngestResult struct {
	RunID    string `json:"run_id"`
	Ingested int    `json:"ingested"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
}

type ListFilter struct {
	Search string
	Source string
	Limit  int
	Offset int
}

type normalizedRun struct {
	ExternalSource string
	Status         string
	FetchedCount   int
	Message        string
	StartedAt      time.Time
}

type normalizedTender struct {
	ExternalSource  string
	ExternalID      string
	Title           string
	Purchaser       string
	Region          string
	NoticeTypeName  string
	PublishDate     any
	Deadline        any
	SourceURL       string
	BudgetText      string
	BudgetAmount    *float64
	RawContent      string
	RequirementDims []byte
	Timeline        []byte
	Attachments     []byte
	ReviewResult    []byte
	RiskFlags       []string
	Status          string
}

type normalizedIngestRequest struct {
	Run     normalizedRun
	Tenders []normalizedTender
	Skipped int
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Ingest 在单个事务里批量 upsert 标讯并记录一次采集运行。任一条标讯校验失败则整批拒绝，
// 避免采集器把半截数据写进公共池。
func (s *Store) Ingest(ctx context.Context, req IngestRequest) (IngestResult, error) {
	normalized, err := normalizeIngestRequest(req)
	if err != nil {
		return IngestResult{}, err
	}
	result := IngestResult{Skipped: normalized.Skipped}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return IngestResult{}, err
	}
	defer tx.Rollback(ctx)
	for _, item := range normalized.Tenders {
		var inserted bool
		if err := tx.QueryRow(ctx, `
			insert into platform_tenders (
				external_source, external_id, title, purchaser, region, notice_type_name, publish_date, deadline,
				source_url, budget_text, budget_amount, raw_content, requirement_dims, timeline, attachments,
				review_result, risk_flags, status
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
			on conflict (external_source, external_id) do update
			set title = excluded.title,
				purchaser = excluded.purchaser,
				region = excluded.region,
				notice_type_name = excluded.notice_type_name,
				publish_date = excluded.publish_date,
				deadline = excluded.deadline,
				source_url = excluded.source_url,
				budget_text = excluded.budget_text,
				budget_amount = excluded.budget_amount,
				raw_content = excluded.raw_content,
				requirement_dims = excluded.requirement_dims,
				timeline = excluded.timeline,
				attachments = excluded.attachments,
				review_result = excluded.review_result,
				risk_flags = excluded.risk_flags,
				status = excluded.status,
				updated_at = now()
			returning xmax = 0
		`, item.ExternalSource, item.ExternalID, item.Title, item.Purchaser, item.Region, item.NoticeTypeName,
			item.PublishDate, item.Deadline, item.SourceURL, item.BudgetText, item.BudgetAmount, item.RawContent,
			item.RequirementDims, item.Timeline, item.Attachments, item.ReviewResult, item.RiskFlags,
			item.Status).Scan(&inserted); err != nil {
			return IngestResult{}, normalizeWriteError(err)
		}
		if inserted {
			result.Ingested++
			continue
		}
		result.Updated++
	}
	if err := tx.QueryRow(ctx, `
		insert into platform_collector_runs (external_source, status, fetched_count, ingested_count, message, started_at)
		values ($1, $2, $3, $4, $5, $6)
		returning id::text
	`, normalized.Run.ExternalSource, normalized.Run.Status, normalized.Run.FetchedCount,
		result.Ingested+result.Updated, normalized.Run.Message, normalized.Run.StartedAt).Scan(&result.RunID); err != nil {
		return IngestResult{}, normalizeWriteError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return IngestResult{}, err
	}
	return result, nil
}

func (s *Store) List(ctx context.Context, filter ListFilter) ([]PlatformTender, error) {
	source, err := normalizeOptionalExternalSource(filter.Source)
	if err != nil {
		return nil, err
	}
	args := []any{}
	conditions := []string{"true"}
	if search := strings.TrimSpace(filter.Search); search != "" {
		if err := validateTextLength(search, maxShortTextRunes); err != nil {
			return nil, err
		}
		args = append(args, "%"+search+"%")
		conditions = append(conditions, fmt.Sprintf("(title ilike $%d or purchaser ilike $%d)", len(args), len(args)))
	}
	if source != "" {
		args = append(args, source)
		conditions = append(conditions, fmt.Sprintf("external_source = $%d", len(args)))
	}
	args = append(args, clampListLimit(filter.Limit), clampListOffset(filter.Offset))
	query := fmt.Sprintf(`
		select id::text, external_source, external_id, title, purchaser, region, notice_type_name,
			publish_date, deadline, source_url, budget_text, budget_amount::float8,
			left(raw_content, %d), requirement_dims, timeline, attachments,
			review_result, risk_flags, status, collected_at, updated_at
		from platform_tenders
		where %s
		order by collected_at desc, id
		limit $%d offset $%d
	`, rawContentPreviewRunes, strings.Join(conditions, " and "), len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tenders := []PlatformTender{}
	for rows.Next() {
		tender, err := scanPlatformTender(rows)
		if err != nil {
			return nil, err
		}
		tenders = append(tenders, tender)
	}
	return tenders, rows.Err()
}

func (s *Store) ListRuns(ctx context.Context, source string, limit int) ([]CollectorRun, error) {
	normalizedSource, err := normalizeOptionalExternalSource(source)
	if err != nil {
		return nil, err
	}
	args := []any{clampRunsLimit(limit)}
	condition := "true"
	if normalizedSource != "" {
		args = append(args, normalizedSource)
		condition = fmt.Sprintf("external_source = $%d", len(args))
	}
	rows, err := s.pool.Query(ctx, `
		select id::text, external_source, status, fetched_count, ingested_count, message, started_at, finished_at
		from platform_collector_runs
		where `+condition+`
		order by finished_at desc, id
		limit $1
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []CollectorRun{}
	for rows.Next() {
		var run CollectorRun
		if err := rows.Scan(&run.ID, &run.ExternalSource, &run.Status, &run.FetchedCount, &run.IngestedCount,
			&run.Message, &run.StartedAt, &run.FinishedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPlatformTender(row scanner) (PlatformTender, error) {
	var tender PlatformTender
	var budgetAmount sql.NullFloat64
	var publishDate, deadline sql.NullTime
	var requirementDims, timeline, attachments, reviewResult []byte
	err := row.Scan(
		&tender.ID, &tender.ExternalSource, &tender.ExternalID, &tender.Title, &tender.Purchaser, &tender.Region,
		&tender.NoticeTypeName, &publishDate, &deadline, &tender.SourceURL, &tender.BudgetText, &budgetAmount,
		&tender.RawContentPreview, &requirementDims, &timeline, &attachments, &reviewResult, &tender.RiskFlags,
		&tender.Status, &tender.CollectedAt, &tender.UpdatedAt,
	)
	if err != nil {
		return PlatformTender{}, err
	}
	if budgetAmount.Valid {
		tender.BudgetAmount = &budgetAmount.Float64
	}
	if publishDate.Valid {
		tender.PublishDate = &publishDate.Time
	}
	if deadline.Valid {
		tender.Deadline = &deadline.Time
	}
	for _, field := range []struct {
		raw    []byte
		target *map[string]any
	}{
		{raw: requirementDims, target: &tender.RequirementDims},
		{raw: timeline, target: &tender.Timeline},
		{raw: attachments, target: &tender.Attachments},
		{raw: reviewResult, target: &tender.ReviewResult},
	} {
		decoded, err := unmarshalJSONObject(field.raw)
		if err != nil {
			return PlatformTender{}, err
		}
		*field.target = decoded
	}
	return tender, nil
}

func normalizeIngestRequest(req IngestRequest) (normalizedIngestRequest, error) {
	run, err := normalizeRunInput(req.Run)
	if err != nil {
		return normalizedIngestRequest{}, err
	}
	if len(req.Tenders) > maxIngestTenders {
		return normalizedIngestRequest{}, ErrInvalidRequest
	}
	positions := make(map[string]int, len(req.Tenders))
	tenders := make([]normalizedTender, 0, len(req.Tenders))
	skipped := 0
	for _, item := range req.Tenders {
		tender, err := normalizeTenderInput(run.ExternalSource, item)
		if err != nil {
			return normalizedIngestRequest{}, err
		}
		if position, exists := positions[tender.ExternalID]; exists {
			tenders[position] = tender
			skipped++
			continue
		}
		positions[tender.ExternalID] = len(tenders)
		tenders = append(tenders, tender)
	}
	return normalizedIngestRequest{Run: run, Tenders: tenders, Skipped: skipped}, nil
}

func normalizeRunInput(req CollectorRunInput) (normalizedRun, error) {
	source, err := normalizeExternalSource(req.ExternalSource)
	if err != nil {
		return normalizedRun{}, err
	}
	status, err := normalizeRunStatus(req.Status)
	if err != nil {
		return normalizedRun{}, err
	}
	message, err := normalizeOptionalText(req.Message, maxRunMessageRunes)
	if err != nil {
		return normalizedRun{}, err
	}
	if req.FetchedCount < 0 || req.FetchedCount > maxFetchedCount {
		return normalizedRun{}, ErrInvalidRequest
	}
	startedAt, err := parseRequiredTimestamp(req.StartedAt)
	if err != nil {
		return normalizedRun{}, err
	}
	return normalizedRun{
		ExternalSource: source,
		Status:         status,
		FetchedCount:   req.FetchedCount,
		Message:        message,
		StartedAt:      startedAt,
	}, nil
}

func normalizeTenderInput(source string, req TenderInput) (normalizedTender, error) {
	externalID, err := normalizeRequiredText(req.ExternalID, maxExternalIDRunes)
	if err != nil {
		return normalizedTender{}, err
	}
	title, err := normalizeRequiredText(req.Title, maxTitleRunes)
	if err != nil {
		return normalizedTender{}, err
	}
	purchaser, err := normalizeOptionalText(req.Purchaser, maxShortTextRunes)
	if err != nil {
		return normalizedTender{}, err
	}
	region, err := normalizeOptionalText(req.Region, maxShortTextRunes)
	if err != nil {
		return normalizedTender{}, err
	}
	noticeTypeName, err := normalizeOptionalText(req.NoticeTypeName, maxShortTextRunes)
	if err != nil {
		return normalizedTender{}, err
	}
	budgetText, err := normalizeOptionalText(req.BudgetText, maxShortTextRunes)
	if err != nil {
		return normalizedTender{}, err
	}
	publishDate, err := parseOptionalDate(req.PublishDate)
	if err != nil {
		return normalizedTender{}, err
	}
	deadline, err := parseOptionalDate(req.Deadline)
	if err != nil {
		return normalizedTender{}, err
	}
	sourceURL, err := normalizeOptionalHTTPURL(req.SourceURL)
	if err != nil {
		return normalizedTender{}, err
	}
	if err := validateOptionalAmount(req.BudgetAmount); err != nil {
		return normalizedTender{}, err
	}
	rawContent, err := normalizeRawContent(req.RawContent)
	if err != nil {
		return normalizedTender{}, err
	}
	riskFlags, err := normalizeTextList(req.RiskFlags)
	if err != nil {
		return normalizedTender{}, err
	}
	status, err := normalizeTenderStatus(req.Status)
	if err != nil {
		return normalizedTender{}, err
	}
	requirementDims, err := normalizeJSONObject(req.RequirementDims)
	if err != nil {
		return normalizedTender{}, err
	}
	timeline, err := normalizeJSONObject(req.Timeline)
	if err != nil {
		return normalizedTender{}, err
	}
	attachments, err := normalizeJSONObject(req.Attachments)
	if err != nil {
		return normalizedTender{}, err
	}
	reviewResult, err := normalizeJSONObject(req.ReviewResult)
	if err != nil {
		return normalizedTender{}, err
	}
	return normalizedTender{
		ExternalSource:  source,
		ExternalID:      externalID,
		Title:           title,
		Purchaser:       purchaser,
		Region:          region,
		NoticeTypeName:  noticeTypeName,
		PublishDate:     publishDate,
		Deadline:        deadline,
		SourceURL:       sourceURL,
		BudgetText:      budgetText,
		BudgetAmount:    req.BudgetAmount,
		RawContent:      rawContent,
		RequirementDims: requirementDims,
		Timeline:        timeline,
		Attachments:     attachments,
		ReviewResult:    reviewResult,
		RiskFlags:       riskFlags,
		Status:          status,
	}, nil
}

func normalizeExternalSource(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zbcg":
		return "zbcg", nil
	case "iccec":
		return "iccec", nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeOptionalExternalSource(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return normalizeExternalSource(value)
}

func normalizeRunStatus(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ok", "partial", "failed", "blocked":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeTenderStatus(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "open", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "open", "closed", "awarded", "cancelled":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeRequiredText(value string, maxRunes int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrInvalidRequest
	}
	return value, validateTextLength(value, maxRunes)
}

func normalizeOptionalText(value string, maxRunes int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return value, validateTextLength(value, maxRunes)
}

// validateTextLength 同时拒绝 NUL 字节：Postgres 的 text 列不接受 \x00，
// 放过去只会在写入时抛驱动错误，不如在入口挡掉。
func validateTextLength(value string, maxRunes int) error {
	if maxRunes <= 0 || len([]rune(value)) > maxRunes || strings.ContainsRune(value, 0) {
		return ErrInvalidRequest
	}
	return nil
}

func normalizeRawContent(value string) (string, error) {
	if len(value) > maxRawContentBytes || strings.ContainsRune(value, 0) {
		return "", ErrInvalidRequest
	}
	return value, nil
}

func normalizeTextList(values []string) ([]string, error) {
	if len(values) > maxRiskFlagItems {
		return nil, ErrInvalidRequest
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if err := validateTextLength(value, maxRiskFlagItemRunes); err != nil {
			return nil, err
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeJSONObject(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > maxTenderJSONBytes {
		return nil, ErrInvalidRequest
	}
	// jsonb 不接受字符串里的 \u0000，json.Marshal 会把 NUL 转义成该字面量。
	if bytes.Contains(raw, []byte(`\u0000`)) {
		return nil, ErrInvalidRequest
	}
	return raw, nil
}

func unmarshalJSONObject(raw []byte) (map[string]any, error) {
	result := map[string]any{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return result, nil
	}
	if err := json.Unmarshal(trimmed, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func parseOptionalDate(value string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	return parsed, nil
}

func parseRequiredTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrInvalidRequest
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, ErrInvalidRequest
	}
	if parsed.After(time.Now().Add(maxRunStartedAtSkewFuture)) {
		return time.Time{}, ErrInvalidRequest
	}
	return parsed, nil
}

func normalizeOptionalHTTPURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if err := validateTextLength(value, maxURLRunes); err != nil {
		return "", err
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", ErrInvalidRequest
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return value, nil
	default:
		return "", ErrInvalidRequest
	}
}

func validateOptionalAmount(value *float64) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > maxBudgetAmount || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return ErrInvalidRequest
	}
	return nil
}

func clampListLimit(value int) int {
	if value <= 0 {
		return defaultListLimit
	}
	if value > maxListLimit {
		return maxListLimit
	}
	return value
}

func clampListOffset(value int) int {
	if value <= 0 {
		return 0
	}
	if value > maxListOffset {
		return maxListOffset
	}
	return value
}

func clampRunsLimit(value int) int {
	if value <= 0 {
		return defaultRunsLimit
	}
	if value > maxRunsLimit {
		return maxRunsLimit
	}
	return value
}

func normalizeWriteError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "22P02", "23502", "23503", "23505", "23514":
			return ErrInvalidRequest
		}
	}
	return err
}
