package bid

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/aihttp"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/frankford824/ZBT/backend/internal/platform/taskstatus"
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
	pdfContentType  = "application/pdf"
	zipContentType  = "application/zip"

	tenderParseSubmitFailureMessage     = "招标文件解读启动失败，请稍后重试"
	chapterGenerateSubmitFailureMessage = "章节生成启动失败，请稍后重试"
	chapterActionSubmitFailureMessage   = "章节处理启动失败，请稍后重试"
	documentExportSubmitFailureMessage  = "导出任务启动失败，请稍后重试"

	maxExportAttachmentCount            = 50
	maxExportInlineAttachmentBytes      = 20 * 1024 * 1024
	maxExportInlineAttachmentTotalBytes = 50 * 1024 * 1024
	maxExactJSONInteger                 = int64(1<<53 - 1)
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

type BidTemplate struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	BidType     string         `json:"bid_type"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Content     map[string]any `json:"content"`
	UsageCount  int            `json:"usage_count"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
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

type GenerationSnapshot struct {
	BidID       string              `json:"bid_id"`
	Summary     GenerationSummary   `json:"summary"`
	Chapters    []GenerationChapter `json:"chapters"`
	Tasks       []GenerationTask    `json:"tasks"`
	GeneratedAt time.Time           `json:"generated_at"`
}

type GenerationSummary struct {
	TotalChapters      int `json:"total_chapters"`
	GeneratingChapters int `json:"generating_chapters"`
	GeneratedChapters  int `json:"generated_chapters"`
	AcceptedChapters   int `json:"accepted_chapters"`
	NeedsFixChapters   int `json:"needs_fix_chapters"`
	QueuedTasks        int `json:"queued_tasks"`
	RunningTasks       int `json:"running_tasks"`
	DoneTasks          int `json:"done_tasks"`
	FailedTasks        int `json:"failed_tasks"`
}

type GenerationChapter struct {
	ID                   string    `json:"id"`
	BidPartID            string    `json:"bid_part_id"`
	Title                string    `json:"title"`
	Status               string    `json:"status"`
	SortOrder            int       `json:"sort_order"`
	SourceRefCount       int       `json:"source_ref_count"`
	NeedsHumanInputCount int       `json:"needs_human_input_count"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type GenerationTask struct {
	ID             string    `json:"id"`
	ExternalTaskID *string   `json:"external_task_id"`
	ChapterID      string    `json:"chapter_id"`
	ChapterTitle   string    `json:"chapter_title"`
	Status         string    `json:"status"`
	ErrorMessage   *string   `json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type GenerationJob struct {
	ID               string     `json:"id"`
	BidDocumentID    string     `json:"bid_document_id"`
	Scope            string     `json:"scope"`
	Status           string     `json:"status"`
	Progress         int        `json:"progress"`
	TotalSteps       int        `json:"total_steps"`
	CompletedSteps   int        `json:"completed_steps"`
	FailedSteps      int        `json:"failed_steps"`
	ModelUsed        string     `json:"model_used"`
	PromptTokens     int        `json:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens"`
	ErrorMessage     *string    `json:"error_message"`
	TraceID          string     `json:"trace_id"`
	CreatedBy        *string    `json:"created_by"`
	StartedAt        *time.Time `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type GenerationStep struct {
	ID             string         `json:"id"`
	JobID          string         `json:"job_id"`
	BidDocumentID  string         `json:"bid_document_id"`
	BidPartID      string         `json:"bid_part_id"`
	ChapterID      string         `json:"chapter_id"`
	ChapterTitle   string         `json:"chapter_title"`
	StepOrder      int            `json:"step_order"`
	Status         string         `json:"status"`
	AITaskID       *string        `json:"ai_task_id"`
	ExternalTaskID *string        `json:"external_task_id"`
	ErrorMessage   *string        `json:"error_message"`
	Metadata       map[string]any `json:"metadata"`
	StartedAt      *time.Time     `json:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type GenerateBidRequest struct {
	Scope      string   `json:"scope"`
	PartCode   string   `json:"part_code"`
	ChapterIDs []string `json:"chapter_ids"`
}

type GenerationJobDetail struct {
	TaskID string           `json:"task_id"`
	Job    GenerationJob    `json:"job"`
	Steps  []GenerationStep `json:"steps"`
}

type CreateDocumentRequest struct {
	Title       string `json:"title"`
	ProjectName string `json:"project_name"`
	BidType     string `json:"bid_type"`
}

type UpdateDocumentRequest struct {
	Title     string  `json:"title"`
	ProjectID *string `json:"project_id"`
	Status    string  `json:"status"`
}

type UseTemplateRequest struct {
	Title       string `json:"title"`
	ProjectName string `json:"project_name"`
}

type UseTemplateResult struct {
	Template BidTemplate `json:"template"`
	Bid      Document    `json:"bid"`
}

type CreateExportRequest struct {
	ExportType  string           `json:"export_type"`
	PartCode    string           `json:"part_code"`
	Layout      map[string]any   `json:"layout"`
	Attachments []map[string]any `json:"attachments"`
	BOQFiles    []map[string]any `json:"boq_files"`
}

type UpdateChapterContentRequest struct {
	Title     string         `json:"title"`
	Content   map[string]any `json:"content"`
	PlainText string         `json:"plain_text"`
}

type ChapterAIActionRequest struct {
	Action      string `json:"action"`
	Instruction string `json:"instruction"`
}

type ChapterRegenerateResponse struct {
	Chapter Chapter `json:"chapter"`
	Task    Task    `json:"task"`
}

type ChapterDiff struct {
	Current  Chapter         `json:"current"`
	Previous *ChapterVersion `json:"previous"`
}

type TenderFile struct {
	ID            string    `json:"id"`
	BidDocumentID string    `json:"bid_document_id"`
	FileAssetID   string    `json:"file_asset_id"`
	ObjectKey     string    `json:"object_key"`
	Filename      string    `json:"filename"`
	ContentType   string    `json:"content_type"`
	SizeBytes     int64     `json:"size_bytes"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ParseResult struct {
	ID               string         `json:"id"`
	BidDocumentID    string         `json:"bid_document_id"`
	FileAssetID      *string        `json:"file_asset_id"`
	Status           string         `json:"status"`
	StructuredResult map[string]any `json:"structured_result"`
	ErrorMessage     *string        `json:"error_message"`
	ConfirmedBy      *string        `json:"confirmed_by"`
	ConfirmedAt      *time.Time     `json:"confirmed_at"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type UploadTenderFileRequest struct {
	FileID string `json:"file_id"`
}

type UploadTenderFileResponse struct {
	File        TenderFile  `json:"file"`
	ParseResult ParseResult `json:"parse_result"`
}

type ParseTenderResponse struct {
	Task        Task        `json:"task"`
	ParseResult ParseResult `json:"parse_result"`
}

type ConfirmParseResultRequest struct {
	StructuredResult map[string]any `json:"structured_result"`
}

type OutlineGenerateResponse struct {
	Task     Task      `json:"task"`
	Parts    []Part    `json:"parts"`
	Chapters []Chapter `json:"chapters"`
}

type PartOutline struct {
	Part     Part      `json:"part"`
	Chapters []Chapter `json:"chapters"`
}

type OutlineChapterInput struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	PlainText string `json:"plain_text"`
	SortOrder int    `json:"sort_order"`
}

type UpdatePartOutlineRequest struct {
	Chapters []OutlineChapterInput `json:"chapters"`
}

type MaterialSelection struct {
	ID            string    `json:"id"`
	BidDocumentID string    `json:"bid_document_id"`
	SelectedRefs  []any     `json:"selected_refs"`
	Notes         string    `json:"notes"`
	UpdatedBy     *string   `json:"updated_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UpdateMaterialSelectionRequest struct {
	SelectedRefs []any  `json:"selected_refs"`
	Notes        string `json:"notes"`
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

func normalizeAcceptedTask(accepted aiTaskAccepted) (aiTaskAccepted, error) {
	accepted.TaskID = strings.TrimSpace(accepted.TaskID)
	if accepted.TaskID == "" {
		return aiTaskAccepted{}, ErrInvalidRequest
	}
	status := normalizeTaskStatus(accepted.Status)
	if status == "" {
		status = "queued"
	}
	accepted.Status = status
	if accepted.Route == nil {
		accepted.Route = map[string]any{}
	}
	return accepted, nil
}

type chapterGenerateRequest struct {
	TaskID                 string                  `json:"task_id,omitempty"`
	TenantID               string                  `json:"tenant_id"`
	BidDocumentID          string                  `json:"bid_document_id"`
	BidPartID              string                  `json:"bid_part_id"`
	ChapterID              string                  `json:"chapter_id"`
	ChapterTitle           string                  `json:"chapter_title"`
	TenderRequirements     []string                `json:"tender_requirements"`
	SelectedKnowledgeRefs  []string                `json:"selected_knowledge_refs"`
	RetrievedKnowledgeRefs []retrievedKnowledgeRef `json:"retrieved_knowledge_refs"`
	CallbackURL            string                  `json:"callback_url,omitempty"`
	ModelHint              *string                 `json:"model_hint,omitempty"`
}

type tenderParseRequest struct {
	TaskID      string `json:"task_id,omitempty"`
	TenantID    string `json:"tenant_id"`
	BidID       string `json:"bid_id,omitempty"`
	BidTitle    string `json:"bid_title,omitempty"`
	FileID      string `json:"file_id"`
	ObjectKey   string `json:"object_key"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	CallbackURL string `json:"callback_url,omitempty"`
}

type chapterActionRequest struct {
	chapterGenerateRequest
	Action            string         `json:"action"`
	Instruction       string         `json:"instruction"`
	CurrentPlainText  string         `json:"current_plain_text"`
	CurrentTiptapJSON map[string]any `json:"current_tiptap_json"`
}

type retrievedKnowledgeRef struct {
	ChunkID     string  `json:"chunk_id"`
	DocumentID  string  `json:"document_id"`
	Title       string  `json:"title"`
	SectionPath string  `json:"section_path"`
	Content     string  `json:"content"`
	PageStart   *int    `json:"page_start"`
	PageEnd     *int    `json:"page_end"`
	Score       float64 `json:"score"`
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

type generationDispatch struct {
	JobID     string
	StepID    string
	TaskID    string
	ChapterID string
	Payload   chapterGenerateRequest
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

func (s *Store) ListTemplates(ctx context.Context, tenantID string) ([]BidTemplate, error) {
	templates := []BidTemplate{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, name, bid_type, category, description, version, content, usage_count, status, created_at, updated_at
			from bid_templates
			where tenant_id = $1 and status = 'active'
			order by usage_count desc, created_at desc
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			template, err := scanBidTemplate(rows)
			if err != nil {
				return err
			}
			templates = append(templates, template)
		}
		return rows.Err()
	})
	return templates, err
}

func (s *Store) UseTemplate(ctx context.Context, tenantID, templateID string, req UseTemplateRequest) (UseTemplateResult, error) {
	templateID = strings.TrimSpace(templateID)
	if _, err := uuid.Parse(templateID); err != nil {
		return UseTemplateResult{}, ErrInvalidRequest
	}

	var template BidTemplate
	var bidID string
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanBidTemplate(tx.QueryRow(ctx, `
			select id::text, name, bid_type, category, description, version, content, usage_count, status, created_at, updated_at
			from bid_templates
			where tenant_id = $1 and id = $2 and status = 'active'
		`, tenantID, templateID))
		if err != nil {
			return err
		}
		template = found

		title := strings.TrimSpace(req.Title)
		if title == "" {
			title = strings.TrimSpace(req.ProjectName)
		}
		if title == "" {
			title = template.Name
		}
		bidType, err := normalizeBidType(template.BidType)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			insert into bid_documents (tenant_id, title, bid_type, status)
			values ($1, $2, $3, 'draft')
			returning id::text
		`, tenantID, title, bidType).Scan(&bidID); err != nil {
			return err
		}
		if err := createDefaultParts(ctx, tx, tenantID, bidID, bidType); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update bid_templates
			set usage_count = usage_count + 1, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, templateID); err != nil {
			return err
		}
		template.UsageCount++
		template.UpdatedAt = time.Now()
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UseTemplateResult{}, ErrNotFound
	}
	if err != nil {
		return UseTemplateResult{}, err
	}
	document, err := s.GetDocument(ctx, tenantID, bidID)
	if err != nil {
		return UseTemplateResult{}, err
	}
	return UseTemplateResult{Template: template, Bid: document}, nil
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
				and bd.status <> 'archived'
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
	bidType, err := normalizeBidType(req.BidType)
	if err != nil {
		return Document{}, err
	}
	var id string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
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

func (s *Store) UpdateDocument(ctx context.Context, tenantID, id string, req UpdateDocumentRequest) (Document, error) {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return Document{}, ErrInvalidRequest
	}
	title := strings.TrimSpace(req.Title)
	status := strings.TrimSpace(req.Status)
	if status != "" {
		status = normalizeDocumentStatus(status)
		if status == "" {
			return Document{}, ErrInvalidRequest
		}
	}
	projectID := ""
	updateProject := req.ProjectID != nil
	if req.ProjectID != nil {
		projectID = strings.TrimSpace(*req.ProjectID)
		if projectID != "" {
			if _, err := uuid.Parse(projectID); err != nil {
				return Document{}, ErrInvalidRequest
			}
		}
	}
	var document Document
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if projectID != "" {
			var exists bool
			if err := tx.QueryRow(ctx, `select exists(select 1 from projects where tenant_id = $1 and id = $2)`, tenantID, projectID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrInvalidRequest
			}
		}
		updated, err := scanDocument(tx.QueryRow(ctx, `
			update bid_documents
			set
				title = coalesce(nullif($3, ''), title),
				status = coalesce(nullif($4, ''), status),
				project_id = case when $5 then nullif($6, '')::uuid else project_id end,
				updated_at = now()
			where tenant_id = $1 and id = $2
			returning id::text, project_id::text, '',
				title, bid_type, status, created_at, updated_at
		`, tenantID, id, title, status, updateProject, projectID))
		if err != nil {
			return err
		}
		document = updated
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}
	return s.GetDocument(ctx, tenantID, document.ID)
}

func (s *Store) DeleteDocument(ctx context.Context, tenantID, id string) error {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidRequest
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update bid_documents
			set status = 'archived', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
	return err
}

func (s *Store) UploadTenderFile(ctx context.Context, tenantID, userID, bidID string, req UploadTenderFileRequest) (UploadTenderFileResponse, error) {
	bidID = strings.TrimSpace(bidID)
	fileID := strings.TrimSpace(req.FileID)
	if _, err := uuid.Parse(bidID); err != nil {
		return UploadTenderFileResponse{}, ErrInvalidRequest
	}
	if _, err := uuid.Parse(fileID); err != nil {
		return UploadTenderFileResponse{}, ErrInvalidRequest
	}

	var result UploadTenderFileResponse
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := bidForExport(ctx, tx, tenantID, bidID); err != nil {
			return err
		}
		var fileStatus, fileBizType string
		var fileBizID sql.NullString
		if err := tx.QueryRow(ctx, `
			select status, biz_type, biz_id::text
			from file_assets
			where tenant_id = $1 and id = $2
		`, tenantID, fileID).Scan(&fileStatus, &fileBizType, &fileBizID); err != nil {
			return err
		}
		if fileStatus != "ready" {
			return platformfile.ErrInvalidObjectState
		}
		if !attachableTenderFileAsset(fileBizType, fileBizID, bidID) {
			return ErrInvalidRequest
		}
		if _, err := tx.Exec(ctx, `
			update bid_tender_files
			set status = 'superseded', updated_at = now()
			where tenant_id = $1 and bid_document_id = $2 and file_asset_id <> $3 and status = 'active'
		`, tenantID, bidID, fileID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			update file_assets
			set biz_type = 'bid_tender',
				biz_id = $3,
				updated_at = now()
			where tenant_id = $1 and id = $2
				and biz_type = 'bid_tender'
				and (biz_id is null or biz_id = $3::uuid)
		`, tenantID, fileID, bidID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrInvalidRequest
		}
		attached, err := scanTenderFile(tx.QueryRow(ctx, `
			with upserted as (
				insert into bid_tender_files (tenant_id, bid_document_id, file_asset_id, status, created_by)
				values ($1, $2, $3, 'active', nullif($4, '')::uuid)
				on conflict (tenant_id, bid_document_id, file_asset_id) do update
				set status = 'active', updated_at = now()
				returning id, bid_document_id, file_asset_id, status, created_at, updated_at
			)
			select u.id::text, u.bid_document_id::text, u.file_asset_id::text,
				f.object_key, f.filename, f.content_type, f.size_bytes,
				u.status, u.created_at, u.updated_at
			from upserted u
			join file_assets f on f.tenant_id = $1 and f.id = u.file_asset_id
		`, tenantID, bidID, fileID, userID))
		if err != nil {
			return err
		}
		parseResult, err := upsertParseResult(ctx, tx, tenantID, bidID, fileID, "queued", map[string]any{}, "")
		if err != nil {
			return err
		}
		result = UploadTenderFileResponse{File: attached, ParseResult: parseResult}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UploadTenderFileResponse{}, ErrNotFound
	}
	return result, err
}

