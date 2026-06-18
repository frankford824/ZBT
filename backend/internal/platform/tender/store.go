package tender

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("tender resource not found")
	ErrInvalidRequest = errors.New("invalid tender request")
)

const (
	sourceVerifySuccessMessage     = "检测通过"
	sourceVerifyUnavailableMessage = "来源暂时无法访问，请稍后重试"
	sourceVerifyFailedMessage      = "来源响应异常，请稍后重试"
	maxSourceConfigEntries         = 50
	maxSourceConfigKeyRunes        = 128
	maxSourceConfigJSONBytes       = 16 * 1024
	maxTenderTitleRunes            = 255
	maxTenderShortTextRunes        = 255
	maxTenderSummaryRunes          = 2000
	maxTenderListItems             = 50
	maxTenderListItemRunes         = 500
	maxTenderMetadataEntries       = 50
	maxTenderMetadataKeyRunes      = 128
	maxTenderMetadataJSONBytes     = 16 * 1024
	maxTenderBudgetAmount          = 999_999_999_999.99
)

type Store struct {
	pool   *pgxpool.Pool
	client *http.Client
}

type Tender struct {
	ID           string         `json:"id"`
	SourceID     *string        `json:"source_id"`
	SourceName   string         `json:"source_name"`
	Title        string         `json:"title"`
	Purchaser    string         `json:"purchaser"`
	Region       string         `json:"region"`
	BudgetAmount *float64       `json:"budget_amount"`
	BudgetText   string         `json:"budget_text"`
	PublishDate  *time.Time     `json:"publish_date"`
	Deadline     *time.Time     `json:"deadline"`
	Status       string         `json:"status"`
	MatchScore   int            `json:"match_score"`
	Summary      string         `json:"summary"`
	Requirements []string       `json:"requirements"`
	RiskFlags    []string       `json:"risk_flags"`
	SourceURL    string         `json:"source_url"`
	Metadata     map[string]any `json:"metadata"`
	Favorite     bool           `json:"favorite"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Source struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	SourceType        string         `json:"source_type"`
	URL               string         `json:"url"`
	Status            string         `json:"status"`
	LastVerifiedAt    *time.Time     `json:"last_verified_at"`
	LastVerifyStatus  *string        `json:"last_verify_status"`
	LastVerifyMessage string         `json:"last_verify_message"`
	Config            map[string]any `json:"config"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListFilter struct {
	Search       string
	Region       string
	Status       string
	SourceID     string
	FavoriteOnly bool
	Recommended  bool
}

type CreateTenderRequest struct {
	SourceID     string   `json:"source_id"`
	Title        string   `json:"title"`
	Purchaser    string   `json:"purchaser"`
	Region       string   `json:"region"`
	BudgetAmount *float64 `json:"budget_amount"`
	BudgetText   string   `json:"budget_text"`
	PublishDate  string   `json:"publish_date"`
	Deadline     string   `json:"deadline"`
	Status       string   `json:"status"`
	MatchScore   int      `json:"match_score"`
	Summary      string   `json:"summary"`
	Requirements []string `json:"requirements"`
	RiskFlags    []string `json:"risk_flags"`
	SourceURL    string   `json:"source_url"`
	Metadata     any      `json:"metadata"`
}

type UpdateTenderRequest struct {
	SourceID     *string         `json:"source_id"`
	Title        *string         `json:"title"`
	Purchaser    *string         `json:"purchaser"`
	Region       *string         `json:"region"`
	BudgetAmount *float64        `json:"budget_amount"`
	BudgetText   *string         `json:"budget_text"`
	PublishDate  *string         `json:"publish_date"`
	Deadline     *string         `json:"deadline"`
	Status       *string         `json:"status"`
	MatchScore   *int            `json:"match_score"`
	Summary      *string         `json:"summary"`
	Requirements *[]string       `json:"requirements"`
	RiskFlags    *[]string       `json:"risk_flags"`
	SourceURL    *string         `json:"source_url"`
	Metadata     *map[string]any `json:"metadata"`
}

type CreateSourceRequest struct {
	Name       string         `json:"name"`
	SourceType string         `json:"source_type"`
	URL        string         `json:"url"`
	Status     string         `json:"status"`
	Config     map[string]any `json:"config"`
}

type UpdateSourceRequest struct {
	Name       *string         `json:"name"`
	SourceType *string         `json:"source_type"`
	URL        *string         `json:"url"`
	Status     *string         `json:"status"`
	Config     *map[string]any `json:"config"`
}

type normalizedTenderWriteRequest struct {
	SourceID     any
	Title        string
	Purchaser    string
	Region       string
	BudgetAmount *float64
	BudgetText   string
	PublishDate  any
	Deadline     any
	Status       string
	MatchScore   int
	Summary      string
	Requirements []string
	RiskFlags    []string
	SourceURL    string
	Metadata     []byte
}

type normalizedSourceWriteRequest struct {
	Name       string
	SourceType string
	URL        string
	Status     string
	Config     []byte
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:   pool,
		client: newVerificationHTTPClient(),
	}
}

