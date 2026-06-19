package cost

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/frankford824/ZBT/backend/internal/platform/aihttp"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	"github.com/frankford824/ZBT/backend/internal/platform/taskstatus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("cost resource not found")
	ErrInvalidRequest = errors.New("invalid cost request")
)

const (
	costAdviceSubmitFailureMessage  = "成本建议生成启动失败，请稍后重试"
	maxCostNameRunes                = 255
	maxCostShortTextRunes           = 128
	maxCostNoteRunes                = 1000
	maxCostAmount                   = 999_999_999_999.99
	maxCostExternalTaskIDRunes      = 128
	maxCostTaskErrorMessageRunes    = 1000
	maxCostTaskPayloadJSONBytes     = 256 * 1024
	maxCostTaskResultJSONBytes      = 128 * 1024
	maxCostTaskRouteJSONBytes       = 16 * 1024
	maxCostReportMetadataJSONBytes  = 16 * 1024
	maxCostProjectMetadataJSONBytes = maxCostTaskResultJSONBytes + 1024
)

type Store struct {
	pool   *pgxpool.Pool
	cfg    config.Config
	client *http.Client
}

type Project struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"project_id"`
	ProjectName  string         `json:"project_name"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	BudgetAmount *float64       `json:"budget_amount"`
	TotalBudget  float64        `json:"total_budget"`
	TotalActual  float64        `json:"total_actual"`
	MarginAmount float64        `json:"margin_amount"`
	MarginRate   float64        `json:"margin_rate"`
	ItemCount    int            `json:"item_count"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Item struct {
	ID            string    `json:"id"`
	CostProjectID string    `json:"cost_project_id"`
	Category      string    `json:"category"`
	Name          string    `json:"name"`
	CostType      string    `json:"cost_type"`
	BudgetAmount  float64   `json:"budget_amount"`
	ActualAmount  float64   `json:"actual_amount"`
	Status        string    `json:"status"`
	Vendor        string    `json:"vendor"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Analysis struct {
	Project         Project         `json:"project"`
	CategoryTotals  []CategoryTotal `json:"category_totals"`
	OverrunItems    []Item          `json:"overrun_items"`
	Recommendations []string        `json:"recommendations"`
}

type CategoryTotal struct {
	Category     string  `json:"category"`
	TotalBudget  float64 `json:"total_budget"`
	TotalActual  float64 `json:"total_actual"`
	MarginAmount float64 `json:"margin_amount"`
}