func attachableTenderFileAsset(bizType string, bizID sql.NullString, bidID string) bool {
	if strings.TrimSpace(bizType) != "bid_tender" {
		return false
	}
	if !bizID.Valid || strings.TrimSpace(bizID.String) == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(bizID.String), strings.TrimSpace(bidID))
}

func confirmableParseResultStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "confirmed":
		return true
	default:
		return false
	}
}

func (s *Store) ParseTender(ctx context.Context, tenantID, userID, bidID string) (ParseTenderResponse, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return ParseTenderResponse{}, ErrInvalidRequest
	}

	var result ParseTenderResponse
	var requestPayload tenderParseRequest
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		document, err := bidForExport(ctx, tx, tenantID, bidID)
		if err != nil {
			return err
		}
		file, err := latestTenderFileForBid(ctx, tx, tenantID, bidID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidRequest
		}
		if err != nil {
			return err
		}
		structured := defaultTenderStructuredResult(document, file)
		parseResult, err := upsertParseResult(ctx, tx, tenantID, bidID, file.FileAssetID, "queued", structured, "")
		if err != nil {
			return err
		}
		externalTaskID := "task-tender-parse-" + uuid.NewString()
		requestPayload = tenderParseRequest{
			TaskID:      externalTaskID,
			TenantID:    tenantID,
			BidID:       bidID,
			BidTitle:    document.Title,
			FileID:      file.FileAssetID,
			ObjectKey:   file.ObjectKey,
			Filename:    file.Filename,
			ContentType: file.ContentType,
			CallbackURL: s.cfg.AICallbackURL,
		}
		payloadJSON, _ := json.Marshal(requestPayload)
		task, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status, external_task_id,
				resource_type, resource_id, payload, route
			)
			values ($1, nullif($2, '')::uuid, 'tender_parse', 'queued', $3,
				'bid_parse_result', $4, $5, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, externalTaskID, parseResult.ID, payloadJSON))
		if err != nil {
			return err
		}
		result = ParseTenderResponse{Task: task, ParseResult: parseResult}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ParseTenderResponse{}, ErrNotFound
	}
	if err != nil {
		return ParseTenderResponse{}, err
	}
	accepted, err := s.submitTenderParse(ctx, requestPayload)
	if err != nil {
		_ = s.markTenderParseFailed(ctx, tenantID, result.Task.ID, result.ParseResult.ID, tenderParseSubmitFailureMessage)
		return ParseTenderResponse{}, err
	}
	updatedTask, err := s.bindAcceptedTask(ctx, tenantID, result.ParseResult.ID, result.Task.ID, accepted, requestPayload, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			update bid_parse_results
			set status = 'processing',
				error_message = null,
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, result.ParseResult.ID)
		return err
	})
	if err != nil {
		return ParseTenderResponse{}, err
	}
	result.Task = updatedTask
	parseResult, err := s.GetParseResult(ctx, tenantID, bidID)
	if err != nil {
		return ParseTenderResponse{}, err
	}
	result.ParseResult = parseResult
	return result, nil
}

func (s *Store) GetParseResult(ctx context.Context, tenantID, bidID string) (ParseResult, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return ParseResult{}, ErrInvalidRequest
	}
	var result ParseResult
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := bidForExport(ctx, tx, tenantID, bidID); err != nil {
			return err
		}
		found, err := parseResultForBid(ctx, tx, tenantID, bidID)
		if errors.Is(err, pgx.ErrNoRows) {
			found, err = upsertParseResult(ctx, tx, tenantID, bidID, "", "queued", map[string]any{}, "")
		}
		if err != nil {
			return err
		}
		result = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ParseResult{}, ErrNotFound
	}
	return result, err
}

func (s *Store) ConfirmParseResult(ctx context.Context, tenantID, userID, bidID string, req ConfirmParseResultRequest) (ParseResult, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return ParseResult{}, ErrInvalidRequest
	}
	var result ParseResult
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := bidForExport(ctx, tx, tenantID, bidID); err != nil {
			return err
		}
		current, err := parseResultForBid(ctx, tx, tenantID, bidID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidRequest
		}
		if err != nil {
			return err
		}
		if !confirmableParseResultStatus(current.Status) {
			return ErrInvalidRequest
		}
		structured := req.StructuredResult
		if structured == nil {
			structured = current.StructuredResult
		}
		structuredJSON, _ := json.Marshal(structured)
		confirmed, err := scanParseResult(tx.QueryRow(ctx, `
			update bid_parse_results
			set status = 'confirmed',
				structured_result = $3,
				error_message = null,
				confirmed_by = nullif($4, '')::uuid,
				confirmed_at = now(),
				updated_at = now()
			where tenant_id = $1 and bid_document_id = $2
			returning id::text, bid_document_id::text, file_asset_id::text, status,
				structured_result, error_message, confirmed_by::text, confirmed_at, created_at, updated_at
		`, tenantID, bidID, structuredJSON, userID))
		if err != nil {
			return err
		}
		if err := ensureMaterialSelection(ctx, tx, tenantID, bidID, userID, defaultMaterialRefs(structured), ""); err != nil {
			return err
		}
		result = confirmed
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ParseResult{}, ErrNotFound
	}
	return result, err
}

func (s *Store) GenerateOutline(ctx context.Context, tenantID, userID, bidID string) (OutlineGenerateResponse, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return OutlineGenerateResponse{}, ErrInvalidRequest
	}
	var task Task
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		document, err := bidForExport(ctx, tx, tenantID, bidID)
		if err != nil {
			return err
		}
		parseResult, err := parseResultForBid(ctx, tx, tenantID, bidID)
		if errors.Is(err, pgx.ErrNoRows) {
			parseResult, err = upsertParseResult(ctx, tx, tenantID, bidID, "", "ready", defaultTenderStructuredResult(document, TenderFile{}), "")
		}
		if err != nil {
			return err
		}
		specs := outlineSpecsFromStructuredResult(document, parseResult.StructuredResult)
		if err := applyOutlineSpecs(ctx, tx, tenantID, bidID, specs); err != nil {
			return err
		}
		resultPayload := map[string]any{
			"bid_document_id": bidID,
			"parts_count":     len(specs),
			"chapters_count":  outlineChapterCount(specs),
		}
		payloadJSON, _ := json.Marshal(map[string]any{
			"tenant_id":       tenantID,
			"bid_document_id": bidID,
			"parse_result_id": parseResult.ID,
			"mode":            "deterministic_bootstrap",
		})
		resultJSON, _ := json.Marshal(resultPayload)
		routeJSON, _ := json.Marshal(map[string]any{"route": "local.outline_generate"})
		task, err = scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status, external_task_id,
				resource_type, resource_id, payload, route, result, started_at, completed_at
			)
			values ($1, nullif($2, '')::uuid, 'outline_generate', 'done', $3,
				'bid_document', $4, $5, $6, $7, now(), now())
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, "task-outline-"+uuid.NewString(), bidID, payloadJSON, routeJSON, resultJSON))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update bid_documents
			set status = 'editing', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, bidID); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return OutlineGenerateResponse{}, ErrNotFound
	}
	if err != nil {
		return OutlineGenerateResponse{}, err
	}
	parts, err := s.ListParts(ctx, tenantID, bidID)
	if err != nil {
		return OutlineGenerateResponse{}, err
	}
	chapters, err := s.ListChapters(ctx, tenantID, bidID)
	if err != nil {
		return OutlineGenerateResponse{}, err
	}
	return OutlineGenerateResponse{Task: task, Parts: parts, Chapters: chapters}, nil
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

