package dashboard

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type Stats struct {
	ActiveProjects     int `json:"active_projects"`
	MonthlyBids        int `json:"monthly_bids"`
	CompliancePassRate int `json:"compliance_pass_rate"`
	WinRate            int `json:"win_rate"`
	PendingTasks       int `json:"pending_tasks"`
	KnowledgeDocs      int `json:"knowledge_docs"`
}

type TrendPoint struct {
	Month   string `json:"month"`
	Bids    int    `json:"bids"`
	WinRate int    `json:"win_rate"`
}

type RecommendedTender struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Region     string  `json:"region"`
	Purchaser  string  `json:"purchaser"`
	MatchScore int     `json:"match_score"`
	Deadline   *string `json:"deadline"`
}

type RecentProject struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	OwnerName string    `json:"owner_name"`
	DueDate   *string   `json:"due_date"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PendingApproval struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	BidTitle    string    `json:"bid_title"`
	CurrentStep int       `json:"current_step"`
	CreatedAt   time.Time `json:"created_at"`
}

type Notification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	ReadAt    *string   `json:"read_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Summary struct {
	Stats              Stats               `json:"stats"`
	Trends             []TrendPoint        `json:"trends"`
	RecommendedTenders []RecommendedTender `json:"recommended_tenders"`
	RecentProjects     []RecentProject     `json:"recent_projects"`
	PendingApprovals   []PendingApproval   `json:"pending_approvals"`
	Notifications      []Notification      `json:"notifications"`
	GeneratedAt        time.Time           `json:"generated_at"`
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Summary(ctx context.Context, tenantID, userID string) (Summary, error) {
	summary := Summary{GeneratedAt: time.Now()}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		stats, err := loadStats(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		summary.Stats = stats
		if summary.Trends, err = loadTrends(ctx, tx, tenantID); err != nil {
			return err
		}
		if summary.RecommendedTenders, err = loadRecommendedTenders(ctx, tx, tenantID); err != nil {
			return err
		}
		if summary.RecentProjects, err = loadRecentProjects(ctx, tx, tenantID); err != nil {
			return err
		}
		if summary.PendingApprovals, err = loadPendingApprovals(ctx, tx, tenantID); err != nil {
			return err
		}
		if summary.Notifications, err = loadNotifications(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		return nil
	})
	return summary, err
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

func loadStats(ctx context.Context, tx pgx.Tx, tenantID string) (Stats, error) {
	var stats Stats
	err := tx.QueryRow(ctx, `
		select
			(select count(*)::int from projects where tenant_id = $1 and status <> 'closed'),
			(select count(*)::int from bid_documents where tenant_id = $1 and created_at >= date_trunc('month', now())),
			(select coalesce(round(100.0 * count(*) filter (where result_status in ('pass', 'warn')) / nullif(count(*), 0)), 0)::int
				from compliance_checks where tenant_id = $1 and status = 'done'),
			(select coalesce(round(100.0 * count(*) filter (where result = 'won') / nullif(count(*) filter (where result in ('won', 'lost')), 0)), 0)::int
				from projects where tenant_id = $1 and status = 'closed'),
			(
				(select count(*)::int from approval_instances where tenant_id = $1 and status = 'pending')
				+ (select count(*)::int from compliance_issues where tenant_id = $1 and status = 'open' and severity in ('fail', 'fail_candidate'))
				+ (select count(*)::int from ai_tasks where tenant_id = $1 and status in ('queued', 'running'))
			),
			(select count(*)::int from knowledge_documents where tenant_id = $1)
	`, tenantID).Scan(
		&stats.ActiveProjects,
		&stats.MonthlyBids,
		&stats.CompliancePassRate,
		&stats.WinRate,
		&stats.PendingTasks,
		&stats.KnowledgeDocs,
	)
	return stats, err
}

func loadTrends(ctx context.Context, tx pgx.Tx, tenantID string) ([]TrendPoint, error) {
	rows, err := tx.Query(ctx, `
		with months as (
			select date_trunc('month', now()) - (gs * interval '1 month') as month_start
			from generate_series(5, 0, -1) as gs
		)
		select
			to_char(month_start, 'YYYY-MM'),
			(select count(*)::int from bid_documents bd where bd.tenant_id = $1 and date_trunc('month', bd.created_at) = month_start),
			(select coalesce(round(100.0 * count(*) filter (where p.result = 'won') / nullif(count(*) filter (where p.result in ('won', 'lost')), 0)), 0)::int
				from projects p
				where p.tenant_id = $1 and p.status = 'closed' and date_trunc('month', p.updated_at) = month_start)
		from months
		order by month_start
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []TrendPoint{}
	for rows.Next() {
		var point TrendPoint
		if err := rows.Scan(&point.Month, &point.Bids, &point.WinRate); err != nil {
			return nil, err
		}
		result = append(result, point)
	}
	return result, rows.Err()
}

func loadRecommendedTenders(ctx context.Context, tx pgx.Tx, tenantID string) ([]RecommendedTender, error) {
	rows, err := tx.Query(ctx, `
		select id::text, title, region, purchaser, match_score, deadline::text
		from tenders
		where tenant_id = $1 and status = 'open'
		order by match_score desc, deadline nulls last, created_at desc
		limit 5
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RecommendedTender{}
	for rows.Next() {
		var item RecommendedTender
		var deadline sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.Region, &item.Purchaser, &item.MatchScore, &deadline); err != nil {
			return nil, err
		}
		if deadline.Valid {
			item.Deadline = &deadline.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadRecentProjects(ctx context.Context, tx pgx.Tx, tenantID string) ([]RecentProject, error) {
	rows, err := tx.Query(ctx, `
		select p.id::text, p.name, p.status, coalesce(owner.name, ''), next_due.due_date::text, p.updated_at
		from projects p
		left join lateral (
			select u.name
			from project_members pm
			join users u on u.id = pm.user_id
			where pm.tenant_id = p.tenant_id and pm.project_id = p.id and pm.role = 'owner'
			order by pm.created_at
			limit 1
		) owner on true
		left join lateral (
			select min(due_date) as due_date
			from project_milestones milestone
			where milestone.tenant_id = p.tenant_id and milestone.project_id = p.id and milestone.status = 'pending'
		) next_due on true
		where p.tenant_id = $1
		order by p.updated_at desc
		limit 6
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []RecentProject{}
	for rows.Next() {
		var project RecentProject
		var due sql.NullString
		if err := rows.Scan(&project.ID, &project.Name, &project.Status, &project.OwnerName, &due, &project.UpdatedAt); err != nil {
			return nil, err
		}
		if due.Valid {
			project.DueDate = &due.String
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func loadPendingApprovals(ctx context.Context, tx pgx.Tx, tenantID string) ([]PendingApproval, error) {
	rows, err := tx.Query(ctx, `
		select ai.id::text, ai.title, coalesce(bd.title, ''), ai.current_step, ai.created_at
		from approval_instances ai
		left join bid_documents bd on bd.tenant_id = ai.tenant_id and bd.id = ai.bid_document_id
		where ai.tenant_id = $1 and ai.status = 'pending'
		order by ai.created_at desc
		limit 6
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []PendingApproval{}
	for rows.Next() {
		var item PendingApproval
		if err := rows.Scan(&item.ID, &item.Title, &item.BidTitle, &item.CurrentStep, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadNotifications(ctx context.Context, tx pgx.Tx, tenantID, userID string) ([]Notification, error) {
	rows, err := tx.Query(ctx, `
		select id::text, title, body, read_at::text, created_at
		from notifications
		where tenant_id = $1 and (user_id is null or user_id = $2)
		order by created_at desc
		limit 6
	`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Notification{}
	for rows.Next() {
		var item Notification
		if err := rows.Scan(&item.ID, &item.Title, &item.Body, &item.ReadAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
