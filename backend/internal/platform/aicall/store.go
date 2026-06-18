package aicall

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/taskstatus"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidRequest = errors.New("invalid ai call log request")

const (
	maxExactJSONInteger = int64(1<<53 - 1)
	maxAIEstimatedCost  = 100000.0
)

type Store struct {
	pool *pgxpool.Pool
}

type Log struct {
	ID            string         `json:"id"`
	UserID        *string        `json:"user_id"`
	UserName      string         `json:"user_name"`
	TraceID       string         `json:"trace_id"`
	TaskType      string         `json:"task_type"`
	Provider      string         `json:"provider"`
	Model         string         `json:"model"`
	InputTokens   int            `json:"input_tokens"`
	OutputTokens  int            `json:"output_tokens"`
	EstimatedCost float64        `json:"estimated_cost"`
	LatencyMS     int            `json:"latency_ms"`
	Status        string         `json:"status"`
	ErrorMessage  *string        `json:"error_message"`
	FallbackFrom  *string        `json:"fallback_from"`
	BizRef        map[string]any `json:"biz_ref"`
	CreatedAt     time.Time      `json:"created_at"`
}

type RecordInput struct {
	TenantID      string
	UserID        string
	TraceID       string
	TaskType      string
	Provider      string
	Model         string
	InputTokens   int
	OutputTokens  int
	EstimatedCost float64
	LatencyMS     int
	Status        string
	ErrorMessage  string
	FallbackFrom  string
	BizRef        map[string]any
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) List(ctx context.Context, tenantID string, limit int) ([]Log, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	logs := []Log{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select
				acl.id::text, acl.user_id::text, coalesce(u.name, ''),
				acl.trace_id, acl.task_type, acl.provider, acl.model,
				acl.input_tokens, acl.output_tokens, acl.estimated_cost::float8,
				acl.latency_ms, acl.status, acl.error_message, acl.fallback_from,
				acl.biz_ref, acl.created_at
			from ai_call_logs acl
			left join users u on u.id = acl.user_id
			where acl.tenant_id = $1
			order by acl.created_at desc
			limit $2
		`, tenantID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanLog(rows)
			if err != nil {
				return err
			}
			logs = append(logs, item)
		}
		return rows.Err()
	})
	return logs, err
}

func (s *Store) Record(ctx context.Context, input RecordInput) (Log, error) {
	input = normalizeRecord(input)
	if input.TenantID == "" || input.TraceID == "" || input.TaskType == "" || input.Provider == "" || input.Model == "" || input.Status == "" {
		return Log{}, ErrInvalidRequest
	}
	var log Log
	err := s.withTenant(ctx, input.TenantID, func(tx pgx.Tx) error {
		bizRefJSON, _ := json.Marshal(input.BizRef)
		created, err := scanLog(tx.QueryRow(ctx, `
			insert into ai_call_logs (
				tenant_id, user_id, trace_id, task_type, provider, model,
				input_tokens, output_tokens, estimated_cost, latency_ms, status,
				error_message, fallback_from, biz_ref
			)
			select $1, nullif($2, '')::uuid, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, nullif($12, ''), nullif($13, ''), $14
			where not exists (
				select 1
				from ai_call_logs
				where tenant_id = $1
					and trace_id = $3
					and task_type = $4
					and coalesce(biz_ref->>'external_task_id', '') = coalesce($15, '')
			)
			returning id::text, user_id::text, '', trace_id, task_type, provider, model,
				input_tokens, output_tokens, estimated_cost::float8, latency_ms, status,
				error_message, fallback_from, biz_ref, created_at
		`, input.TenantID, input.UserID, input.TraceID, input.TaskType, input.Provider, input.Model,
			input.InputTokens, input.OutputTokens, input.EstimatedCost, input.LatencyMS, input.Status, input.ErrorMessage,
			input.FallbackFrom, bizRefJSON, stringFromMap(input.BizRef, "external_task_id")))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				existing, err := scanLog(tx.QueryRow(ctx, `
					select
						acl.id::text, acl.user_id::text, coalesce(u.name, ''),
						acl.trace_id, acl.task_type, acl.provider, acl.model,
						acl.input_tokens, acl.output_tokens, acl.estimated_cost::float8,
						acl.latency_ms, acl.status, acl.error_message, acl.fallback_from,
						acl.biz_ref, acl.created_at
					from ai_call_logs acl
					left join users u on u.id = acl.user_id
					where acl.tenant_id = $1
						and acl.trace_id = $2
						and acl.task_type = $3
						and coalesce(acl.biz_ref->>'external_task_id', '') = coalesce($4, '')
					order by acl.created_at desc
					limit 1
				`, input.TenantID, input.TraceID, input.TaskType, stringFromMap(input.BizRef, "external_task_id")))
				if err != nil {
					return err
				}
				if shouldUpdateExistingLog(existing.Status, input.Status) {
					updated, err := scanLog(tx.QueryRow(ctx, `
						update ai_call_logs
						set user_id = nullif($3, '')::uuid,
							provider = $4,
							model = $5,
							input_tokens = $6,
							output_tokens = $7,
							estimated_cost = $8,
							latency_ms = $9,
							status = $10,
							error_message = nullif($11, ''),
							fallback_from = nullif($12, ''),
							biz_ref = $13
						where tenant_id = $1 and id = $2
						returning id::text, user_id::text, '', trace_id, task_type, provider, model,
							input_tokens, output_tokens, estimated_cost::float8, latency_ms, status,
							error_message, fallback_from, biz_ref, created_at
					`, input.TenantID, existing.ID, input.UserID, input.Provider, input.Model, input.InputTokens,
						input.OutputTokens, input.EstimatedCost, input.LatencyMS, input.Status, input.ErrorMessage,
						input.FallbackFrom, bizRefJSON))
					if err != nil {
						return err
					}
					log = updated
					return nil
				}
				log = existing
				return nil
			}
			return err
		}
		log = created
		return nil
	})
	return log, err
}

func (s *Store) RecordTaskCallback(ctx context.Context, tenantID, externalTaskID string, callbackResult map[string]any, callbackStatus, callbackError string) (Log, error) {
	if tenantID == "" || externalTaskID == "" {
		return Log{}, ErrInvalidRequest
	}
	var input RecordInput
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var task struct {
			ID           string
			UserID       sql.NullString
			TaskType     string
			Status       string
			ResourceType string
			ResourceID   string
			Route        map[string]any
			Result       map[string]any
			ErrorMessage sql.NullString
			CreatedAt    time.Time
			CompletedAt  sql.NullTime
		}
		var routeRaw, resultRaw []byte
		if err := tx.QueryRow(ctx, `
			select id::text, user_id::text, task_type, status, resource_type, resource_id::text,
				route, result, error_message, created_at, completed_at
			from ai_tasks
			where tenant_id = $1 and external_task_id = $2
		`, tenantID, externalTaskID).Scan(
			&task.ID, &task.UserID, &task.TaskType, &task.Status, &task.ResourceType, &task.ResourceID,
			&routeRaw, &resultRaw, &task.ErrorMessage, &task.CreatedAt, &task.CompletedAt,
		); err != nil {
			return err
		}
		_ = json.Unmarshal(routeRaw, &task.Route)
		_ = json.Unmarshal(resultRaw, &task.Result)
		if callbackResult != nil {
			task.Result = callbackResult
		}
		status := task.Status
		if callbackStatus != "" {
			status = callbackStatus
		}
		errorMessage := ""
		if task.ErrorMessage.Valid {
			errorMessage = task.ErrorMessage.String
		}
		if callbackError != "" {
			errorMessage = callbackError
		}
		input = recordFromTask(tenantID, externalTaskID, task.ID, task.UserID, task.TaskType, status, task.ResourceType, task.ResourceID, task.Route, task.Result, errorMessage, task.CreatedAt, task.CompletedAt)
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Log{}, ErrInvalidRequest
		}
		return Log{}, err
	}
	return s.Record(ctx, input)
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

func recordFromTask(
	tenantID, externalTaskID, localTaskID string,
	userID sql.NullString,
	taskType, status, resourceType, resourceID string,
	route map[string]any,
	result map[string]any,
	errorMessage string,
	createdAt time.Time,
	completedAt sql.NullTime,
) RecordInput {
	modelMetadata := mapFromMap(result, "model_metadata")
	tokenUsage := mapFromMap(result, "token_usage")
	provider := firstNonEmpty(stringFromMap(modelMetadata, "provider"), stringFromMap(route, "provider"), "unknown")
	model := firstNonEmpty(stringFromMap(modelMetadata, "model"), stringFromMap(route, "model"), "unknown")
	traceID := firstNonEmpty(stringFromMap(result, "trace_id"), externalTaskID)
	completed := time.Now()
	if completedAt.Valid {
		completed = completedAt.Time
	}
	latencyMS := int(completed.Sub(createdAt).Milliseconds())
	if latencyMS < 0 {
		latencyMS = 0
	}
	user := ""
	if userID.Valid {
		user = userID.String
	}
	return RecordInput{
		TenantID:      tenantID,
		UserID:        user,
		TraceID:       traceID,
		TaskType:      taskType,
		Provider:      provider,
		Model:         model,
		InputTokens:   intFromMap(tokenUsage, "input_tokens"),
		OutputTokens:  intFromMap(tokenUsage, "output_tokens"),
		EstimatedCost: estimatedCostFromResult(result, modelMetadata),
		LatencyMS:     latencyMS,
		Status:        normalizeStatus(status),
		ErrorMessage:  errorMessage,
		FallbackFrom:  firstNonEmpty(stringFromMap(modelMetadata, "fallback_from"), stringFromMap(route, "fallback_from")),
		BizRef: map[string]any{
			"local_task_id":     localTaskID,
			"external_task_id":  externalTaskID,
			"resource_type":     resourceType,
			"resource_id":       resourceID,
			"module":            moduleForTask(resourceType, taskType),
			"stage":             stageForTask(taskType),
			"provider_from":     providerSource(modelMetadata, route),
			"callback_recorded": true,
		},
	}
}

func estimatedCostFromResult(result, modelMetadata map[string]any) float64 {
	if cost := floatFromMap(modelMetadata, "estimated_cost"); cost != 0 {
		return cost
	}
	return floatFromMap(result, "estimated_cost")
}

func normalizeRecord(input RecordInput) RecordInput {
	input.TraceID = strings.TrimSpace(input.TraceID)
	input.TaskType = strings.TrimSpace(input.TaskType)
	input.Provider = strings.TrimSpace(input.Provider)
	input.Model = strings.TrimSpace(input.Model)
	input.Status = normalizeStatus(input.Status)
	if input.Provider == "" {
		input.Provider = "unknown"
	}
	if input.Model == "" {
		input.Model = "unknown"
	}
	if input.BizRef == nil {
		input.BizRef = map[string]any{}
	}
	if input.InputTokens < 0 {
		input.InputTokens = 0
	}
	if input.OutputTokens < 0 {
		input.OutputTokens = 0
	}
	input.EstimatedCost = sanitizeCost(input.EstimatedCost)
	if input.EstimatedCost == 0 {
		input.EstimatedCost = sanitizeCost(estimateCost(input.Provider, input.Model, input.InputTokens, input.OutputTokens))
	}
	if input.LatencyMS < 0 {
		input.LatencyMS = 0
	}
	return input
}

func sanitizeCost(value float64) float64 {
	if value < 0 || value > maxAIEstimatedCost || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*10000) / 10000
}

func shouldUpdateExistingLog(currentStatus, nextStatus string) bool {
	return taskstatus.ShouldApplyCallback(currentStatus, nextStatus)
}

func moduleForTask(resourceType, taskType string) string {
	switch strings.TrimSpace(strings.ToLower(resourceType)) {
	case "bid_parse_result", "bid_document", "bid_chapter", "bid_export":
		return "bid"
	case "knowledge_document":
		return "knowledge"
	case "cost_project":
		return "cost"
	case "compliance_check":
		return "compliance"
	}
	switch strings.TrimSpace(strings.ToLower(taskType)) {
	case "knowledge_process", "knowledge_embedding", "knowledge_rerank":
		return "knowledge"
	case "cost_advice":
		return "cost"
	case "compliance_check":
		return "compliance"
	case "tender_parse", "outline_generate", "chapter_generate", "chapter_ai_action", "document_export":
		return "bid"
	default:
		return ""
	}
}

func stageForTask(taskType string) string {
	switch strings.TrimSpace(strings.ToLower(taskType)) {
	case "tender_parse":
		return "interpret"
	case "outline_generate":
		return "plan"
	case "chapter_generate", "chapter_ai_action":
		return "generate"
	case "compliance_check":
		return "check"
	case "document_export":
		return "format"
	case "knowledge_process":
		return "ingest"
	case "knowledge_embedding":
		return "embed"
	case "knowledge_rerank":
		return "rerank"
	case "cost_advice":
		return "advise"
	default:
		return ""
	}
}

type pricingRate struct {
	InputPer1K  float64 `json:"input_per_1k"`
	OutputPer1K float64 `json:"output_per_1k"`
	InputPer1M  float64 `json:"input_per_1m"`
	OutputPer1M float64 `json:"output_per_1m"`
}

func estimateCost(provider, model string, inputTokens, outputTokens int) float64 {
	if inputTokens <= 0 && outputTokens <= 0 {
		return 0
	}
	rate, ok := pricingFor(provider, model)
	if !ok {
		return 0
	}
	inputPer1K := sanitizeCost(rate.InputPer1K)
	outputPer1K := sanitizeCost(rate.OutputPer1K)
	if inputPer1K == 0 && sanitizeCost(rate.InputPer1M) > 0 {
		inputPer1K = sanitizeCost(rate.InputPer1M) / 1000
	}
	if outputPer1K == 0 && sanitizeCost(rate.OutputPer1M) > 0 {
		outputPer1K = sanitizeCost(rate.OutputPer1M) / 1000
	}
	return sanitizeCost((float64(inputTokens)*inputPer1K + float64(outputTokens)*outputPer1K) / 1000)
}

func pricingFor(provider, model string) (pricingRate, bool) {
	raw := strings.TrimSpace(os.Getenv("AI_MODEL_PRICING_JSON"))
	if raw == "" {
		return pricingRate{}, false
	}
	var pricing map[string]pricingRate
	if err := json.Unmarshal([]byte(raw), &pricing); err != nil {
		return pricingRate{}, false
	}
	for _, key := range pricingLookupKeys(provider, model) {
		if rate, ok := pricing[key]; ok {
			return rate, true
		}
	}
	return pricingRate{}, false
}

func pricingLookupKeys(provider, model string) []string {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	lowerProvider := strings.ToLower(provider)
	lowerModel := strings.ToLower(model)
	keys := make([]string, 0, 7)
	seen := map[string]bool{}
	for _, key := range []string{
		provider + "/" + model,
		model,
		provider + "/*",
		"*",
		lowerProvider + "/" + lowerModel,
		lowerModel,
		lowerProvider + "/*",
	} {
		if !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	return keys
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "done", "success", "succeeded":
		return "done"
	case "failed", "error":
		return "failed"
	case "running", "queued", "cancelled", "canceled":
		if strings.ToLower(strings.TrimSpace(value)) == "canceled" {
			return "cancelled"
		}
		return strings.ToLower(strings.TrimSpace(value))
	default:
		if strings.TrimSpace(value) == "" {
			return "done"
		}
		return ""
	}
}

func scanLog(row scanner) (Log, error) {
	var log Log
	var userID, errorMessage, fallbackFrom sql.NullString
	var bizRefRaw []byte
	err := row.Scan(
		&log.ID, &userID, &log.UserName, &log.TraceID, &log.TaskType, &log.Provider, &log.Model,
		&log.InputTokens, &log.OutputTokens, &log.EstimatedCost, &log.LatencyMS, &log.Status,
		&errorMessage, &fallbackFrom, &bizRefRaw, &log.CreatedAt,
	)
	if userID.Valid {
		log.UserID = &userID.String
	}
	if errorMessage.Valid {
		log.ErrorMessage = &errorMessage.String
	}
	if fallbackFrom.Valid {
		log.FallbackFrom = &fallbackFrom.String
	}
	log.BizRef = map[string]any{}
	_ = json.Unmarshal(bizRefRaw, &log.BizRef)
	return log, err
}

type scanner interface {
	Scan(dest ...any) error
}

func mapFromMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return map[string]any{}
	}
	value, ok := parent[key]
	if !ok {
		return map[string]any{}
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[string]string:
		result := map[string]any{}
		for key, value := range typed {
			result[key] = value
		}
		return result
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return map[string]any{}
		}
		result := map[string]any{}
		_ = json.Unmarshal(raw, &result)
		return result
	}
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	switch typed := values[key].(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func intFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	switch typed := values[key].(type) {
	case int:
		if typed < 0 {
			return 0
		}
		return typed
	case int64:
		if typed < 0 || typed > int64(maxInt()) {
			return 0
		}
		return int(typed)
	case float64:
		if typed < 0 || math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed || typed > float64(maxFloatTokenInteger()) {
			return 0
		}
		return int(typed)
	case json.Number:
		value, err := typed.Int64()
		if err != nil || value < 0 || value > int64(maxInt()) {
			return 0
		}
		return int(value)
	case string:
		value, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 0)
		if err == nil && value >= 0 {
			return int(value)
		}
		return 0
	default:
		return 0
	}
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func maxFloatTokenInteger() int64 {
	max := int64(maxInt())
	if max > maxExactJSONInteger {
		return maxExactJSONInteger
	}
	return max
}

func floatFromMap(values map[string]any, key string) float64 {
	if values == nil {
		return 0
	}
	switch typed := values[key].(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		value, _ := typed.Float64()
		return value
	case string:
		value, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return value
		}
		return 0
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func providerSource(modelMetadata, route map[string]any) string {
	if stringFromMap(modelMetadata, "provider") != "" || stringFromMap(modelMetadata, "model") != "" {
		return "model_metadata"
	}
	if stringFromMap(route, "provider") != "" || stringFromMap(route, "model") != "" {
		return "route"
	}
	return "unknown"
}
