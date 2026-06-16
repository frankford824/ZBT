package knowledge

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/frankford824/ZBT/backend/internal/platform/aicall"
	"github.com/frankford824/ZBT/backend/internal/platform/aihttp"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	"github.com/frankford824/ZBT/backend/internal/platform/taskstatus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("knowledge resource not found")
	ErrInvalidRequest = errors.New("invalid knowledge request")
)

const knowledgeProcessSubmitFailureMessage = "知识库文档整理启动失败，请稍后重试"

const (
	maxKnowledgeCallbackChunks     = 300
	maxKnowledgeChunkContentChars  = 6000
	maxKnowledgeChunkTitleChars    = 300
	maxKnowledgeChunkSectionChars  = 500
	maxKnowledgeChunkMetadataBytes = 32 * 1024
)

type Store struct {
	pool     *pgxpool.Pool
	cfg      config.Config
	client   *http.Client
	aiLogger *aicall.Store
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
	Embedding   []float64      `json:"embedding"`
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

type DocumentReference struct {
	ID               string         `json:"id"`
	SourceDocumentID string         `json:"source_document_id"`
	BidDocumentID    *string        `json:"bid_document_id"`
	BidTitle         string         `json:"bid_title"`
	ChapterID        *string        `json:"chapter_id"`
	ChapterTitle     string         `json:"chapter_title"`
	ChunkID          *string        `json:"chunk_id"`
	Title            string         `json:"title"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
}

type DocumentTemplate struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Content     map[string]any `json:"content"`
	UsageCount  int            `json:"usage_count"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type CreateDocumentTemplateRequest struct {
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Content     map[string]any `json:"content"`
}

type aiTaskAccepted struct {
	TaskID string         `json:"task_id"`
	Status string         `json:"status"`
	Route  map[string]any `json:"route"`
}

type embeddingRequest struct {
	TenantID string   `json:"tenant_id"`
	Texts    []string `json:"texts"`
}

type embeddingResponse struct {
	Provider   string         `json:"provider"`
	Model      string         `json:"model"`
	Dimensions int            `json:"dimensions"`
	Embeddings [][]float64    `json:"embeddings"`
	Route      map[string]any `json:"route"`
}

type rerankDocument struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	SectionPath string  `json:"section_path"`
	Score       float64 `json:"score"`
}

type rerankRequest struct {
	TenantID  string           `json:"tenant_id"`
	Query     string           `json:"query"`
	Documents []rerankDocument `json:"documents"`
	TopK      int              `json:"top_k"`
}

type rerankResult struct {
	ID    string  `json:"id"`
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

type rerankResponse struct {
	Provider string         `json:"provider"`
	Model    string         `json:"model"`
	Results  []rerankResult `json:"results"`
	Route    map[string]any `json:"route"`
}

func NewStore(cfg config.Config, pool *pgxpool.Pool, aiLogger *aicall.Store) *Store {
	return &Store{
		pool:     pool,
		cfg:      cfg,
		aiLogger: aiLogger,
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
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, ErrInvalidRequest
	}
	var category Category
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanCategory(tx.QueryRow(ctx, `
			insert into knowledge_categories (tenant_id, name, description)
			values ($1, $2, $3)
			on conflict (tenant_id, name) do update
			set description = knowledge_categories.description
			returning id::text, name, description, parent_id::text, created_at, updated_at
		`, tenantID, name, description))
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
			update knowledge_categories
			set parent_id = null, updated_at = now()
			where tenant_id = $1 and parent_id = $2
		`, tenantID, id); err != nil {
			return err
		}
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
	name = strings.TrimSpace(name)
	if name == "" {
		return Tag{}, ErrInvalidRequest
	}
	if color == "" {
		color = "blue"
	}
	var tag Tag
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanTag(tx.QueryRow(ctx, `
			insert into knowledge_tags (tenant_id, name, color)
			values ($1, $2, $3)
			on conflict (tenant_id, name) do update
			set color = knowledge_tags.color
			returning id::text, name, color, created_at, updated_at
		`, tenantID, name, color))
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
		if _, err := tx.Exec(ctx, `
			update knowledge_references
			set source_document_id = null, updated_at = now()
			where tenant_id = $1 and source_document_id = $2
		`, tenantID, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `delete from knowledge_chunks where tenant_id = $1 and document_id = $2`, tenantID, id); err != nil {
			return err
		}
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

func (s *Store) DocumentReferences(ctx context.Context, tenantID, id string) ([]DocumentReference, error) {
	references := []DocumentReference{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `
			select exists(select 1 from knowledge_documents where tenant_id = $1 and id = $2)
		`, tenantID, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		rows, err := tx.Query(ctx, `
			select
				kr.id::text,
				kr.source_document_id::text,
				kr.bid_document_id::text,
				coalesce(bd.title, ''),
				kr.chapter_id::text,
				coalesce(bc.title, ''),
				kr.chunk_id::text,
				kr.title,
				kr.metadata,
				kr.created_at
			from knowledge_references kr
			left join bid_documents bd on bd.tenant_id = kr.tenant_id and bd.id = kr.bid_document_id
			left join bid_chapters bc on bc.tenant_id = kr.tenant_id and bc.id = kr.chapter_id
			where kr.tenant_id = $1 and kr.source_document_id = $2
			order by kr.created_at desc
			limit 100
		`, tenantID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			reference, err := scanDocumentReference(rows)
			if err != nil {
				return err
			}
			references = append(references, reference)
		}
		return rows.Err()
	})
	return references, err
}

func (s *Store) ListDocumentTemplates(ctx context.Context, tenantID string) ([]DocumentTemplate, error) {
	templates := []DocumentTemplate{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, name, category, description, version, content, usage_count, status, created_at, updated_at
			from document_templates
			where tenant_id = $1 and status = 'active'
			order by created_at desc
			limit 100
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			template, err := scanDocumentTemplate(rows)
			if err != nil {
				return err
			}
			templates = append(templates, template)
		}
		return rows.Err()
	})
	return templates, err
}

func (s *Store) CreateDocumentTemplate(ctx context.Context, tenantID string, req CreateDocumentTemplateRequest) (DocumentTemplate, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return DocumentTemplate{}, ErrInvalidRequest
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "通用模板"
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = "v1.0"
	}
	content := req.Content
	if content == nil {
		content = map[string]any{}
	}
	contentJSON, _ := json.Marshal(content)
	var template DocumentTemplate
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanDocumentTemplate(tx.QueryRow(ctx, `
			insert into document_templates (
				tenant_id, name, category, description, version, content
			)
			values ($1, $2, $3, $4, $5, $6)
			on conflict (tenant_id, name, version) do update
			set content = document_templates.content
			returning id::text, name, category, description, version, content, usage_count, status, created_at, updated_at
		`, tenantID, name, category, req.Description, version, contentJSON))
		if err != nil {
			return err
		}
		template = created
		return nil
	})
	return template, err
}

func (s *Store) ProcessDocument(ctx context.Context, tenantID, userID, id string) (Task, error) {
	var task Task
	var payload map[string]any
	externalTaskID := "task-knowledge-" + uuid.NewString()
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
		payload = map[string]any{
			"task_id":      externalTaskID,
			"tenant_id":    tenantID,
			"document_id":  doc.ID,
			"file_id":      doc.File.ID,
			"object_key":   objectKey,
			"filename":     doc.File.Filename,
			"content_type": doc.File.ContentType,
			"callback_url": s.cfg.AICallbackURL,
		}
		payloadJSON, _ := json.Marshal(payload)
		created, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status, external_task_id,
				resource_type, resource_id, payload, route
			)
			values ($1, $2, 'knowledge_process', 'queued', $3, 'knowledge_document', $4, $5, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, externalTaskID, id, payloadJSON))
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
	if err != nil {
		return Task{}, err
	}
	accepted, err := s.submitKnowledgeProcess(ctx, payload)
	if err != nil {
		_ = s.markKnowledgeProcessFailed(ctx, tenantID, task.ID, task.ResourceID, knowledgeProcessSubmitFailureMessage)
		return Task{}, err
	}
	if err := s.applyAcceptedKnowledgeTask(ctx, tenantID, task.ID, accepted); err != nil {
		return Task{}, err
	}
	return s.GetTask(ctx, tenantID, task.ID)
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
		current, err := lockTaskByExternalID(ctx, tx, payload.TenantID, payload.TaskID)
		if err != nil {
			return err
		}
		task = current
		if !taskstatus.ShouldApplyCallback(current.Status, status) {
			return nil
		}
		resultJSON, _ := json.Marshal(payload.Result)
		updated, err := scanTask(tx.QueryRow(ctx, `
			update ai_tasks
			set status = $3,
				result = $4,
				error_message = nullif($5, ''),
				started_at = coalesce(started_at, now()),
				completed_at = case when $3 in ('done', 'failed', 'cancelled') then now() else completed_at end,
				updated_at = now()
			where tenant_id = $1 and id = $2
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, payload.TenantID, current.ID, status, resultJSON, payload.ErrorMessage))
		if err != nil {
			return err
		}
		task = updated
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

func (s *Store) Search(ctx context.Context, tenantID, userID string, req SearchRequest) ([]SearchResult, error) {
	query := strings.TrimSpace(req.Query)
	limit := req.Limit
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	candidateLimit := searchCandidateLimit(limit)
	queryEmbedding := ""
	if query != "" {
		if response, err := s.embedKnowledgeTexts(ctx, tenantID, userID, []string{query}); err == nil && len(response.Embeddings) > 0 {
			if literal, err := vectorLiteralFromEmbedding(response.Embeddings[0]); err == nil {
				queryEmbedding = literal
			}
		} else if err != nil && strings.Contains(err.Error(), "record ai call log") {
			return nil, err
		}
	}
	results := []SearchResult{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			with query_vector as (
				select nullif($5, '')::vector as embedding
			), vector_hits as (
				select
					kc.id::text as chunk_id,
					row_number() over (order by kc.embedding <=> qv.embedding, kc.created_at desc) as rank
				from knowledge_chunks kc
				join knowledge_documents d on d.id = kc.document_id and d.tenant_id = kc.tenant_id
				cross join query_vector qv
				where kc.tenant_id = $1
					and qv.embedding is not null
					and kc.embedding is not null
					and ($4 = '' or d.doc_type = $4)
				limit $3
			), keyword_scored as (
				select
					kc.id::text as chunk_id,
					kc.created_at,
					case
						when $2 = '' then 0
						else ts_rank(
							to_tsvector('simple', coalesce(kc.title, '') || ' ' || coalesce(kc.content, '') || ' ' || coalesce(kc.section_path, '')),
							plainto_tsquery('simple', $2)
						)
					end
					+ case
						when $2 <> '' and (
							kc.title ilike '%' || $2 || '%'
							or kc.content ilike '%' || $2 || '%'
							or kc.section_path ilike '%' || $2 || '%'
						) then 0.35
						else 0
					end
					+ case
						when $2 <> '' and exists (
							select 1
							from unnest(regexp_split_to_array($2, '\s+')) as term(value)
							where term.value <> ''
								and (
									kc.title ilike '%' || term.value || '%'
									or kc.content ilike '%' || term.value || '%'
									or kc.section_path ilike '%' || term.value || '%'
								)
						) then 0.2
						else 0
					end
					as score
				from knowledge_chunks kc
				join knowledge_documents d on d.id = kc.document_id and d.tenant_id = kc.tenant_id
				where kc.tenant_id = $1
					and $2 <> ''
					and (
						to_tsvector('simple', coalesce(kc.title, '') || ' ' || coalesce(kc.content, '') || ' ' || coalesce(kc.section_path, '')) @@ plainto_tsquery('simple', $2)
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
						)
					)
					and ($4 = '' or d.doc_type = $4)
			), keyword_hits as (
				select
					chunk_id,
					row_number() over (order by score desc, created_at desc) as rank
				from keyword_scored
				where score > 0
				limit $3
			), recent_hits as (
				select
					kc.id::text as chunk_id,
					row_number() over (order by kc.created_at desc) as rank
				from knowledge_chunks kc
				join knowledge_documents d on d.id = kc.document_id and d.tenant_id = kc.tenant_id
				where kc.tenant_id = $1
					and $2 = ''
					and ($4 = '' or d.doc_type = $4)
				limit $3
			), fused as (
				select chunk_id, sum(rrf_score) as score
				from (
					select chunk_id, 1.0 / (60 + rank) as rrf_score from vector_hits
					union all
					select chunk_id, 1.0 / (60 + rank) as rrf_score from keyword_hits
					union all
					select chunk_id, 1.0 / (60 + rank) as rrf_score from recent_hits
				) hits
				group by chunk_id
			), ranked as (
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
					f.score
				from fused f
				join knowledge_chunks kc on kc.id::text = f.chunk_id
				join knowledge_documents d on d.id = kc.document_id and d.tenant_id = kc.tenant_id
				where kc.tenant_id = $1
					and ($4 = '' or d.doc_type = $4)
				order by f.score desc, kc.created_at desc
				limit $3
			)
			select
				r.chunk_id, r.document_id, r.title, r.content, r.section_path,
				r.page_start, r.page_end, r.metadata, r.score, r.created_at,
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
				r.page_start, r.page_end, r.metadata, r.score, r.created_at,
				d.id, fa.id, c.id
			order by r.score desc, r.created_at desc
			limit $3
		`, tenantID, query, candidateLimit, strings.TrimSpace(req.DocType), queryEmbedding)
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
	if err != nil {
		return nil, err
	}
	if query != "" && len(results) > 1 {
		reranked, err := s.rerankKnowledgeResults(ctx, tenantID, userID, query, results, limit)
		if err == nil && len(reranked) > 0 {
			return reranked, nil
		}
		if err != nil && strings.Contains(err.Error(), "record ai call log") {
			return nil, err
		}
	}
	return limitSearchResults(results, limit), nil
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

func (s *Store) applyAcceptedKnowledgeTask(ctx context.Context, tenantID, taskID string, accepted aiTaskAccepted) error {
	status := normalizeTaskStatus(accepted.Status)
	if status == "" {
		status = "queued"
	}
	routeJSON, _ := json.Marshal(accepted.Route)
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update ai_tasks
			set status = case when status in ('done', 'failed', 'cancelled') then status else $3 end,
				external_task_id = $4,
				route = $5,
				started_at = coalesce(started_at, now()),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, status, accepted.TaskID, routeJSON)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
}

func (s *Store) markKnowledgeProcessFailed(ctx context.Context, tenantID, taskID, documentID, message string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update ai_tasks
			set status = 'failed',
				error_message = $3,
				completed_at = now(),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, message); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			update knowledge_documents
			set parse_status = 'failed',
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, documentID)
		return err
	})
}

func (s *Store) embedKnowledgeTexts(ctx context.Context, tenantID, userID string, texts []string) (embeddingResponse, error) {
	if len(texts) == 0 {
		return embeddingResponse{}, ErrInvalidRequest
	}
	startedAt := time.Now()
	body, err := json.Marshal(embeddingRequest{
		TenantID: tenantID,
		Texts:    texts,
	})
	if err != nil {
		return embeddingResponse{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/embeddings/knowledge"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return embeddingResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	aihttp.Sign(req, body, s.cfg.AIServiceHMACSecret)
	resp, err := s.client.Do(req)
	if err != nil {
		return embeddingResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return embeddingResponse{}, fmt.Errorf("ai service embedding returned %s", resp.Status)
	}
	var decoded embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return embeddingResponse{}, err
	}
	if len(decoded.Embeddings) != len(texts) {
		return embeddingResponse{}, fmt.Errorf("ai service embedding count mismatch: got %d want %d", len(decoded.Embeddings), len(texts))
	}
	if s.aiLogger != nil {
		if _, err := s.aiLogger.Record(ctx, aicall.RecordInput{
			TenantID:    tenantID,
			UserID:      userID,
			TraceID:     fmt.Sprintf("knowledge-embedding-%d", time.Now().UnixNano()),
			TaskType:    "knowledge_embedding",
			Provider:    decoded.Provider,
			Model:       decoded.Model,
			InputTokens: estimateTokens(strings.Join(texts, "\n")),
			LatencyMS:   int(time.Since(startedAt).Milliseconds()),
			Status:      "done",
			BizRef: map[string]any{
				"endpoint":   "/embeddings/knowledge",
				"text_count": len(texts),
				"dimensions": decoded.Dimensions,
			},
		}); err != nil {
			return embeddingResponse{}, fmt.Errorf("record ai call log: %w", err)
		}
	}
	return decoded, nil
}

func (s *Store) rerankKnowledgeResults(ctx context.Context, tenantID, userID, query string, candidates []SearchResult, limit int) ([]SearchResult, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	startedAt := time.Now()
	documents := make([]rerankDocument, 0, len(candidates))
	for _, candidate := range candidates {
		documents = append(documents, rerankDocument{
			ID:          candidate.ChunkID,
			Title:       candidate.Title,
			Content:     truncateForRerank(candidate.Content, 2400),
			SectionPath: candidate.SectionPath,
			Score:       candidate.Score,
		})
	}
	body, err := json.Marshal(rerankRequest{
		TenantID:  tenantID,
		Query:     query,
		Documents: documents,
		TopK:      limit,
	})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/rerank/knowledge"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	aihttp.Sign(req, body, s.cfg.AIServiceHMACSecret)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai service rerank returned %s", resp.Status)
	}
	var decoded rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if s.aiLogger != nil {
		if _, err := s.aiLogger.Record(ctx, aicall.RecordInput{
			TenantID:     tenantID,
			UserID:       userID,
			TraceID:      fmt.Sprintf("knowledge-rerank-%d", time.Now().UnixNano()),
			TaskType:     "knowledge_rerank",
			Provider:     decoded.Provider,
			Model:        decoded.Model,
			InputTokens:  estimateTokens(query) + estimateTokensForRerank(documents),
			OutputTokens: len(decoded.Results),
			LatencyMS:    int(time.Since(startedAt).Milliseconds()),
			Status:       "done",
			BizRef: map[string]any{
				"endpoint":        "/rerank/knowledge",
				"candidate_count": len(documents),
				"result_count":    len(decoded.Results),
			},
		}); err != nil {
			return nil, fmt.Errorf("record ai call log: %w", err)
		}
	}
	return applyKnowledgeRerank(candidates, decoded.Results, limit), nil
}

func searchCandidateLimit(limit int) int {
	candidateLimit := limit * 4
	if candidateLimit < 30 {
		candidateLimit = 30
	}
	if candidateLimit > 60 {
		candidateLimit = 60
	}
	return candidateLimit
}

func limitSearchResults(results []SearchResult, limit int) []SearchResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}
	return results[:limit]
}

func applyKnowledgeRerank(candidates []SearchResult, ranks []rerankResult, limit int) []SearchResult {
	if len(candidates) == 0 {
		return nil
	}
	byChunkID := make(map[string]SearchResult, len(candidates))
	for _, candidate := range candidates {
		byChunkID[candidate.ChunkID] = candidate
	}
	used := make(map[string]bool, len(candidates))
	reranked := make([]SearchResult, 0, min(limit, len(candidates)))
	for _, rank := range ranks {
		candidate, ok := byChunkID[rank.ID]
		if !ok || used[rank.ID] {
			continue
		}
		candidate.Score = rank.Score
		reranked = append(reranked, candidate)
		used[rank.ID] = true
		if len(reranked) >= limit {
			return reranked
		}
	}
	for _, candidate := range candidates {
		if used[candidate.ChunkID] {
			continue
		}
		reranked = append(reranked, candidate)
		if len(reranked) >= limit {
			break
		}
	}
	return reranked
}

func truncateForRerank(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes])
}

func estimateTokensForRerank(documents []rerankDocument) int {
	total := 0
	for _, document := range documents {
		total += estimateTokens(document.Title)
		total += estimateTokens(document.SectionPath)
		total += estimateTokens(document.Content)
	}
	return total
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
	if err := validateKnowledgeChunks(chunks); err != nil {
		return err
	}
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
		metadataJSON, err := knowledgeChunkMetadataJSON(chunk.Metadata, index)
		if err != nil {
			return err
		}
		embedding, err := vectorLiteralFromEmbedding(chunk.Embedding)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into knowledge_chunks (
				tenant_id, document_id, title, content, section_path,
				page_start, page_end, metadata, embedding
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, case when $9 = '' then null else $9::vector end)
		`, tenantID, documentID, title, content, sectionPath, chunk.PageStart, chunk.PageEnd, metadataJSON, embedding); err != nil {
			return err
		}
	}
	return nil
}

