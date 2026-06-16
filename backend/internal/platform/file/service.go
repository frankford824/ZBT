package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	ErrInvalidRequest     = errors.New("invalid file request")
	ErrNotFound           = errors.New("file asset not found")
	ErrObjectNotUploaded  = errors.New("file object is not uploaded")
	ErrInvalidObjectState = errors.New("file object is not ready")
)

const presignTTL = 15 * time.Minute

const (
	defaultUploadBizType    = "knowledge"
	defaultGeneratedBizType = "generated"
	maxUploadSizeBytes      = 200 * 1024 * 1024
	maxContentTypeBytes     = 255
)

var bizTypeAccessModules = map[string]string{
	"knowledge":      "knowledge",
	"knowledge_case": "knowledge",
	"generated":      "knowledge",
	"bid_tender":     "bid",
	"bid_export":     "bid",
}

type Service struct {
	pool     *pgxpool.Pool
	bucket   string
	internal *minio.Client
	public   *minio.Client
}

type Asset struct {
	ID          string    `json:"id"`
	BizType     string    `json:"biz_type"`
	BizID       *string   `json:"biz_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	objectKey   string
}

type PresignUploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	BizType     string `json:"biz_type"`
	BizID       string `json:"biz_id"`
}

type PresignUploadResponse struct {
	File      Asset             `json:"file"`
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type PresignedURLResponse struct {
	File      Asset     `json:"file"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type GeneratedAssetRequest struct {
	Filename    string
	ContentType string
	Content     []byte
	BizType     string
	BizID       string
}

func NewService(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) (*Service, error) {
	internal, err := newClient(cfg.MinIOEndpoint, cfg)
	if err != nil {
		return nil, err
	}
	publicEndpoint := cfg.MinIOPublicEndpoint
	if publicEndpoint == "" {
		publicEndpoint = cfg.MinIOEndpoint
	}
	public, err := newClient(publicEndpoint, cfg)
	if err != nil {
		return nil, err
	}
	service := &Service{
		pool:     pool,
		bucket:   cfg.MinIOBucket,
		internal: internal,
		public:   public,
	}
	if err := service.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return service, nil
}

func ObjectKey(tenantID, bizType string) string {
	return fmt.Sprintf("%s/%s/%s", tenantID, bizType, uuid.NewString())
}

func GeneratedObjectKey(tenantID, bizType, bizID string) string {
	bizType = sanitizeSegment(bizType)
	if bizType == "" {
		bizType = "generated"
	}
	bizID = strings.TrimSpace(bizID)
	if bizID == "" {
		return ObjectKey(tenantID, bizType)
	}
	return fmt.Sprintf("%s/%s/%s", tenantID, bizType, bizID)
}

func (s *Service) PresignUpload(ctx context.Context, tenantID, userID string, req PresignUploadRequest) (PresignUploadResponse, error) {
	bizType, err := normalizeBizType(req.BizType, defaultUploadBizType)
	if err != nil {
		return PresignUploadResponse{}, err
	}
	bizID, err := normalizeOptionalUUID(req.BizID)
	if err != nil {
		return PresignUploadResponse{}, err
	}
	if err := validateUploadSize(req.SizeBytes); err != nil {
		return PresignUploadResponse{}, ErrInvalidRequest
	}
	filename := sanitizeFilename(req.Filename)
	if filename == "" {
		return PresignUploadResponse{}, ErrInvalidRequest
	}
	contentType, err := normalizeContentType(req.ContentType)
	if err != nil {
		return PresignUploadResponse{}, err
	}

	objectKey := ObjectKey(tenantID, bizType)
	uploadURL, err := s.public.PresignedPutObject(ctx, s.bucket, objectKey, presignTTL)
	if err != nil {
		return PresignUploadResponse{}, err
	}

	asset := Asset{}
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanAsset(tx.QueryRow(ctx, `
			insert into file_assets (
				tenant_id, owner_user_id, biz_type, biz_id,
				object_key, filename, content_type, size_bytes, status
			)
			values ($1, $2, $3, nullif($4, '')::uuid, $5, $6, $7, $8, 'pending')
			returning
				id::text, biz_type, coalesce(biz_id::text, ''), object_key,
				filename, content_type, size_bytes, status, created_at, updated_at
		`, tenantID, userID, bizType, bizID, objectKey, filename, contentType, req.SizeBytes))
		if err != nil {
			return err
		}
		asset = created
		return nil
	})
	if err != nil {
		return PresignUploadResponse{}, err
	}

	return PresignUploadResponse{
		File:      asset,
		UploadURL: uploadURL.String(),
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: time.Now().Add(presignTTL).UTC(),
	}, nil
}

