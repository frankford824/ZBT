package bid

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/config"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("bid resource not found")
	ErrInvalidRequest = errors.New("invalid bid request")
)

const (
	docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	zipContentType  = "application/zip"
)

type Store struct {
	pool   *pgxpool.Pool
	cfg    config.Config
	client *http.Client
}

type Document struct {
	ID          string    `json:"id"`
	ProjectID   *string   `json:"project_id"`
	ProjectName string    `json:"project_name"`
	Title       string    `json:"title"`
	BidType     string    `json:"bid_type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Part struct {
	ID            string         `json:"id"`
	BidDocumentID string         `json:"bid_document_id"`
	Code          string         `json:"code"`
	Title         string         `json:"title"`
	SortOrder     int            `json:"sort_order"`
	Status        string         `json:"status"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type Chapter struct {
	ID              string         `json:"id"`
	BidDocumentID   string         `json:"bid_document_id"`
	BidPartID       string         `json:"bid_part_id"`
	Title           string         `json:"title"`
	Content         map[string]any `json:"content"`
	PlainText       string         `json:"plain_text"`
	Status          string         `json:"status"`
	SortOrder       int            `json:"sort_order"`
	SourceRefs      []any          `json:"source_refs"`
	NeedsHumanInput []string       `json:"needs_human_input"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type ChapterVersion struct {
	ID              string         `json:"id"`
	ChapterID       string         `json:"chapter_id"`
	BidDocumentID   string         `json:"bid_document_id"`
	BidPartID       string         `json:"bid_part_id"`
	VersionNo       int            `json:"version_no"`
	Title           string         `json:"title"`
	Content         map[string]any `json:"content"`
	PlainText       string         `json:"plain_text"`
	Status          string         `json:"status"`
	SourceRefs      []any          `json:"source_refs"`
	NeedsHumanInput []string       `json:"needs_human_input"`
	ChangeReason    string         `json:"change_reason"`
	ModelMetadata   map[string]any `json:"model_metadata"`
	TokenUsage      map[string]int `json:"token_usage"`
	CreatedBy       *string        `json:"created_by"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type Export struct {
	ID            string         `json:"id"`
	BidDocumentID string         `json:"bid_document_id"`
	BidPartID     *string        `json:"bid_part_id"`
	ExportType    string         `json:"export_type"`
	PartCode      string         `json:"part_code"`
	Status        string         `json:"status"`
	FileAssetID   *string        `json:"file_asset_id"`
	Filename      string         `json:"filename"`
	Metadata      map[string]any `json:"metadata"`
	ErrorMessage  *string        `json:"error_message"`
	CompletedAt   *time.Time     `json:"completed_at"`
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

type CreateDocumentRequest struct {
	Title       string `json:"title"`
	ProjectName string `json:"project_name"`
	BidType     string `json:"bid_type"`
}

type CreateExportRequest struct {
	ExportType string `json:"export_type"`
	PartCode   string `json:"part_code"`
}

type UpdateChapterContentRequest struct {
	Title     string         `json:"title"`
	Content   map[string]any `json:"content"`
	PlainText string         `json:"plain_text"`
}

type ChapterRegenerateResponse struct {
	Chapter    Chapter        `json:"chapter"`
	Version    ChapterVersion `json:"version"`
	Generation map[string]any `json:"generation"`
}

type ChapterDiff struct {
	Current  Chapter         `json:"current"`
	Previous *ChapterVersion `json:"previous"`
}

type CreateExportResponse struct {
	Export Export `json:"export"`
	Task   Task   `json:"task"`
}

type CallbackPayload struct {
	TenantID     string         `json:"tenant_id"`
	TaskID       string         `json:"task_id"`
	Status       string         `json:"status"`
	Result       map[string]any `json:"result"`
	ErrorMessage string         `json:"error_message"`
}

type exportChapterPayload struct {
	Title     string `json:"title"`
	PlainText string `json:"plain_text"`
}

type exportPartPayload struct {
	Code     string                 `json:"code"`
	Title    string                 `json:"title"`
	Chapters []exportChapterPayload `json:"chapters"`
}

type aiTaskAccepted struct {
	TaskID string         `json:"task_id"`
	Status string         `json:"status"`
	Route  map[string]any `json:"route"`
}

type chapterGenerateRequest struct {
	TenantID              string   `json:"tenant_id"`
	BidDocumentID         string   `json:"bid_document_id"`
	BidPartID             string   `json:"bid_part_id"`
	ChapterID             string   `json:"chapter_id"`
	ChapterTitle          string   `json:"chapter_title"`
	TenderRequirements    []string `json:"tender_requirements"`
	SelectedKnowledgeRefs []string `json:"selected_knowledge_refs"`
	ModelHint             *string  `json:"model_hint,omitempty"`
}

type sourceRef struct {
	ChunkID    string `json:"chunk_id"`
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	PageStart  *int   `json:"page_start"`
	PageEnd    *int   `json:"page_end"`
}

type chapterGenerateResponse struct {
	TraceID         string         `json:"trace_id"`
	TiptapJSON      map[string]any `json:"tiptap_json"`
	SourceRefs      []sourceRef    `json:"source_refs"`
	SelfCheck       map[string]any `json:"self_check"`
	NeedsHumanInput []string       `json:"needs_human_input"`
	ModelMetadata   map[string]any `json:"model_metadata"`
	TokenUsage      map[string]int `json:"token_usage"`
}

func NewStore(cfg config.Config, pool *pgxpool.Pool) *Store {
	return &Store{
		pool: pool,
		cfg:  cfg,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (s *Store) ListDocuments(ctx context.Context, tenantID string) ([]Document, error) {
	documents := []Document{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select bd.id::text, bd.project_id::text, coalesce(p.name, ''),
				bd.title, bd.bid_type, bd.status, bd.created_at, bd.updated_at
			from bid_documents bd
			left join projects p on p.id = bd.project_id and p.tenant_id = bd.tenant_id
			where bd.tenant_id = $1
			order by bd.created_at desc
			limit 100
		`, tenantID)
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
		found, err := scanDocument(tx.QueryRow(ctx, `
			select bd.id::text, bd.project_id::text, coalesce(p.name, ''),
				bd.title, bd.bid_type, bd.status, bd.created_at, bd.updated_at
			from bid_documents bd
			left join projects p on p.id = bd.project_id and p.tenant_id = bd.tenant_id
			where bd.tenant_id = $1 and bd.id = $2
		`, tenantID, id))
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

func (s *Store) CreateDocument(ctx context.Context, tenantID string, req CreateDocumentRequest) (Document, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(req.ProjectName)
	}
	if title == "" {
		return Document{}, ErrInvalidRequest
	}
	bidType := normalizeBidType(req.BidType)
	var id string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			insert into bid_documents (tenant_id, title, bid_type, status)
			values ($1, $2, $3, 'draft')
			returning id::text
		`, tenantID, title, bidType).Scan(&id); err != nil {
			return err
		}
		return createDefaultParts(ctx, tx, tenantID, id, bidType)
	})
	if err != nil {
		return Document{}, err
	}
	return s.GetDocument(ctx, tenantID, id)
}

func (s *Store) ListParts(ctx context.Context, tenantID, bidID string) ([]Part, error) {
	parts := []Part{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, bid_document_id::text, code, title, sort_order, status, metadata, created_at, updated_at
			from bid_parts
			where tenant_id = $1 and bid_document_id = $2
			order by sort_order, created_at
		`, tenantID, bidID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			part, err := scanPart(rows)
			if err != nil {
				return err
			}
			parts = append(parts, part)
		}
		return rows.Err()
	})
	return parts, err
}

func (s *Store) ListChapters(ctx context.Context, tenantID, bidID string) ([]Chapter, error) {
	chapters := []Chapter{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, bid_document_id::text, bid_part_id::text, title, content, plain_text,
				status, sort_order, source_refs, needs_human_input, created_at, updated_at
			from bid_chapters
			where tenant_id = $1 and bid_document_id = $2
			order by sort_order, created_at
		`, tenantID, bidID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			chapter, err := scanChapter(rows)
			if err != nil {
				return err
			}
			chapters = append(chapters, chapter)
		}
		return rows.Err()
	})
	return chapters, err
}

func (s *Store) UpdateChapterContent(ctx context.Context, tenantID, userID, chapterID string, req UpdateChapterContentRequest) (ChapterVersion, error) {
	var version ChapterVersion
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		title := strings.TrimSpace(req.Title)
		if title == "" {
			title = chapter.Title
		}
		content := req.Content
		plainText := strings.TrimSpace(req.PlainText)
		if content == nil {
			content = tiptapFromPlainText(plainText)
		}
		if plainText == "" {
			plainText = plainTextFromTiptap(content)
		}
		if plainText == "" {
			return ErrInvalidRequest
		}
		contentJSON, _ := json.Marshal(content)
		if _, err := tx.Exec(ctx, `
			update bid_chapters
			set title = $3,
				content = $4,
				plain_text = $5,
				status = 'edited',
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapterID, title, contentJSON, plainText); err != nil {
			return err
		}
		updated, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		created, err := insertChapterVersion(ctx, tx, tenantID, userID, updated, "manual_edit", nil, nil)
		if err != nil {
			return err
		}
		version = created
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterVersion{}, ErrNotFound
	}
	return version, err
}

func (s *Store) AcceptChapter(ctx context.Context, tenantID, userID, chapterID string) (ChapterVersion, error) {
	var version ChapterVersion
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update bid_chapters
			set status = 'accepted', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapterID); err != nil {
			return err
		}
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		created, err := insertChapterVersion(ctx, tx, tenantID, userID, chapter, "accepted", nil, nil)
		if err != nil {
			return err
		}
		version = created
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterVersion{}, ErrNotFound
	}
	return version, err
}

func (s *Store) RegenerateChapter(ctx context.Context, tenantID, userID, chapterID string) (ChapterRegenerateResponse, error) {
	var result ChapterRegenerateResponse
	var requestPayload chapterGenerateRequest
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		requestPayload = chapterGenerateRequest{
			TenantID:              tenantID,
			BidDocumentID:         chapter.BidDocumentID,
			BidPartID:             chapter.BidPartID,
			ChapterID:             chapter.ID,
			ChapterTitle:          chapter.Title,
			TenderRequirements:    []string{"响应招标文件要求", "保留事实性内容引用", "无来源内容标记人工确认"},
			SelectedKnowledgeRefs: []string{},
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterRegenerateResponse{}, ErrNotFound
	}
	if err != nil {
		return ChapterRegenerateResponse{}, err
	}

	generation, err := s.submitChapterGenerate(ctx, requestPayload)
	if err != nil {
		return ChapterRegenerateResponse{}, err
	}

	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		sourceRefs := sourceRefsAsAny(generation.SourceRefs)
		content := generation.TiptapJSON
		plainText := plainTextFromTiptap(content)
		if plainText == "" {
			plainText = chapter.PlainText
		}
		contentJSON, _ := json.Marshal(content)
		sourceRefsJSON, _ := json.Marshal(sourceRefs)
		needsHumanInputJSON, _ := json.Marshal(generation.NeedsHumanInput)
		if _, err := tx.Exec(ctx, `
			update bid_chapters
			set content = $3,
				plain_text = $4,
				status = 'generated',
				source_refs = $5,
				needs_human_input = $6,
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapterID, contentJSON, plainText, sourceRefsJSON, needsHumanInputJSON); err != nil {
			return err
		}
		updated, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		version, err := insertChapterVersion(ctx, tx, tenantID, userID, updated, "ai_regenerate", generation.ModelMetadata, generation.TokenUsage)
		if err != nil {
			return err
		}
		if err := replaceKnowledgeReferences(ctx, tx, tenantID, updated, generation.SourceRefs, generation.TraceID); err != nil {
			return err
		}
		result = ChapterRegenerateResponse{
			Chapter: updated,
			Version: version,
			Generation: map[string]any{
				"trace_id":          generation.TraceID,
				"self_check":        generation.SelfCheck,
				"model_metadata":    generation.ModelMetadata,
				"token_usage":       generation.TokenUsage,
				"source_refs":       sourceRefs,
				"needs_human_input": generation.NeedsHumanInput,
			},
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterRegenerateResponse{}, ErrNotFound
	}
	return result, err
}

func (s *Store) ListChapterVersions(ctx context.Context, tenantID, chapterID string) ([]ChapterVersion, error) {
	versions := []ChapterVersion{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, chapter_id::text, bid_document_id::text, bid_part_id::text,
				version_no, title, content, plain_text, status, source_refs, needs_human_input,
				change_reason, model_metadata, token_usage, created_by::text, created_at, updated_at
			from bid_chapter_versions
			where tenant_id = $1 and chapter_id = $2
			order by version_no desc
		`, tenantID, chapterID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			version, err := scanChapterVersion(rows)
			if err != nil {
				return err
			}
			versions = append(versions, version)
		}
		return rows.Err()
	})
	return versions, err
}

func (s *Store) ChapterDiff(ctx context.Context, tenantID, chapterID string) (ChapterDiff, error) {
	var diff ChapterDiff
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		diff.Current = chapter
		version, err := scanChapterVersion(tx.QueryRow(ctx, `
			select id::text, chapter_id::text, bid_document_id::text, bid_part_id::text,
				version_no, title, content, plain_text, status, source_refs, needs_human_input,
				change_reason, model_metadata, token_usage, created_by::text, created_at, updated_at
			from bid_chapter_versions
			where tenant_id = $1 and chapter_id = $2
			order by version_no desc
			offset 1 limit 1
		`, tenantID, chapterID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		diff.Previous = &version
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterDiff{}, ErrNotFound
	}
	return diff, err
}

func (s *Store) ListExports(ctx context.Context, tenantID, bidID string) ([]Export, error) {
	exports := []Export{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, bid_document_id::text, bid_part_id::text, export_type, part_code, status,
				file_asset_id::text, filename, metadata, error_message, completed_at, created_at, updated_at
			from bid_exports
			where tenant_id = $1 and bid_document_id = $2
			order by created_at desc
			limit 20
		`, tenantID, bidID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			export, err := scanExport(rows)
			if err != nil {
				return err
			}
			exports = append(exports, export)
		}
		return rows.Err()
	})
	return exports, err
}