func (s *Store) GetPartOutline(ctx context.Context, tenantID, bidID, partID string) (PartOutline, error) {
	bidID = strings.TrimSpace(bidID)
	partID = strings.TrimSpace(partID)
	if _, err := uuid.Parse(bidID); err != nil {
		return PartOutline{}, ErrInvalidRequest
	}
	if _, err := uuid.Parse(partID); err != nil {
		return PartOutline{}, ErrInvalidRequest
	}
	var outline PartOutline
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		part, err := partByID(ctx, tx, tenantID, bidID, partID)
		if err != nil {
			return err
		}
		chapters, err := chaptersForPart(ctx, tx, tenantID, bidID, partID)
		if err != nil {
			return err
		}
		outline = PartOutline{Part: part, Chapters: chapters}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PartOutline{}, ErrNotFound
	}
	return outline, err
}

func (s *Store) UpdatePartOutline(ctx context.Context, tenantID, userID, bidID, partID string, req UpdatePartOutlineRequest) (PartOutline, error) {
	bidID = strings.TrimSpace(bidID)
	partID = strings.TrimSpace(partID)
	if _, err := uuid.Parse(bidID); err != nil {
		return PartOutline{}, ErrInvalidRequest
	}
	if _, err := uuid.Parse(partID); err != nil {
		return PartOutline{}, ErrInvalidRequest
	}
	if len(req.Chapters) == 0 {
		return PartOutline{}, ErrInvalidRequest
	}

	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := partByID(ctx, tx, tenantID, bidID, partID); err != nil {
			return err
		}
		for index, input := range req.Chapters {
			title := strings.TrimSpace(input.Title)
			if title == "" {
				return ErrInvalidRequest
			}
			sortOrder := input.SortOrder
			if sortOrder <= 0 {
				sortOrder = (index + 1) * 10
			}
			plainText := strings.TrimSpace(input.PlainText)
			chapterID := strings.TrimSpace(input.ID)
			if chapterID == "" {
				if plainText == "" {
					plainText = "请补充" + title + "内容。"
				}
				contentJSON, _ := json.Marshal(tiptapFromPlainText(plainText))
				if _, err := tx.Exec(ctx, `
					insert into bid_chapters (
						tenant_id, bid_document_id, bid_part_id, title,
						content, plain_text, status, sort_order
					)
					values ($1, $2, $3, $4, $5, $6, 'pending', $7)
				`, tenantID, bidID, partID, title, contentJSON, plainText, sortOrder); err != nil {
					return err
				}
				continue
			}
			if _, err := uuid.Parse(chapterID); err != nil {
				return ErrInvalidRequest
			}
			var contentJSON []byte
			if plainText != "" {
				contentJSON, _ = json.Marshal(tiptapFromPlainText(plainText))
			}
			tag, err := tx.Exec(ctx, `
				update bid_chapters
				set title = $5,
					plain_text = case when nullif($6, '') is null then plain_text else $6 end,
					content = case when nullif($6, '') is null then content else $7 end,
					sort_order = $8,
					status = case when status = 'accepted' then status else 'edited' end,
					updated_at = now()
				where tenant_id = $1 and bid_document_id = $2 and bid_part_id = $3 and id = $4
			`, tenantID, bidID, partID, chapterID, title, plainText, contentJSON, sortOrder)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrNotFound
			}
		}
		metadataJSON, _ := json.Marshal(map[string]any{
			"outline_updated_by": userID,
			"outline_updated_at": time.Now().UTC().Format(time.RFC3339),
		})
		if _, err := tx.Exec(ctx, `
			update bid_parts
			set status = 'generated',
				metadata = metadata || $4::jsonb,
				updated_at = now()
			where tenant_id = $1 and bid_document_id = $2 and id = $3
		`, tenantID, bidID, partID, metadataJSON); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PartOutline{}, ErrNotFound
	}
	if err != nil {
		return PartOutline{}, err
	}
	return s.GetPartOutline(ctx, tenantID, bidID, partID)
}

func (s *Store) GetMaterialSelection(ctx context.Context, tenantID, bidID string) (MaterialSelection, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return MaterialSelection{}, ErrInvalidRequest
	}
	var selection MaterialSelection
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := bidForExport(ctx, tx, tenantID, bidID); err != nil {
			return err
		}
		found, err := materialSelectionForBid(ctx, tx, tenantID, bidID)
		if errors.Is(err, pgx.ErrNoRows) {
			refs := []any{}
			if parseResult, parseErr := parseResultForBid(ctx, tx, tenantID, bidID); parseErr == nil {
				refs = defaultMaterialRefs(parseResult.StructuredResult)
			}
			if err := ensureMaterialSelection(ctx, tx, tenantID, bidID, "", refs, ""); err != nil {
				return err
			}
			found, err = materialSelectionForBid(ctx, tx, tenantID, bidID)
		}
		if err != nil {
			return err
		}
		selection = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return MaterialSelection{}, ErrNotFound
	}
	return selection, err
}

func (s *Store) UpdateMaterialSelection(ctx context.Context, tenantID, userID, bidID string, req UpdateMaterialSelectionRequest) (MaterialSelection, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return MaterialSelection{}, ErrInvalidRequest
	}
	var selection MaterialSelection
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := bidForExport(ctx, tx, tenantID, bidID); err != nil {
			return err
		}
		if req.SelectedRefs == nil {
			req.SelectedRefs = []any{}
		}
		if err := ensureMaterialSelection(ctx, tx, tenantID, bidID, userID, req.SelectedRefs, req.Notes); err != nil {
			return err
		}
		found, err := materialSelectionForBid(ctx, tx, tenantID, bidID)
		if err != nil {
			return err
		}
		selection = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return MaterialSelection{}, ErrNotFound
	}
	return selection, err
}

func (s *Store) GenerationSnapshot(ctx context.Context, tenantID, bidID string) (GenerationSnapshot, error) {
	snapshot := GenerationSnapshot{
		BidID:       bidID,
		Chapters:    []GenerationChapter{},
		Tasks:       []GenerationTask{},
		GeneratedAt: time.Now().UTC(),
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `
			select exists(select 1 from bid_documents where tenant_id = $1 and id = $2)
		`, tenantID, bidID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}

		rows, err := tx.Query(ctx, `
			select id::text, bid_part_id::text, title, status, sort_order,
				jsonb_array_length(coalesce(source_refs, '[]'::jsonb)),
				jsonb_array_length(coalesce(needs_human_input, '[]'::jsonb)),
				updated_at
			from bid_chapters
			where tenant_id = $1 and bid_document_id = $2
			order by sort_order, created_at
		`, tenantID, bidID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var chapter GenerationChapter
			if err := rows.Scan(
				&chapter.ID, &chapter.BidPartID, &chapter.Title, &chapter.Status, &chapter.SortOrder,
				&chapter.SourceRefCount, &chapter.NeedsHumanInputCount, &chapter.UpdatedAt,
			); err != nil {
				return err
			}
			snapshot.Chapters = append(snapshot.Chapters, chapter)
			snapshot.Summary.TotalChapters++
			switch chapter.Status {
			case "generating":
				snapshot.Summary.GeneratingChapters++
			case "generated":
				snapshot.Summary.GeneratedChapters++
			case "accepted":
				snapshot.Summary.AcceptedChapters++
			case "needs_fix":
				snapshot.Summary.NeedsFixChapters++
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}

		taskRows, err := tx.Query(ctx, `
			select t.id::text, t.external_task_id::text, t.resource_id::text, c.title,
				t.status, t.error_message, t.created_at, t.updated_at
			from ai_tasks t
			join bid_chapters c on c.tenant_id = t.tenant_id and c.id = t.resource_id
			where t.tenant_id = $1
				and c.bid_document_id = $2
				and t.resource_type = 'bid_chapter'
			order by t.created_at desc
			limit 20
		`, tenantID, bidID)
		if err != nil {
			return err
		}
		defer taskRows.Close()
		for taskRows.Next() {
			var task GenerationTask
			var externalTaskID, errorMessage sql.NullString
			if err := taskRows.Scan(
				&task.ID, &externalTaskID, &task.ChapterID, &task.ChapterTitle,
				&task.Status, &errorMessage, &task.CreatedAt, &task.UpdatedAt,
			); err != nil {
				return err
			}
			if externalTaskID.Valid {
				task.ExternalTaskID = &externalTaskID.String
			}
			if errorMessage.Valid {
				task.ErrorMessage = &errorMessage.String
			}
			snapshot.Tasks = append(snapshot.Tasks, task)
			switch task.Status {
			case "queued":
				snapshot.Summary.QueuedTasks++
			case "running":
				snapshot.Summary.RunningTasks++
			case "done":
				snapshot.Summary.DoneTasks++
			case "failed", "cancelled":
				snapshot.Summary.FailedTasks++
			}
		}
		return taskRows.Err()
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationSnapshot{}, ErrNotFound
	}
	return snapshot, err
}

func (s *Store) GenerateBid(ctx context.Context, tenantID, userID, bidID string, req GenerateBidRequest) (GenerationJobDetail, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return GenerationJobDetail{}, ErrInvalidRequest
	}
	scope, err := normalizeGenerationScope(req.Scope)
	if err != nil {
		return GenerationJobDetail{}, err
	}
	partCode, err := normalizeGenerationPartCode(req.PartCode)
	if err != nil {
		return GenerationJobDetail{}, err
	}
	chapterFilter := map[string]bool{}
	for _, id := range req.ChapterIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			return GenerationJobDetail{}, ErrInvalidRequest
		}
		chapterFilter[id] = true
	}
	if len(chapterFilter) > 0 {
		scope = "chapter"
	}
	if partCode != "" {
		scope = "part"
	}

	var jobID string
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := bidForExport(ctx, tx, tenantID, bidID); err != nil {
			return err
		}
		chapters, err := chaptersForGeneration(ctx, tx, tenantID, bidID, partCode, chapterFilter)
		if err != nil {
			return err
		}
		if len(chapters) == 0 {
			return ErrInvalidRequest
		}
		traceID := "trace-bid-generate-" + uuid.NewString()
		if err := tx.QueryRow(ctx, `
			insert into bid_generation_jobs (
				tenant_id, bid_document_id, scope, status, progress,
				total_steps, trace_id, created_by
			)
			values ($1, $2, $3, 'queued', 0, $4, $5, nullif($6, '')::uuid)
			returning id::text
		`, tenantID, bidID, scope, len(chapters), traceID, userID).Scan(&jobID); err != nil {
			return err
		}
		for index, chapter := range chapters {
			if _, err := tx.Exec(ctx, `
				insert into bid_generation_steps (
					tenant_id, job_id, bid_document_id, bid_part_id,
					chapter_id, step_order, status
				)
				values ($1, $2, $3, $4, $5, $6, 'queued')
			`, tenantID, jobID, bidID, chapter.BidPartID, chapter.ID, (index+1)*10); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationJobDetail{}, ErrNotFound
	}
	if err != nil {
		return GenerationJobDetail{}, err
	}
	if err := s.dispatchNextGenerationStep(ctx, tenantID, jobID); err != nil {
		return GenerationJobDetail{}, err
	}
	return s.GetGenerationJob(ctx, tenantID, jobID)
}

func (s *Store) ListGenerationJobs(ctx context.Context, tenantID, bidID string) ([]GenerationJob, error) {
	bidID = strings.TrimSpace(bidID)
	if _, err := uuid.Parse(bidID); err != nil {
		return nil, ErrInvalidRequest
	}
	jobs := []GenerationJob{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := bidForExport(ctx, tx, tenantID, bidID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			select id::text, bid_document_id::text, scope, status, progress,
				total_steps, completed_steps, failed_steps, model_used,
				prompt_tokens, completion_tokens, error_message, trace_id, created_by::text,
				started_at, completed_at, created_at, updated_at
			from bid_generation_jobs
			where tenant_id = $1 and bid_document_id = $2
			order by created_at desc
			limit 20
		`, tenantID, bidID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			job, err := scanGenerationJob(rows)
			if err != nil {
				return err
			}
			jobs = append(jobs, job)
		}
		return rows.Err()
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return jobs, err
}

func (s *Store) GetGenerationJob(ctx context.Context, tenantID, jobID string) (GenerationJobDetail, error) {
	jobID = strings.TrimSpace(jobID)
	if _, err := uuid.Parse(jobID); err != nil {
		return GenerationJobDetail{}, ErrInvalidRequest
	}
	var detail GenerationJobDetail
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := generationJobDetailByID(ctx, tx, tenantID, jobID)
		if err != nil {
			return err
		}
		detail = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationJobDetail{}, ErrNotFound
	}
	return detail, err
}

func (s *Store) PauseGenerationJob(ctx context.Context, tenantID, jobID string) (GenerationJobDetail, error) {
	jobID = strings.TrimSpace(jobID)
	if _, err := uuid.Parse(jobID); err != nil {
		return GenerationJobDetail{}, ErrInvalidRequest
	}
	var detail GenerationJobDetail
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update bid_generation_jobs
			set status = 'paused', updated_at = now()
			where tenant_id = $1 and id = $2 and status in ('queued', 'running')
		`, tenantID, jobID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `select exists(select 1 from bid_generation_jobs where tenant_id = $1 and id = $2)`, tenantID, jobID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
		}
		found, err := generationJobDetailByID(ctx, tx, tenantID, jobID)
		if err != nil {
			return err
		}
		detail = found
		return nil
	})
	return detail, err
}

