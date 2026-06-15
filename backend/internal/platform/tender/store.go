package tender

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("tender resource not found")
	ErrInvalidRequest = errors.New("invalid tender request")
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

type UpdateTenderRequest = CreateTenderRequest

type CreateSourceRequest struct {
	Name       string         `json:"name"`
	SourceType string         `json:"source_type"`
	URL        string         `json:"url"`
	Status     string         `json:"status"`
	Config     map[string]any `json:"config"`
}

type UpdateSourceRequest = CreateSourceRequest

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
		if status := normalizeTenderStatus(filter.Status); status != "" {
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
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return Tender{}, ErrInvalidRequest
	}
	status := normalizeTenderStatus(req.Status)
	if status == "" {
		status = "open"
	}
	publishDate, err := parseOptionalDate(req.PublishDate)
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	deadline, err := parseOptionalDate(req.Deadline)
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	sourceID, err := nullableUUID(req.SourceID)
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	metadata, err := json.Marshal(normalizeMetadata(req.Metadata))
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	matchScore := clampScore(req.MatchScore)
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			insert into tenders (
				tenant_id, source_id, title, purchaser, region, budget_amount, budget_text, publish_date, deadline,
				status, match_score, summary, requirements, risk_flags, source_url, metadata
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			returning id::text
		`, tenantID, sourceID, title, strings.TrimSpace(req.Purchaser), strings.TrimSpace(req.Region), req.BudgetAmount,
			strings.TrimSpace(req.BudgetText), publishDate, deadline, status, matchScore, strings.TrimSpace(req.Summary),
			req.Requirements, req.RiskFlags, strings.TrimSpace(req.SourceURL), metadata).Scan(&id); err != nil {
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
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return Tender{}, ErrInvalidRequest
	}
	status := normalizeTenderStatus(req.Status)
	if status == "" {
		status = "open"
	}
	publishDate, err := parseOptionalDate(req.PublishDate)
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	deadline, err := parseOptionalDate(req.Deadline)
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	sourceID, err := nullableUUID(req.SourceID)
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	metadata, err := json.Marshal(normalizeMetadata(req.Metadata))
	if err != nil {
		return Tender{}, ErrInvalidRequest
	}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update tenders
			set source_id = $3, title = $4, purchaser = $5, region = $6, budget_amount = $7, budget_text = $8,
				publish_date = $9, deadline = $10, status = $11, match_score = $12, summary = $13,
				requirements = $14, risk_flags = $15, source_url = $16, metadata = $17, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, sourceID, title, strings.TrimSpace(req.Purchaser), strings.TrimSpace(req.Region), req.BudgetAmount,
			strings.TrimSpace(req.BudgetText), publishDate, deadline, status, clampScore(req.MatchScore), strings.TrimSpace(req.Summary),
			req.Requirements, req.RiskFlags, strings.TrimSpace(req.SourceURL), metadata)
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
	name := strings.TrimSpace(req.Name)
	url := strings.TrimSpace(req.URL)
	if name == "" || !validHTTPURL(url) {
		return Source{}, ErrInvalidRequest
	}
	status := normalizeSourceStatus(req.Status)
	config, _ := json.Marshal(req.Config)
	var id string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			insert into tender_sources (tenant_id, name, source_type, url, status, config)
			values ($1, $2, $3, $4, $5, $6)
			returning id::text
		`, tenantID, name, defaultString(req.SourceType, "其他"), url, status, config).Scan(&id)
	})
	if err != nil {
		return Source{}, err
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
	name := strings.TrimSpace(req.Name)
	url := strings.TrimSpace(req.URL)
	if name == "" || !validHTTPURL(url) {
		return Source{}, ErrInvalidRequest
	}
	config, _ := json.Marshal(req.Config)
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update tender_sources
			set name = $3, source_type = $4, url = $5, status = $6, config = $7, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, name, defaultString(req.SourceType, "其他"), url, normalizeSourceStatus(req.Status), config)
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
		return Source{}, err
	}
	return s.GetSource(ctx, tenantID, id)
}

func (s *Store) DeleteSource(ctx context.Context, tenantID, id string) error {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return ErrInvalidRequest
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
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
	status := "ok"
	message := ""
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
	if err != nil {
		status = "failed"
		message = err.Error()
	} else if resp == nil {
		status = "failed"
		message = "no response"
	} else {
		message = resp.Status
		if resp.StatusCode >= 400 {
			status = "failed"
		}
	}
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

func normalizeTenderStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "open", "closed", "awarded", "cancelled":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeSourceStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "inactive", "failed":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "active"
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

func normalizeMetadata(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok && typed != nil {
		return typed
	}
	return map[string]any{}
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
	return addr.IsValid() &&
		addr.IsGlobalUnicast() &&
		!addr.IsPrivate() &&
		!addr.IsLoopback() &&
		!addr.IsLinkLocalUnicast() &&
		!addr.IsLinkLocalMulticast() &&
		!addr.IsMulticast() &&
		!addr.IsUnspecified()
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