type Report struct {
	ID            string         `json:"id"`
	CostProjectID string         `json:"cost_project_id"`
	ReportType    string         `json:"report_type"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type Task struct {
	ID             string         `json:"id"`
	TaskType       string         `json:"task_type"`
	Status         string         `json:"status"`
	ExternalTaskID *string        `json:"external_task_id"`
	ResourceType   string         `json:"resource_type"`
	ResourceID     string         `json:"resource_id"`
	Payload        map[string]any `json:"payload"`
	Route          map[string]any `json:"route"`
	Result         map[string]any `json:"result"`
	ErrorMessage   *string        `json:"error_message"`
	StartedAt      *time.Time     `json:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateProjectRequest struct {
	ProjectID    string   `json:"project_id"`
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	BudgetAmount *float64 `json:"budget_amount"`
}

type UpdateProjectRequest struct {
	Name         *string  `json:"name"`
	Status       *string  `json:"status"`
	BudgetAmount *float64 `json:"budget_amount"`
}

type CreateItemRequest struct {
	Category     string  `json:"category"`
	Name         string  `json:"name"`
	CostType     string  `json:"cost_type"`
	BudgetAmount float64 `json:"budget_amount"`
	ActualAmount float64 `json:"actual_amount"`
	Status       string  `json:"status"`
	Vendor       string  `json:"vendor"`
	Note         string  `json:"note"`
}

type UpdateItemRequest struct {
	Category     *string  `json:"category"`
	Name         *string  `json:"name"`
	CostType     *string  `json:"cost_type"`
	BudgetAmount *float64 `json:"budget_amount"`
	ActualAmount *float64 `json:"actual_amount"`
	Status       *string  `json:"status"`
	Vendor       *string  `json:"vendor"`
	Note         *string  `json:"note"`
}

type CallbackPayload struct {
	TenantID     string         `json:"tenant_id"`
	TaskID       string         `json:"task_id"`
	Status       string         `json:"status"`
	Result       map[string]any `json:"result"`
	ErrorMessage string         `json:"error_message"`
}

type aiTaskAccepted struct {
	TaskID string         `json:"task_id"`
	Status string         `json:"status"`
	Route  map[string]any `json:"route"`
}

func NewStore(cfg config.Config, pool *pgxpool.Pool) *Store {
	return &Store{
		pool: pool,
		cfg:  cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *Store) ListProjects(ctx context.Context, tenantID string) ([]Project, error) {
	projects := []Project{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, projectSelectSQL()+`
			where cp.tenant_id = $1
			order by cp.updated_at desc, cp.created_at desc
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			project, err := scanProject(rows)
			if err != nil {
				return err
			}
			projects = append(projects, project)
		}
		return rows.Err()
	})
	return projects, err
}

func (s *Store) CreateProject(ctx context.Context, tenantID string, req CreateProjectRequest) (Project, error) {
	if err := validateUUID(req.ProjectID); err != nil {
		return Project{}, err
	}
	if err := validateOptionalAmount(req.BudgetAmount); err != nil {
		return Project{}, err
	}
	status, err := normalizeProjectStatus(req.Status)
	if err != nil {
		return Project{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name != "" {
		if err := validateCostTextLength(name, maxCostNameRunes); err != nil {
			return Project{}, err
		}
	}
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var projectName string
		if err := tx.QueryRow(ctx, `select name from projects where tenant_id = $1 and id = $2`, tenantID, req.ProjectID).Scan(&projectName); err != nil {
			return err
		}
		if name == "" {
			name = boundedCostText(projectName+"成本项目", maxCostNameRunes)
			if name == "" {
				name = "成本项目"
			}
		}
		return tx.QueryRow(ctx, `
			insert into cost_projects (tenant_id, project_id, name, status, budget_amount, metadata)
			values ($1, $2, $3, $4, $5, '{"source":"cost-api"}')
			on conflict (tenant_id, project_id)
			do update set name = excluded.name, status = excluded.status, budget_amount = excluded.budget_amount, updated_at = now()
			returning id::text
		`, tenantID, req.ProjectID, name, status, req.BudgetAmount).Scan(&id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	return s.GetProject(ctx, tenantID, id)
}

func (s *Store) GetProject(ctx context.Context, tenantID, id string) (Project, error) {
	if err := validateUUID(id); err != nil {
		return Project{}, err
	}
	var project Project
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanProject(tx.QueryRow(ctx, projectSelectSQL()+`
			where cp.tenant_id = $1 and cp.id = $2
		`, tenantID, id))
		if err != nil {
			return err
		}
		project = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return project, err
}

func (s *Store) UpdateProject(ctx context.Context, tenantID, id string, req UpdateProjectRequest) (Project, error) {
	if err := validateUUID(id); err != nil {
		return Project{}, err
	}
	if err := validateOptionalAmount(req.BudgetAmount); err != nil {
		return Project{}, err
	}
	var requestedName *string
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return Project{}, ErrInvalidRequest
		}
		if err := validateCostTextLength(name, maxCostNameRunes); err != nil {
			return Project{}, err
		}
		requestedName = &name
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		current, err := scanProject(tx.QueryRow(ctx, projectSelectSQL()+` where cp.tenant_id = $1 and cp.id = $2`, tenantID, id))
		if err != nil {
			return err
		}
		name := current.Name
		if requestedName != nil {
			name = *requestedName
		}
		status := current.Status
		if req.Status != nil {
			normalized, err := normalizeProjectStatus(*req.Status)
			if err != nil {
				return err
			}
			status = normalized
		}
		budgetAmount := current.BudgetAmount
		if req.BudgetAmount != nil {
			budgetAmount = req.BudgetAmount
		}
		tag, err := tx.Exec(ctx, `
			update cost_projects
			set name = $3, status = $4, budget_amount = $5, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, name, status, budgetAmount)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	return s.GetProject(ctx, tenantID, id)
}

func (s *Store) ListItems(ctx context.Context, tenantID, projectID string) ([]Item, error) {
	if err := validateUUID(projectID); err != nil {
		return nil, err
	}
	items := []Item{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, cost_project_id::text, category, name, cost_type, budget_amount::float8, actual_amount::float8,
				status, vendor, note, created_at, updated_at
			from cost_items
			where tenant_id = $1 and cost_project_id = $2
			order by category, created_at
		`, tenantID, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanItem(rows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (s *Store) CreateItem(ctx context.Context, tenantID, projectID string, req CreateItemRequest) (Item, error) {
	if err := validateUUID(projectID); err != nil {
		return Item{}, err
	}
	normalized, err := normalizeItemRequest(req)
	if err != nil {
		return Item{}, err
	}
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := ensureCostProject(ctx, tx, tenantID, projectID); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			insert into cost_items (tenant_id, cost_project_id, category, name, cost_type, budget_amount, actual_amount, status, vendor, note)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			returning id::text
		`, tenantID, projectID, normalized.Category, normalized.Name, normalized.CostType, normalized.BudgetAmount,
			normalized.ActualAmount, normalized.Status, normalized.Vendor, normalized.Note).Scan(&id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return s.GetItem(ctx, tenantID, id)
}

func (s *Store) UpdateItem(ctx context.Context, tenantID, itemID string, req UpdateItemRequest) (Item, error) {
	if err := validateUUID(itemID); err != nil {
		return Item{}, err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		current, err := scanItem(tx.QueryRow(ctx, `
			select id::text, cost_project_id::text, category, name, cost_type, budget_amount::float8, actual_amount::float8,
				status, vendor, note, created_at, updated_at
			from cost_items
			where tenant_id = $1 and id = $2
			for update
		`, tenantID, itemID))
		if err != nil {
			return err
		}
		normalized, err := mergeItemUpdateRequest(current, req)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			update cost_items
			set category = $3, name = $4, cost_type = $5, budget_amount = $6, actual_amount = $7,
				status = $8, vendor = $9, note = $10, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, itemID, normalized.Category, normalized.Name, normalized.CostType, normalized.BudgetAmount,
			normalized.ActualAmount, normalized.Status, normalized.Vendor, normalized.Note)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return s.GetItem(ctx, tenantID, itemID)
}

func (s *Store) DeleteItem(ctx context.Context, tenantID, itemID string) error {
	if err := validateUUID(itemID); err != nil {
		return err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `delete from cost_items where tenant_id = $1 and id = $2`, tenantID, itemID)
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

func (s *Store) GetItem(ctx context.Context, tenantID, itemID string) (Item, error) {
	var item Item
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanItem(tx.QueryRow(ctx, `
			select id::text, cost_project_id::text, category, name, cost_type, budget_amount::float8, actual_amount::float8,
				status, vendor, note, created_at, updated_at
			from cost_items
			where tenant_id = $1 and id = $2
		`, tenantID, itemID))
		if err != nil {
			return err
		}
		item = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
}

func (s *Store) Analysis(ctx context.Context, tenantID, projectID string) (Analysis, error) {
	project, err := s.GetProject(ctx, tenantID, projectID)
	if err != nil {
		return Analysis{}, err
	}
	items, err := s.ListItems(ctx, tenantID, projectID)
	if err != nil {
		return Analysis{}, err
	}
	totalsByCategory := map[string]*CategoryTotal{}
	overruns := []Item{}
	for _, item := range items {
		total, ok := totalsByCategory[item.Category]
		if !ok {
			total = &CategoryTotal{Category: item.Category}
			totalsByCategory[item.Category] = total
		}
		total.TotalBudget += item.BudgetAmount
		total.TotalActual += item.ActualAmount
		total.MarginAmount = total.TotalBudget - total.TotalActual
		if item.ActualAmount > item.BudgetAmount {
			overruns = append(overruns, item)
		}
	}
	categoryTotals := make([]CategoryTotal, 0, len(totalsByCategory))
	for _, total := range totalsByCategory {
		categoryTotals = append(categoryTotals, *total)
	}
	return Analysis{
		Project:         project,
		CategoryTotals:  categoryTotals,
		OverrunItems:    overruns,
		Recommendations: recommendations(project, overruns),
	}, nil
}

func (s *Store) Advice(ctx context.Context, tenantID, userID, projectID string) (Task, error) {
	analysis, err := s.Analysis(ctx, tenantID, projectID)
	if err != nil {
		return Task{}, err
	}
	externalTaskID := "task-cost-advice-" + uuid.NewString()
	payload := map[string]any{
		"task_id":           externalTaskID,
		"tenant_id":         tenantID,
		"cost_project_id":   analysis.Project.ID,
		"project_name":      analysis.Project.ProjectName,
		"cost_project_name": analysis.Project.Name,
		"budget_amount":     analysis.Project.BudgetAmount,
		"total_budget":      analysis.Project.TotalBudget,
		"total_actual":      analysis.Project.TotalActual,
		"margin_rate":       analysis.Project.MarginRate,
		"category_totals":   analysis.CategoryTotals,
		"overrun_items":     analysis.OverrunItems,
		"recommendations":   analysis.Recommendations,
		"callback_url":      s.cfg.AICallbackURL,
	}
	payloadJSON, err := marshalCostTaskJSON(payload, maxCostTaskPayloadJSONBytes)
	if err != nil {
		return Task{}, err
	}
	var task Task
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status, external_task_id,
				resource_type, resource_id, payload, route
			)
			values ($1, $2, 'cost_advice', 'queued', $3, 'cost_project', $4, $5, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, nullableUserID(userID), externalTaskID, projectID, payloadJSON))
		if err != nil {
			return err
		}
		task = created
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	accepted, err := s.submitCostAdvice(ctx, payload)
	if err != nil {
		_ = s.markTaskFailed(ctx, tenantID, task.ID, costAdviceSubmitFailureMessage)
		return Task{}, err
	}
	if err := s.applyAcceptedTask(ctx, tenantID, task.ID, accepted); err != nil {
		return Task{}, err
	}
	return s.GetTask(ctx, tenantID, task.ID)
}

func (s *Store) GetTask(ctx context.Context, tenantID, taskID string) (Task, error) {
	if err := validateUUID(taskID); err != nil {
		return Task{}, err
	}
	var task Task
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanTask(tx.QueryRow(ctx, `
			select id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
			from ai_tasks
			where tenant_id = $1 and id = $2
		`, tenantID, taskID))
		if err != nil {
			return err
		}
		task = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) ApplyAdviceCallback(ctx context.Context, payload CallbackPayload) (Task, error) {
	payload, resultJSON, err := normalizeCostAdviceCallbackPayload(payload)
	if err != nil {
		return Task{}, err
	}
	var task Task
	err = s.withTenant(ctx, payload.TenantID, func(tx pgx.Tx) error {
		current, err := lockAdviceTaskByExternalID(ctx, tx, payload.TenantID, payload.TaskID)
		if err != nil {
			return err
		}
		task = current
		if !taskstatus.ShouldApplyCallback(current.Status, payload.Status) {
			return nil
		}
		updated, err := scanTask(tx.QueryRow(ctx, `
			update ai_tasks
			set status = $3,
				result = $4,
				error_message = nullif($5, ''),
				started_at = coalesce(started_at, now()),
				completed_at = case when $3 in ('done', 'failed', 'cancelled') then now() else completed_at end,
				updated_at = now()
			where tenant_id = $1 and id = $2 and resource_type = 'cost_project'
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, payload.TenantID, current.ID, payload.Status, resultJSON, payload.ErrorMessage))
		if err != nil {
			return err
		}
		task = updated
		if payload.Status == "done" {
			metadataJSON, err := marshalCostTaskJSON(map[string]any{
				"last_ai_advice_task_id": task.ID,
				"last_ai_advice":         payload.Result,
			}, maxCostTaskResultJSONBytes+1024)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				update cost_projects
				set metadata = metadata || $3::jsonb,
					updated_at = now()
				where tenant_id = $1 and id = $2
			`, payload.TenantID, task.ResourceID, metadataJSON); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) CreateReport(ctx context.Context, tenantID, projectID string) (Report, error) {
	analysis, err := s.Analysis(ctx, tenantID, projectID)
	if err != nil {
		return Report{}, err
	}
	metadata, err := marshalCostTaskJSON(map[string]any{
		"total_budget": analysis.Project.TotalBudget,
		"total_actual": analysis.Project.TotalActual,
		"margin_rate":  analysis.Project.MarginRate,
		"overruns":     len(analysis.OverrunItems),
	}, maxCostReportMetadataJSONBytes)
	if err != nil {
		return Report{}, err
	}
	summary := "总预算 " + formatAmount(analysis.Project.TotalBudget) + "，实际成本 " + formatAmount(analysis.Project.TotalActual)
	var report Report
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var metadataRaw []byte
		if err := tx.QueryRow(ctx, `
			insert into cost_reports (tenant_id, cost_project_id, report_type, status, summary, metadata)
			values ($1, $2, 'summary', 'generated', $3, $4)
			returning id::text, cost_project_id::text, report_type, status, summary, metadata, created_at, updated_at
		`, tenantID, projectID, summary, metadata).Scan(
			&report.ID, &report.CostProjectID, &report.ReportType, &report.Status, &report.Summary, &metadataRaw, &report.CreatedAt, &report.UpdatedAt,
		); err != nil {
			return err
		}
		decoded, err := unmarshalCostReportMetadata(metadataRaw)
		if err != nil {
			return err
		}
		report.Metadata = decoded
		return nil
	})
	return report, err
}

func (s *Store) submitCostAdvice(ctx context.Context, payload map[string]any) (aiTaskAccepted, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/cost-advice"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return aiTaskAccepted{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	aihttp.Sign(req, body, s.cfg.AIServiceHMACSecret)
	resp, err := s.client.Do(req)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aiTaskAccepted{}, fmt.Errorf("ai service returned %s", resp.Status)
	}
	var accepted aiTaskAccepted
	if err := aihttp.DecodeJSON(resp.Body, &accepted); err != nil {
		return aiTaskAccepted{}, err
	}
	normalized, _, err := normalizeAcceptedTask(accepted)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	return normalized, nil
}

func (s *Store) applyAcceptedTask(ctx context.Context, tenantID, taskID string, accepted aiTaskAccepted) error {
	accepted, routeJSON, err := normalizeAcceptedTask(accepted)
	if err != nil {
		return err
	}
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update ai_tasks
			set status = case when status in ('done', 'failed', 'cancelled') then status else $3 end,
				external_task_id = $4,
				route = $5,
				started_at = coalesce(started_at, now()),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, accepted.Status, accepted.TaskID, routeJSON)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
}

func (s *Store) markTaskFailed(ctx context.Context, tenantID, taskID, message string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			update ai_tasks
			set status = 'failed',
				error_message = $3,
				completed_at = now(),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, message)
		return err
	})
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