func (s *Store) List(ctx context.Context, tenantID, userID string, filter ListFilter) ([]Tender, error) {
	tenders := []Tender{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		args := []any{tenantID, userID}
		conditions := []string{"t.tenant_id = $1"}
		if search := strings.TrimSpace(filter.Search); search != "" {
			args = append(args, "%"+search+"%")
			conditions = append(conditions, fmt.Sprintf("(t.title ilike $%d or t.purchaser ilike $%d or t.summary ilike $%d)", len(args), len(args), len(args)))
		}
		if region := strings.TrimSpace(filter.Region); region != "" {
			args = append(args, region)
			conditions = append(conditions, fmt.Sprintf("t.region = $%d", len(args)))
		}
		status, err := normalizeTenderStatus(filter.Status)
		if err != nil {
			return err
		}
		if status != "" {
			args = append(args, status)
			conditions = append(conditions, fmt.Sprintf("t.status = $%d", len(args)))
		}
		if sourceID := strings.TrimSpace(filter.SourceID); sourceID != "" {
			if _, err := uuid.Parse(sourceID); err != nil {
				return ErrInvalidRequest
			}
			args = append(args, sourceID)
			conditions = append(conditions, fmt.Sprintf("t.source_id = $%d", len(args)))
		}
		if filter.FavoriteOnly {
			conditions = append(conditions, "coalesce(tus.favorite, false)")
		}
		if filter.Recommended {
			conditions = append(conditions, "t.match_score >= 80")
		}
		rows, err := tx.Query(ctx, `
			select t.id::text, t.source_id::text, coalesce(ts.name, ''), t.title, t.purchaser, t.region,
				t.budget_amount::float8, t.budget_text, t.publish_date, t.deadline, t.status, t.match_score,
				t.summary, t.requirements, t.risk_flags, t.source_url, t.metadata, coalesce(tus.favorite, false),
				t.created_at, t.updated_at
			from tenders t
			left join tender_sources ts on ts.id = t.source_id and ts.tenant_id = t.tenant_id
			left join tender_user_states tus on tus.tenant_id = t.tenant_id and tus.tender_id = t.id and tus.user_id = $2
			where `+strings.Join(conditions, " and ")+`
			order by t.match_score desc, t.deadline nulls last, t.created_at desc
			limit 200
		`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			tender, err := scanTender(rows)
			if err != nil {
				return err
			}
			tenders = append(tenders, tender)
		}
		return rows.Err()
	})
	return tenders, err
}