func (s *Service) CreateGeneratedAsset(ctx context.Context, tenantID, userID string, req GeneratedAssetRequest) (Asset, error) {
	bizType, err := normalizeBizType(req.BizType, defaultGeneratedBizType)
	if err != nil {
		return Asset{}, err
	}
	bizID, err := normalizeOptionalUUID(req.BizID)
	if err != nil {
		return Asset{}, err
	}
	filename := sanitizeFilename(req.Filename)
	if filename == "" || len(req.Content) == 0 {
		return Asset{}, ErrInvalidRequest
	}
	if err := validateUploadSize(int64(len(req.Content))); err != nil {
		return Asset{}, err
	}
	contentType, err := normalizeContentType(req.ContentType)
	if err != nil {
		return Asset{}, err
	}
	objectKey := GeneratedObjectKey(tenantID, bizType, bizID)
	if _, err := s.internal.PutObject(ctx, s.bucket, objectKey, bytes.NewReader(req.Content), int64(len(req.Content)), minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return Asset{}, err
	}
	var asset Asset
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanAsset(tx.QueryRow(ctx, `
			insert into file_assets (
				tenant_id, owner_user_id, biz_type, biz_id,
				object_key, filename, content_type, size_bytes, status, confirmed_at
			)
			values ($1, $2, $3, nullif($4, '')::uuid, $5, $6, $7, $8, 'ready', now())
			on conflict (object_key)
			do update set
				owner_user_id = excluded.owner_user_id,
				biz_type = excluded.biz_type,
				biz_id = excluded.biz_id,
				filename = excluded.filename,
				content_type = excluded.content_type,
				size_bytes = excluded.size_bytes,
				status = 'ready',
				confirmed_at = now(),
				updated_at = now()
			returning
				id::text, biz_type, coalesce(biz_id::text, ''), object_key,
				filename, content_type, size_bytes, status, created_at, updated_at
			`, tenantID, userID, bizType, bizID, objectKey, filename, contentType, int64(len(req.Content))))
		if err != nil {
			return err
		}
		asset = created
		return nil
	})
	if err != nil {
		return Asset{}, err
	}
	return asset, nil
}

func (s *Service) ConfirmUpload(ctx context.Context, tenantID, fileID string) (Asset, error) {
	asset, err := s.assetByID(ctx, tenantID, fileID)
	if err != nil {
		return Asset{}, err
	}
	if asset.Status == "ready" {
		return asset, nil
	}
	if asset.Status != "pending" {
		return Asset{}, ErrInvalidObjectState
	}

	info, err := s.internal.StatObject(ctx, s.bucket, asset.objectKey, minio.StatObjectOptions{})
	if err != nil {
		if isObjectMissing(err) {
			return Asset{}, ErrObjectNotUploaded
		}
		return Asset{}, err
	}
	if info.ContentType != "" {
		asset.ContentType = info.ContentType
	}
	asset.SizeBytes = info.Size
	if err := validateUploadSize(asset.SizeBytes); err != nil {
		return Asset{}, err
	}

	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		updated, err := scanAsset(tx.QueryRow(ctx, `
			update file_assets
			set size_bytes = $3,
				content_type = $4,
				status = 'ready',
				confirmed_at = now(),
				updated_at = now()
			where tenant_id = $1 and id = $2 and status = 'pending'
			returning
				id::text, biz_type, coalesce(biz_id::text, ''), object_key,
				filename, content_type, size_bytes, status, created_at, updated_at
		`, tenantID, fileID, asset.SizeBytes, asset.ContentType))
		if err != nil {
			return err
		}
		asset = updated
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	return asset, err
}

func (s *Service) GetAsset(ctx context.Context, tenantID, fileID string) (Asset, error) {
	return s.assetByID(ctx, tenantID, fileID)
}

