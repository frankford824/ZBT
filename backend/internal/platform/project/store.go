package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("project resource not found")
	ErrInvalidRequest = errors.New("invalid project request")
)

type Store struct {
	pool *pgxpool.Pool
}

type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Result         *string   `json:"result"`
	OwnerID        *string   `json:"owner_id"`
	OwnerName      string    `json:"owner_name"`
	BidCount       int       `json:"bid_count"`
	MilestoneCount int       `json:"milestone_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Milestone struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date"`
	CompletedAt *time.Time `json:"completed_at"`
	SortOrder   int        `json:"sort_order"`
	Note        string     `json:"note"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Member struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	User      User      `json:"user"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Activity struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"project_id"`
	ActorUserID *string        `json:"actor_user_id"`
	ActorName   string         `json:"actor_name"`
	Action      string         `json:"action"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type CostProject struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"project_id"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	BudgetAmount *float64       `json:"budget_amount"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateProjectRequest struct {
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Result *string `json:"result"`
}

type UpdateProjectRequest struct {
	Name   *string `json:"name"`
	Status *string `json:"status"`
	Result *string `json:"result"`
}

type TransitionRequest struct {
	Status string `json:"status"`
	Result string `json:"result"`
}

type CreateMilestoneRequest struct {
	Title     string `json:"title"`
	Status    string `json:"status"`
	DueDate   string `json:"due_date"`
	SortOrder int    `json:"sort_order"`
	Note      string `json:"note"`
}

type UpdateMilestoneRequest = CreateMilestoneRequest

type AddMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) List(ctx context.Context, tenantID, status string) ([]Project, error) {
	projects := []Project{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		args := []any{tenantID}
		conditions := []string{"p.tenant_id = $1"}
		if normalized := normalizeStatus(status); normalized != "" {
			args = append(args, normalized)
			conditions = append(conditions, fmt.Sprintf("p.status = $%d", len(args)))
		}
		rows, err := tx.Query(ctx, projectSelectSQL()+`
			where `+strings.Join(conditions, " and ")+`
			order by p.updated_at desc, p.created_at desc
			limit 200
		`, args...)
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

func (s *Store) Get(ctx context.Context, tenantID, id string) (Project, error) {
	if err := validateUUID(id); err != nil {
		return Project{}, err
	}
	var project Project
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanProject(tx.QueryRow(ctx, projectSelectSQL()+`
			where p.tenant_id = $1 and p.id = $2
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

func (s *Store) Create(ctx context.Context, tenantID, userID string, req CreateProjectRequest) (Project, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Project{}, ErrInvalidRequest
	}
	status := normalizeStatus(req.Status)
	if status == "" {
		status = "opportunity"
	}
	result := normalizeResultPointer(req.Result, status)
	var id string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			insert into projects (tenant_id, name, status, result)
			values ($1, $2, $3, $4)
			returning id::text
		`, tenantID, name, status, result).Scan(&id); err != nil {
			return err
		}
		if err := insertProjectMember(ctx, tx, tenantID, id, userID, "owner"); err != nil {
			return err
		}
		return insertLog(ctx, tx, tenantID, id, userID, "project.created", map[string]any{"status": status})
	})
	if err != nil {
		return Project{}, err
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Store) Update(ctx context.Context, tenantID, userID, id string, req UpdateProjectRequest) (Project, error) {
	if err := validateUUID(id); err != nil {
		return Project{}, err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		current, err := scanProject(tx.QueryRow(ctx, projectSelectSQL()+` where p.tenant_id = $1 and p.id = $2`, tenantID, id))
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
			status = normalizeStatus(*req.Status)
			if status == "" {
				return ErrInvalidRequest
			}
		}
		result := normalizeResultPointer(req.Result, status)
		if req.Result == nil {
			result = current.Result
			if status != "closed" {
				result = nil
			}
		}
		tag, err := tx.Exec(ctx, `
			update projects
			set name = $3, status = $4, result = $5, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, name, status, result)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return insertLog(ctx, tx, tenantID, id, userID, "project.updated", map[string]any{"status": status})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Store) Delete(ctx context.Context, tenantID, id string) error {
	if err := validateUUID(id); err != nil {
		return err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `delete from projects where tenant_id = $1 and id = $2`, tenantID, id)
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

func (s *Store) Transition(ctx context.Context, tenantID, userID, id string, req TransitionRequest) (Project, error) {
	if err := validateUUID(id); err != nil {
		return Project{}, err
	}
	target := normalizeStatus(req.Status)
	if target == "" {
		return Project{}, ErrInvalidRequest
	}
	result := normalizeResultValue(req.Result, target)
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var current string
		if err := tx.QueryRow(ctx, `select status from projects where tenant_id = $1 and id = $2`, tenantID, id).Scan(&current); err != nil {
			return err
		}
		if !allowedTransition(current, target) {
			return ErrInvalidRequest
		}
		if _, err := tx.Exec(ctx, `
			update projects
			set status = $3, result = $4, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, target, result); err != nil {
			return err
		}
		return insertLog(ctx, tx, tenantID, id, userID, "project.transitioned", map[string]any{"from": current, "to": target, "result": result})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Store) ListMilestones(ctx context.Context, tenantID, projectID string) ([]Milestone, error) {
	if err := validateUUID(projectID); err != nil {
		return nil, err
	}
	milestones := []Milestone{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, project_id::text, title, status, due_date, completed_at, sort_order, note, created_at, updated_at
			from project_milestones
			where tenant_id = $1 and project_id = $2
			order by sort_order, due_date nulls last, created_at
		`, tenantID, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			milestone, err := scanMilestone(rows)
			if err != nil {
				return err
			}
			milestones = append(milestones, milestone)
		}
		return rows.Err()
	})
	return milestones, err
}