func (s *Store) ResumeGenerationJob(ctx context.Context, tenantID, jobID string) (GenerationJobDetail, error) {
	jobID = strings.TrimSpace(jobID)
	if _, err := uuid.Parse(jobID); err != nil {
		return GenerationJobDetail{}, ErrInvalidRequest
	}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update bid_generation_jobs
			set status = 'running', started_at = coalesce(started_at, now()), updated_at = now()
			where tenant_id = $1 and id = $2 and status = 'paused'
		`, tenantID, jobID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `select exists(select 1 from bid_generation_jobs where tenant_id = $1 and id = $2)`, tenantID, jobID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
		}
		return nil
	})
	if err != nil {
		return GenerationJobDetail{}, err
	}
	if err := s.dispatchNextGenerationStep(ctx, tenantID, jobID); err != nil {
		return GenerationJobDetail{}, err
	}
	return s.GetGenerationJob(ctx, tenantID, jobID)
}

func (s *Store) CancelGenerationJob(ctx context.Context, tenantID, jobID string) (GenerationJobDetail, error) {
	jobID = strings.TrimSpace(jobID)
	if _, err := uuid.Parse(jobID); err != nil {
		return GenerationJobDetail{}, ErrInvalidRequest
	}
	var detail GenerationJobDetail
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update bid_generation_jobs
			set status = 'cancelled',
				completed_at = now(),
				updated_at = now()
			where tenant_id = $1 and id = $2 and status in ('queued', 'running', 'paused')
		`, tenantID, jobID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `select exists(select 1 from bid_generation_jobs where tenant_id = $1 and id = $2)`, tenantID, jobID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
		}
		if _, err := tx.Exec(ctx, `
			update bid_generation_steps
			set status = 'cancelled',
				completed_at = now(),
				updated_at = now()
			where tenant_id = $1 and job_id = $2 and status in ('queued', 'paused')
		`, tenantID, jobID); err != nil {
			return err
		}
		if err := refreshGenerationJob(ctx, tx, tenantID, jobID); err != nil {
			return err
		}
		found, err := generationJobDetailByID(ctx, tx, tenantID, jobID)
		if err != nil {
			return err
		}
		detail = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationJobDetail{}, ErrNotFound
	}
	return detail, err
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
	var task Task
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		knowledgeRefs, err := retrieveKnowledgeRefsForChapter(ctx, tx, tenantID, chapter)
		if err != nil {
			return err
		}
		selectedRefs := make([]string, 0, len(knowledgeRefs))
		for _, ref := range knowledgeRefs {
			selectedRefs = append(selectedRefs, ref.ChunkID)
		}
		requestPayload = chapterGenerateRequest{
			TaskID:                 "task-chapter-" + uuid.NewString(),
			TenantID:               tenantID,
			BidDocumentID:          chapter.BidDocumentID,
			BidPartID:              chapter.BidPartID,
			ChapterID:              chapter.ID,
			ChapterTitle:           chapter.Title,
			TenderRequirements:     []string{"响应招标文件要求", "保留事实性内容引用", "无来源内容标记人工确认"},
			SelectedKnowledgeRefs:  selectedRefs,
			RetrievedKnowledgeRefs: knowledgeRefs,
			CallbackURL:            s.cfg.AICallbackURL,
		}
		payloadJSON, _ := json.Marshal(requestPayload)
		createdTask, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status,
				external_task_id, resource_type, resource_id, payload, route
			)
			values ($1, $2, 'chapter_generate', 'queued', $3, 'bid_chapter', $4, $5, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, requestPayload.TaskID, chapter.ID, payloadJSON))
		if err != nil {
			return err
		}
		task = createdTask
		if _, err := tx.Exec(ctx, `
			update bid_chapters
			set status = 'generating', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapter.ID); err != nil {
			return err
		}
		updated, err := chapterByID(ctx, tx, tenantID, chapter.ID)
		if err != nil {
			return err
		}
		result = ChapterRegenerateResponse{Chapter: updated, Task: task}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterRegenerateResponse{}, ErrNotFound
	}
	if err != nil {
		return ChapterRegenerateResponse{}, err
	}

	accepted, err := s.submitChapterGenerate(ctx, requestPayload)
	if err != nil {
		_ = s.markChapterGenerateFailed(ctx, tenantID, chapterID, task.ID, chapterGenerateSubmitFailureMessage)
		return ChapterRegenerateResponse{}, err
	}
	updated, err := s.bindAcceptedTask(ctx, tenantID, chapterID, task.ID, accepted, requestPayload, nil)
	if err != nil {
		return ChapterRegenerateResponse{}, err
	}
	result.Task = updated
	return result, nil
}

func (s *Store) ChapterAIAction(ctx context.Context, tenantID, userID, chapterID string, req ChapterAIActionRequest) (ChapterRegenerateResponse, error) {
	action := normalizeChapterAction(req.Action)
	if action == "" {
		return ChapterRegenerateResponse{}, ErrInvalidRequest
	}
	var result ChapterRegenerateResponse
	var requestPayload chapterActionRequest
	var task Task
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		knowledgeRefs, err := retrieveKnowledgeRefsForChapter(ctx, tx, tenantID, chapter)
		if err != nil {
			return err
		}
		selectedRefs := make([]string, 0, len(knowledgeRefs))
		for _, ref := range knowledgeRefs {
			selectedRefs = append(selectedRefs, ref.ChunkID)
		}
		requestPayload = chapterActionRequest{
			chapterGenerateRequest: chapterGenerateRequest{
				TaskID:                 "task-chapter-action-" + uuid.NewString(),
				TenantID:               tenantID,
				BidDocumentID:          chapter.BidDocumentID,
				BidPartID:              chapter.BidPartID,
				ChapterID:              chapter.ID,
				ChapterTitle:           chapter.Title,
				TenderRequirements:     []string{"所有事实性内容必须保留引用", "缺少来源的企业资质、人员、证书、金额和日期必须标记人工确认"},
				SelectedKnowledgeRefs:  selectedRefs,
				RetrievedKnowledgeRefs: knowledgeRefs,
				CallbackURL:            s.cfg.AICallbackURL,
			},
			Action:            action,
			Instruction:       strings.TrimSpace(req.Instruction),
			CurrentPlainText:  chapter.PlainText,
			CurrentTiptapJSON: chapter.Content,
		}
		payloadJSON, _ := json.Marshal(requestPayload)
		createdTask, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status,
				external_task_id, resource_type, resource_id, payload, route
			)
			values ($1, $2, 'chapter_ai_action', 'queued', $3, 'bid_chapter', $4, $5, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, requestPayload.TaskID, chapter.ID, payloadJSON))
		if err != nil {
			return err
		}
		task = createdTask
		if _, err := tx.Exec(ctx, `
			update bid_chapters
			set status = 'generating', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapter.ID); err != nil {
			return err
		}
		updated, err := chapterByID(ctx, tx, tenantID, chapter.ID)
		if err != nil {
			return err
		}
		result = ChapterRegenerateResponse{Chapter: updated, Task: task}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterRegenerateResponse{}, ErrNotFound
	}
	if err != nil {
		return ChapterRegenerateResponse{}, err
	}

	accepted, err := s.submitChapterAction(ctx, requestPayload)
	if err != nil {
		_ = s.markChapterGenerateFailed(ctx, tenantID, chapterID, task.ID, chapterActionSubmitFailureMessage)
		return ChapterRegenerateResponse{}, err
	}
	updated, err := s.bindAcceptedTask(ctx, tenantID, chapterID, task.ID, accepted, requestPayload, nil)
	if err != nil {
		return ChapterRegenerateResponse{}, err
	}
	result.Task = updated
	return result, nil
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
	if err := validateExportAttachments(tenantID, req.Attachments, req.BOQFiles); err != nil {
		return CreateExportResponse{}, err
	}
	partCode := "all"
	if exportType == "zip" {
		partCode = "all"
	} else {
		normalizedPartCode, err := normalizeRequestedPartCode(req.PartCode)
		if err != nil {
			return CreateExportResponse{}, err
		}
		partCode = normalizedPartCode
	}

	var export Export
	var task Task
	var payload map[string]any
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		document, err := bidForExport(ctx, tx, tenantID, bidID)
		if err != nil {
			return err
		}
		if err := ensureExportAttachmentObjectKeysReady(ctx, tx, tenantID, req.Attachments, req.BOQFiles); err != nil {
			return err
		}

		exportID := uuid.NewString()
		fileID := uuid.NewString()
		filename := exportFilename(document.Title, partCode, exportType)
		objectKey := platformfile.ObjectKey(tenantID, "bid_export")
		externalTaskID := "task-export-" + strings.ReplaceAll(exportID, "-", "")[:12]
		var bidPartID any
		payload = map[string]any{
			"task_id":      externalTaskID,
			"tenant_id":    tenantID,
			"export_id":    exportID,
			"bid_id":       bidID,
			"bid_title":    document.Title,
			"export_type":  exportType,
			"filename":     filename,
			"object_key":   objectKey,
			"callback_url": s.cfg.AICallbackURL,
		}
		if len(req.Layout) > 0 {
			payload["layout"] = req.Layout
		}
		if len(req.Attachments) > 0 {
			payload["attachments"] = req.Attachments
		}
		if len(req.BOQFiles) > 0 {
			payload["boq_files"] = req.BOQFiles
		}
		if exportType != "zip" {
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
				external_task_id, resource_type, resource_id, payload, route
			)
			values ($1, $2, 'document_export', 'queued', $3, 'bid_export', $4, $5, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, userID, externalTaskID, exportID, payloadJSON))
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
		_ = s.markExportFailed(ctx, tenantID, export.ID, task.ID, documentExportSubmitFailureMessage)
		return CreateExportResponse{}, err
	}
	updated, updateErr := s.bindAcceptedTask(ctx, tenantID, export.ID, task.ID, accepted, payload, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			update bid_exports
			set status = $3, updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, export.ID, accepted.Status)
		return err
	})
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
	var nextJobID string
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
		switch task.ResourceType {
		case "bid_parse_result":
			switch status {
			case "done":
				structured, ok := tenderStructuredResultFromCallback(payload.Result)
				if !ok {
					return ErrInvalidRequest
				}
				structuredJSON, _ := json.Marshal(structured)
				var bidID string
				if err := tx.QueryRow(ctx, `
					update bid_parse_results
					set status = 'ready',
						structured_result = $3,
						error_message = null,
						updated_at = now()
					where tenant_id = $1 and id = $2
					returning bid_document_id::text
				`, payload.TenantID, task.ResourceID, structuredJSON).Scan(&bidID); err != nil {
					return err
				}
				if err := ensureMaterialSelection(ctx, tx, payload.TenantID, bidID, "", defaultMaterialRefs(structured), ""); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `
					update bid_documents
					set status = 'editing', updated_at = now()
					where tenant_id = $1 and id = $2
				`, payload.TenantID, bidID); err != nil {
					return err
				}
			case "failed", "cancelled":
				if _, err := tx.Exec(ctx, `
					update bid_parse_results
					set status = 'failed',
						error_message = nullif($3, ''),
						updated_at = now()
					where tenant_id = $1 and id = $2
				`, payload.TenantID, task.ResourceID, payload.ErrorMessage); err != nil {
					return err
				}
			case "queued", "running":
				parseStatus := "queued"
				if status == "running" {
					parseStatus = "processing"
				}
				if _, err := tx.Exec(ctx, `
					update bid_parse_results
					set status = $3,
						updated_at = now()
					where tenant_id = $1 and id = $2
				`, payload.TenantID, task.ResourceID, parseStatus); err != nil {
					return err
				}
			}
		case "bid_export":
			if status == "done" {
				var exportType string
				if err := tx.QueryRow(ctx, `
					select export_type
					from bid_exports
					where tenant_id = $1 and id = $2
				`, payload.TenantID, task.ResourceID).Scan(&exportType); err != nil {
					return err
				}
				sizeBytes, contentType, err := exportCallbackFileResult(exportType, payload.Result)
				if err != nil {
					return err
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
			if status == "failed" || status == "cancelled" {
				if _, err := tx.Exec(ctx, `
					update file_assets
					set status = 'failed',
						updated_at = now()
					where tenant_id = $1
						and id = (select file_asset_id from bid_exports where tenant_id = $1 and id = $2)
						and status <> 'ready'
				`, payload.TenantID, task.ResourceID); err != nil {
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
		case "bid_chapter":
			applyChapterSideEffects, err := shouldApplyChapterTaskSideEffects(ctx, tx, payload.TenantID, task.ID)
			if err != nil {
				return err
			}
			if status == "done" {
				generation, err := chapterGenerationFromResult(payload.Result)
				if err != nil {
					return err
				}
				changeReason := "ai_regenerate"
				if task.TaskType == "chapter_ai_action" {
					changeReason = "ai_action"
				}
				if applyChapterSideEffects {
					if err := applyChapterGeneration(ctx, tx, payload.TenantID, task.ResourceID, generation, changeReason); err != nil {
						return err
					}
				}
			}
			if applyChapterSideEffects && (status == "failed" || status == "cancelled") {
				if _, err := tx.Exec(ctx, `
					update bid_chapters
					set status = 'needs_fix',
						needs_human_input = case
							when nullif($3, '') is null then needs_human_input
							else jsonb_build_array($3)
						end,
						updated_at = now()
					where tenant_id = $1 and id = $2
				`, payload.TenantID, task.ResourceID, payload.ErrorMessage); err != nil {
					return err
				}
			}
			jobID, shouldDispatch, err := updateGenerationStepForTask(ctx, tx, payload.TenantID, task.ID, status, payload.ErrorMessage)
			if err != nil {
				return err
			}
			if shouldDispatch {
				nextJobID = jobID
			}
		default:
			return ErrInvalidRequest
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err == nil && nextJobID != "" {
		err = s.dispatchNextGenerationStep(ctx, payload.TenantID, nextJobID)
	}
	return task, err
}

func chapterGenerationFromResult(result map[string]any) (chapterGenerateResponse, error) {
	body, err := json.Marshal(result)
	if err != nil {
		return chapterGenerateResponse{}, err
	}
	var generation chapterGenerateResponse
	if err := json.Unmarshal(body, &generation); err != nil {
		return chapterGenerateResponse{}, err
	}
	if generation.TiptapJSON == nil || generation.TraceID == "" {
		return chapterGenerateResponse{}, ErrInvalidRequest
	}
	return generation, nil
}

func shouldApplyChapterTaskSideEffects(ctx context.Context, tx pgx.Tx, tenantID, taskID string) (bool, error) {
	var jobStatus, stepStatus string
	err := tx.QueryRow(ctx, `
		select j.status, s.status
		from bid_generation_steps s
		join bid_generation_jobs j on j.tenant_id = s.tenant_id and j.id = s.job_id
		where s.tenant_id = $1 and s.ai_task_id = $2
	`, tenantID, taskID).Scan(&jobStatus, &stepStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return chapterTaskSideEffectsAllowed("", "", false), nil
	}
	if err != nil {
		return false, err
	}
	return chapterTaskSideEffectsAllowed(jobStatus, stepStatus, true), nil
}

func chapterTaskSideEffectsAllowed(jobStatus, stepStatus string, linkedGenerationStep bool) bool {
	if !linkedGenerationStep {
		return true
	}
	return strings.TrimSpace(strings.ToLower(jobStatus)) != "cancelled" &&
		strings.TrimSpace(strings.ToLower(stepStatus)) != "cancelled"
}

func tenderStructuredResultFromCallback(result map[string]any) (map[string]any, bool) {
	if result == nil {
		return nil, false
	}
	value, ok := result["structured_result"]
	if !ok {
		return nil, false
	}
	if structured, ok := value.(map[string]any); ok {
		return structured, true
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var structured map[string]any
	if err := json.Unmarshal(body, &structured); err != nil {
		return nil, false
	}
	return structured, structured != nil
}

func applyChapterGeneration(ctx context.Context, tx pgx.Tx, tenantID, chapterID string, generation chapterGenerateResponse, changeReason string) error {
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
	if changeReason == "" {
		changeReason = "ai_regenerate"
	}
	if _, err := insertChapterVersion(ctx, tx, tenantID, "", updated, changeReason, generation.ModelMetadata, generation.TokenUsage); err != nil {
		return err
	}
	return replaceKnowledgeReferences(ctx, tx, tenantID, updated, generation.SourceRefs, generation.TraceID)
}

func (s *Store) markChapterGenerateFailed(ctx context.Context, tenantID, chapterID, taskID, message string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update ai_tasks
			set status = 'failed', error_message = $3, completed_at = now(), updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, message); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			update bid_chapters
			set status = 'needs_fix',
				needs_human_input = case
					when nullif($3, '') is null then needs_human_input
					else jsonb_build_array($3)
				end,
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapterID, message)
		return err
	})
}