func (s *Store) Get(ctx context.Context, tenantID, userID, id string) (Tender, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Tender{}, ErrInvalidRequest
	}
	var tender Tender
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanTender(tx.QueryRow(ctx, `
			select t.id::text, t.source_id::text, coalesce(ts.name, ''), t.title, t.purchaser, t.region,
				t.budget_amount::float8, t.budget_text, t.publish_date, t.deadline, t.status, t.match_score,
				t.summary, t.requirements, t.risk_flags, t.source_url, t.metadata, coalesce(tus.favorite, false),
				t.created_at, t.updated_at
			from tenders t
			left join tender_sources ts on ts.id = t.source_id and ts.tenant_id = t.tenant_id
			left join tender_user_states tus on tus.tenant_id = t.tenant_id and tus.tender_id = t.id and tus.user_id = $2
			where t.tenant_id = $1 and t.id = $3
		`, tenantID, userID, id))
		if err != nil {
			return err
		}
		tender = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Tender{}, ErrNotFound
	}
	return tender, err
}

func (s *Store) Create(ctx context.Context, tenantID, userID string, req CreateTenderRequest) (Tender, error) {
	normalized, err := normalizeTenderWriteRequest(req)
	if err != nil {
		return Tender{}, err
	}
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			insert into tenders (
				tenant_id, source_id, title, purchaser, region, budget_amount, budget_text, publish_date, deadline,
				status, match_score, summary, requirements, risk_flags, source_url, metadata
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			returning id::text
		`, tenantID, normalized.SourceID, normalized.Title, normalized.Purchaser, normalized.Region, normalized.BudgetAmount,
			normalized.BudgetText, normalized.PublishDate, normalized.Deadline, normalized.Status, normalized.MatchScore, normalized.Summary,
			normalized.Requirements, normalized.RiskFlags, normalized.SourceURL, normalized.Metadata).Scan(&id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Tender{}, err
	}
	return s.Get(ctx, tenantID, userID, id)
}

func (s *Store) Update(ctx context.Context, tenantID, userID, id string, req UpdateTenderRequest) (Tender, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Tender{}, ErrInvalidRequest
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		current, err := scanTender(tx.QueryRow(ctx, `
			select t.id::text, t.source_id::text, coalesce(ts.name, ''), t.title, t.purchaser, t.region,
				t.budget_amount::float8, t.budget_text, t.publish_date, t.deadline, t.status, t.match_score,
				t.summary, t.requirements, t.risk_flags, t.source_url, t.metadata, false,
				t.created_at, t.updated_at
			from tenders t
			left join tender_sources ts on ts.id = t.source_id and ts.tenant_id = t.tenant_id
			where t.tenant_id = $1 and t.id = $2
			for update of t
		`, tenantID, id))
		if err != nil {
			return err
		}
		normalized, err := normalizeTenderWriteRequest(mergeTenderUpdateRequest(current, req))
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			update tenders
			set source_id = $3, title = $4, purchaser = $5, region = $6, budget_amount = $7, budget_text = $8,
				publish_date = $9, deadline = $10, status = $11, match_score = $12, summary = $13,
				requirements = $14, risk_flags = $15, source_url = $16, metadata = $17, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, normalized.SourceID, normalized.Title, normalized.Purchaser, normalized.Region, normalized.BudgetAmount,
			normalized.BudgetText, normalized.PublishDate, normalized.Deadline, normalized.Status, normalized.MatchScore, normalized.Summary,
			normalized.Requirements, normalized.RiskFlags, normalized.SourceURL, normalized.Metadata)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Tender{}, ErrNotFound
	}
	if err != nil {
		return Tender{}, err
	}
	return s.Get(ctx, tenantID, userID, id)
}

func (s *Store) SetFavorite(ctx context.Context, tenantID, userID, id string, favorite bool) (Tender, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Tender{}, ErrInvalidRequest
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from tenders where tenant_id = $1 and id = $2)`, tenantID, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}
		_, err := tx.Exec(ctx, `
			insert into tender_user_states (tenant_id, tender_id, user_id, favorite)
			values ($1, $2, $3, $4)
			on conflict (tenant_id, tender_id, user_id)
			do update set favorite = excluded.favorite, updated_at = now()
		`, tenantID, id, userID, favorite)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Tender{}, ErrNotFound
	}
	if err != nil {
		return Tender{}, err
	}
	return s.Get(ctx, tenantID, userID, id)
}