func (s *Store) CreateMilestone(ctx context.Context, tenantID, userID, projectID string, req CreateMilestoneRequest) (Milestone, error) {
	if err := validateUUID(projectID); err != nil {
		return Milestone{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return Milestone{}, ErrInvalidRequest
	}
	status := normalizeMilestoneStatus(req.Status)
	dueDate, err := parseOptionalDate(req.DueDate)
	if err != nil {
		return Milestone{}, ErrInvalidRequest
	}
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := ensureProject(ctx, tx, tenantID, projectID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			insert into project_milestones (tenant_id, project_id, title, status, due_date, completed_at, sort_order, note)
			values ($1, $2, $3, $4, $5, case when $4 = 'done' then now() else null end, $6, $7)
			returning id::text
		`, tenantID, projectID, title, status, dueDate, req.SortOrder, strings.TrimSpace(req.Note)).Scan(&id); err != nil {
			return err
		}
		return insertLog(ctx, tx, tenantID, projectID, userID, "project.milestone_created", map[string]any{"title": title})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Milestone{}, ErrNotFound
	}
	if err != nil {
		return Milestone{}, err
	}
	return s.getMilestone(ctx, tenantID, projectID, id)
}

func (s *Store) UpdateMilestone(ctx context.Context, tenantID, userID, projectID, milestoneID string, req UpdateMilestoneRequest) (Milestone, error) {
	if err := validateUUID(projectID); err != nil {
		return Milestone{}, err
	}
	if err := validateUUID(milestoneID); err != nil {
		return Milestone{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return Milestone{}, ErrInvalidRequest
	}
	status := normalizeMilestoneStatus(req.Status)
	dueDate, err := parseOptionalDate(req.DueDate)
	if err != nil {
		return Milestone{}, ErrInvalidRequest
	}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update project_milestones
			set title = $4, status = $5, due_date = $6,
				completed_at = case when $5 = 'done' and completed_at is null then now() when $5 = 'pending' then null else completed_at end,
				sort_order = $7, note = $8, updated_at = now()
			where tenant_id = $1 and project_id = $2 and id = $3
		`, tenantID, projectID, milestoneID, title, status, dueDate, req.SortOrder, strings.TrimSpace(req.Note))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return insertLog(ctx, tx, tenantID, projectID, userID, "project.milestone_updated", map[string]any{"title": title, "status": status})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Milestone{}, ErrNotFound
	}
	if err != nil {
		return Milestone{}, err
	}
	return s.getMilestone(ctx, tenantID, projectID, milestoneID)
}

