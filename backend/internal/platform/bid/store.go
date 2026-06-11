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

const docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

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
	partCode := normalizePartCode(req.PartCode)
	if exportType != "docx" {
		return CreateExportResponse{}, ErrInvalidRequest
	}

	var export Export
	var task Task
	var payload map[string]any
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		document, err := bidForExport(ctx, tx, tenantID, bidID)
		if err != nil {
			return err
		}
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

		exportID := uuid.NewString()
		fileID := uuid.NewString()
		filename := exportFilename(document.Title, part.Code, exportType)
		objectKey := platformfile.ObjectKey(tenantID, "bid_export")
		chapterPayload := make([]exportChapterPayload, 0, len(chapters))
		for _, chapter := range chapters {
			chapterPayload = append(chapterPayload, exportChapterPayload{
				Title:     chapter.Title,
				PlainText: chapter.PlainText,
			})
		}
		payload = map[string]any{
			"tenant_id":    tenantID,
			"export_id":    exportID,
			"bid_id":       bidID,
			"bid_title":    document.Title,
			"part_code":    part.Code,
			"part_title":   part.Title,
			"filename":     filename,
			"object_key":   objectKey,
			"chapters":     chapterPayload,
			"callback_url": s.cfg.AICallbackURL,
		}
		payloadJSON, _ := json.Marshal(payload)
		if _, err := tx.Exec(ctx, `
			insert into file_assets (
				id, tenant_id, owner_user_id, biz_type, biz_id,
				object_key, filename, content_type, size_bytes, status
			)
			values ($1, $2, $3, 'bid_export', $4, $5, $6, $7, 0, 'pending')
		`, fileID, tenantID, userID, exportID, objectKey, filename, docxContentType); err != nil {
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
		`, exportID, tenantID, bidID, part.ID, exportType, part.Code, fileID, filename))
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

	accepted, err := s.submitDocxExport(ctx, payload)
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
			if _, err := tx.Exec(ctx, `
				update file_assets
				set status = 'ready',
					size_bytes = $3,
					content_type = $4,
					confirmed_at = now(),
					updated_at = now()
				where tenant_id = $1
					and id = (select file_asset_id from bid_exports where tenant_id = $1 and id = $2)
			`, payload.TenantID, task.ResourceID, sizeBytes, docxContentType); err != nil {
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

func (s *Store) submitDocxExport(ctx context.Context, payload map[string]any) (aiTaskAccepted, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/export/docx"
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

func exportFilename(title, partCode, exportType string) string {
	label := map[string]string{
		"combined_body": "综合标书",
		"tech":          "技术标",
		"business":      "商务标",
		"boq":           "工程量清单",
		"attachment":    "附件",
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
	case "pdf", "zip":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "docx"
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

func normalizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "done", "failed", "cancelled":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}