func (s *Store) CreateProject(ctx context.Context, tenantID, id string) (Project, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Project{}, ErrInvalidRequest
	}
	var project Project
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var title string
		if err := tx.QueryRow(ctx, `select title from tenders where tenant_id = $1 and id = $2`, tenantID, id).Scan(&title); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			insert into projects (tenant_id, name, status)
			values ($1, $2, 'opportunity')
			returning id::text, name, status, created_at, updated_at
		`, tenantID, title).Scan(&project.ID, &project.Name, &project.Status, &project.CreatedAt, &project.UpdatedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return project, err
}

func (s *Store) ListSources(ctx context.Context, tenantID string) ([]Source, error) {
	sources := []Source{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, name, source_type, url, status, last_verified_at, last_verify_status, last_verify_message, config, created_at, updated_at
			from tender_sources
			where tenant_id = $1
			order by created_at desc
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			source, err := scanSource(rows)
			if err != nil {
				return err
			}
			sources = append(sources, source)
		}
		return rows.Err()
	})
	return sources, err
}

func (s *Store) CreateSource(ctx context.Context, tenantID string, req CreateSourceRequest) (Source, error) {
	normalized, err := normalizeSourceWriteRequest(req)
	if err != nil {
		return Source{}, err
	}
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			insert into tender_sources (tenant_id, name, source_type, url, status, config)
			values ($1, $2, $3, $4, $5, $6)
			on conflict (tenant_id, name) do update
			set source_type = excluded.source_type,
				url = excluded.url,
				status = excluded.status,
				config = excluded.config,
				updated_at = now()
			returning id::text
		`, tenantID, normalized.Name, normalized.SourceType, normalized.URL, normalized.Status, normalized.Config).Scan(&id)
	})
	if err != nil {
		return Source{}, normalizeSourceWriteError(err)
	}
	return s.GetSource(ctx, tenantID, id)
}

func (s *Store) GetSource(ctx context.Context, tenantID, id string) (Source, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Source{}, ErrInvalidRequest
	}
	var source Source
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanSource(tx.QueryRow(ctx, `
			select id::text, name, source_type, url, status, last_verified_at, last_verify_status, last_verify_message, config, created_at, updated_at
			from tender_sources
			where tenant_id = $1 and id = $2
		`, tenantID, id))
		if err != nil {
			return err
		}
		source = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	return source, err
}

func (s *Store) UpdateSource(ctx context.Context, tenantID, id string, req UpdateSourceRequest) (Source, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Source{}, ErrInvalidRequest
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		current, err := scanSource(tx.QueryRow(ctx, `
			select id::text, name, source_type, url, status, last_verified_at, last_verify_status, last_verify_message, config, created_at, updated_at
			from tender_sources
			where tenant_id = $1 and id = $2
			for update
		`, tenantID, id))
		if err != nil {
			return err
		}
		normalized, err := normalizeSourceWriteRequest(mergeSourceUpdateRequest(current, req))
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			update tender_sources
			set name = $3, source_type = $4, url = $5, status = $6, config = $7, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, normalized.Name, normalized.SourceType, normalized.URL, normalized.Status, normalized.Config)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	if err != nil {
		return Source{}, normalizeSourceWriteError(err)
	}
	return s.GetSource(ctx, tenantID, id)
}

