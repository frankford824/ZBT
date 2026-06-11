package knowledge

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("knowledge resource not found")
	ErrInvalidRequest = errors.New("invalid knowledge request")
)

type Store struct {
	pool   *pgxpool.Pool
	cfg    config.Config
	client *http.Client
}

type Category struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ParentID    *string   `json:"parent_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FileInfo struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Status      string `json:"status"`
}

type Document struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	DocType     string         `json:"doc_type"`
	ParseStatus string         `json:"parse_status"`
	Summary     string         `json:"summary"`
	Metadata    map[string]any `json:"metadata"`
	File        FileInfo       `json:"file"`
	Category    *Category      `json:"category"`
	Tags        []Tag          `json:"tags"`
	ProcessedAt *time.Time     `json:"processed_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
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

type Stats struct {
	DocumentCount  int            `json:"document_count"`
	ReadyCount     int            `json:"ready_count"`
	QueuedCount    int            `json:"queued_count"`
	ProcessedCount int            `json:"processed_count"`
	FailedCount    int            `json:"failed_count"`
	CategoryCounts map[string]int `json:"category_counts"`
	TagCounts      map[string]int `json:"tag_counts"`
}

type UpdateDocumentRequest struct {
	Title      string   `json:"title"`
	DocType    string   `json:"doc_type"`
	CategoryID *string  `json:"category_id"`
	TagIDs     []string `json:"tag_ids"`
	Summary    string   `json:"summary"`
}

type CallbackPayload struct {
	TenantID       string         `json:"tenant_id"`
	TaskID         string         `json:"task_id"`
	Status         string         `json:"status"`
	Result         map[string]any `json:"result"`
	Chunks         []ChunkInput   `json:"chunks"`
	ErrorMessage   string         `json:"error_message"`
	ProcessedTitle string         `json:"processed_title"`
	Summary        string         `json:"summary"`
}

type ChunkInput struct {
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	SectionPath string         `json:"section_path"`
	PageStart   *int           `json:"page_start"`
	PageEnd     *int           `json:"page_end"`
	Metadata    map[string]any `json:"metadata"`
}

type SearchRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	DocType string `json:"doc_type"`
}

type SourceRef struct {
	ChunkID    string `json:"chunk_id"`
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	PageStart  *int   `json:"page_start"`
	PageEnd    *int   `json:"page_end"`
}

type SearchResult struct {
	ChunkID     string         `json:"chunk_id"`
	DocumentID  string         `json:"document_id"`
	Document    Document       `json:"document"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	SectionPath string         `json:"section_path"`
	PageStart   *int           `json:"page_start"`
	PageEnd     *int           `json:"page_end"`
	Metadata    map[string]any `json:"metadata"`
	Score       float64        `json:"score"`
	SourceRef   SourceRef      `json:"source_ref"`
	CreatedAt   time.Time      `json:"created_at"`
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

func (s *Store) ListCategories(ctx context.Context, tenantID string) ([]Category, error) {
	categories := []Category{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, name, description, parent_id::text, created_at, updated_at
			from knowledge_categories
			where tenant_id = $1
			order by name
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			category, err := scanCategory(rows)
			if err != nil {
				return err
			}
			categories = append(categories, category)
		}
		return rows.Err()
	})
	return categories, err
}

func (s *Store) CreateCategory(ctx context.Context, tenantID, name, description string) (Category, error) {
	var category Category
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanCategory(tx.QueryRow(ctx, `
			insert into knowledge_categories (tenant_id, name, description)
			values ($1, $2, $3)
			returning id::text, name, description, parent_id::text, created_at, updated_at
		`, tenantID, strings.TrimSpace(name), description))
		if err != nil {
			return err
		}
		category = created
		return nil
	})
	return category, err
}

func (s *Store) UpdateCategory(ctx context.Context, tenantID, id, name, description string) (Category, error) {
	var category Category
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		updated, err := scanCategory(tx.QueryRow(ctx, `
			update knowledge_categories
			set name = coalesce(nullif($3, ''), name),
				description = coalesce($4, description),
				updated_at = now()
			where tenant_id = $1 and id = $2
			returning id::text, name, description, parent_id::text, created_at, updated_at
		`, tenantID, id, strings.TrimSpace(name), description))
		if err != nil {
			return err
		}
		category = updated
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return category, err
}

func (s *Store) DeleteCategory(ctx context.Context, tenantID, id string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update knowledge_documents
			set category_id = null, updated_at = now()
			where tenant_id = $1 and category_id = $2
		`, tenantID, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `delete from knowledge_categories where tenant_id = $1 and id = $2`, tenantID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) ListTags(ctx context.Context, tenantID string) ([]Tag, error) {
	tags := []Tag{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, name, color, created_at, updated_at
			from knowledge_tags
			where tenant_id = $1
			order by name
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			tag, err := scanTag(rows)
			if err != nil {
				return err
			}
			tags = append(tags, tag)
		}
		return rows.Err()
	})
	return tags, err
}