func lockAdviceTaskByExternalID(ctx context.Context, tx pgx.Tx, tenantID, externalTaskID string) (Task, error) {
	return scanTask(tx.QueryRow(ctx, `
		select id::text, task_type, status, external_task_id::text,
			resource_type, resource_id::text, payload, route, result, error_message,
			started_at, completed_at, created_at, updated_at
		from ai_tasks
		where tenant_id = $1 and external_task_id = $2 and resource_type = 'cost_project'
		for update
	`, tenantID, externalTaskID))
}

func projectSelectSQL() string {
	return `
		select cp.id::text, cp.project_id::text, p.name, cp.name, cp.status, cp.budget_amount::float8,
			coalesce(items.total_budget, 0)::float8, coalesce(items.total_actual, 0)::float8,
			(coalesce(items.total_budget, 0) - coalesce(items.total_actual, 0))::float8,
			case when coalesce(items.total_budget, 0) = 0 then 0
				else round(((coalesce(items.total_budget, 0) - coalesce(items.total_actual, 0)) / items.total_budget * 100)::numeric, 2)::float8
			end,
			coalesce(items.item_count, 0)::int,
			cp.metadata, cp.created_at, cp.updated_at
		from cost_projects cp
		join projects p on p.id = cp.project_id and p.tenant_id = cp.tenant_id
		left join lateral (
			select sum(ci.budget_amount) as total_budget, sum(ci.actual_amount) as total_actual, count(*) as item_count
			from cost_items ci
			where ci.tenant_id = cp.tenant_id and ci.cost_project_id = cp.id
		) items on true
	`
}

