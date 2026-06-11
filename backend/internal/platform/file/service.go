package file

import (
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

func (s *Service) PresignUpload(ctx context.Context, tenantID, userID string, req PresignUploadRequest) (PresignUploadResponse, error) {
	bizType := sanitizeSegment(req.BizType)
	if bizType == "" {
		bizType = "knowledge"
	}
	if req.SizeBytes < 0 {
		return PresignUploadResponse{}, ErrInvalidRequest
	}
	filename := sanitizeFilename(req.Filename)
	if filename == "" {
		return PresignUploadResponse{}, ErrInvalidRequest
	}
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
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
		`, tenantID, userID, bizType, req.BizID, objectKey, filename, contentType, req.SizeBytes))
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
	return path.Base(cleaned)
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

func contentDisposition(dispositionType, filename string) string {
	asciiName := strings.ReplaceAll(filename, `"`, "")
	encoded := url.PathEscape(filename)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, dispositionType, asciiName, encoded)
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