func (s *Store) GetExport(ctx context.Context, tenantID, exportID string) (Export, error) {
	var export Export
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanExport(tx.QueryRow(ctx, `
			select id::text, bid_document_id::text, bid_part_id::text, export_type, part_code, status,
				file_asset_id::text, filename, metadata, error_message, completed_at, created_at, updated_at
			from bid_exports
			where tenant_id = $1 and id = $2
		`, tenantID, exportID))
		if err != nil {
			return err
		}
		export = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	return export, err
}

func (s *Store) CreateExport(ctx context.Context, tenantID, userID, bidID string, req CreateExportRequest) (CreateExportResponse, error) {
	exportType := normalizeExportType(req.ExportType)
	if exportType == "" {
		return CreateExportResponse{}, ErrInvalidRequest
	}
	partCode := normalizePartCode(req.PartCode)
	if exportType == "zip" {
		partCode = "all"
	}

	var export Export
	var task Task
	var payload map[string]any
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		document, err := bidForExport(ctx, tx, tenantID, bidID)
		if err != nil {
			return err
		}

		exportID := uuid.NewString()
		fileID := uuid.NewString()
		filename := exportFilename(document.Title, partCode, exportType)
		objectKey := platformfile.ObjectKey(tenantID, "bid_export")
		var bidPartID any
		payload = map[string]any{
			"tenant_id":    tenantID,
			"export_id":    exportID,
			"bid_id":       bidID,
			"bid_title":    document.Title,
			"export_type":  exportType,
			"filename":     filename,
			"object_key":   objectKey,
			"callback_url": s.cfg.AICallbackURL,
		}
		if exportType == "docx" {
			part, err := partForExport(ctx, tx, tenantID, bidID, partCode)
			if err != nil {
				return err
			}
			chapters, err := chaptersForExport(ctx, tx, tenantID, bidID, part.ID)
			if err != nil {
				return err
			}
			if len(chapters) == 0 {
				return ErrInvalidRequest
			}
			bidPartID = part.ID
			payload["part_code"] = part.Code
			payload["part_title"] = part.Title
			payload["chapters"] = exportChapters(chapters)
		} else {
			parts, err := partsForZipExport(ctx, tx, tenantID, bidID)
			if err != nil {
				return err
			}
			if len(parts) < 2 {
				return ErrInvalidRequest
			}
			partPayload := make([]exportPartPayload, 0, len(parts))
			for _, part := range parts {
				chapters, err := chaptersForExport(ctx, tx, tenantID, bidID, part.ID)
				if err != nil {
					return err
				}
				if len(chapters) == 0 {
					continue
				}
				partPayload = append(partPayload, exportPartPayload{
					Code:     part.Code,
					Title:    part.Title,
					Chapters: exportChapters(chapters),
				})
			}
			if len(partPayload) < 2 {
				return ErrInvalidRequest
			}
			payload["part_code"] = "all"
			payload["part_title"] = "投标文件全套"
			payload["parts"] = partPayload
		}
		payloadJSON, _ := json.Marshal(payload)
		if _, err := tx.Exec(ctx, `
			insert into file_assets (
				id, tenant_id, owner_user_id, biz_type, biz_id,
				object_key, filename, content_type, size_bytes, status
			)
			values ($1, $2, $3, 'bid_export', $4, $5, $6, $7, 0, 'pending')
		`, fileID, tenantID, userID, exportID, objectKey, filename, contentTypeForExport(exportType)); err != nil {
			return err
		}
		createdExport, err := scanExport(tx.QueryRow(ctx, `
			insert into bid_exports (
				id, tenant_id, bid_document_id, bid_part_id, export_type, part_code,
				status, file_asset_id, filename, metadata
			)
			values ($1, $2, $3, $4, $5, $6, 'queued', $7, $8, '{}')
			returning id::text, bid_document_id::text, bid_part_id::text, export_type, part_code, status,
				file_asset_id::text, filename, metadata, error_message, completed_at, created_at, updated_at
		`, exportID, tenantID, bidID, bidPartID, exportType, partCode, fileID, filename))
		if err != nil {
			return err
		}
		export = createdExport
		createdTask, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status,
				resource_type, resource_id, payload, route
			)
			values ($1, $2, 'document_export', 'queued', 'bid_export', $3, $4, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, exportID, payloadJSON))
		if err != nil {
			return err
		}
		task = createdTask
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CreateExportResponse{}, ErrNotFound
	}
	if err != nil {
		return CreateExportResponse{}, err
	}

	accepted, err := s.submitDocumentExport(ctx, exportType, payload)
	if err != nil {
		_ = s.markExportFailed(ctx, tenantID, export.ID, task.ID, err.Error())
		return CreateExportResponse{}, err
	}
	updated, updateErr := s.bindAcceptedTask(ctx, tenantID, export.ID, task.ID, accepted, payload)
	if updateErr != nil {
		return CreateExportResponse{}, updateErr
	}
	task = updated
	return CreateExportResponse{Export: export, Task: task}, nil
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
			where tenant_id = $1 and external_task_id = $2 and resource_type = 'bid_export'
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, payload.TenantID, payload.TaskID, status, resultJSON, payload.ErrorMessage))
		if err != nil {
			return err
		}
		task = found
		if status == "done" {
			sizeBytes := int64(0)
			if value, ok := payload.Result["size_bytes"].(float64); ok {
				sizeBytes = int64(value)
			}
			contentType := ""
			if value, ok := payload.Result["content_type"].(string); ok {
				contentType = strings.TrimSpace(value)
			}
			if contentType == "" {
				contentType = docxContentType
			}
			if _, err := tx.Exec(ctx, `
				update file_assets
				set status = 'ready',
					size_bytes = $3,
					content_type = $4,
					confirmed_at = now(),
					updated_at = now()
				where tenant_id = $1
					and id = (select file_asset_id from bid_exports where tenant_id = $1 and id = $2)
			`, payload.TenantID, task.ResourceID, sizeBytes, contentType); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			update bid_exports
			set status = $3,
				metadata = case when $4::jsonb = '{}'::jsonb then metadata else $4::jsonb end,
				error_message = nullif($5, ''),
				completed_at = case when $3 in ('done', 'failed', 'cancelled') then now() else completed_at end,
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, payload.TenantID, task.ResourceID, status, resultJSON, payload.ErrorMessage); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) bindAcceptedTask(ctx context.Context, tenantID, exportID, taskID string, accepted aiTaskAccepted, payload map[string]any) (Task, error) {
	var task Task
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		payloadJSON, _ := json.Marshal(payload)
		routeJSON, _ := json.Marshal(accepted.Route)
		found, err := scanTask(tx.QueryRow(ctx, `
			update ai_tasks
			set status = $4,
				external_task_id = $5,
				payload = $6,
				route = $7,
				updated_at = now()
			where tenant_id = $1 and id = $2 and resource_id = $3
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, taskID, exportID, accepted.Status, accepted.TaskID, payloadJSON, routeJSON))
		if err != nil {
			return err
		}
		task = found
		_, err = tx.Exec(ctx, `
			update bid_exports
			set status = $3, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, exportID, accepted.Status)
		return err
	})
	return task, err
}

