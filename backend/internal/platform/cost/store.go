package cost

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("cost resource not found")
	ErrInvalidRequest = errors.New("invalid cost request")
)

type Store struct {
	pool *pgxpool.Pool
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

type UpdateItemRequest = CreateItemRequest

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
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
	status := normalizeProjectStatus(req.Status)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "成本项目"
	}
	var id string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var projectName string
		if err := tx.QueryRow(ctx, `select name from projects where tenant_id = $1 and id = $2`, tenantID, req.ProjectID).Scan(&projectName); err != nil {
			return err
		}
		if strings.TrimSpace(req.Name) == "" {
			name = projectName + "成本项目"
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
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		current, err := scanProject(tx.QueryRow(ctx, projectSelectSQL()+` where cp.tenant_id = $1 and cp.id = $2`, tenantID, id))
		if err != nil {
			return err
		}
		name := current.Name
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
			if name == "" {
				return ErrInvalidRequest
			}
		}
		status := current.Status
		if req.Status != nil {
			status = normalizeProjectStatus(*req.Status)
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
	normalized, err := normalizeItemRequest(req)
	if err != nil {
		return Item{}, err
	}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
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

func (s *Store) Advice(ctx context.Context, tenantID, projectID string) (Analysis, error) {
	return s.Analysis(ctx, tenantID, projectID)
}

func (s *Store) CreateReport(ctx context.Context, tenantID, projectID string) (Report, error) {
	analysis, err := s.Analysis(ctx, tenantID, projectID)
	if err != nil {
		return Report{}, err
	}
	metadata, _ := json.Marshal(map[string]any{
		"total_budget": analysis.Project.TotalBudget,
		"total_actual": analysis.Project.TotalActual,
		"margin_rate":  analysis.Project.MarginRate,
		"overruns":     len(analysis.OverrunItems),
	})
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
		report.Metadata = map[string]any{}
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &report.Metadata)
		}
		return nil
	})
	return report, err
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
	if budget.Valid {
		project.BudgetAmount = &budget.Float64
	}
	project.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &project.Metadata)
	}
	return project, err
}

func scanItem(row scanner) (Item, error) {
	var item Item
	err := row.Scan(
		&item.ID, &item.CostProjectID, &item.Category, &item.Name, &item.CostType, &item.BudgetAmount,
		&item.ActualAmount, &item.Status, &item.Vendor, &item.Note, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func validateUUID(value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return ErrInvalidRequest
	}
	return nil
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

func normalizeProjectStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "closed":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "draft"
	}
}

func normalizeItemRequest(req CreateItemRequest) (CreateItemRequest, error) {
	req.Category = strings.TrimSpace(req.Category)
	req.Name = strings.TrimSpace(req.Name)
	req.CostType = normalizeCostType(req.CostType)
	req.Status = normalizeItemStatus(req.Status)
	req.Vendor = strings.TrimSpace(req.Vendor)
	req.Note = strings.TrimSpace(req.Note)
	req.BudgetAmount = math.Max(req.BudgetAmount, 0)
	req.ActualAmount = math.Max(req.ActualAmount, 0)
	if req.Category == "" {
		req.Category = "其他"
	}
	if req.Name == "" {
		return req, ErrInvalidRequest
	}
	return req, nil
}

func normalizeCostType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "labor", "material", "equipment", "service":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "other"
	}
}

func normalizeItemStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "committed", "actual":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "planned"
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