func (s *Service) ListAssets(ctx context.Context, tenantID, bizType string) ([]Asset, error) {
	assets := []Asset{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select
				id::text, biz_type, coalesce(biz_id::text, ''), object_key,
				filename, content_type, size_bytes, status, created_at, updated_at
			from file_assets
			where tenant_id = $1 and ($2 = '' or biz_type = $2)
			order by created_at desc
			limit 100
		`, tenantID, sanitizeSegment(bizType))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			asset, err := scanAsset(rows)
			if err != nil {
				return err
			}
			assets = append(assets, asset)
		}
		return rows.Err()
	})
	return assets, err
}

func (s *Service) DownloadURL(ctx context.Context, tenantID, fileID string, preview bool) (PresignedURLResponse, error) {
	asset, err := s.assetByID(ctx, tenantID, fileID)
	if err != nil {
		return PresignedURLResponse{}, err
	}
	if asset.Status != "ready" {
		return PresignedURLResponse{}, ErrObjectNotUploaded
	}

	reqParams := make(url.Values)
	dispositionType := "attachment"
	if preview {
		dispositionType = "inline"
	}
	reqParams.Set("response-content-type", asset.ContentType)
	reqParams.Set("response-content-disposition", contentDisposition(dispositionType, asset.Filename))
	downloadURL, err := s.public.PresignedGetObject(ctx, s.bucket, asset.objectKey, presignTTL, reqParams)
	if err != nil {
		return PresignedURLResponse{}, err
	}
	return PresignedURLResponse{
		File:      asset,
		URL:       downloadURL.String(),
		ExpiresAt: time.Now().Add(presignTTL).UTC(),
	}, nil
}

func (s *Service) ensureBucket(ctx context.Context) error {
	exists, err := s.internal.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.internal.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

func (s *Service) assetByID(ctx context.Context, tenantID, fileID string) (Asset, error) {
	if err := validateUUID(fileID); err != nil {
		return Asset{}, err
	}
	var asset Asset
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanAsset(tx.QueryRow(ctx, `
			select
				id::text, biz_type, coalesce(biz_id::text, ''), object_key,
				filename, content_type, size_bytes, status, created_at, updated_at
			from file_assets
			where tenant_id = $1 and id = $2
		`, tenantID, fileID))
		if err != nil {
			return err
		}
		asset = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	return asset, err
}

func (s *Service) withTenant(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
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

func newClient(endpoint string, cfg config.Config) (*minio.Client, error) {
	if endpoint == "" || cfg.MinIOAccessKey == "" || cfg.MinIOSecretKey == "" {
		return nil, ErrInvalidRequest
	}
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
		Region: cfg.MinIORegion,
	})
}

func sanitizeFilename(filename string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	if cleaned == "" {
		return ""
	}
	base := stripControlChars(path.Base(cleaned))
	if base == "." || base == ".." {
		return ""
	}
	return base
}

func sanitizeSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			builder.WriteRune(ch)
		}
	}
	return builder.String()
}

func normalizeOptionalUUID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if err := validateUUID(value); err != nil {
		return "", err
	}
	return value, nil
}

func normalizeBizType(value, fallback string) (string, error) {
	bizType := sanitizeSegment(value)
	if bizType == "" {
		bizType = fallback
	}
	if _, ok := AccessModuleForBizType(bizType); !ok {
		return "", ErrInvalidRequest
	}
	return bizType, nil
}

func normalizeContentType(value string) (string, error) {
	contentType := strings.TrimSpace(value)
	if contentType == "" {
		return "application/octet-stream", nil
	}
	if len(contentType) > maxContentTypeBytes || hasControlChars(contentType) {
		return "", ErrInvalidRequest
	}
	return contentType, nil
}

func validateUploadSize(sizeBytes int64) error {
	if sizeBytes <= 0 || sizeBytes > maxUploadSizeBytes {
		return ErrInvalidRequest
	}
	return nil
}

func AccessModuleForBizType(bizType string) (string, bool) {
	normalized := sanitizeSegment(bizType)
	if normalized == "" {
		normalized = defaultUploadBizType
	}
	module, ok := bizTypeAccessModules[normalized]
	return module, ok
}

func validateUUID(value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func contentDisposition(dispositionType, filename string) string {
	cleaned := stripControlChars(filename)
	asciiName := headerFallbackFilename(cleaned)
	encoded := url.PathEscape(cleaned)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, dispositionType, asciiName, encoded)
}

func stripControlChars(value string) string {
	return strings.Map(func(ch rune) rune {
		if isControlChar(ch) {
			return -1
		}
		return ch
	}, value)
}

func hasControlChars(value string) bool {
	for _, ch := range value {
		if isControlChar(ch) {
			return true
		}
	}
	return false
}

func isControlChar(ch rune) bool {
	return ch < 0x20 || ch == 0x7f
}

func headerFallbackFilename(filename string) string {
	cleaned := stripControlChars(filename)
	cleaned = strings.ReplaceAll(cleaned, `"`, "")
	cleaned = strings.ReplaceAll(cleaned, `\`, "")
	return cleaned
}

type assetScanner interface {
	Scan(dest ...any) error
}

func scanAsset(scanner assetScanner) (Asset, error) {
	var asset Asset
	var bizID string
	err := scanner.Scan(
		&asset.ID, &asset.BizType, &bizID, &asset.objectKey,
		&asset.Filename, &asset.ContentType, &asset.SizeBytes, &asset.Status, &asset.CreatedAt, &asset.UpdatedAt,
	)
	if bizID != "" {
		asset.BizID = &bizID
	}
	return asset, err
}

func isObjectMissing(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.Code == "NoSuchKey" || resp.Code == "NoSuchBucket" || resp.StatusCode == 404
}
