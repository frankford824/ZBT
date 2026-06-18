package approval

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("approval resource not found")
	ErrInvalidRequest = errors.New("invalid approval request")
	ErrForbidden      = errors.New("approval action forbidden")
)

const (
	maxApprovalChainNameRunes         = 255
	maxApprovalChainDescriptionRunes  = 1000
	maxApprovalResourceTypeRunes      = 64
	maxApprovalSteps                  = 20
	maxApprovalStepsJSONBytes         = 16 * 1024
	maxApprovalStepNameRunes          = 128
	maxApprovalStepRoleCodeRunes      = 128
	maxApprovalStepConditionRunes     = 1000
	maxApprovalDecisionCommentRunes   = 1000
	maxApprovalInstanceTitleRunes     = 255
	maxApprovalNotificationTitleRunes = 128
	maxApprovalNotificationBodyRunes  = 1000
)

type Store struct {
	pool *pgxpool.Pool
}

type Step struct {
	Order     int    `json:"order"`
	Name      string `json:"name"`
	RoleCode  string `json:"role_code"`
	UserID    string `json:"user_id,omitempty"`
	Required  bool   `json:"required"`
	Condition string `json:"condition"`
}

type Chain struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	ResourceType string    `json:"resource_type"`
	Steps        []Step    `json:"steps"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Instance struct {
	ID              string     `json:"id"`
	ChainID         *string    `json:"chain_id"`
	BidDocumentID   *string    `json:"bid_document_id"`
	BidTitle        string     `json:"bid_title"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	CurrentStep     int        `json:"current_step"`
	SubmittedBy     *string    `json:"submitted_by"`
	SubmittedByName string     `json:"submitted_by_name"`
	Snapshot        []Step     `json:"snapshot"`
	ActionCount     int        `json:"action_count"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Action struct {
	ID          string    `json:"id"`
	InstanceID  string    `json:"instance_id"`
	ActorUserID *string   `json:"actor_user_id"`
	ActorName   string    `json:"actor_name"`
	Action      string    `json:"action"`
	StepOrder   int       `json:"step_order"`
	Comment     string    `json:"comment"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type InstanceDetail struct {
	Instance Instance `json:"instance"`
	Actions  []Action `json:"actions"`
}

type CreateChainRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ResourceType string `json:"resource_type"`
	Steps        []Step `json:"steps"`
	Enabled      bool   `json:"enabled"`
}

type UpdateChainRequest = CreateChainRequest

type DecisionRequest struct {
	Comment string `json:"comment"`
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListChains(ctx context.Context, tenantID string) ([]Chain, error) {
	chains := []Chain{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, name, description, resource_type, steps, enabled, created_at, updated_at
			from approval_chains
			where tenant_id = $1
			order by enabled desc, updated_at desc
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			chain, err := scanChain(rows)
			if err != nil {
				return err
			}
			chains = append(chains, chain)
		}
		return rows.Err()
	})
	return chains, err
}

func (s *Store) CreateChain(ctx context.Context, tenantID string, req CreateChainRequest) (Chain, error) {
	normalized, err := normalizeChain(req)
	if err != nil {
		return Chain{}, err
	}
	stepsRaw, err := marshalApprovalSteps(normalized.Steps)
	if err != nil {
		return Chain{}, err
	}
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := validateChainStepActors(ctx, tx, tenantID, normalized.Steps); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			insert into approval_chains (tenant_id, name, description, resource_type, steps, enabled)
			values ($1, $2, $3, $4, $5, $6)
			returning id::text
		`, tenantID, normalized.Name, normalized.Description, normalized.ResourceType, stepsRaw, normalized.Enabled).Scan(&id)
	})
	if err != nil {
		return Chain{}, err
	}
	return s.GetChain(ctx, tenantID, id)
}

func (s *Store) GetChain(ctx context.Context, tenantID, id string) (Chain, error) {
	if err := validateUUID(id); err != nil {
		return Chain{}, err
	}
	var chain Chain
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanChain(tx.QueryRow(ctx, `
			select id::text, name, description, resource_type, steps, enabled, created_at, updated_at
			from approval_chains
			where tenant_id = $1 and id = $2
		`, tenantID, id))
		if err != nil {
			return err
		}
		chain = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Chain{}, ErrNotFound
	}
	return chain, err
}

func (s *Store) UpdateChain(ctx context.Context, tenantID, id string, req UpdateChainRequest) (Chain, error) {
	if err := validateUUID(id); err != nil {
		return Chain{}, err
	}
	normalized, err := normalizeChain(req)
	if err != nil {
		return Chain{}, err
	}
	stepsRaw, err := marshalApprovalSteps(normalized.Steps)
	if err != nil {
		return Chain{}, err
	}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := validateChainStepActors(ctx, tx, tenantID, normalized.Steps); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			update approval_chains
			set name = $3, description = $4, resource_type = $5, steps = $6, enabled = $7, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, normalized.Name, normalized.Description, normalized.ResourceType, stepsRaw, normalized.Enabled)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Chain{}, ErrNotFound
	}
	if err != nil {
		return Chain{}, err
	}
	return s.GetChain(ctx, tenantID, id)
}

func (s *Store) DeleteChain(ctx context.Context, tenantID, id string) error {
	if err := validateUUID(id); err != nil {
		return err
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update approval_instances
			set chain_id = null, updated_at = now()
			where tenant_id = $1 and chain_id = $2
		`, tenantID, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `delete from approval_chains where tenant_id = $1 and id = $2`, tenantID, id)
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