func (s *Store) markExportFailed(ctx context.Context, tenantID, exportID, taskID, message string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update ai_tasks
			set status = 'failed', error_message = $3, completed_at = now(), updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, message); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			update bid_exports
			set status = 'failed', error_message = $3, completed_at = now(), updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, exportID, message)
		return err
	})
}

func (s *Store) submitDocumentExport(ctx context.Context, exportType string, payload map[string]any) (aiTaskAccepted, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/export/" + exportType
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

func (s *Store) submitChapterGenerate(ctx context.Context, payload chapterGenerateRequest) (chapterGenerateResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return chapterGenerateResponse{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/chapter-generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return chapterGenerateResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return chapterGenerateResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return chapterGenerateResponse{}, fmt.Errorf("ai service returned %s", resp.Status)
	}
	var result chapterGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return chapterGenerateResponse{}, err
	}
	if result.TiptapJSON == nil || result.TraceID == "" {
		return chapterGenerateResponse{}, ErrInvalidRequest
	}
	return result, nil
}

func chapterByID(ctx context.Context, tx pgx.Tx, tenantID, chapterID string) (Chapter, error) {
	return scanChapter(tx.QueryRow(ctx, `
		select id::text, bid_document_id::text, bid_part_id::text, title, content, plain_text,
			status, sort_order, source_refs, needs_human_input, created_at, updated_at
		from bid_chapters
		where tenant_id = $1 and id = $2
	`, tenantID, chapterID))
}