func (s *Store) bindAcceptedTask(
	ctx context.Context,
	tenantID string,
	resourceID string,
	taskID string,
	accepted aiTaskAccepted,
	payload any,
	after func(context.Context, pgx.Tx) error,
) (Task, error) {
	normalized, err := normalizeAcceptedTask(accepted)
	if err != nil {
		return Task{}, err
	}
	accepted = normalized
	var task Task
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		payloadJSON, _ := json.Marshal(payload)
		routeJSON, _ := json.Marshal(accepted.Route)
		found, err := scanTask(tx.QueryRow(ctx, `
			update ai_tasks
			set status = case
					when status in ('done', 'failed', 'cancelled') then status
					else $4
				end,
				external_task_id = coalesce(external_task_id, $5),
				payload = $6,
				route = $7,
				updated_at = now()
			where tenant_id = $1 and id = $2 and resource_id = $3
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, taskID, resourceID, accepted.Status, accepted.TaskID, payloadJSON, routeJSON))
		if err != nil {
			return err
		}
		task = found
		if after != nil {
			return after(ctx, tx)
		}
		return nil
	})
	return task, err
}

func (s *Store) dispatchNextGenerationStep(ctx context.Context, tenantID, jobID string) error {
	var dispatch *generationDispatch
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var status, bidID, createdBy string
		if err := tx.QueryRow(ctx, `
			select status, bid_document_id::text, coalesce(created_by::text, '')
			from bid_generation_jobs
			where tenant_id = $1 and id = $2
		`, tenantID, jobID).Scan(&status, &bidID, &createdBy); err != nil {
			return err
		}
		if status == "paused" || status == "done" || status == "failed" || status == "cancelled" {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			update bid_generation_jobs
			set status = 'running',
				started_at = coalesce(started_at, now()),
				updated_at = now()
			where tenant_id = $1 and id = $2 and status = 'queued'
		`, tenantID, jobID); err != nil {
			return err
		}
		var hasRunning bool
		if err := tx.QueryRow(ctx, `
			select exists(
				select 1 from bid_generation_steps
				where tenant_id = $1 and job_id = $2 and status = 'running'
			)
		`, tenantID, jobID).Scan(&hasRunning); err != nil {
			return err
		}
		if hasRunning {
			return nil
		}
		var stepID, chapterID string
		if err := tx.QueryRow(ctx, `
			select id::text, chapter_id::text
			from bid_generation_steps
			where tenant_id = $1 and job_id = $2 and status = 'queued'
			order by step_order, created_at
			limit 1
		`, tenantID, jobID).Scan(&stepID, &chapterID); errors.Is(err, pgx.ErrNoRows) {
			return refreshGenerationJob(ctx, tx, tenantID, jobID)
		} else if err != nil {
			return err
		}
		chapter, err := chapterByID(ctx, tx, tenantID, chapterID)
		if err != nil {
			return err
		}
		knowledgeRefs, err := retrieveKnowledgeRefsForChapter(ctx, tx, tenantID, chapter)
		if err != nil {
			return err
		}
		selectedRefs := make([]string, 0, len(knowledgeRefs))
		for _, ref := range knowledgeRefs {
			selectedRefs = append(selectedRefs, ref.ChunkID)
		}
		requestPayload := chapterGenerateRequest{
			TaskID:                 "task-chapter-" + uuid.NewString(),
			TenantID:               tenantID,
			BidDocumentID:          chapter.BidDocumentID,
			BidPartID:              chapter.BidPartID,
			ChapterID:              chapter.ID,
			ChapterTitle:           chapter.Title,
			TenderRequirements:     []string{"响应招标文件要求", "保留事实性内容引用", "无来源内容标记人工确认"},
			SelectedKnowledgeRefs:  selectedRefs,
			RetrievedKnowledgeRefs: knowledgeRefs,
			CallbackURL:            s.cfg.AICallbackURL,
		}
		payloadJSON, _ := json.Marshal(requestPayload)
		createdTask, err := scanTask(tx.QueryRow(ctx, `
			insert into ai_tasks (
				tenant_id, user_id, task_type, status,
				external_task_id, resource_type, resource_id, payload, route
			)
			values ($1, nullif($2, '')::uuid, 'chapter_generate', 'queued', $3, 'bid_chapter', $4, $5, '{}')
			returning id::text, task_type, status, external_task_id::text,
				resource_type, resource_id::text, payload, route, result, error_message,
				started_at, completed_at, created_at, updated_at
		`, tenantID, createdBy, requestPayload.TaskID, chapter.ID, payloadJSON))
		if err != nil {
			return err
		}
		metadataJSON, _ := json.Marshal(map[string]any{
			"job_id":           jobID,
			"bid_document_id":  bidID,
			"chapter_title":    chapter.Title,
			"knowledge_ref_ct": len(knowledgeRefs),
		})
		if _, err := tx.Exec(ctx, `
			update bid_generation_steps
			set status = 'running',
				ai_task_id = $3,
				external_task_id = $4,
				metadata = metadata || $5::jsonb,
				started_at = coalesce(started_at, now()),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, stepID, createdTask.ID, requestPayload.TaskID, metadataJSON); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update bid_chapters
			set status = 'generating', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapter.ID); err != nil {
			return err
		}
		dispatch = &generationDispatch{
			JobID:     jobID,
			StepID:    stepID,
			TaskID:    createdTask.ID,
			ChapterID: chapter.ID,
			Payload:   requestPayload,
		}
		return refreshGenerationJob(ctx, tx, tenantID, jobID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil || dispatch == nil {
		return err
	}
	accepted, err := s.submitChapterGenerate(ctx, dispatch.Payload)
	if err != nil {
		_ = s.markGenerationStepFailed(ctx, tenantID, dispatch.JobID, dispatch.StepID, dispatch.TaskID, dispatch.ChapterID, chapterGenerateSubmitFailureMessage)
		return err
	}
	_, err = s.bindAcceptedTask(ctx, tenantID, dispatch.ChapterID, dispatch.TaskID, accepted, dispatch.Payload, func(ctx context.Context, tx pgx.Tx) error {
		routeJSON, _ := json.Marshal(accepted.Route)
		_, err := tx.Exec(ctx, `
			update bid_generation_steps
			set external_task_id = $3,
				metadata = metadata || jsonb_build_object('route', $4::jsonb),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, dispatch.StepID, accepted.TaskID, routeJSON)
		return err
	})
	return err
}