func (s *Store) SubmitBid(ctx context.Context, tenantID, userID, bidID string) (InstanceDetail, error) {
	if err := validateUUID(bidID); err != nil {
		return InstanceDetail{}, err
	}
	var instanceID string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var bidTitle string
		if err := tx.QueryRow(ctx, `
			select title from bid_documents
			where tenant_id = $1 and id = $2
		`, tenantID, bidID).Scan(&bidTitle); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			select exists(
				select 1 from approval_instances
				where tenant_id = $1 and bid_document_id = $2 and status = 'pending'
			)
		`, tenantID, bidID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrInvalidRequest
		}
		chain, err := activeBidChain(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		snapshotRaw, err := marshalApprovalSteps(chain.Steps)
		if err != nil {
			return err
		}
		title := boundedApprovalText(bidTitle+" 审批", maxApprovalInstanceTitleRunes)
		if title == "" {
			title = "标书审批"
		}
		if err := tx.QueryRow(ctx, `
			insert into approval_instances (tenant_id, chain_id, bid_document_id, title, status, current_step, submitted_by, snapshot)
			values ($1, $2, $3, $4, 'pending', 1, $5, $6)
			returning id::text
		`, tenantID, chain.ID, bidID, title, userID, snapshotRaw).Scan(&instanceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into approval_actions (tenant_id, instance_id, actor_user_id, action, step_order, comment)
			values ($1, $2, $3, 'submit', 0, '提交审批')
		`, tenantID, instanceID, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update bid_documents
			set status = 'in_review', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, bidID); err != nil {
			return err
		}
		return notifyUsersForStep(ctx, tx, tenantID, chain.Steps[0], "标书审批待处理", bidTitle+" 已提交审批，请及时处理。")
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return InstanceDetail{}, ErrNotFound
	}
	if err != nil {
		return InstanceDetail{}, err
	}
	return s.GetInstance(ctx, tenantID, instanceID)
}

func (s *Store) ListInstances(ctx context.Context, tenantID, status string) ([]Instance, error) {
	status, err := normalizeInstanceStatusFilter(status)
	if err != nil {
		return nil, err
	}
	instances := []Instance{}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, instanceSelectSQL()+`
			where ai.tenant_id = $1 and ($2 = '' or ai.status = $2)
			order by ai.created_at desc
			limit 100
		`, tenantID, status)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			instance, err := scanInstance(rows)
			if err != nil {
				return err
			}
			instances = append(instances, instance)
		}
		return rows.Err()
	})
	return instances, err
}

func (s *Store) GetInstance(ctx context.Context, tenantID, id string) (InstanceDetail, error) {
	if err := validateUUID(id); err != nil {
		return InstanceDetail{}, err
	}
	var detail InstanceDetail
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		instance, err := scanInstance(tx.QueryRow(ctx, instanceSelectSQL()+` where ai.tenant_id = $1 and ai.id = $2`, tenantID, id))
		if err != nil {
			return err
		}
		actions, err := listActions(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		detail = InstanceDetail{Instance: instance, Actions: actions}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return InstanceDetail{}, ErrNotFound
	}
	return detail, err
}