func validateKnowledgeChunks(chunks []ChunkInput) error {
	if len(chunks) == 0 || len(chunks) > maxKnowledgeCallbackChunks {
		return ErrInvalidRequest
	}
	usableChunks := 0
	for _, chunk := range chunks {
		content := strings.TrimSpace(chunk.Content)
		if content == "" {
			continue
		}
		usableChunks++
		if utf8.RuneCountInString(content) > maxKnowledgeChunkContentChars {
			return ErrInvalidRequest
		}
		if utf8.RuneCountInString(strings.TrimSpace(chunk.Title)) > maxKnowledgeChunkTitleChars {
			return ErrInvalidRequest
		}
		if utf8.RuneCountInString(strings.TrimSpace(chunk.SectionPath)) > maxKnowledgeChunkSectionChars {
			return ErrInvalidRequest
		}
	}
	if usableChunks == 0 {
		return ErrInvalidRequest
	}
	return nil
}

func knowledgeChunkMetadataJSON(metadata map[string]any, index int) ([]byte, error) {
	normalized := map[string]any{}
	for key, value := range metadata {
		normalized[key] = value
	}
	normalized["chunk_index"] = index
	metadataJSON, err := json.Marshal(normalized)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	if len(metadataJSON) > maxKnowledgeChunkMetadataBytes {
		return nil, ErrInvalidRequest
	}
	return metadataJSON, nil
}