func (s *Store) markGenerationStepFailed(ctx context.Context, tenantID, jobID, stepID, taskID, chapterID, message string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update ai_tasks
			set status = 'failed', error_message = $3, completed_at = now(), updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, message); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update bid_generation_steps
			set status = 'failed',
				error_message = $3,
				completed_at = now(),
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, stepID, message); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update bid_chapters
			set status = 'needs_fix',
				needs_human_input = case
					when nullif($3, '') is null then needs_human_input
					else jsonb_build_array($3)
				end,
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, chapterID, message); err != nil {
			return err
		}
		return refreshGenerationJob(ctx, tx, tenantID, jobID)
	})
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
		if _, err := tx.Exec(ctx, `
			update file_assets
			set status = 'failed',
				updated_at = now()
			where tenant_id = $1
				and id = (select file_asset_id from bid_exports where tenant_id = $1 and id = $2)
				and status <> 'ready'
		`, tenantID, exportID); err != nil {
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

func (s *Store) markTenderParseFailed(ctx context.Context, tenantID, taskID, parseResultID, message string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update ai_tasks
			set status = 'failed', error_message = $3, completed_at = now(), updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, taskID, message); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			update bid_parse_results
			set status = 'failed',
				error_message = $3,
				updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, parseResultID, message)
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
	if err := aihttp.DecodeJSON(resp.Body, &accepted); err != nil {
		return aiTaskAccepted{}, err
	}
	return normalizeAcceptedTask(accepted)
}

func (s *Store) submitTenderParse(ctx context.Context, payload tenderParseRequest) (aiTaskAccepted, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/tender-parse"
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
	if err := aihttp.DecodeJSON(resp.Body, &accepted); err != nil {
		return aiTaskAccepted{}, err
	}
	return normalizeAcceptedTask(accepted)
}

func (s *Store) submitChapterGenerate(ctx context.Context, payload chapterGenerateRequest) (aiTaskAccepted, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/chapter-generate"
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
	if err := aihttp.DecodeJSON(resp.Body, &accepted); err != nil {
		return aiTaskAccepted{}, err
	}
	return normalizeAcceptedTask(accepted)
}

func (s *Store) submitChapterAction(ctx context.Context, payload chapterActionRequest) (aiTaskAccepted, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return aiTaskAccepted{}, err
	}
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/tasks/chapter-action"
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
	if err := aihttp.DecodeJSON(resp.Body, &accepted); err != nil {
		return aiTaskAccepted{}, err
	}
	return normalizeAcceptedTask(accepted)
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