func (s *Store) DeleteSource(ctx context.Context, tenantID, id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return ErrInvalidRequest
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update tenders
			set source_id = null, updated_at = now()
			where tenant_id = $1 and source_id = $2
		`, tenantID, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `delete from tender_sources where tenant_id = $1 and id = $2`, tenantID, id)
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

func (s *Store) VerifySource(ctx context.Context, tenantID, id string) (Source, error) {
	source, err := s.GetSource(ctx, tenantID, id)
	if err != nil {
		return Source{}, err
	}
	if !validHTTPURL(source.URL) {
		return Source{}, ErrInvalidRequest
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, source.URL, nil)
	if err != nil {
		return Source{}, ErrInvalidRequest
	}
	resp, err := s.client.Do(req)
	if err == nil && resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 400) {
		resp.Body.Close()
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
		resp, err = s.client.Do(req)
	}
	if err == nil && resp != nil {
		defer resp.Body.Close()
	}
	status, message := sourceVerifyOutcome(resp, err)
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			update tender_sources
			set status = $3, last_verified_at = now(), last_verify_status = $4, last_verify_message = $5, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, mapVerifyStatusToSourceStatus(status), status, message)
		return err
	})
	if err != nil {
		return Source{}, err
	}
	return s.GetSource(ctx, tenantID, id)
}

func normalizeSourceWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23505":
			return ErrInvalidRequest
		}
	}
	return err
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

func scanTender(row scanner) (Tender, error) {
	var tender Tender
	var sourceID sql.NullString
	var budgetAmount sql.NullFloat64
	var publishDate, deadline sql.NullTime
	var metadataRaw []byte
	err := row.Scan(
		&tender.ID, &sourceID, &tender.SourceName, &tender.Title, &tender.Purchaser, &tender.Region,
		&budgetAmount, &tender.BudgetText, &publishDate, &deadline, &tender.Status, &tender.MatchScore,
		&tender.Summary, &tender.Requirements, &tender.RiskFlags, &tender.SourceURL, &metadataRaw, &tender.Favorite,
		&tender.CreatedAt, &tender.UpdatedAt,
	)
	if sourceID.Valid {
		tender.SourceID = &sourceID.String
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
	tender.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &tender.Metadata)
	}
	return tender, err
}

func mergeTenderUpdateRequest(current Tender, req UpdateTenderRequest) CreateTenderRequest {
	sourceID := ""
	if current.SourceID != nil {
		sourceID = *current.SourceID
	}
	budgetAmount := current.BudgetAmount
	metadata := current.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	merged := CreateTenderRequest{
		SourceID:     sourceID,
		Title:        current.Title,
		Purchaser:    current.Purchaser,
		Region:       current.Region,
		BudgetAmount: budgetAmount,
		BudgetText:   current.BudgetText,
		PublishDate:  formatOptionalDate(current.PublishDate),
		Deadline:     formatOptionalDate(current.Deadline),
		Status:       current.Status,
		MatchScore:   current.MatchScore,
		Summary:      current.Summary,
		Requirements: current.Requirements,
		RiskFlags:    current.RiskFlags,
		SourceURL:    current.SourceURL,
		Metadata:     metadata,
	}
	if req.SourceID != nil {
		merged.SourceID = *req.SourceID
	}
	if req.Title != nil {
		merged.Title = *req.Title
	}
	if req.Purchaser != nil {
		merged.Purchaser = *req.Purchaser
	}
	if req.Region != nil {
		merged.Region = *req.Region
	}
	if req.BudgetAmount != nil {
		merged.BudgetAmount = req.BudgetAmount
	}
	if req.BudgetText != nil {
		merged.BudgetText = *req.BudgetText
	}
	if req.PublishDate != nil {
		merged.PublishDate = *req.PublishDate
	}
	if req.Deadline != nil {
		merged.Deadline = *req.Deadline
	}
	if req.Status != nil {
		merged.Status = *req.Status
	}
	if req.MatchScore != nil {
		merged.MatchScore = *req.MatchScore
	}
	if req.Summary != nil {
		merged.Summary = *req.Summary
	}
	if req.Requirements != nil {
		merged.Requirements = *req.Requirements
	}
	if req.RiskFlags != nil {
		merged.RiskFlags = *req.RiskFlags
	}
	if req.SourceURL != nil {
		merged.SourceURL = *req.SourceURL
	}
	if req.Metadata != nil {
		merged.Metadata = *req.Metadata
	}
	return merged
}

func formatOptionalDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func normalizeTenderWriteRequest(req CreateTenderRequest) (normalizedTenderWriteRequest, error) {
	title, err := normalizeTenderRequiredText(req.Title, maxTenderTitleRunes)
	if err != nil {
		return normalizedTenderWriteRequest{}, ErrInvalidRequest
	}
	status, err := normalizeTenderStatus(req.Status)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	if status == "" {
		status = "open"
	}
	publishDate, err := parseOptionalDate(req.PublishDate)
	if err != nil {
		return normalizedTenderWriteRequest{}, ErrInvalidRequest
	}
	deadline, err := parseOptionalDate(req.Deadline)
	if err != nil {
		return normalizedTenderWriteRequest{}, ErrInvalidRequest
	}
	sourceID, err := nullableUUID(req.SourceID)
	if err != nil {
		return normalizedTenderWriteRequest{}, ErrInvalidRequest
	}
	sourceURL, err := normalizeOptionalSourceURL(req.SourceURL)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	if err := validateOptionalTenderAmount(req.BudgetAmount); err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	purchaser, err := normalizeTenderOptionalText(req.Purchaser, maxTenderShortTextRunes)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	region, err := normalizeTenderOptionalText(req.Region, maxTenderShortTextRunes)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	budgetText, err := normalizeTenderOptionalText(req.BudgetText, maxTenderShortTextRunes)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	summary, err := normalizeTenderOptionalText(req.Summary, maxTenderSummaryRunes)
	if err != nil {
		return normalizedTenderWriteRequest{}, ErrInvalidRequest
	}
	requirements, err := normalizeTenderTextList(req.Requirements)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	riskFlags, err := normalizeTenderTextList(req.RiskFlags)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	metadata, err := normalizeTenderMetadata(req.Metadata)
	if err != nil {
		return normalizedTenderWriteRequest{}, err
	}
	return normalizedTenderWriteRequest{
		SourceID:     sourceID,
		Title:        title,
		Purchaser:    purchaser,
		Region:       region,
		BudgetAmount: req.BudgetAmount,
		BudgetText:   budgetText,
		PublishDate:  publishDate,
		Deadline:     deadline,
		Status:       status,
		MatchScore:   clampScore(req.MatchScore),
		Summary:      summary,
		Requirements: requirements,
		RiskFlags:    riskFlags,
		SourceURL:    sourceURL,
		Metadata:     metadata,
	}, nil
}

func mergeSourceUpdateRequest(current Source, req UpdateSourceRequest) CreateSourceRequest {
	config := current.Config
	if config == nil {
		config = map[string]any{}
	}
	merged := CreateSourceRequest{
		Name:       current.Name,
		SourceType: current.SourceType,
		URL:        current.URL,
		Status:     current.Status,
		Config:     config,
	}
	if req.Name != nil {
		merged.Name = *req.Name
	}
	if req.SourceType != nil {
		merged.SourceType = *req.SourceType
	}
	if req.URL != nil {
		merged.URL = *req.URL
	}
	if req.Status != nil {
		merged.Status = *req.Status
	}
	if req.Config != nil {
		merged.Config = *req.Config
	}
	return merged
}

func normalizeSourceWriteRequest(req CreateSourceRequest) (normalizedSourceWriteRequest, error) {
	name := strings.TrimSpace(req.Name)
	url := strings.TrimSpace(req.URL)
	if name == "" || !validHTTPURL(url) {
		return normalizedSourceWriteRequest{}, ErrInvalidRequest
	}
	status, err := normalizeSourceStatus(req.Status)
	if err != nil {
		return normalizedSourceWriteRequest{}, err
	}
	config, err := normalizeSourceConfig(req.Config)
	if err != nil {
		return normalizedSourceWriteRequest{}, err
	}
	return normalizedSourceWriteRequest{
		Name:       name,
		SourceType: defaultString(req.SourceType, "其他"),
		URL:        url,
		Status:     status,
		Config:     config,
	}, nil
}

func scanSource(row scanner) (Source, error) {
	var source Source
	var lastVerifiedAt sql.NullTime
	var lastVerifyStatus sql.NullString
	var configRaw []byte
	err := row.Scan(
		&source.ID, &source.Name, &source.SourceType, &source.URL, &source.Status,
		&lastVerifiedAt, &lastVerifyStatus, &source.LastVerifyMessage, &configRaw, &source.CreatedAt, &source.UpdatedAt,
	)
	if lastVerifiedAt.Valid {
		source.LastVerifiedAt = &lastVerifiedAt.Time
	}
	if lastVerifyStatus.Valid {
		source.LastVerifyStatus = &lastVerifyStatus.String
	}
	source.Config = map[string]any{}
	if len(configRaw) > 0 {
		_ = json.Unmarshal(configRaw, &source.Config)
	}
	return source, err
}

func normalizeTenderStatus(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "open", "closed", "awarded", "cancelled":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeSourceStatus(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "active", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "inactive", "failed":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
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

func normalizeTenderRequiredText(value string, maxRunes int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrInvalidRequest
	}
	return value, validateTenderTextLength(value, maxRunes)
}

func normalizeTenderOptionalText(value string, maxRunes int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return value, validateTenderTextLength(value, maxRunes)
}

func validateTenderTextLength(value string, maxRunes int) error {
	if maxRunes <= 0 || len([]rune(strings.TrimSpace(value))) > maxRunes {
		return ErrInvalidRequest
	}
	return nil
}

func normalizeTenderTextList(values []string) ([]string, error) {
	if len(values) > maxTenderListItems {
		return nil, ErrInvalidRequest
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if err := validateTenderTextLength(value, maxTenderListItemRunes); err != nil {
			return nil, err
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeTenderMetadata(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return nil, ErrInvalidRequest
	}
	if len(typed) == 0 {
		return []byte("{}"), nil
	}
	if len(typed) > maxTenderMetadataEntries {
		return nil, ErrInvalidRequest
	}
	normalized := make(map[string]any, len(typed))
	for key, item := range typed {
		key = strings.TrimSpace(key)
		if key == "" || len([]rune(key)) > maxTenderMetadataKeyRunes {
			return nil, ErrInvalidRequest
		}
		normalized[key] = item
	}
	raw, err := json.Marshal(normalized)
	if err != nil || len(raw) > maxTenderMetadataJSONBytes {
		return nil, ErrInvalidRequest
	}
	return raw, nil
}

func validateOptionalTenderAmount(value *float64) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > maxTenderBudgetAmount || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return ErrInvalidRequest
	}
	return nil
}

func normalizeSourceConfig(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	if len(value) > maxSourceConfigEntries {
		return nil, ErrInvalidRequest
	}
	normalized := make(map[string]any, len(value))
	for key, item := range value {
		key = strings.TrimSpace(key)
		if key == "" || len([]rune(key)) > maxSourceConfigKeyRunes {
			return nil, ErrInvalidRequest
		}
		normalized[key] = item
	}
	raw, err := json.Marshal(normalized)
	if err != nil || len(raw) > maxSourceConfigJSONBytes {
		return nil, ErrInvalidRequest
	}
	return raw, nil
}

func clampScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	if parsed.User != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return false
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return publicNetIP(addr)
	}
	return strings.Contains(host, ".")
}

func normalizeOptionalSourceURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !validHTTPURL(value) {
		return "", ErrInvalidRequest
	}
	return value, nil
}

func newVerificationHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
				if err != nil {
					return nil, err
				}
				if len(ips) == 0 {
					return nil, ErrInvalidRequest
				}
				for _, ip := range ips {
					if !publicNetIP(ip) {
						return nil, ErrInvalidRequest
					}
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
			},
		},
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if !validHTTPURL(req.URL.String()) {
				return ErrInvalidRequest
			}
			return nil
		},
	}
}

func publicNetIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() ||
		!addr.IsGlobalUnicast() ||
		addr.IsPrivate() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return false
	}
	for _, prefix := range specialUseIPPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var specialUseIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func mapVerifyStatusToSourceStatus(value string) string {
	if value == "failed" {
		return "failed"
	}
	return "active"
}

func sourceVerifyOutcome(resp *http.Response, err error) (string, string) {
	if err != nil || resp == nil {
		return "failed", sourceVerifyUnavailableMessage
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "failed", sourceVerifyFailedMessage
	}
	return "ok", sourceVerifySuccessMessage
}