func bidForExport(ctx context.Context, tx pgx.Tx, tenantID, bidID string) (Document, error) {
	return scanDocument(tx.QueryRow(ctx, `
		select bd.id::text, bd.project_id::text, coalesce(p.name, ''),
			bd.title, bd.bid_type, bd.status, bd.created_at, bd.updated_at
		from bid_documents bd
		left join projects p on p.id = bd.project_id and p.tenant_id = bd.tenant_id
		where bd.tenant_id = $1 and bd.id = $2
	`, tenantID, bidID))
}

func partForExport(ctx context.Context, tx pgx.Tx, tenantID, bidID, partCode string) (Part, error) {
	return scanPart(tx.QueryRow(ctx, `
		select id::text, bid_document_id::text, code, title, sort_order, status, metadata, created_at, updated_at
		from bid_parts
		where tenant_id = $1 and bid_document_id = $2 and code = $3
	`, tenantID, bidID, partCode))
}

func chaptersForExport(ctx context.Context, tx pgx.Tx, tenantID, bidID, partID string) ([]Chapter, error) {
	chapters := []Chapter{}
	rows, err := tx.Query(ctx, `
		select id::text, bid_document_id::text, bid_part_id::text, title, content, plain_text,
			status, sort_order, source_refs, needs_human_input, created_at, updated_at
		from bid_chapters
		where tenant_id = $1 and bid_document_id = $2 and bid_part_id = $3
		order by sort_order, created_at
	`, tenantID, bidID, partID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		chapter, err := scanChapter(rows)
		if err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}
	return chapters, rows.Err()
}