func (s *Store) DeleteMilestone(ctx context.Context, tenantID, userID, projectID, milestoneID string) error {
	if err := validateUUID(projectID); err != nil {
		return err
	}
	if err := validateUUID(milestoneID); err != nil {
		return err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `delete from project_milestones where tenant_id = $1 and project_id = $2 and id = $3`, tenantID, projectID, milestoneID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return insertLog(ctx, tx, tenantID, projectID, userID, "project.milestone_deleted", map[string]any{"milestone_id": milestoneID})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) AddMember(ctx context.Context, tenantID, userID, projectID string, req AddMemberRequest) (Member, error) {
	if err := validateUUID(projectID); err != nil {
		return Member{}, err
	}
	if err := validateUUID(req.UserID); err != nil {
		return Member{}, err
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "member"
	}
	var id string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := ensureProject(ctx, tx, tenantID, projectID); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			select exists(select 1 from tenant_members where tenant_id = $1 and user_id = $2 and status = 'active')
		`, tenantID, req.UserID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}
		if err := tx.QueryRow(ctx, `
			insert into project_members (tenant_id, project_id, user_id, role)
			values ($1, $2, $3, $4)
			on conflict (tenant_id, project_id, user_id)
			do update set role = excluded.role, updated_at = now()
			returning id::text
		`, tenantID, projectID, req.UserID, role).Scan(&id); err != nil {
			return err
		}
		return insertLog(ctx, tx, tenantID, projectID, userID, "project.member_added", map[string]any{"user_id": req.UserID, "role": role})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	if err != nil {
		return Member{}, err
	}
	return s.getMember(ctx, tenantID, projectID, id)
}

func (s *Store) DeleteMember(ctx context.Context, tenantID, userID, projectID, memberID string) error {
	if err := validateUUID(projectID); err != nil {
		return err
	}
	if err := validateUUID(memberID); err != nil {
		return err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `delete from project_members where tenant_id = $1 and project_id = $2 and id = $3`, tenantID, projectID, memberID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return insertLog(ctx, tx, tenantID, projectID, userID, "project.member_removed", map[string]any{"member_id": memberID})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) ListMembers(ctx context.Context, tenantID, projectID string) ([]Member, error) {
	if err := validateUUID(projectID); err != nil {
		return nil, err
	}
	members := []Member{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select pm.id::text, pm.project_id::text, u.id::text, u.email, u.name, pm.role, pm.created_at, pm.updated_at
			from project_members pm
			join users u on u.id = pm.user_id
			where pm.tenant_id = $1 and pm.project_id = $2
			order by case when pm.role = 'owner' then 0 else 1 end, u.name
		`, tenantID, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			member, err := scanMember(rows)
			if err != nil {
				return err
			}
			members = append(members, member)
		}
		return rows.Err()
	})
	return members, err
}

func (s *Store) ListActivities(ctx context.Context, tenantID, projectID string) ([]Activity, error) {
	if err := validateUUID(projectID); err != nil {
		return nil, err
	}
	activities := []Activity{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select pl.id::text, pl.project_id::text, pl.actor_user_id::text, coalesce(u.name, ''), pl.action, pl.metadata, pl.created_at
			from project_logs pl
			left join users u on u.id = pl.actor_user_id
			where pl.tenant_id = $1 and pl.project_id = $2
			order by pl.created_at desc
			limit 100
		`, tenantID, projectID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			activity, err := scanActivity(rows)
			if err != nil {
				return err
			}
			activities = append(activities, activity)
		}
		return rows.Err()
	})
	return activities, err
}

func (s *Store) CreateCostProject(ctx context.Context, tenantID, userID, projectID string) (CostProject, error) {
	if err := validateUUID(projectID); err != nil {
		return CostProject{}, err
	}
	var cost CostProject
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var name, status string
		var result sql.NullString
		if err := tx.QueryRow(ctx, `select name, status, result from projects where tenant_id = $1 and id = $2`, tenantID, projectID).Scan(&name, &status, &result); err != nil {
			return err
		}
		if status != "closed" || !result.Valid || result.String != "won" {
			return ErrInvalidRequest
		}
		if err := tx.QueryRow(ctx, `
			insert into cost_projects (tenant_id, project_id, name, status, metadata)
			values ($1, $2, $3, 'draft', '{"source":"project"}')
			on conflict (tenant_id, project_id)
			do update set updated_at = now()
			returning id::text, project_id::text, name, status, budget_amount::float8, metadata, created_at, updated_at
		`, tenantID, projectID, name+"成本项目").Scan(&cost.ID, &cost.ProjectID, &cost.Name, &cost.Status, &cost.BudgetAmount, &cost.Metadata, &cost.CreatedAt, &cost.UpdatedAt); err != nil {
			return err
		}
		return insertLog(ctx, tx, tenantID, projectID, userID, "project.cost_project_created", map[string]any{"cost_project_id": cost.ID})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CostProject{}, ErrNotFound
	}
	return cost, err
}

func (s *Store) getMilestone(ctx context.Context, tenantID, projectID, milestoneID string) (Milestone, error) {
	var milestone Milestone
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanMilestone(tx.QueryRow(ctx, `
			select id::text, project_id::text, title, status, due_date, completed_at, sort_order, note, created_at, updated_at
			from project_milestones
			where tenant_id = $1 and project_id = $2 and id = $3
		`, tenantID, projectID, milestoneID))
		if err != nil {
			return err
		}
		milestone = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Milestone{}, ErrNotFound
	}
	return milestone, err
}

func (s *Store) getMember(ctx context.Context, tenantID, projectID, memberID string) (Member, error) {
	var member Member
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanMember(tx.QueryRow(ctx, `
			select pm.id::text, pm.project_id::text, u.id::text, u.email, u.name, pm.role, pm.created_at, pm.updated_at
			from project_members pm
			join users u on u.id = pm.user_id
			where pm.tenant_id = $1 and pm.project_id = $2 and pm.id = $3
		`, tenantID, projectID, memberID))
		if err != nil {
			return err
		}
		member = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return member, err
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
		select p.id::text, p.name, p.status, p.result,
			owner.user_id::text, coalesce(owner.name, ''),
			coalesce(bids.bid_count, 0), coalesce(milestones.milestone_count, 0),
			p.created_at, p.updated_at
		from projects p
		left join lateral (
			select pm.user_id, u.name
			from project_members pm
			join users u on u.id = pm.user_id
			where pm.tenant_id = p.tenant_id and pm.project_id = p.id
			order by case when pm.role = 'owner' then 0 else 1 end, pm.created_at
			limit 1
		) owner on true
		left join lateral (
			select count(*)::int as bid_count
			from bid_documents bd
			where bd.tenant_id = p.tenant_id and bd.project_id = p.id
		) bids on true
		left join lateral (
			select count(*)::int as milestone_count
			from project_milestones m
			where m.tenant_id = p.tenant_id and m.project_id = p.id
		) milestones on true
	`
}

func scanProject(row scanner) (Project, error) {
	var project Project
	var result, ownerID sql.NullString
	err := row.Scan(
		&project.ID, &project.Name, &project.Status, &result,
		&ownerID, &project.OwnerName, &project.BidCount, &project.MilestoneCount,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if result.Valid {
		project.Result = &result.String
	}
	if ownerID.Valid {
		project.OwnerID = &ownerID.String
	}
	return project, err
}

func scanMilestone(row scanner) (Milestone, error) {
	var milestone Milestone
	var dueDate, completedAt sql.NullTime
	err := row.Scan(
		&milestone.ID, &milestone.ProjectID, &milestone.Title, &milestone.Status,
		&dueDate, &completedAt, &milestone.SortOrder, &milestone.Note, &milestone.CreatedAt, &milestone.UpdatedAt,
	)
	if dueDate.Valid {
		milestone.DueDate = &dueDate.Time
	}
	if completedAt.Valid {
		milestone.CompletedAt = &completedAt.Time
	}
	return milestone, err
}

func scanMember(row scanner) (Member, error) {
	var member Member
	err := row.Scan(
		&member.ID, &member.ProjectID, &member.User.ID, &member.User.Email,
		&member.User.Name, &member.Role, &member.CreatedAt, &member.UpdatedAt,
	)
	return member, err
}

func scanActivity(row scanner) (Activity, error) {
	var activity Activity
	var actorUserID sql.NullString
	var metadataRaw []byte
	err := row.Scan(&activity.ID, &activity.ProjectID, &actorUserID, &activity.ActorName, &activity.Action, &metadataRaw, &activity.CreatedAt)
	if actorUserID.Valid {
		activity.ActorUserID = &actorUserID.String
	}
	activity.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &activity.Metadata)
	}
	return activity, err
}