func (s *Store) Approve(ctx context.Context, tenantID, userID, instanceID string, req DecisionRequest) (InstanceDetail, error) {
	if err := validateUUID(instanceID); err != nil {
		return InstanceDetail{}, err
	}
	comment, err := normalizeDecisionComment(req.Comment)
	if err != nil {
		return InstanceDetail{}, err
	}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		instance, err := scanInstance(tx.QueryRow(ctx, instanceSelectSQL()+` where ai.tenant_id = $1 and ai.id = $2 for update of ai`, tenantID, instanceID))
		if err != nil {
			return err
		}
		if instance.Status != "pending" {
			return ErrInvalidRequest
		}
		step, steps, err := currentApprovalStep(instance.Snapshot, instance.CurrentStep)
		if err != nil {
			return err
		}
		if err := ensureStepActor(ctx, tx, tenantID, userID, step); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into approval_actions (tenant_id, instance_id, actor_user_id, action, step_order, comment)
			values ($1, $2, $3, 'approve', $4, $5)
		`, tenantID, instance.ID, userID, instance.CurrentStep, comment); err != nil {
			return err
		}
		if instance.CurrentStep >= len(steps) {
			if _, err := tx.Exec(ctx, `
				update approval_instances
				set status = 'approved', completed_at = now(), updated_at = now()
				where tenant_id = $1 and id = $2
			`, tenantID, instance.ID); err != nil {
				return err
			}
			if instance.BidDocumentID != nil {
				if _, err := tx.Exec(ctx, `
					update bid_documents
					set status = 'approved', updated_at = now()
					where tenant_id = $1 and id = $2
				`, tenantID, *instance.BidDocumentID); err != nil {
					return err
				}
			}
			return notifySubmitter(ctx, tx, tenantID, instance.SubmittedBy, "审批通过", instance.Title+" 已通过全部审批。")
		}
		nextStep := steps[instance.CurrentStep]
		if _, err := tx.Exec(ctx, `
			update approval_instances
			set current_step = current_step + 1, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, instance.ID); err != nil {
			return err
		}
		return notifyUsersForStep(ctx, tx, tenantID, nextStep, "标书审批待处理", instance.Title+" 进入下一审批级。")
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return InstanceDetail{}, ErrNotFound
	}
	if err != nil {
		return InstanceDetail{}, err
	}
	return s.GetInstance(ctx, tenantID, instanceID)
}