func retrieveKnowledgeRefsForChapter(ctx context.Context, tx pgx.Tx, tenantID string, chapter Chapter) ([]retrievedKnowledgeRef, error) {
	query := strings.TrimSpace(chapter.Title + " " + chapter.PlainText)
	if len([]rune(query)) > 240 {
		query = string([]rune(query)[:240])
	}
	rows, err := tx.Query(ctx, `
		with candidates as (
			select
				kc.id::text as chunk_id,
				kc.document_id::text as document_id,
				coalesce(nullif(kc.title, ''), d.title) as title,
				kc.section_path,
				kc.content,
				kc.page_start,
				kc.page_end,
				kc.created_at,
				case
					when $2 = '' then false
					else (
						to_tsvector('simple', coalesce(kc.title, '') || ' ' || coalesce(kc.content, '') || ' ' || coalesce(kc.section_path, '')) @@ plainto_tsquery('simple', $2)
						or kc.title ilike '%' || $2 || '%'
						or kc.content ilike '%' || $2 || '%'
						or kc.section_path ilike '%' || $2 || '%'
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
				end as matched,
				case
					when $2 = '' then 0
					else ts_rank(
						to_tsvector('simple', coalesce(kc.title, '') || ' ' || coalesce(kc.content, '') || ' ' || coalesce(kc.section_path, '')),
						plainto_tsquery('simple', $2)
					)
				end as score
			from knowledge_chunks kc
			join knowledge_documents d on d.tenant_id = kc.tenant_id and d.id = kc.document_id
			where kc.tenant_id = $1
				and d.parse_status = 'processed'
				and trim(kc.content) <> ''
		)
		select chunk_id, document_id, title, section_path, content, page_start, page_end, score
		from candidates
		order by matched desc, score desc, created_at desc
		limit 5
	`, tenantID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := []retrievedKnowledgeRef{}
	for rows.Next() {
		var ref retrievedKnowledgeRef
		if err := rows.Scan(
			&ref.ChunkID, &ref.DocumentID, &ref.Title, &ref.SectionPath, &ref.Content,
			&ref.PageStart, &ref.PageEnd, &ref.Score,
		); err != nil {
			return nil, err
		}
		ref.Content = truncateRunes(strings.TrimSpace(ref.Content), 700)
		refs = append(refs, ref)
	}
	return refs, rows.Err()
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

func chaptersForGeneration(ctx context.Context, tx pgx.Tx, tenantID, bidID, partCode string, chapterFilter map[string]bool) ([]Chapter, error) {
	args := []any{tenantID, bidID}
	where := []string{"c.tenant_id = $1", "c.bid_document_id = $2"}
	if partCode != "" {
		args = append(args, partCode)
		where = append(where, fmt.Sprintf("p.code = $%d", len(args)))
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		select c.id::text, c.bid_document_id::text, c.bid_part_id::text, c.title, c.content, c.plain_text,
			c.status, c.sort_order, c.source_refs, c.needs_human_input, c.created_at, c.updated_at
		from bid_chapters c
		join bid_parts p on p.tenant_id = c.tenant_id and p.id = c.bid_part_id
		where %s
		order by p.sort_order, c.sort_order, c.created_at
	`, strings.Join(where, " and ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chapters := []Chapter{}
	for rows.Next() {
		chapter, err := scanChapter(rows)
		if err != nil {
			return nil, err
		}
		if len(chapterFilter) > 0 && !chapterFilter[chapter.ID] {
			continue
		}
		chapters = append(chapters, chapter)
	}
	return chapters, rows.Err()
}

func generationJobDetailByID(ctx context.Context, tx pgx.Tx, tenantID, jobID string) (GenerationJobDetail, error) {
	job, err := scanGenerationJob(tx.QueryRow(ctx, `
		select id::text, bid_document_id::text, scope, status, progress,
			total_steps, completed_steps, failed_steps, model_used,
			prompt_tokens, completion_tokens, error_message, trace_id, created_by::text,
			started_at, completed_at, created_at, updated_at
		from bid_generation_jobs
		where tenant_id = $1 and id = $2
	`, tenantID, jobID))
	if err != nil {
		return GenerationJobDetail{}, err
	}
	steps, err := generationStepsByJobID(ctx, tx, tenantID, jobID)
	if err != nil {
		return GenerationJobDetail{}, err
	}
	return GenerationJobDetail{TaskID: job.ID, Job: job, Steps: steps}, nil
}

func generationStepsByJobID(ctx context.Context, tx pgx.Tx, tenantID, jobID string) ([]GenerationStep, error) {
	rows, err := tx.Query(ctx, `
		select s.id::text, s.job_id::text, s.bid_document_id::text, s.bid_part_id::text,
			s.chapter_id::text, c.title, s.step_order, s.status, s.ai_task_id::text,
			s.external_task_id, s.error_message, s.metadata, s.started_at, s.completed_at,
			s.created_at, s.updated_at
		from bid_generation_steps s
		join bid_chapters c on c.tenant_id = s.tenant_id and c.id = s.chapter_id
		where s.tenant_id = $1 and s.job_id = $2
		order by s.step_order, s.created_at
	`, tenantID, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	steps := []GenerationStep{}
	for rows.Next() {
		step, err := scanGenerationStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func updateGenerationStepForTask(ctx context.Context, tx pgx.Tx, tenantID, taskID, status, message string) (string, bool, error) {
	var jobID string
	err := tx.QueryRow(ctx, `
		update bid_generation_steps
		set status = $3,
			error_message = nullif($4, ''),
			completed_at = case when $3 in ('done', 'failed', 'cancelled') then now() else completed_at end,
			updated_at = now()
		where tenant_id = $1 and ai_task_id = $2
		returning job_id::text
	`, tenantID, taskID, status, message).Scan(&jobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	shouldDispatch, err := refreshGenerationJobAndShouldDispatch(ctx, tx, tenantID, jobID)
	return jobID, shouldDispatch, err
}

func refreshGenerationJob(ctx context.Context, tx pgx.Tx, tenantID, jobID string) error {
	_, err := refreshGenerationJobAndShouldDispatch(ctx, tx, tenantID, jobID)
	return err
}

func refreshGenerationJobAndShouldDispatch(ctx context.Context, tx pgx.Tx, tenantID, jobID string) (bool, error) {
	var currentStatus string
	var total, done, failed, cancelled, running, queued, promptTokens, completionTokens int
	summarySQL := fmt.Sprintf(`
		select
			j.status,
			count(s.id)::int,
			count(*) filter (where s.status = 'done')::int,
			count(*) filter (where s.status = 'failed')::int,
			count(*) filter (where s.status = 'cancelled')::int,
			count(*) filter (where s.status = 'running')::int,
			count(*) filter (where s.status = 'queued')::int,
			%s,
			%s
		from bid_generation_jobs j
		left join bid_generation_steps s on s.tenant_id = j.tenant_id and s.job_id = j.id
		left join ai_tasks t on t.tenant_id = s.tenant_id and t.id = s.ai_task_id
		where j.tenant_id = $1 and j.id = $2
		group by j.id, j.status
	`, tokenUsageSumSQL("input_tokens"), tokenUsageSumSQL("output_tokens"))
	if err := tx.QueryRow(ctx, summarySQL, tenantID, jobID).Scan(&currentStatus, &total, &done, &failed, &cancelled, &running, &queued, &promptTokens, &completionTokens); err != nil {
		return false, err
	}
	progress := 0
	if total > 0 {
		progress = int(float64(done+failed+cancelled) / float64(total) * 100)
	}
	nextStatus := currentStatus
	completedAtExpr := "completed_at"
	if currentStatus != "paused" && currentStatus != "cancelled" {
		switch {
		case failed > 0:
			nextStatus = "failed"
			completedAtExpr = "now()"
		case total > 0 && done+cancelled == total:
			if cancelled > 0 && done == 0 {
				nextStatus = "cancelled"
			} else {
				nextStatus = "done"
			}
			completedAtExpr = "now()"
		case running > 0 || queued > 0:
			nextStatus = "running"
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		update bid_generation_jobs
		set status = $3,
			progress = $4,
			completed_steps = $5,
			failed_steps = $6,
			prompt_tokens = $7,
			completion_tokens = $8,
			completed_at = %s,
			updated_at = now()
		where tenant_id = $1 and id = $2
	`, completedAtExpr), tenantID, jobID, nextStatus, progress, done, failed, promptTokens, completionTokens); err != nil {
		return false, err
	}
	shouldDispatch := currentStatus != "paused" && currentStatus != "cancelled" && failed == 0 && running == 0 && queued > 0
	return shouldDispatch, nil
}

func tokenUsageSumSQL(field string) string {
	switch field {
	case "input_tokens", "output_tokens":
	default:
		return "0::int"
	}
	value := fmt.Sprintf("trim(coalesce(t.result->'token_usage'->>'%s', ''))", field)
	return fmt.Sprintf(
		"least(coalesce(sum(case when %[1]s ~ '^[0-9]{1,12}$' then %[1]s::bigint else 0 end), 0), 2147483647)::int",
		value,
	)
}

type outlinePartSpec struct {
	Code      string
	Title     string
	SortOrder int
	Chapters  []outlineChapterSpec
}

type outlineChapterSpec struct {
	Title     string
	PlainText string
	SortOrder int
}

func latestTenderFileForBid(ctx context.Context, tx pgx.Tx, tenantID, bidID string) (TenderFile, error) {
	return scanTenderFile(tx.QueryRow(ctx, `
		select btf.id::text, btf.bid_document_id::text, btf.file_asset_id::text,
			f.object_key, f.filename, f.content_type, f.size_bytes,
			btf.status, btf.created_at, btf.updated_at
		from bid_tender_files btf
		join file_assets f on f.tenant_id = btf.tenant_id and f.id = btf.file_asset_id
		where btf.tenant_id = $1 and btf.bid_document_id = $2 and btf.status = 'active'
		order by btf.updated_at desc
		limit 1
	`, tenantID, bidID))
}

func parseResultForBid(ctx context.Context, tx pgx.Tx, tenantID, bidID string) (ParseResult, error) {
	return scanParseResult(tx.QueryRow(ctx, `
		select id::text, bid_document_id::text, file_asset_id::text, status,
			structured_result, error_message, confirmed_by::text, confirmed_at, created_at, updated_at
		from bid_parse_results
		where tenant_id = $1 and bid_document_id = $2
	`, tenantID, bidID))
}

func upsertParseResult(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	bidID string,
	fileID string,
	status string,
	structured map[string]any,
	errorMessage string,
) (ParseResult, error) {
	if structured == nil {
		structured = map[string]any{}
	}
	if status == "" {
		status = "queued"
	}
	body, _ := json.Marshal(structured)
	return scanParseResult(tx.QueryRow(ctx, `
		insert into bid_parse_results (
			tenant_id, bid_document_id, file_asset_id, status, structured_result, error_message
		)
		values ($1, $2, nullif($3, '')::uuid, $4, $5, nullif($6, ''))
		on conflict (tenant_id, bid_document_id) do update
		set file_asset_id = coalesce(excluded.file_asset_id, bid_parse_results.file_asset_id),
			status = excluded.status,
			structured_result = excluded.structured_result,
			error_message = excluded.error_message,
			updated_at = now()
		returning id::text, bid_document_id::text, file_asset_id::text, status,
			structured_result, error_message, confirmed_by::text, confirmed_at, created_at, updated_at
	`, tenantID, bidID, fileID, status, body, errorMessage))
}

func partByID(ctx context.Context, tx pgx.Tx, tenantID, bidID, partID string) (Part, error) {
	return scanPart(tx.QueryRow(ctx, `
		select id::text, bid_document_id::text, code, title, sort_order, status, metadata, created_at, updated_at
		from bid_parts
		where tenant_id = $1 and bid_document_id = $2 and id = $3
	`, tenantID, bidID, partID))
}

func chaptersForPart(ctx context.Context, tx pgx.Tx, tenantID, bidID, partID string) ([]Chapter, error) {
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

func materialSelectionForBid(ctx context.Context, tx pgx.Tx, tenantID, bidID string) (MaterialSelection, error) {
	return scanMaterialSelection(tx.QueryRow(ctx, `
		select id::text, bid_document_id::text, selected_refs, notes, updated_by::text, created_at, updated_at
		from bid_material_selections
		where tenant_id = $1 and bid_document_id = $2
	`, tenantID, bidID))
}

func ensureMaterialSelection(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	bidID string,
	userID string,
	selectedRefs []any,
	notes string,
) error {
	if selectedRefs == nil {
		selectedRefs = []any{}
	}
	body, _ := json.Marshal(selectedRefs)
	_, err := tx.Exec(ctx, `
		insert into bid_material_selections (tenant_id, bid_document_id, selected_refs, notes, updated_by)
		values ($1, $2, $3, $4, nullif($5, '')::uuid)
		on conflict (tenant_id, bid_document_id) do update
		set selected_refs = excluded.selected_refs,
			notes = case when excluded.notes = '' then bid_material_selections.notes else excluded.notes end,
			updated_by = coalesce(excluded.updated_by, bid_material_selections.updated_by),
			updated_at = now()
	`, tenantID, bidID, body, strings.TrimSpace(notes), userID)
	return err
}

func defaultTenderStructuredResult(document Document, file TenderFile) map[string]any {
	outline := defaultOutlineSpecs(document)
	parts := make([]any, 0, len(outline))
	for _, part := range outline {
		chapters := make([]any, 0, len(part.Chapters))
		for _, chapter := range part.Chapters {
			chapters = append(chapters, map[string]any{
				"title":      chapter.Title,
				"plain_text": chapter.PlainText,
				"sort_order": chapter.SortOrder,
			})
		}
		parts = append(parts, map[string]any{
			"code":       part.Code,
			"title":      part.Title,
			"sort_order": part.SortOrder,
			"chapters":   chapters,
		})
	}
	sourceFile := map[string]any{}
	if file.FileAssetID != "" {
		sourceFile = map[string]any{
			"file_asset_id": file.FileAssetID,
			"object_key":    file.ObjectKey,
			"filename":      file.Filename,
			"content_type":  file.ContentType,
			"size_bytes":    file.SizeBytes,
		}
	}
	return map[string]any{
		"project_name": document.Title,
		"bid_type":     document.BidType,
		"source_file":  sourceFile,
		"deadline":     time.Now().UTC().AddDate(0, 0, 14).Format("2006-01-02"),
		"qualification_requirements": []any{
			"营业执照、法定代表人授权及签章文件齐备",
			"具备类似项目实施经验或信息系统建设能力证明",
			"提供项目团队、服务承诺和安全管理相关材料",
		},
		"invalid_clause_risks": []any{
			"签章、报价、投标有效期不一致可能导致无效投标",
			"资格证明材料缺失或过期需要人工复核",
			"技术响应未逐条覆盖评分点会影响得分",
		},
		"scoring_points": []any{
			"实施方案完整性",
			"项目团队与案例经验",
			"数据安全和运维服务能力",
		},
		"outline": map[string]any{"parts": parts},
		"material_suggestions": []any{
			map[string]any{"title": "企业资质证书", "ref_type": "qualification", "reason": "响应资格审查要求", "selected": true},
			map[string]any{"title": "同类项目案例", "ref_type": "case", "reason": "支撑评分项中的经验能力", "selected": true},
			map[string]any{"title": "技术方案素材", "ref_type": "solution", "reason": "复用实施方案和安全保障描述", "selected": true},
		},
	}
}

func outlineSpecsFromStructuredResult(document Document, structured map[string]any) []outlinePartSpec {
	if structured != nil {
		if outline, ok := structured["outline"].(map[string]any); ok {
			if rawParts, ok := outline["parts"].([]any); ok {
				specs := make([]outlinePartSpec, 0, len(rawParts))
				for index, rawPart := range rawParts {
					partMap, ok := rawPart.(map[string]any)
					if !ok {
						continue
					}
					code := normalizePartCode(asString(partMap["code"]))
					title := strings.TrimSpace(asString(partMap["title"]))
					if title == "" {
						title = defaultPartTitle(code)
					}
					sortOrder := asInt(partMap["sort_order"])
					if sortOrder <= 0 {
						sortOrder = (index + 1) * 10
					}
					chapters := outlineChaptersFromAny(partMap["chapters"])
					if len(chapters) == 0 {
						chapters = defaultOutlineChapters(code)
					}
					specs = append(specs, outlinePartSpec{Code: code, Title: title, SortOrder: sortOrder, Chapters: chapters})
				}
				if len(specs) > 0 {
					return specs
				}
			}
		}
	}
	return defaultOutlineSpecs(document)
}

func defaultOutlineSpecs(document Document) []outlinePartSpec {
	if document.BidType == "separated" {
		return []outlinePartSpec{
			{Code: "tech", Title: "技术标", SortOrder: 10, Chapters: defaultOutlineChapters("tech")},
			{Code: "business", Title: "商务标", SortOrder: 20, Chapters: defaultOutlineChapters("business")},
		}
	}
	return []outlinePartSpec{
		{Code: "combined_body", Title: "综合标书主体", SortOrder: 10, Chapters: defaultOutlineChapters("combined_body")},
	}
}

func defaultOutlineChapters(partCode string) []outlineChapterSpec {
	switch partCode {
	case "business":
		return []outlineChapterSpec{
			{Title: "一、投标函", PlainText: "响应招标文件商务条款，确认投标有效期、报价和签章要求。", SortOrder: 10},
			{Title: "二、资格证明文件", PlainText: "整理营业执照、授权书、资质证书和承诺函。", SortOrder: 20},
			{Title: "三、商务偏离表", PlainText: "逐条核对付款、服务期、验收和违约责任条款。", SortOrder: 30},
		}
	case "tech":
		return []outlineChapterSpec{
			{Title: "一、项目理解", PlainText: "提炼建设目标、范围边界、关键约束和响应策略。", SortOrder: 10},
			{Title: "二、总体技术方案", PlainText: "描述系统架构、数据流程、安全设计和集成方式。", SortOrder: 20},
			{Title: "三、实施计划与保障", PlainText: "说明项目组织、里程碑、质量控制和运维服务。", SortOrder: 30},
		}
	default:
		return []outlineChapterSpec{
			{Title: "一、项目理解", PlainText: "提炼招标需求、评分重点和废标风险。", SortOrder: 10},
			{Title: "二、实施方案", PlainText: "响应技术路线、交付计划、团队安排和服务承诺。", SortOrder: 20},
			{Title: "三、商务响应", PlainText: "汇总报价、资格证明、合同条款和偏离说明。", SortOrder: 30},
		}
	}
}

func outlineChaptersFromAny(value any) []outlineChapterSpec {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	chapters := make([]outlineChapterSpec, 0, len(raw))
	for index, item := range raw {
		sortOrder := (index + 1) * 10
		switch typed := item.(type) {
		case string:
			title := strings.TrimSpace(typed)
			if title != "" {
				chapters = append(chapters, outlineChapterSpec{Title: title, PlainText: "请补充" + title + "内容。", SortOrder: sortOrder})
			}
		case map[string]any:
			title := strings.TrimSpace(asString(typed["title"]))
			if title == "" {
				continue
			}
			if parsed := asInt(typed["sort_order"]); parsed > 0 {
				sortOrder = parsed
			}
			plainText := strings.TrimSpace(asString(typed["plain_text"]))
			if plainText == "" {
				plainText = "请补充" + title + "内容。"
			}
			chapters = append(chapters, outlineChapterSpec{Title: title, PlainText: plainText, SortOrder: sortOrder})
		}
	}
	return chapters
}

func applyOutlineSpecs(ctx context.Context, tx pgx.Tx, tenantID, bidID string, specs []outlinePartSpec) error {
	for _, spec := range specs {
		code := normalizePartCode(spec.Code)
		title := strings.TrimSpace(spec.Title)
		if title == "" {
			title = defaultPartTitle(code)
		}
		metadataJSON, _ := json.Marshal(map[string]any{
			"outline_generated": true,
			"outline_source":    "parse_result",
		})
		var partID string
		if err := tx.QueryRow(ctx, `
			insert into bid_parts (tenant_id, bid_document_id, code, title, sort_order, status, metadata)
			values ($1, $2, $3, $4, $5, 'generated', $6)
			on conflict (tenant_id, bid_document_id, code) do update
			set title = excluded.title,
				sort_order = excluded.sort_order,
				status = 'generated',
				metadata = bid_parts.metadata || excluded.metadata,
				updated_at = now()
			returning id::text
		`, tenantID, bidID, code, title, spec.SortOrder, metadataJSON).Scan(&partID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			delete from bid_chapters
			where tenant_id = $1 and bid_document_id = $2 and bid_part_id = $3
		`, tenantID, bidID, partID); err != nil {
			return err
		}
		for index, chapter := range spec.Chapters {
			chapterTitle := strings.TrimSpace(chapter.Title)
			if chapterTitle == "" {
				continue
			}
			sortOrder := chapter.SortOrder
			if sortOrder <= 0 {
				sortOrder = (index + 1) * 10
			}
			plainText := strings.TrimSpace(chapter.PlainText)
			if plainText == "" {
				plainText = "请补充" + chapterTitle + "内容。"
			}
			contentJSON, _ := json.Marshal(tiptapFromPlainText(plainText))
			if _, err := tx.Exec(ctx, `
				insert into bid_chapters (
					tenant_id, bid_document_id, bid_part_id, title,
					content, plain_text, status, sort_order
				)
				values ($1, $2, $3, $4, $5, $6, 'pending', $7)
			`, tenantID, bidID, partID, chapterTitle, contentJSON, plainText, sortOrder); err != nil {
				return err
			}
		}
	}
	return nil
}

func outlineChapterCount(specs []outlinePartSpec) int {
	total := 0
	for _, spec := range specs {
		total += len(spec.Chapters)
	}
	return total
}

func defaultMaterialRefs(structured map[string]any) []any {
	if structured != nil {
		if refs, ok := structured["material_suggestions"].([]any); ok {
			return refs
		}
	}
	return []any{}
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
		var chunkID any
		title := strings.TrimSpace(ref.Title)
		if parsedChunkID, err := uuid.Parse(ref.ChunkID); err == nil {
			var resolvedDocumentID, resolvedTitle string
			err := tx.QueryRow(ctx, `
				select kc.document_id::text, coalesce(nullif(kc.title, ''), d.title)
				from knowledge_chunks kc
				join knowledge_documents d on d.tenant_id = kc.tenant_id and d.id = kc.document_id
				where kc.tenant_id = $1 and kc.id = $2
			`, tenantID, parsedChunkID.String()).Scan(&resolvedDocumentID, &resolvedTitle)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				sourceDocumentID = resolvedDocumentID
				chunkID = parsedChunkID.String()
				metadata["source_ref"].(map[string]any)["resolved"] = true
				metadata["source_ref"].(map[string]any)["resolved_by"] = "knowledge_chunk"
				if title == "" {
					title = resolvedTitle
				}
			}
		}
		if sourceDocumentID == nil {
			if parsed, err := uuid.Parse(ref.DocumentID); err == nil {
				var exists bool
				if err := tx.QueryRow(ctx, `
					select exists(select 1 from knowledge_documents where tenant_id = $1 and id = $2)
				`, tenantID, parsed.String()).Scan(&exists); err != nil {
					return err
				}
				if exists {
					sourceDocumentID = parsed.String()
					metadata["source_ref"].(map[string]any)["resolved_by"] = "knowledge_document"
				}
			}
		}
		if title == "" {
			title = "知识库引用"
		}
		metadataJSON, _ := json.Marshal(metadata)
		if _, err := tx.Exec(ctx, `
			insert into knowledge_references (
				tenant_id, source_document_id, bid_document_id, chapter_id, chunk_id, title, metadata
			)
			values ($1, $2, $3, $4, $5, $6, $7)
		`, tenantID, sourceDocumentID, chapter.BidDocumentID, chapter.ID, chunkID, title, metadataJSON); err != nil {
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

func scanBidTemplate(row scanner) (BidTemplate, error) {
	var template BidTemplate
	var contentRaw []byte
	err := row.Scan(
		&template.ID, &template.Name, &template.BidType, &template.Category, &template.Description,
		&template.Version, &contentRaw, &template.UsageCount, &template.Status, &template.CreatedAt, &template.UpdatedAt,
	)
	template.Content = map[string]any{}
	if len(contentRaw) > 0 {
		_ = json.Unmarshal(contentRaw, &template.Content)
	}
	return template, err
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
	return task, err
}

func scanGenerationJob(row scanner) (GenerationJob, error) {
	var job GenerationJob
	var errorMessage, createdBy sql.NullString
	var startedAt, completedAt sql.NullTime
	err := row.Scan(
		&job.ID, &job.BidDocumentID, &job.Scope, &job.Status, &job.Progress,
		&job.TotalSteps, &job.CompletedSteps, &job.FailedSteps, &job.ModelUsed,
		&job.PromptTokens, &job.CompletionTokens, &errorMessage, &job.TraceID,
		&createdBy, &startedAt, &completedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if errorMessage.Valid {
		job.ErrorMessage = &errorMessage.String
	}
	if createdBy.Valid {
		job.CreatedBy = &createdBy.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	return job, err
}

func scanGenerationStep(row scanner) (GenerationStep, error) {
	var step GenerationStep
	var aiTaskID, externalTaskID, errorMessage sql.NullString
	var metadataRaw []byte
	var startedAt, completedAt sql.NullTime
	err := row.Scan(
		&step.ID, &step.JobID, &step.BidDocumentID, &step.BidPartID,
		&step.ChapterID, &step.ChapterTitle, &step.StepOrder, &step.Status,
		&aiTaskID, &externalTaskID, &errorMessage, &metadataRaw,
		&startedAt, &completedAt, &step.CreatedAt, &step.UpdatedAt,
	)
	if aiTaskID.Valid {
		step.AITaskID = &aiTaskID.String
	}
	if externalTaskID.Valid {
		step.ExternalTaskID = &externalTaskID.String
	}
	if errorMessage.Valid {
		step.ErrorMessage = &errorMessage.String
	}
	step.Metadata = map[string]any{}
	_ = json.Unmarshal(metadataRaw, &step.Metadata)
	if startedAt.Valid {
		step.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		step.CompletedAt = &completedAt.Time
	}
	return step, err
}

func scanTenderFile(row scanner) (TenderFile, error) {
	var file TenderFile
	err := row.Scan(
		&file.ID, &file.BidDocumentID, &file.FileAssetID,
		&file.ObjectKey, &file.Filename, &file.ContentType, &file.SizeBytes,
		&file.Status, &file.CreatedAt, &file.UpdatedAt,
	)
	return file, err
}

func scanParseResult(row scanner) (ParseResult, error) {
	var result ParseResult
	var fileAssetID, errorMessage, confirmedBy sql.NullString
	var confirmedAt sql.NullTime
	var structuredRaw []byte
	err := row.Scan(
		&result.ID, &result.BidDocumentID, &fileAssetID, &result.Status,
		&structuredRaw, &errorMessage, &confirmedBy, &confirmedAt, &result.CreatedAt, &result.UpdatedAt,
	)
	if fileAssetID.Valid {
		result.FileAssetID = &fileAssetID.String
	}
	if errorMessage.Valid {
		result.ErrorMessage = &errorMessage.String
	}
	if confirmedBy.Valid {
		result.ConfirmedBy = &confirmedBy.String
	}
	if confirmedAt.Valid {
		result.ConfirmedAt = &confirmedAt.Time
	}
	result.StructuredResult = map[string]any{}
	_ = json.Unmarshal(structuredRaw, &result.StructuredResult)
	return result, err
}

func scanMaterialSelection(row scanner) (MaterialSelection, error) {
	var selection MaterialSelection
	var selectedRaw []byte
	var updatedBy sql.NullString
	err := row.Scan(
		&selection.ID, &selection.BidDocumentID, &selectedRaw, &selection.Notes,
		&updatedBy, &selection.CreatedAt, &selection.UpdatedAt,
	)
	if updatedBy.Valid {
		selection.UpdatedBy = &updatedBy.String
	}
	selection.SelectedRefs = []any{}
	_ = json.Unmarshal(selectedRaw, &selection.SelectedRefs)
	return selection, err
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

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func asInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
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

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
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

func normalizeBidType(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "combined", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "combined", "separated", "custom":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeDocumentStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft", "generating", "editing", "in_review", "approved", "submitted", "archived":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeExportType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "docx"
	case "docx", "pdf", "zip":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func validateExportAttachments(tenantID string, groups ...[]map[string]any) error {
	if strings.TrimSpace(tenantID) == "" {
		return ErrInvalidRequest
	}
	attachmentCount := 0
	inlineBytesTotal := 0
	for _, attachments := range groups {
		for _, attachment := range attachments {
			attachmentCount++
			if attachmentCount > maxExportAttachmentCount {
				return ErrInvalidRequest
			}
			if attachment == nil {
				return ErrInvalidRequest
			}
			filename, filenamePresent, filenameOK := exportStringField(attachment, "filename")
			if !filenameOK || !filenamePresent || filename == "" {
				return ErrInvalidRequest
			}
			for _, optionalStringField := range []string{"category", "content_type", "zip_path"} {
				if _, _, ok := exportStringField(attachment, optionalStringField); !ok {
					return ErrInvalidRequest
				}
			}
			if localPath, present, ok := exportStringField(attachment, "local_path"); !ok || (present && localPath != "") {
				return ErrInvalidRequest
			}
			objectKey, objectPresent, objectOK := exportRawStringField(attachment, "object_key")
			if !objectOK {
				return ErrInvalidRequest
			}
			contentBase64, contentPresent, contentOK := exportStringField(attachment, "content_base64")
			if !contentOK {
				return ErrInvalidRequest
			}
			hasObjectKey := objectPresent && objectKey != ""
			hasInlineContent := contentPresent && contentBase64 != ""
			if hasObjectKey && !validTenantObjectKey(tenantID, objectKey) {
				return ErrInvalidRequest
			}
			if hasObjectKey == hasInlineContent {
				return ErrInvalidRequest
			}
			if hasInlineContent {
				contentSize, err := validateExportInlineAttachmentContent(contentBase64)
				if err != nil {
					return err
				}
				inlineBytesTotal += contentSize
				if inlineBytesTotal > maxExportInlineAttachmentTotalBytes {
					return ErrInvalidRequest
				}
			}
		}
	}
	return nil
}

func validTenantObjectKey(tenantID, objectKey string) bool {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" || strings.ContainsAny(tenantID, `/\`) {
		return false
	}
	if objectKey == "" || strings.TrimSpace(objectKey) != objectKey {
		return false
	}
	if strings.HasPrefix(objectKey, "/") || strings.Contains(objectKey, `\`) || strings.Contains(objectKey, "://") {
		return false
	}
	parts := strings.Split(objectKey, "/")
	if len(parts) < 2 || parts[0] != tenantID {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func validateExportInlineAttachmentContent(contentBase64 string) (int, error) {
	if len(contentBase64) > base64.StdEncoding.EncodedLen(maxExportInlineAttachmentBytes) {
		return 0, ErrInvalidRequest
	}
	content, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil || len(content) == 0 || len(content) > maxExportInlineAttachmentBytes {
		return 0, ErrInvalidRequest
	}
	return len(content), nil
}

func ensureExportAttachmentObjectKeysReady(ctx context.Context, tx pgx.Tx, tenantID string, groups ...[]map[string]any) error {
	for _, objectKey := range exportAttachmentObjectKeys(groups...) {
		var exists bool
		if err := tx.QueryRow(ctx, `
			select exists(
				select 1
				from file_assets
				where tenant_id = $1 and object_key = $2 and status = 'ready'
			)
		`, tenantID, objectKey).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrInvalidRequest
		}
	}
	return nil
}

func exportAttachmentObjectKeys(groups ...[]map[string]any) []string {
	objectKeys := []string{}
	seen := map[string]struct{}{}
	for _, attachments := range groups {
		for _, attachment := range attachments {
			objectKey, _, ok := exportRawStringField(attachment, "object_key")
			if !ok || objectKey == "" {
				continue
			}
			if _, exists := seen[objectKey]; exists {
				continue
			}
			seen[objectKey] = struct{}{}
			objectKeys = append(objectKeys, objectKey)
		}
	}
	return objectKeys
}

func exportStringField(values map[string]any, key string) (string, bool, bool) {
	value, present := values[key]
	if !present || value == nil {
		return "", false, true
	}
	text, ok := value.(string)
	if !ok {
		return "", true, false
	}
	return strings.TrimSpace(text), true, true
}

func exportRawStringField(values map[string]any, key string) (string, bool, bool) {
	value, present := values[key]
	if !present || value == nil {
		return "", false, true
	}
	text, ok := value.(string)
	if !ok {
		return "", true, false
	}
	return text, true, true
}

func normalizePartCode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tech", "business", "boq", "attachment":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "combined_body"
	}
}

func normalizeRequestedPartCode(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "combined_body", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "combined_body", "tech", "business", "boq", "attachment":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeGenerationScope(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "full", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full", "part", "chapter":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeGenerationPartCode(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "combined_body", "tech", "business", "boq", "attachment":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", ErrInvalidRequest
	}
}

func normalizeChapterAction(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "optimize", "expand", "shorten", "add_detail", "self_check":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func defaultPartTitle(code string) string {
	switch code {
	case "tech":
		return "技术标"
	case "business":
		return "商务标"
	case "boq":
		return "工程量清单"
	case "attachment":
		return "附件"
	default:
		return "综合标书主体"
	}
}

func contentTypeForExport(exportType string) string {
	if exportType == "zip" {
		return zipContentType
	}
	if exportType == "pdf" {
		return pdfContentType
	}
	return docxContentType
}

func exportCallbackFileResult(exportType string, result map[string]any) (int64, string, error) {
	sizeBytes, ok := exportCallbackSizeBytes(result["size_bytes"])
	if !ok {
		return 0, "", ErrInvalidRequest
	}
	expectedContentType := contentTypeForExport(exportType)
	contentType := expectedContentType
	if rawContentType, exists := result["content_type"]; exists && rawContentType != nil {
		value, ok := rawContentType.(string)
		if !ok {
			return 0, "", ErrInvalidRequest
		}
		contentType = strings.TrimSpace(value)
		if contentType == "" {
			contentType = expectedContentType
		}
	}
	if !strings.EqualFold(contentType, expectedContentType) {
		return 0, "", ErrInvalidRequest
	}
	return sizeBytes, expectedContentType, nil
}

func exportCallbackSizeBytes(value any) (int64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, true
	case int:
		if typed < 0 {
			return 0, false
		}
		return int64(typed), true
	case int32:
		if typed < 0 {
			return 0, false
		}
		return int64(typed), true
	case int64:
		if typed < 0 {
			return 0, false
		}
		return typed, true
	case float64:
		if typed < 0 || typed > float64(maxExactJSONInteger) || math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
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