func ensureProject(ctx context.Context, tx pgx.Tx, tenantID, projectID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from projects where tenant_id = $1 and id = $2)`, tenantID, projectID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return nil
}

func insertProjectMember(ctx context.Context, tx pgx.Tx, tenantID, projectID, userID, role string) error {
	_, err := tx.Exec(ctx, `
		insert into project_members (tenant_id, project_id, user_id, role)
		values ($1, $2, $3, $4)
		on conflict (tenant_id, project_id, user_id)
		do update set role = excluded.role, updated_at = now()
	`, tenantID, projectID, userID, role)
	return err
}

func insertLog(ctx context.Context, tx pgx.Tx, tenantID, projectID, userID, action string, metadata map[string]any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into project_logs (tenant_id, project_id, actor_user_id, action, metadata)
		values ($1, $2, $3, $4, $5)
	`, tenantID, projectID, nullableUserID(userID), action, raw)
	return err
}

func validateUUID(value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func nullableUserID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "opportunity", "bidding", "compliance_review", "submitted", "closed":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeResultPointer(value *string, status string) *string {
	if status != "closed" {
		return nil
	}
	if value == nil {
		result := "pending"
		return &result
	}
	result := normalizeResultValue(*value, status)
	if result == nil {
		fallback := "pending"
		return &fallback
	}
	return result
}

func normalizeResultValue(value, status string) *string {
	if status != "closed" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "won", "lost", "pending":
		result := strings.ToLower(strings.TrimSpace(value))
		return &result
	default:
		result := "pending"
		return &result
	}
}

func allowedTransition(current, target string) bool {
	if current == target {
		return true
	}
	allowed := map[string][]string{
		"opportunity":       {"bidding", "closed"},
		"bidding":           {"compliance_review", "closed"},
		"compliance_review": {"submitted", "closed"},
		"submitted":         {"closed"},
		"closed":            {},
	}
	for _, next := range allowed[current] {
		if next == target {
			return true
		}
	}
	return false
}

func normalizeMilestoneStatus(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "done") {
		return "done"
	}
	return "pending"
}

func parseOptionalDate(value string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