func (s *Store) CreateTag(ctx context.Context, tenantID, name, color string) (Tag, error) {
	if color == "" {
		color = "blue"
	}
	var tag Tag
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanTag(tx.QueryRow(ctx, `
			insert into knowledge_tags (tenant_id, name, color)
			values ($1, $2, $3)
			returning id::text, name, color, created_at, updated_at
		`, tenantID, strings.TrimSpace(name), color))
		if err != nil {
			return err
		}
		tag = created
		return nil
	})
	return tag, err
}

func (s *Store) UpdateTag(ctx context.Context, tenantID, id, name, color string) (Tag, error) {
	var tag Tag
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		updated, err := scanTag(tx.QueryRow(ctx, `
			update knowledge_tags
			set name = coalesce(nullif($3, ''), name),
				color = coalesce(nullif($4, ''), color),
				updated_at = now()
			where tenant_id = $1 and id = $2
			returning id::text, name, color, created_at, updated_at
		`, tenantID, id, strings.TrimSpace(name), color))
		if err != nil {
			return err
		}
		tag = updated
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Tag{}, ErrNotFound
	}
	return tag, err
}

func (s *Store) DeleteTag(ctx context.Context, tenantID, id string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `delete from knowledge_tags where tenant_id = $1 and id = $2`, tenantID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) EnsureDocumentForFile(ctx context.Context, tenantID, fileID string) (Document, error) {
	var docID string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var filename, contentType string
		err := tx.QueryRow(ctx, `
			select filename, content_type
			from file_assets
			where tenant_id = $1 and id = $2 and status = 'ready'
		`, tenantID, fileID).Scan(&filename, &contentType)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		docType := inferDocType(filename, contentType)
		title := strings.TrimSuffix(filename, filepath.Ext(filename))
		if title == "" {
			title = filename
		}
		return tx.QueryRow(ctx, `
			insert into knowledge_documents (tenant_id, file_asset_id, title, doc_type, parse_status)
			values ($1, $2, $3, $4, 'ready')
			on conflict (tenant_id, file_asset_id) do update
			set updated_at = now()
			returning id::text
		`, tenantID, fileID, title, docType).Scan(&docID)
	})
	if err != nil {
		return Document{}, err
	}
	return s.GetDocument(ctx, tenantID, docID)
}

func (s *Store) ListDocuments(ctx context.Context, tenantID string) ([]Document, error) {
	documents := []Document{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, documentSelectSQL(`
			where d.tenant_id = $1
			group by d.id, fa.id, c.id
			order by d.created_at desc
			limit 100
		`), tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			document, err := scanDocument(rows)
			if err != nil {
				return err
			}
			documents = append(documents, document)
		}
		return rows.Err()
	})
	return documents, err
}