func (s *Store) Reject(ctx context.Context, tenantID, userID, instanceID string, req DecisionRequest) (InstanceDetail, error) {
	if err := validateUUID(instanceID); err != nil {
		return InstanceDetail{}, err
	}
	comment, err := normalizeDecisionComment(req.Comment)
	if err != nil {
		return InstanceDetail{}, err
	}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		instance, err := scanInstance(tx.QueryRow(ctx, instanceSelectSQL()+` where ai.tenant_id = $1 and ai.id = $2 for update of ai`, tenantID, instanceID))
		if err != nil {
			return err
		}
		if instance.Status != "pending" {
			return ErrInvalidRequest
		}
		step, _, err := currentApprovalStep(instance.Snapshot, instance.CurrentStep)
		if err != nil {
			return err
		}
		if err := ensureStepActor(ctx, tx, tenantID, userID, step); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into approval_actions (tenant_id, instance_id, actor_user_id, action, step_order, comment)
			values ($1, $2, $3, 'reject', $4, $5)
		`, tenantID, instance.ID, userID, instance.CurrentStep, comment); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update approval_instances
			set status = 'rejected', completed_at = now(), updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, instance.ID); err != nil {
			return err
		}
		if instance.BidDocumentID != nil {
			if _, err := tx.Exec(ctx, `
				update bid_documents
				set status = 'editing', updated_at = now()
				where tenant_id = $1 and id = $2
			`, tenantID, *instance.BidDocumentID); err != nil {
				return err
			}
		}
		return notifySubmitter(ctx, tx, tenantID, instance.SubmittedBy, "审批驳回", instance.Title+" 已驳回，请根据意见修改。")
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return InstanceDetail{}, ErrNotFound
	}
	if err != nil {
		return InstanceDetail{}, err
	}
	return s.GetInstance(ctx, tenantID, instanceID)
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

func scanChain(row scanner) (Chain, error) {
	var chain Chain
	var raw []byte
	err := row.Scan(&chain.ID, &chain.Name, &chain.Description, &chain.ResourceType, &raw, &chain.Enabled, &chain.CreatedAt, &chain.UpdatedAt)
	if err != nil {
		return Chain{}, err
	}
	chain.Steps, err = unmarshalApprovalSteps(raw)
	if err != nil {
		return Chain{}, err
	}
	return chain, nil
}

func instanceSelectSQL() string {
	return `
		select ai.id::text, ai.chain_id::text, ai.bid_document_id::text, coalesce(bd.title, ''),
			ai.title, ai.status, ai.current_step, ai.submitted_by::text, coalesce(u.name, ''),
			ai.snapshot, coalesce(action_counts.action_count, 0)::int,
			ai.started_at, ai.completed_at, ai.created_at, ai.updated_at
		from approval_instances ai
		left join bid_documents bd on bd.id = ai.bid_document_id and bd.tenant_id = ai.tenant_id
		left join users u on u.id = ai.submitted_by
		left join lateral (
			select count(*) as action_count
			from approval_actions aa
			where aa.tenant_id = ai.tenant_id and aa.instance_id = ai.id
		) action_counts on true
	`
}

func scanInstance(row scanner) (Instance, error) {
	var instance Instance
	var chainID, bidID, submittedBy sql.NullString
	var completedAt sql.NullTime
	var raw []byte
	err := row.Scan(
		&instance.ID, &chainID, &bidID, &instance.BidTitle, &instance.Title, &instance.Status, &instance.CurrentStep,
		&submittedBy, &instance.SubmittedByName, &raw, &instance.ActionCount, &instance.StartedAt,
		&completedAt, &instance.CreatedAt, &instance.UpdatedAt,
	)
	if chainID.Valid {
		instance.ChainID = &chainID.String
	}
	if bidID.Valid {
		instance.BidDocumentID = &bidID.String
	}
	if submittedBy.Valid {
		instance.SubmittedBy = &submittedBy.String
	}
	if completedAt.Valid {
		instance.CompletedAt = &completedAt.Time
	}
	instance.Snapshot, err = unmarshalApprovalSteps(raw)
	if err != nil {
		return Instance{}, err
	}
	return instance, nil
}

func listActions(ctx context.Context, tx pgx.Tx, tenantID, instanceID string) ([]Action, error) {
	actions := []Action{}
	rows, err := tx.Query(ctx, `
		select aa.id::text, aa.instance_id::text, aa.actor_user_id::text, coalesce(u.name, ''),
			aa.action, aa.step_order, aa.comment, aa.created_at, aa.updated_at
		from approval_actions aa
		left join users u on u.id = aa.actor_user_id
		where aa.tenant_id = $1 and aa.instance_id = $2
		order by aa.created_at
	`, tenantID, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var action Action
		var actorID sql.NullString
		if err := rows.Scan(&action.ID, &action.InstanceID, &actorID, &action.ActorName, &action.Action, &action.StepOrder, &action.Comment, &action.CreatedAt, &action.UpdatedAt); err != nil {
			return nil, err
		}
		if actorID.Valid {
			action.ActorUserID = &actorID.String
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func activeBidChain(ctx context.Context, tx pgx.Tx, tenantID string) (Chain, error) {
	chain, err := scanChain(tx.QueryRow(ctx, `
		select id::text, name, description, resource_type, steps, enabled, created_at, updated_at
		from approval_chains
		where tenant_id = $1 and resource_type = 'bid' and enabled
		order by updated_at desc
		limit 1
	`, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Chain{}, ErrNotFound
	}
	if err != nil {
		return Chain{}, err
	}
	if len(activeSteps(chain.Steps)) == 0 {
		return Chain{}, ErrInvalidRequest
	}
	chain.Steps = activeSteps(chain.Steps)
	return chain, nil
}

func notifyUsersForStep(ctx context.Context, tx pgx.Tx, tenantID string, step Step, title, body string) error {
	title, body = normalizeApprovalNotification(title, body)
	userIDs := []string{}
	if strings.TrimSpace(step.UserID) != "" {
		userIDs = append(userIDs, strings.TrimSpace(step.UserID))
	} else if strings.TrimSpace(step.RoleCode) != "" {
		rows, err := tx.Query(ctx, `
			select u.id::text
			from tenant_members tm
			join users u on u.id = tm.user_id
			join tenant_member_roles tmr on tmr.tenant_member_id = tm.id and tmr.tenant_id = tm.tenant_id
			join roles r on r.id = tmr.role_id and r.tenant_id = tm.tenant_id
			where tm.tenant_id = $1 and tm.status = 'active' and r.code = $2
			order by u.name
		`, tenantID, strings.TrimSpace(step.RoleCode))
		if err != nil {
			return err
		}
		for rows.Next() {
			var userID string
			if err := rows.Scan(&userID); err != nil {
				rows.Close()
				return err
			}
			userIDs = append(userIDs, userID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	if len(userIDs) == 0 {
		_, err := tx.Exec(ctx, `
			insert into notifications (tenant_id, user_id, title, body)
			values ($1, null, $2, $3)
		`, tenantID, title, body)
		return err
	}
	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx, `
			insert into notifications (tenant_id, user_id, title, body)
			values ($1, $2, $3, $4)
		`, tenantID, userID, title, body); err != nil {
			return err
		}
	}
	return nil
}

func notifySubmitter(ctx context.Context, tx pgx.Tx, tenantID string, submittedBy *string, title, body string) error {
	title, body = normalizeApprovalNotification(title, body)
	if submittedBy == nil || *submittedBy == "" {
		_, err := tx.Exec(ctx, `
			insert into notifications (tenant_id, user_id, title, body)
			values ($1, null, $2, $3)
		`, tenantID, title, body)
		return err
	}
	_, err := tx.Exec(ctx, `
		insert into notifications (tenant_id, user_id, title, body)
		values ($1, $2, $3, $4)
	`, tenantID, *submittedBy, title, body)
	return err
}

func currentApprovalStep(snapshot []Step, currentStep int) (Step, []Step, error) {
	steps := activeSteps(snapshot)
	if len(steps) == 0 || currentStep <= 0 || currentStep > len(steps) {
		return Step{}, nil, ErrInvalidRequest
	}
	return steps[currentStep-1], steps, nil
}

func ensureStepActor(ctx context.Context, tx pgx.Tx, tenantID, userID string, step Step) error {
	actorID, err := normalizeUUID(userID)
	if err != nil {
		return err
	}
	stepUserID := strings.TrimSpace(step.UserID)
	if stepUserID != "" {
		allowedUserID, err := normalizeUUID(stepUserID)
		if err != nil {
			return err
		}
		if actorID != allowedUserID {
			return ErrForbidden
		}
		return nil
	}
	roleCode := strings.TrimSpace(step.RoleCode)
	if roleCode == "" {
		return ErrInvalidRequest
	}
	var allowed bool
	if err := tx.QueryRow(ctx, `
		select exists(
			select 1
			from tenant_members tm
			join tenant_member_roles tmr on tmr.tenant_member_id = tm.id and tmr.tenant_id = tm.tenant_id
			join roles r on r.id = tmr.role_id and r.tenant_id = tm.tenant_id
			where tm.tenant_id = $1
				and tm.user_id = $2
				and tm.status = 'active'
				and r.code = $3
		)
	`, tenantID, actorID, roleCode).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func validateChainStepActors(ctx context.Context, tx pgx.Tx, tenantID string, steps []Step) error {
	seenUsers := map[string]bool{}
	seenRoles := map[string]bool{}
	for _, step := range steps {
		if strings.TrimSpace(step.UserID) != "" {
			userID, err := normalizeUUID(step.UserID)
			if err != nil {
				return err
			}
			if seenUsers[userID] {
				continue
			}
			var exists bool
			if err := tx.QueryRow(ctx, `
				select exists(
					select 1
					from tenant_members
					where tenant_id = $1
						and user_id = $2
						and status = 'active'
				)
			`, tenantID, userID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrInvalidRequest
			}
			seenUsers[userID] = true
			continue
		}
		roleCode := strings.TrimSpace(step.RoleCode)
		if roleCode == "" || seenRoles[roleCode] {
			continue
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			select exists(select 1 from roles where tenant_id = $1 and code = $2)
		`, tenantID, roleCode).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrInvalidRequest
		}
		seenRoles[roleCode] = true
	}
	return nil
}