func vectorLiteralFromEmbedding(values []float64) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	if len(values) != 1024 {
		return "", fmt.Errorf("embedding dimension mismatch: got %d want 1024", len(values))
	}
	var builder strings.Builder
	builder.Grow(len(values) * 10)
	builder.WriteByte('[')
	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return "", fmt.Errorf("embedding contains non-finite value at index %d", index)
		}
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	}
	builder.WriteByte(']')
	return builder.String(), nil
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

func lockTaskByExternalID(ctx context.Context, tx pgx.Tx, tenantID, externalTaskID string) (Task, error) {
	return scanTask(tx.QueryRow(ctx, `
		select id::text, task_type, status, external_task_id::text,
			resource_type, resource_id::text, payload, route, result, error_message,
			started_at, completed_at, created_at, updated_at
		from ai_tasks
		where tenant_id = $1 and external_task_id = $2
		for update
	`, tenantID, externalTaskID))
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

func scanDocumentReference(row scanner) (DocumentReference, error) {
	var reference DocumentReference
	var bidDocumentID, chapterID, chunkID sql.NullString
	var metadataRaw []byte
	err := row.Scan(
		&reference.ID,
		&reference.SourceDocumentID,
		&bidDocumentID,
		&reference.BidTitle,
		&chapterID,
		&reference.ChapterTitle,
		&chunkID,
		&reference.Title,
		&metadataRaw,
		&reference.CreatedAt,
	)
	if err != nil {
		return DocumentReference{}, err
	}
	if bidDocumentID.Valid {
		reference.BidDocumentID = &bidDocumentID.String
	}
	if chapterID.Valid {
		reference.ChapterID = &chapterID.String
	}
	if chunkID.Valid {
		reference.ChunkID = &chunkID.String
	}
	reference.Metadata = map[string]any{}
	_ = json.Unmarshal(metadataRaw, &reference.Metadata)
	return reference, nil
}

func scanDocumentTemplate(row scanner) (DocumentTemplate, error) {
	var template DocumentTemplate
	var contentRaw []byte
	err := row.Scan(
		&template.ID,
		&template.Name,
		&template.Category,
		&template.Description,
		&template.Version,
		&contentRaw,
		&template.UsageCount,
		&template.Status,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return DocumentTemplate{}, err
	}
	template.Content = map[string]any{}
	_ = json.Unmarshal(contentRaw, &template.Content)
	return template, nil
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

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	count := len([]rune(text)) / 4
	if count < 1 {
		return 1
	}
	return count
}

func NewID() string {
	return uuid.NewString()
}