func scanProject(row scanner) (Project, error) {
	var project Project
	var budget sql.NullFloat64
	var metadataRaw []byte
	err := row.Scan(
		&project.ID, &project.ProjectID, &project.ProjectName, &project.Name, &project.Status, &budget,
		&project.TotalBudget, &project.TotalActual, &project.MarginAmount, &project.MarginRate,
		&project.ItemCount, &metadataRaw, &project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		return Project{}, err
	}
	if budget.Valid {
		project.BudgetAmount = &budget.Float64
	}
	project.Metadata, err = unmarshalCostProjectMetadata(metadataRaw)
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

func scanItem(row scanner) (Item, error) {
	var item Item
	err := row.Scan(
		&item.ID, &item.CostProjectID, &item.Category, &item.Name, &item.CostType, &item.BudgetAmount,
		&item.ActualAmount, &item.Status, &item.Vendor, &item.Note, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func scanTask(row scanner) (Task, error) {
	var task Task
	var externalTaskID, errorMessage sql.NullString
	var payloadRaw, routeRaw, resultRaw []byte
	var startedAt, completedAt sql.NullTime
	err := row.Scan(
		&task.ID, &task.TaskType, &task.Status, &externalTaskID,
		&task.ResourceType, &task.ResourceID, &payloadRaw, &routeRaw, &resultRaw, &errorMessage,
		&startedAt, &completedAt, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		return Task{}, err
	}
	if externalTaskID.Valid {
		task.ExternalTaskID = &externalTaskID.String
	}
	if errorMessage.Valid {
		task.ErrorMessage = &errorMessage.String
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	task.Payload = map[string]any{}
	task.Route = map[string]any{}
	task.Result = map[string]any{}
	if task.Payload, err = unmarshalCostTaskJSON(payloadRaw, maxCostTaskPayloadJSONBytes); err != nil {
		return Task{}, err
	}
	if task.Route, err = unmarshalCostTaskJSON(routeRaw, maxCostTaskRouteJSONBytes); err != nil {
		return Task{}, err
	}
	if task.Result, err = unmarshalCostTaskJSON(resultRaw, maxCostTaskResultJSONBytes); err != nil {
		return Task{}, err
	}
	return task, nil
}

func validateUUID(value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func nullableUserID(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func ensureCostProject(ctx context.Context, tx pgx.Tx, tenantID, projectID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from cost_projects where tenant_id = $1 and id = $2)`, tenantID, projectID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return nil
}

func normalizeProjectStatus(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "draft", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "closed":
		return strings.ToLower(strings.TrimSpace(value)), nil
	case "draft":
		return "draft", nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeItemRequest(req CreateItemRequest) (CreateItemRequest, error) {
	req.Category = strings.TrimSpace(req.Category)
	req.Name = strings.TrimSpace(req.Name)
	req.Vendor = strings.TrimSpace(req.Vendor)
	req.Note = strings.TrimSpace(req.Note)
	costType, err := normalizeCostType(req.CostType)
	if err != nil {
		return req, err
	}
	status, err := normalizeItemStatus(req.Status)
	if err != nil {
		return req, err
	}
	req.CostType = costType
	req.Status = status
	if err := validateAmount(req.BudgetAmount); err != nil {
		return req, err
	}
	if err := validateAmount(req.ActualAmount); err != nil {
		return req, err
	}
	if req.Category == "" {
		req.Category = "其他"
	}
	if req.Name == "" {
		return req, ErrInvalidRequest
	}
	for _, check := range []struct {
		value string
		limit int
	}{
		{value: req.Category, limit: maxCostShortTextRunes},
		{value: req.Name, limit: maxCostNameRunes},
		{value: req.Vendor, limit: maxCostNameRunes},
		{value: req.Note, limit: maxCostNoteRunes},
	} {
		if err := validateCostTextLength(check.value, check.limit); err != nil {
			return req, err
		}
	}
	return req, nil
}

func mergeItemUpdateRequest(current Item, req UpdateItemRequest) (CreateItemRequest, error) {
	merged := CreateItemRequest{
		Category:     current.Category,
		Name:         current.Name,
		CostType:     current.CostType,
		BudgetAmount: current.BudgetAmount,
		ActualAmount: current.ActualAmount,
		Status:       current.Status,
		Vendor:       current.Vendor,
		Note:         current.Note,
	}
	if req.Category != nil {
		merged.Category = *req.Category
	}
	if req.Name != nil {
		merged.Name = *req.Name
	}
	if req.CostType != nil {
		merged.CostType = *req.CostType
	}
	if req.BudgetAmount != nil {
		merged.BudgetAmount = *req.BudgetAmount
	}
	if req.ActualAmount != nil {
		merged.ActualAmount = *req.ActualAmount
	}
	if req.Status != nil {
		merged.Status = *req.Status
	}
	if req.Vendor != nil {
		merged.Vendor = *req.Vendor
	}
	if req.Note != nil {
		merged.Note = *req.Note
	}
	return normalizeItemRequest(merged)
}

func normalizeCostType(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "other", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "labor", "material", "equipment", "service", "other":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeItemStatus(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "planned", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "planned", "committed", "actual":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func validateOptionalAmount(value *float64) error {
	if value == nil {
		return nil
	}
	return validateAmount(*value)
}

func validateAmount(value float64) error {
	if value < 0 || value > maxCostAmount || math.IsNaN(value) || math.IsInf(value, 0) {
		return ErrInvalidRequest
	}
	return nil
}

func validateCostTextLength(value string, maxRunes int) error {
	if maxRunes <= 0 {
		return ErrInvalidRequest
	}
	if utf8.RuneCountInString(strings.TrimSpace(value)) > maxRunes {
		return ErrInvalidRequest
	}
	return nil
}

func marshalCostTaskJSON(value any, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, ErrInvalidRequest
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > maxBytes {
		return nil, ErrInvalidRequest
	}
	return raw, nil
}

func unmarshalCostTaskJSON(raw []byte, maxBytes int) (map[string]any, error) {
	result := map[string]any{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return result, nil
	}
	if maxBytes <= 0 || len(trimmed) > maxBytes {
		return nil, ErrInvalidRequest
	}
	if err := json.Unmarshal(trimmed, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func unmarshalCostReportMetadata(raw []byte) (map[string]any, error) {
	return unmarshalCostTaskJSON(raw, maxCostReportMetadataJSONBytes)
}

func unmarshalCostProjectMetadata(raw []byte) (map[string]any, error) {
	return unmarshalCostTaskJSON(raw, maxCostProjectMetadataJSONBytes)
}

func normalizeCostAdviceCallbackPayload(payload CallbackPayload) (CallbackPayload, []byte, error) {
	payload.TenantID = strings.TrimSpace(payload.TenantID)
	payload.TaskID = strings.TrimSpace(payload.TaskID)
	payload.Status = normalizeTaskStatus(payload.Status)
	payload.ErrorMessage = strings.TrimSpace(payload.ErrorMessage)
	if payload.Result == nil {
		payload.Result = map[string]any{}
	}
	if payload.TenantID == "" || payload.TaskID == "" || payload.Status == "" {
		return payload, nil, ErrInvalidRequest
	}
	if err := validateCostTextLength(payload.TaskID, maxCostExternalTaskIDRunes); err != nil {
		return payload, nil, err
	}
	if err := validateCostTextLength(payload.ErrorMessage, maxCostTaskErrorMessageRunes); err != nil {
		return payload, nil, err
	}
	resultJSON, err := marshalCostTaskJSON(payload.Result, maxCostTaskResultJSONBytes)
	if err != nil {
		return payload, nil, err
	}
	return payload, resultJSON, nil
}

func normalizeAcceptedTask(accepted aiTaskAccepted) (aiTaskAccepted, []byte, error) {
	accepted.TaskID = strings.TrimSpace(accepted.TaskID)
	accepted.Status = normalizeTaskStatus(accepted.Status)
	if accepted.Status == "" {
		accepted.Status = "queued"
	}
	if accepted.Route == nil {
		accepted.Route = map[string]any{}
	}
	if accepted.TaskID == "" {
		return accepted, nil, ErrInvalidRequest
	}
	if err := validateCostTextLength(accepted.TaskID, maxCostExternalTaskIDRunes); err != nil {
		return accepted, nil, err
	}
	routeJSON, err := marshalCostTaskJSON(accepted.Route, maxCostTaskRouteJSONBytes)
	if err != nil {
		return accepted, nil, err
	}
	return accepted, routeJSON, nil
}

func boundedCostText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}

func normalizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "done", "failed", "cancelled":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

func recommendations(project Project, overruns []Item) []string {
	result := []string{}
	if project.MarginRate < 20 {
		result = append(result, "利润率低于 20%，建议复核人力投入和外采成本。")
	}
	if len(overruns) > 0 {
		result = append(result, "存在成本超预算条目，优先处理金额偏差最大的分类。")
	}
	if len(result) == 0 {
		result = append(result, "当前成本结构健康，建议继续跟踪 committed 条目的实际结算。")
	}
	return result
}

func formatAmount(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}