func normalizeChain(req CreateChainRequest) (CreateChainRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.ResourceType = strings.TrimSpace(req.ResourceType)
	if req.Name == "" {
		req.Name = "默认审批链"
	}
	if req.ResourceType == "" {
		req.ResourceType = "bid"
	}
	for _, check := range []struct {
		value string
		limit int
	}{
		{req.Name, maxApprovalChainNameRunes},
		{req.Description, maxApprovalChainDescriptionRunes},
		{req.ResourceType, maxApprovalResourceTypeRunes},
	} {
		if err := validateApprovalTextLength(check.value, check.limit); err != nil {
			return req, err
		}
	}
	if req.ResourceType != "bid" {
		return req, ErrInvalidRequest
	}
	if len(req.Steps) == 0 {
		req.Steps = []Step{
			{Order: 1, Name: "部门主管审批", RoleCode: "department_admin", Required: true},
			{Order: 2, Name: "项目经理审批", RoleCode: "project_manager", Required: true},
		}
	}
	if len(req.Steps) > maxApprovalSteps {
		return req, ErrInvalidRequest
	}
	for i := range req.Steps {
		req.Steps[i].Order = i + 1
		req.Steps[i].Name = strings.TrimSpace(req.Steps[i].Name)
		req.Steps[i].RoleCode = strings.TrimSpace(req.Steps[i].RoleCode)
		req.Steps[i].UserID = strings.TrimSpace(req.Steps[i].UserID)
		req.Steps[i].Condition = strings.TrimSpace(req.Steps[i].Condition)
		if req.Steps[i].UserID != "" {
			userID, err := normalizeUUID(req.Steps[i].UserID)
			if err != nil {
				return req, err
			}
			req.Steps[i].UserID = userID
		}
		if req.Steps[i].Name == "" {
			req.Steps[i].Name = "审批级"
		}
		if req.Steps[i].RoleCode == "" && req.Steps[i].UserID == "" {
			req.Steps[i].RoleCode = "company_admin"
		}
		for _, check := range []struct {
			value string
			limit int
		}{
			{req.Steps[i].Name, maxApprovalStepNameRunes},
			{req.Steps[i].RoleCode, maxApprovalStepRoleCodeRunes},
			{req.Steps[i].Condition, maxApprovalStepConditionRunes},
		} {
			if err := validateApprovalTextLength(check.value, check.limit); err != nil {
				return req, err
			}
		}
	}
	return req, nil
}