func (s *Store) GetDocument(ctx context.Context, tenantID, id string) (Document, error) {
	var document Document
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanDocument(tx.QueryRow(ctx, documentSelectSQL(`
			where d.tenant_id = $1 and d.id = $2
			group by d.id, fa.id, c.id
		`), tenantID, id))
		if err != nil {
			return err
		}
		document = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	return document, err
}

func (s *Store) UpdateDocument(ctx context.Context, tenantID, id string, req UpdateDocumentRequest) (Document, error) {
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update knowledge_documents
			set title = coalesce(nullif($3, ''), title),
				doc_type = coalesce(nullif($4, ''), doc_type),
				category_id = $5,
				summary = coalesce($6, summary),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id, strings.TrimSpace(req.Title), strings.TrimSpace(req.DocType), req.CategoryID, req.Summary)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if req.TagIDs != nil {
			if _, err := tx.Exec(ctx, `delete from knowledge_document_tags where tenant_id = $1 and document_id = $2`, tenantID, id); err != nil {
				return err
			}
			for _, tagID := range req.TagIDs {
				if _, err := tx.Exec(ctx, `
					insert into knowledge_document_tags (tenant_id, document_id, tag_id)
					values ($1, $2, $3)
					on conflict do nothing
				`, tenantID, id, tagID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return Document{}, err
	}
	return s.GetDocument(ctx, tenantID, id)
}

func (s *Store) DeleteDocument(ctx context.Context, tenantID, id string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `delete from knowledge_documents where tenant_id = $1 and id = $2`, tenantID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) ProcessDocument(ctx context.Context, tenantID, userID, id string) (Task, error) {
	var task Task
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var doc Document
		var objectKey string
		row := tx.QueryRow(ctx, `
			select
				d.id::text, d.title, d.doc_type, d.parse_status,
				fa.id::text, fa.object_key, fa.filename, fa.content_type
			from knowledge_documents d
			join file_assets fa on fa.id = d.file_asset_id and fa.tenant_id = d.tenant_id
			where d.tenant_id = $1 and d.id = $2 and fa.status = 'ready'
		`, tenantID, id)
		if err := row.Scan(&doc.ID, &doc.Title, &doc.DocType, &doc.ParseStatus, &doc.File.ID, &objectKey, &doc.File.Filename, &doc.File.ContentType); err != nil {
			return err
		}
		payload := map[string]any{
			"tenant_id":    tenantID,
			"document_id":  doc.ID,
			"file_id":      doc.File.ID,
			"object_key":   objectKey,
			"filename":     doc.File.Filename,
			"content_type": doc.File.ContentType,
			"callback_url": s.cfg.AICallbackURL,
		}
		accepted, err := s.submitKnowledgeProcess(ctx, payload)
		if err != nil {
			return err
		}
		payloadJSON, _ := json.Marshal(payload)
		routeJSON, _ := json.Marshal(accepted.Route)
		created, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status, external_task_id,
				resource_type, resource_id, payload, route
			)
			values ($1, $2, 'knowledge_process', $3, $4, 'knowledge_document', $5, $6, $7)
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, accepted.Status, accepted.TaskID, id, payloadJSON, routeJSON))
		if err != nil {
			return err
		}
		task = created
		if _, err := tx.Exec(ctx, `
			update knowledge_documents
			set parse_status = 'queued', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) GetTask(ctx context.Context, tenantID, taskID string) (Task, error) {
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

func (s *Store) GetTaskByExternalID(ctx context.Context, tenantID, externalTaskID string) (Task, error) {
	var task Task
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanTask(tx.QueryRow(ctx, `
			select id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
			from ai_tasks
			where tenant_id = $1 and external_task_id = $2
		`, tenantID, externalTaskID))
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

func (s *Store) ApplyCallback(ctx context.Context, payload CallbackPayload) (Task, error) {
	status := normalizeTaskStatus(payload.Status)
	if status == "" || payload.TenantID == "" || payload.TaskID == "" {
		return Task{}, ErrInvalidRequest
	}
	var task Task
	err := s.withTenant(ctx, payload.TenantID, func(tx pgx.Tx) error {
		resultJSON, _ := json.Marshal(payload.Result)
		found, err := scanTask(tx.QueryRow(ctx, `
			update ai_tasks
			set status = $3,
				result = $4,
				error_message = nullif($5, ''),
				started_at = coalesce(started_at, now()),
				completed_at = case when $3 in ('done', 'failed', 'cancelled') then now() else completed_at end,
				updated_at = now()
			where tenant_id = $1 and external_task_id = $2
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, payload.TenantID, payload.TaskID, status, resultJSON, payload.ErrorMessage))
		if err != nil {
			return err
		}
		task = found
		parseStatus := "processing"
		if status == "done" {
			parseStatus = "processed"
		}
		if status == "failed" || status == "cancelled" {
			parseStatus = "failed"
		}
		summary := payload.Summary
		if summary == "" && payload.Result != nil {
			if value, ok := payload.Result["summary"].(string); ok {
				summary = value
			}
		}
		if _, err := tx.Exec(ctx, `
			update knowledge_documents
			set parse_status = $3,
				title = coalesce(nullif($4, ''), title),
				summary = coalesce(nullif($5, ''), summary),
				processed_at = case when $3 = 'processed' then now() else processed_at end,
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, payload.TenantID, task.ResourceID, parseStatus, payload.ProcessedTitle, summary); err != nil {
			return err
		}
		if status == "done" {
			if err := replaceChunks(ctx, tx, payload.TenantID, task.ResourceID, payload.Chunks); err != nil {
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

func (s *Store) Stats(ctx context.Context, tenantID string) (Stats, error) {
	stats := Stats{
		CategoryCounts: map[string]int{},
		TagCounts:      map[string]int{},
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			select
				count(*),
				count(*) filter (where parse_status = 'ready'),
				count(*) filter (where parse_status = 'queued'),
				count(*) filter (where parse_status = 'processed'),
				count(*) filter (where parse_status = 'failed')
			from knowledge_documents
			where tenant_id = $1
		`, tenantID).Scan(&stats.DocumentCount, &stats.ReadyCount, &stats.QueuedCount, &stats.ProcessedCount, &stats.FailedCount); err != nil {
			return err
		}
		categoryRows, err := tx.Query(ctx, `
			select coalesce(c.name, '未分类'), count(d.id)
			from knowledge_documents d
			left join knowledge_categories c on c.id = d.category_id and c.tenant_id = d.tenant_id
			where d.tenant_id = $1
			group by c.name
			order by c.name
		`, tenantID)
		if err != nil {
			return err
		}
		defer categoryRows.Close()
		for categoryRows.Next() {
			var name string
			var count int
			if err := categoryRows.Scan(&name, &count); err != nil {
				return err
			}
			stats.CategoryCounts[name] = count
		}
		if err := categoryRows.Err(); err != nil {
			return err
		}
		tagRows, err := tx.Query(ctx, `
			select t.name, count(kdt.document_id)
			from knowledge_tags t
			left join knowledge_document_tags kdt on kdt.tag_id = t.id and kdt.tenant_id = t.tenant_id
			where t.tenant_id = $1
			group by t.name
			order by t.name
		`, tenantID)
		if err != nil {
			return err
		}
		defer tagRows.Close()
		for tagRows.Next() {
			var name string
			var count int
			if err := tagRows.Scan(&name, &count); err != nil {
				return err
			}
			stats.TagCounts[name] = count
		}
		return tagRows.Err()
	})
	return stats, err
}