func partsForZipExport(ctx context.Context, tx pgx.Tx, tenantID, bidID string) ([]Part, error) {
	parts := []Part{}
	rows, err := tx.Query(ctx, `
		select id::text, bid_document_id::text, code, title, sort_order, status, metadata, created_at, updated_at
		from bid_parts
		where tenant_id = $1
			and bid_document_id = $2
			and code in ('combined_body', 'tech', 'business')
		order by sort_order, created_at
	`, tenantID, bidID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		part, err := scanPart(rows)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, rows.Err()
}

func exportChapters(chapters []Chapter) []exportChapterPayload {
	payload := make([]exportChapterPayload, 0, len(chapters))
	for _, chapter := range chapters {
		payload = append(payload, exportChapterPayload{
			Title:     chapter.Title,
			PlainText: chapter.PlainText,
		})
	}
	return payload
}

func insertChapterVersion(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	userID string,
	chapter Chapter,
	changeReason string,
	modelMetadata map[string]any,
	tokenUsage map[string]int,
) (ChapterVersion, error) {
	if modelMetadata == nil {
		modelMetadata = map[string]any{}
	}
	if tokenUsage == nil {
		tokenUsage = map[string]int{}
	}
	contentJSON, _ := json.Marshal(chapter.Content)
	sourceRefsJSON, _ := json.Marshal(chapter.SourceRefs)
	needsHumanInputJSON, _ := json.Marshal(chapter.NeedsHumanInput)
	modelMetadataJSON, _ := json.Marshal(modelMetadata)
	tokenUsageJSON, _ := json.Marshal(tokenUsage)
	return scanChapterVersion(tx.QueryRow(ctx, `
		insert into bid_chapter_versions (
			tenant_id, chapter_id, bid_document_id, bid_part_id, version_no,
			title, content, plain_text, status, source_refs, needs_human_input,
			change_reason, model_metadata, token_usage, created_by
		)
		values (
			$1, $2, $3, $4,
			coalesce((select max(version_no) + 1 from bid_chapter_versions where tenant_id = $1 and chapter_id = $2), 1),
			$5, $6, $7, $8, $9, $10, $11, $12, $13, nullif($14, '')::uuid
		)
		returning id::text, chapter_id::text, bid_document_id::text, bid_part_id::text,
			version_no, title, content, plain_text, status, source_refs, needs_human_input,
			change_reason, model_metadata, token_usage, created_by::text, created_at, updated_at
	`, tenantID, chapter.ID, chapter.BidDocumentID, chapter.BidPartID, chapter.Title,
		contentJSON, chapter.PlainText, chapter.Status, sourceRefsJSON, needsHumanInputJSON,
		changeReason, modelMetadataJSON, tokenUsageJSON, userID))
}

func replaceKnowledgeReferences(ctx context.Context, tx pgx.Tx, tenantID string, chapter Chapter, refs []sourceRef, traceID string) error {
	if _, err := tx.Exec(ctx, `
		delete from knowledge_references
		where tenant_id = $1 and bid_document_id = $2 and chapter_id = $3
	`, tenantID, chapter.BidDocumentID, chapter.ID); err != nil {
		return err
	}
	for _, ref := range refs {
		metadata := map[string]any{
			"source_ref": map[string]any{
				"chunk_id":      ref.ChunkID,
				"document_id":   ref.DocumentID,
				"title":         ref.Title,
				"page_start":    ref.PageStart,
				"page_end":      ref.PageEnd,
				"trace_id":      traceID,
				"resolved":      false,
				"chapter_title": chapter.Title,
			},
		}
		var sourceDocumentID any
		if parsed, err := uuid.Parse(ref.DocumentID); err == nil {
			var exists bool
			if err := tx.QueryRow(ctx, `
				select exists(select 1 from knowledge_documents where tenant_id = $1 and id = $2)
			`, tenantID, parsed.String()).Scan(&exists); err != nil {
				return err
			}
			if exists {
				sourceDocumentID = parsed.String()
				metadata["source_ref"].(map[string]any)["resolved"] = true
			}
		}
		var chunkID any
		if parsed, err := uuid.Parse(ref.ChunkID); err == nil {
			chunkID = parsed.String()
		}
		metadataJSON, _ := json.Marshal(metadata)
		if _, err := tx.Exec(ctx, `
			insert into knowledge_references (
				tenant_id, source_document_id, bid_document_id, chapter_id, chunk_id, title, metadata
			)
			values ($1, $2, $3, $4, $5, $6, $7)
		`, tenantID, sourceDocumentID, chapter.BidDocumentID, chapter.ID, chunkID, ref.Title, metadataJSON); err != nil {
			return err
		}
	}
	return nil
}

func createDefaultParts(ctx context.Context, tx pgx.Tx, tenantID, bidID, bidType string) error {
	parts := []struct {
		Code  string
		Title string
		Order int
	}{
		{Code: "combined_body", Title: "综合标书主体", Order: 10},
	}
	if bidType == "separated" {
		parts = []struct {
			Code  string
			Title string
			Order int
		}{
			{Code: "tech", Title: "技术标", Order: 10},
			{Code: "business", Title: "商务标", Order: 20},
		}
	}
	for _, part := range parts {
		var partID string
		if err := tx.QueryRow(ctx, `
			insert into bid_parts (tenant_id, bid_document_id, code, title, sort_order)
			values ($1, $2, $3, $4, $5)
			returning id::text
		`, tenantID, bidID, part.Code, part.Title, part.Order).Scan(&partID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into bid_chapters (tenant_id, bid_document_id, bid_part_id, title, plain_text, sort_order)
			values
				($1, $2, $3, '一、项目理解', '请在编辑器中补充项目理解、响应范围和关键约束。', 10),
				($1, $2, $3, '二、实施方案', '请在编辑器中补充实施方案、人员安排和交付计划。', 20)
		`, tenantID, bidID, partID); err != nil {
			return err
		}
	}
	return nil
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

func scanDocument(row scanner) (Document, error) {
	var document Document
	var projectID sql.NullString
	err := row.Scan(
		&document.ID, &projectID, &document.ProjectName,
		&document.Title, &document.BidType, &document.Status, &document.CreatedAt, &document.UpdatedAt,
	)
	if projectID.Valid {
		document.ProjectID = &projectID.String
	}
	return document, err
}

func scanPart(row scanner) (Part, error) {
	var part Part
	var metadataRaw []byte
	err := row.Scan(
		&part.ID, &part.BidDocumentID, &part.Code, &part.Title, &part.SortOrder,
		&part.Status, &metadataRaw, &part.CreatedAt, &part.UpdatedAt,
	)
	part.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &part.Metadata)
	}
	return part, err
}

func scanChapter(row scanner) (Chapter, error) {
	var chapter Chapter
	var contentRaw, sourceRefsRaw, needsHumanInputRaw []byte
	err := row.Scan(
		&chapter.ID, &chapter.BidDocumentID, &chapter.BidPartID, &chapter.Title, &contentRaw,
		&chapter.PlainText, &chapter.Status, &chapter.SortOrder, &sourceRefsRaw, &needsHumanInputRaw,
		&chapter.CreatedAt, &chapter.UpdatedAt,
	)
	chapter.Content = map[string]any{}
	chapter.SourceRefs = []any{}
	chapter.NeedsHumanInput = []string{}
	_ = json.Unmarshal(contentRaw, &chapter.Content)
	_ = json.Unmarshal(sourceRefsRaw, &chapter.SourceRefs)
	_ = json.Unmarshal(needsHumanInputRaw, &chapter.NeedsHumanInput)
	return chapter, err
}

func scanChapterVersion(row scanner) (ChapterVersion, error) {
	var version ChapterVersion
	var contentRaw, sourceRefsRaw, needsHumanInputRaw, modelMetadataRaw, tokenUsageRaw []byte
	var createdBy sql.NullString
	err := row.Scan(
		&version.ID, &version.ChapterID, &version.BidDocumentID, &version.BidPartID,
		&version.VersionNo, &version.Title, &contentRaw, &version.PlainText, &version.Status,
		&sourceRefsRaw, &needsHumanInputRaw, &version.ChangeReason, &modelMetadataRaw, &tokenUsageRaw,
		&createdBy, &version.CreatedAt, &version.UpdatedAt,
	)
	if createdBy.Valid {
		version.CreatedBy = &createdBy.String
	}
	version.Content = map[string]any{}
	version.SourceRefs = []any{}
	version.NeedsHumanInput = []string{}
	version.ModelMetadata = map[string]any{}
	version.TokenUsage = map[string]int{}
	_ = json.Unmarshal(contentRaw, &version.Content)
	_ = json.Unmarshal(sourceRefsRaw, &version.SourceRefs)
	_ = json.Unmarshal(needsHumanInputRaw, &version.NeedsHumanInput)
	_ = json.Unmarshal(modelMetadataRaw, &version.ModelMetadata)
	_ = json.Unmarshal(tokenUsageRaw, &version.TokenUsage)
	return version, err
}

func scanExport(row scanner) (Export, error) {
	var export Export
	var bidPartID, fileAssetID, errorMessage sql.NullString
	var completedAt sql.NullTime
	var metadataRaw []byte
	err := row.Scan(
		&export.ID, &export.BidDocumentID, &bidPartID, &export.ExportType, &export.PartCode,
		&export.Status, &fileAssetID, &export.Filename, &metadataRaw, &errorMessage, &completedAt,
		&export.CreatedAt, &export.UpdatedAt,
	)
	if bidPartID.Valid {
		export.BidPartID = &bidPartID.String
	}
	if fileAssetID.Valid {
		export.FileAssetID = &fileAssetID.String
	}
	if errorMessage.Valid {
		export.ErrorMessage = &errorMessage.String
	}
	if completedAt.Valid {
		export.CompletedAt = &completedAt.Time
	}
	export.Metadata = map[string]any{}
	_ = json.Unmarshal(metadataRaw, &export.Metadata)
	return export, err
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
	return task, err
}

func tiptapFromPlainText(text string) map[string]any {
	paragraphs := []any{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paragraphs = append(paragraphs, map[string]any{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": line},
			},
		})
	}
	if len(paragraphs) == 0 {
		paragraphs = append(paragraphs, map[string]any{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": "请补充本章节内容。"},
			},
		})
	}
	return map[string]any{"type": "doc", "content": paragraphs}
}

func plainTextFromTiptap(content map[string]any) string {
	lines := []string{}
	collectText(content, &lines)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func collectText(value any, lines *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		if text, ok := typed["text"].(string); ok && strings.TrimSpace(text) != "" {
			*lines = append(*lines, strings.TrimSpace(text))
		}
		if children, ok := typed["content"].([]any); ok {
			for _, child := range children {
				collectText(child, lines)
			}
		}
	case []any:
		for _, child := range typed {
			collectText(child, lines)
		}
	}
}

func sourceRefsAsAny(refs []sourceRef) []any {
	result := make([]any, 0, len(refs))
	for _, ref := range refs {
		result = append(result, map[string]any{
			"chunk_id":    ref.ChunkID,
			"document_id": ref.DocumentID,
			"title":       ref.Title,
			"page_start":  ref.PageStart,
			"page_end":    ref.PageEnd,
		})
	}
	return result
}

func exportFilename(title, partCode, exportType string) string {
	label := map[string]string{
		"combined_body": "综合标书",
		"tech":          "技术标",
		"business":      "商务标",
		"boq":           "工程量清单",
		"attachment":    "附件",
		"all":           "投标文件全套",
	}[partCode]
	if label == "" {
		label = partCode
	}
	base := sanitizeFilename(title)
	if base == "" {
		base = "bid-document"
	}
	return fmt.Sprintf("%s-%s.%s", base, label, exportType)
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "-")
	return replacer.Replace(value)
}

func normalizeBidType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "separated", "custom":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "combined"
	}
}

func normalizeExportType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "docx"
	case "docx", "zip":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizePartCode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tech", "business", "boq", "attachment":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "combined_body"
	}
}

func contentTypeForExport(exportType string) string {
	if exportType == "zip" {
		return zipContentType
	}
	return docxContentType
}

func normalizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "done", "failed", "cancelled":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}