func normalizeDecisionComment(value string) (string, error) {
	value = strings.TrimSpace(value)
	if err := validateApprovalTextLength(value, maxApprovalDecisionCommentRunes); err != nil {
		return "", err
	}
	return value, nil
}

func marshalApprovalSteps(steps []Step) ([]byte, error) {
	if len(steps) > maxApprovalSteps {
		return nil, ErrInvalidRequest
	}
	for _, step := range steps {
		for _, check := range []struct {
			value string
			limit int
		}{
			{step.Name, maxApprovalStepNameRunes},
			{step.RoleCode, maxApprovalStepRoleCodeRunes},
			{step.Condition, maxApprovalStepConditionRunes},
		} {
			if err := validateApprovalTextLength(check.value, check.limit); err != nil {
				return nil, err
			}
		}
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	if len(raw) > maxApprovalStepsJSONBytes {
		return nil, ErrInvalidRequest
	}
	return raw, nil
}

func unmarshalApprovalSteps(raw []byte) ([]Step, error) {
	steps := []Step{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return steps, nil
	}
	if len(trimmed) > maxApprovalStepsJSONBytes {
		return nil, ErrInvalidRequest
	}
	if err := json.Unmarshal(trimmed, &steps); err != nil {
		return nil, err
	}
	if steps == nil {
		steps = []Step{}
	}
	if _, err := marshalApprovalSteps(steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func normalizeApprovalNotification(title, body string) (string, string) {
	title = boundedApprovalText(title, maxApprovalNotificationTitleRunes)
	if title == "" {
		title = "审批通知"
	}
	body = boundedApprovalText(body, maxApprovalNotificationBodyRunes)
	return title, body
}

func validateApprovalTextLength(value string, maxRunes int) error {
	if maxRunes <= 0 {
		return ErrInvalidRequest
	}
	if utf8.RuneCountInString(strings.TrimSpace(value)) > maxRunes {
		return ErrInvalidRequest
	}
	return nil
}

func boundedApprovalText(value string, maxRunes int) string {
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

func activeSteps(steps []Step) []Step {
	result := []Step{}
	for _, step := range steps {
		if step.Required {
			result = append(result, step)
		}
	}
	if len(result) == 0 {
		return steps
	}
	return result
}

func validateUUID(value string) error {
	_, err := normalizeUUID(value)
	return err
}

func normalizeUUID(value string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", ErrInvalidRequest
	}
	return id.String(), nil
}

func normalizeInstanceStatusFilter(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending", "approved", "rejected", "cancelled":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}