func (s *Store) Search(ctx context.Context, tenantID string, req SearchRequest) ([]SearchResult, error) {
	query := strings.TrimSpace(req.Query)
	limit := req.Limit
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	results := []SearchResult{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			with ranked as (
				select
					kc.id::text as chunk_id,
					kc.document_id::text,
					kc.title,
					kc.content,
					kc.section_path,
					kc.page_start,
					kc.page_end,
					kc.metadata,
					kc.created_at,
					case
						when $2 = '' then 0
						else ts_rank(
							to_tsvector('simple', coalesce(kc.title, '') || ' ' || coalesce(kc.content, '') || ' ' || coalesce(kc.section_path, '')),
							plainto_tsquery('simple', $2)
						)
					end as rank_score
				from knowledge_chunks kc
				join knowledge_documents d on d.id = kc.document_id and d.tenant_id = kc.tenant_id
					where kc.tenant_id = $1
						and ($2 = ''
							or to_tsvector('simple', coalesce(kc.title, '') || ' ' || coalesce(kc.content, '') || ' ' || coalesce(kc.section_path, '')) @@ plainto_tsquery('simple', $2)
							or kc.title ilike '%' || $2 || '%'
							or kc.content ilike '%' || $2 || '%'
							or exists (
								select 1
								from unnest(regexp_split_to_array($2, '\s+')) as term(value)
								where term.value <> ''
									and (
										kc.title ilike '%' || term.value || '%'
										or kc.content ilike '%' || term.value || '%'
										or kc.section_path ilike '%' || term.value || '%'
									)
							))
					and ($4 = '' or d.doc_type = $4)
			)
			select
				r.chunk_id, r.document_id, r.title, r.content, r.section_path,
				r.page_start, r.page_end, r.metadata, r.rank_score, r.created_at,
				d.id::text, d.title, d.doc_type, d.parse_status, d.summary, d.metadata,
				d.processed_at, d.created_at, d.updated_at,
				fa.id::text, fa.filename, fa.content_type, fa.size_bytes, fa.status,
				c.id::text, c.name, c.description, c.parent_id::text, c.created_at, c.updated_at,
				coalesce(jsonb_agg(
					jsonb_build_object(
						'id', t.id::text,
						'name', t.name,
						'color', t.color,
						'created_at', t.created_at,
						'updated_at', t.updated_at
					)
				) filter (where t.id is not null), '[]'::jsonb)
			from ranked r
			join knowledge_documents d on d.id::text = r.document_id
			join file_assets fa on fa.id = d.file_asset_id and fa.tenant_id = d.tenant_id
			left join knowledge_categories c on c.id = d.category_id and c.tenant_id = d.tenant_id
			left join knowledge_document_tags kdt on kdt.document_id = d.id and kdt.tenant_id = d.tenant_id
			left join knowledge_tags t on t.id = kdt.tag_id and t.tenant_id = d.tenant_id
			group by r.chunk_id, r.document_id, r.title, r.content, r.section_path,
				r.page_start, r.page_end, r.metadata, r.rank_score, r.created_at,
				d.id, fa.id, c.id
			order by r.rank_score desc, r.created_at desc
			limit $3
		`, tenantID, query, limit, strings.TrimSpace(req.DocType))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			result, err := scanSearchResult(rows)
			if err != nil {
				return err
			}
			results = append(results, result)
		}
		return rows.Err()
	})
	return results, err
}

func (s *Store) submitKnowledgeProcess(ctx context.Context, payload map[string]any) (aiTaskAccepted, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/knowledge-process"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return aiTaskAccepted{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aiTaskAccepted{}, fmt.Errorf("ai service returned %s", resp.Status)
	}
	var accepted aiTaskAccepted
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		return aiTaskAccepted{}, err
	}
	if accepted.TaskID == "" {
		return aiTaskAccepted{}, ErrInvalidRequest
	}
	if accepted.Status == "" {
		accepted.Status = "queued"
	}
	if accepted.Route == nil {
		accepted.Route = map[string]any{}
	}
	return accepted, nil
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

func replaceChunks(ctx context.Context, tx pgx.Tx, tenantID, documentID string, chunks []ChunkInput) error {
	if _, err := tx.Exec(ctx, `delete from knowledge_chunks where tenant_id = $1 and document_id = $2`, tenantID, documentID); err != nil {
		return err
	}
	for index, chunk := range chunks {
		content := strings.TrimSpace(chunk.Content)
		if content == "" {
			continue
		}
		title := strings.TrimSpace(chunk.Title)
		if title == "" {
			title = fmt.Sprintf("chunk-%03d", index+1)
		}
		sectionPath := strings.TrimSpace(chunk.SectionPath)
		if sectionPath == "" {
			sectionPath = title
		}
		metadata := chunk.Metadata
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["chunk_index"] = index
		metadataJSON, _ := json.Marshal(metadata)
		if _, err := tx.Exec(ctx, `
			insert into knowledge_chunks (
				tenant_id, document_id, title, content, section_path,
				page_start, page_end, metadata
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
		`, tenantID, documentID, title, content, sectionPath, chunk.PageStart, chunk.PageEnd, metadataJSON); err != nil {
			return err
		}
	}
	return nil
}

func documentSelectSQL(suffix string) string {
	return `
		select
			d.id::text, d.title, d.doc_type, d.parse_status, d.summary, d.metadata,
			d.processed_at, d.created_at, d.updated_at,
			fa.id::text, fa.filename, fa.content_type, fa.size_bytes, fa.status,
			c.id::text, c.name, c.description, c.parent_id::text, c.created_at, c.updated_at,
			coalesce(jsonb_agg(
				jsonb_build_object(
					'id', t.id::text,
					'name', t.name,
					'color', t.color,
					'created_at', t.created_at,
					'updated_at', t.updated_at
				)
			) filter (where t.id is not null), '[]'::jsonb)
		from knowledge_documents d
		join file_assets fa on fa.id = d.file_asset_id and fa.tenant_id = d.tenant_id
		left join knowledge_categories c on c.id = d.category_id and c.tenant_id = d.tenant_id
		left join knowledge_document_tags kdt on kdt.document_id = d.id and kdt.tenant_id = d.tenant_id
		left join knowledge_tags t on t.id = kdt.tag_id and t.tenant_id = d.tenant_id
	` + suffix
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCategory(row scanner) (Category, error) {
	var category Category
	var parentID sql.NullString
	err := row.Scan(&category.ID, &category.Name, &category.Description, &parentID, &category.CreatedAt, &category.UpdatedAt)
	if parentID.Valid {
		category.ParentID = &parentID.String
	}
	return category, err
}

func scanTag(row scanner) (Tag, error) {
	var tag Tag
	err := row.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt)
	return tag, err
}

func scanDocument(row scanner) (Document, error) {
	var document Document
	var metadataRaw []byte
	var processedAt sql.NullTime
	var categoryID, categoryName, categoryDescription, categoryParentID sql.NullString
	var categoryCreatedAt, categoryUpdatedAt sql.NullTime
	var tagsRaw []byte
	err := row.Scan(
		&document.ID, &document.Title, &document.DocType, &document.ParseStatus, &document.Summary, &metadataRaw,
		&processedAt, &document.CreatedAt, &document.UpdatedAt,
		&document.File.ID, &document.File.Filename, &document.File.ContentType, &document.File.SizeBytes, &document.File.Status,
		&categoryID, &categoryName, &categoryDescription, &categoryParentID, &categoryCreatedAt, &categoryUpdatedAt,
		&tagsRaw,
	)
	if err != nil {
		return Document{}, err
	}
	document.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &document.Metadata)
	}
	if processedAt.Valid {
		document.ProcessedAt = &processedAt.Time
	}
	if categoryID.Valid {
		category := Category{
			ID:          categoryID.String,
			Name:        categoryName.String,
			Description: categoryDescription.String,
		}
		if categoryParentID.Valid {
			category.ParentID = &categoryParentID.String
		}
		if categoryCreatedAt.Valid {
			category.CreatedAt = categoryCreatedAt.Time
		}
		if categoryUpdatedAt.Valid {
			category.UpdatedAt = categoryUpdatedAt.Time
		}
		document.Category = &category
	}
	document.Tags = []Tag{}
	if len(tagsRaw) > 0 {
		_ = json.Unmarshal(tagsRaw, &document.Tags)
	}
	return document, nil
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
	_ = json.Unmarshal(payloadRaw, &task.Payload)
	_ = json.Unmarshal(routeRaw, &task.Route)
	_ = json.Unmarshal(resultRaw, &task.Result)
	return task, nil
}

func scanSearchResult(row scanner) (SearchResult, error) {
	var result SearchResult
	var pageStart, pageEnd sql.NullInt32
	var metadataRaw []byte
	var document Document
	var documentMetadataRaw, tagsRaw []byte
	var processedAt sql.NullTime
	var categoryID, categoryName, categoryDescription, categoryParentID sql.NullString
	var categoryCreatedAt, categoryUpdatedAt sql.NullTime
	err := row.Scan(
		&result.ChunkID, &result.DocumentID, &result.Title, &result.Content, &result.SectionPath,
		&pageStart, &pageEnd, &metadataRaw, &result.Score, &result.CreatedAt,
		&document.ID, &document.Title, &document.DocType, &document.ParseStatus, &document.Summary, &documentMetadataRaw,
		&processedAt, &document.CreatedAt, &document.UpdatedAt,
		&document.File.ID, &document.File.Filename, &document.File.ContentType, &document.File.SizeBytes, &document.File.Status,
		&categoryID, &categoryName, &categoryDescription, &categoryParentID, &categoryCreatedAt, &categoryUpdatedAt,
		&tagsRaw,
	)
	if err != nil {
		return SearchResult{}, err
	}
	if pageStart.Valid {
		value := int(pageStart.Int32)
		result.PageStart = &value
	}
	if pageEnd.Valid {
		value := int(pageEnd.Int32)
		result.PageEnd = &value
	}
	result.Metadata = map[string]any{}
	_ = json.Unmarshal(metadataRaw, &result.Metadata)
	document.Metadata = map[string]any{}
	_ = json.Unmarshal(documentMetadataRaw, &document.Metadata)
	if processedAt.Valid {
		document.ProcessedAt = &processedAt.Time
	}
	if categoryID.Valid {
		category := Category{
			ID:          categoryID.String,
			Name:        categoryName.String,
			Description: categoryDescription.String,
		}
		if categoryParentID.Valid {
			category.ParentID = &categoryParentID.String
		}
		if categoryCreatedAt.Valid {
			category.CreatedAt = categoryCreatedAt.Time
		}
		if categoryUpdatedAt.Valid {
			category.UpdatedAt = categoryUpdatedAt.Time
		}
		document.Category = &category
	}
	document.Tags = []Tag{}
	_ = json.Unmarshal(tagsRaw, &document.Tags)
	result.Document = document
	result.SourceRef = SourceRef{
		ChunkID:    result.ChunkID,
		DocumentID: result.DocumentID,
		Title:      result.Title,
		PageStart:  result.PageStart,
		PageEnd:    result.PageEnd,
	}
	return result, nil
}

func inferDocType(filename, contentType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case strings.Contains(contentType, "pdf") || ext == ".pdf":
		return "pdf"
	case strings.Contains(contentType, "word") || ext == ".doc" || ext == ".docx":
		return "word"
	case strings.Contains(contentType, "excel") || ext == ".xls" || ext == ".xlsx":
		return "spreadsheet"
	case strings.Contains(contentType, "presentation") || ext == ".ppt" || ext == ".pptx":
		return "presentation"
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	default:
		return "general"
	}
}

func normalizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "done", "failed", "cancelled":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

func NewID() string {
	return uuid.NewString()
}
