package file

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeOptionalUUIDRejectsInvalidValue(t *testing.T) {
	if _, err := normalizeOptionalUUID("not-a-uuid"); err == nil {
		t.Fatal("expected invalid uuid to be rejected")
	}
}

func TestNormalizeOptionalUUIDTrimsValidValue(t *testing.T) {
	value, err := normalizeOptionalUUID(" 00000000-0000-4000-8000-000000000001 ")
	if err != nil {
		t.Fatalf("expected valid uuid: %v", err)
	}
	if value != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("unexpected normalized uuid: %q", value)
	}
}

func TestNormalizeBizTypeDefaultsAndRejectsUnsupportedValues(t *testing.T) {
	got, err := normalizeBizType("", defaultUploadBizType)
	if err != nil {
		t.Fatalf("expected default biz type: %v", err)
	}
	if got != "knowledge" {
		t.Fatalf("unexpected default biz type: %q", got)
	}

	got, err = normalizeBizType(" BID_TENDER ", defaultUploadBizType)
	if err != nil {
		t.Fatalf("expected valid bid biz type: %v", err)
	}
	if got != "bid_tender" {
		t.Fatalf("unexpected normalized biz type: %q", got)
	}

	if _, err := normalizeBizType("project_archive", defaultUploadBizType); err == nil {
		t.Fatal("expected unsupported biz type to be rejected")
	}
}

func TestAccessModuleForBizTypeUsesExplicitAllowList(t *testing.T) {
	for _, tc := range []struct {
		bizType string
		module  string
	}{
		{bizType: "", module: "knowledge"},
		{bizType: "knowledge_case", module: "knowledge"},
		{bizType: "bid_tender", module: "bid"},
		{bizType: "bid_export", module: "bid"},
	} {
		module, ok := AccessModuleForBizType(tc.bizType)
		if !ok || module != tc.module {
			t.Fatalf("expected %q to map to %q, got module=%q ok=%v", tc.bizType, tc.module, module, ok)
		}
	}

	if _, ok := AccessModuleForBizType("bid_anything"); ok {
		t.Fatal("expected unsupported bid-like biz type to be rejected")
	}
}

func TestSanitizeFilenameKeepsOnlyBaseName(t *testing.T) {
	if got := sanitizeFilename(`..\..\bid.docx`); got != "bid.docx" {
		t.Fatalf("unexpected filename: %q", got)
	}
}

func TestSanitizeFilenameRejectsSpecialPathSegments(t *testing.T) {
	for _, filename := range []string{".", "..", "../.."} {
		if got := sanitizeFilename(filename); got != "" {
			t.Fatalf("expected %q to be rejected, got %q", filename, got)
		}
	}
}

func TestSanitizeFilenameRemovesControlCharacters(t *testing.T) {
	got := sanitizeFilename("..\\bad\r\nX-Injected: yes.pdf")
	if got != "badX-Injected: yes.pdf" {
		t.Fatalf("unexpected sanitized filename: %q", got)
	}
}

func TestContentDispositionUsesHeaderSafeFallbackName(t *testing.T) {
	header := contentDisposition("attachment", "投标\"\r\n文件\\demo.pdf")
	if header != `attachment; filename="投标文件demo.pdf"; filename*=UTF-8''%E6%8A%95%E6%A0%87%22%E6%96%87%E4%BB%B6%5Cdemo.pdf` {
		t.Fatalf("unexpected content disposition: %q", header)
	}
}

func TestContentDispositionTypeAllowsOnlySafeInlinePreviewTypes(t *testing.T) {
	for _, contentType := range []string{
		"application/pdf",
		"image/png",
		"image/jpeg",
		"image/gif",
		"image/webp",
		"text/plain; charset=utf-8",
	} {
		if got := contentDispositionType(contentType, true); got != "inline" {
			t.Fatalf("expected safe preview type %q to be inline, got %q", contentType, got)
		}
	}
}

func TestContentDispositionTypeFallsBackToAttachmentForUnsafePreviewTypes(t *testing.T) {
	for _, contentType := range []string{
		"text/html",
		"image/svg+xml",
		"application/xml",
		"application/octet-stream",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"",
	} {
		if got := contentDispositionType(contentType, true); got != "attachment" {
			t.Fatalf("expected unsafe preview type %q to be attachment, got %q", contentType, got)
		}
	}
}

func TestContentDispositionTypeUsesAttachmentForDownloads(t *testing.T) {
	if got := contentDispositionType("application/pdf", false); got != "attachment" {
		t.Fatalf("expected download disposition to be attachment, got %q", got)
	}
}

func TestNormalizeContentTypeDefaultsAndRejectsOversizedValues(t *testing.T) {
	contentType, err := normalizeContentType(" ")
	if err != nil {
		t.Fatalf("expected default content type: %v", err)
	}
	if contentType != "application/octet-stream" {
		t.Fatalf("unexpected default content type: %q", contentType)
	}

	if _, err := normalizeContentType(strings.Repeat("a", maxContentTypeBytes+1)); err != ErrInvalidRequest {
		t.Fatalf("expected oversized content type to be rejected, got %v", err)
	}
}

func TestNormalizeContentTypeRejectsControlCharacters(t *testing.T) {
	if _, err := normalizeContentType("text/plain\r\nX-Injected: yes"); err != ErrInvalidRequest {
		t.Fatalf("expected content type with controls to be rejected, got %v", err)
	}
}

func TestConfirmedContentTypeUsesObservedSafeValue(t *testing.T) {
	contentType, err := confirmedContentType("application/pdf", " text/plain ")
	if err != nil {
		t.Fatalf("expected observed content type to normalize: %v", err)
	}
	if contentType != "text/plain" {
		t.Fatalf("unexpected confirmed content type: %q", contentType)
	}
}

func TestConfirmedContentTypeFallsBackToClaimedValue(t *testing.T) {
	contentType, err := confirmedContentType("application/pdf", " ")
	if err != nil {
		t.Fatalf("expected blank observed content type to use claimed value: %v", err)
	}
	if contentType != "application/pdf" {
		t.Fatalf("unexpected fallback content type: %q", contentType)
	}
}

func TestConfirmedContentTypeRejectsUnsafeObservedValue(t *testing.T) {
	if _, err := confirmedContentType("application/pdf", "text/plain\r\nX-Injected: yes"); err != ErrInvalidRequest {
		t.Fatalf("expected unsafe observed content type to be rejected, got %v", err)
	}
}

func TestStorageEndpointSupportsCloudflareR2HTTPSURL(t *testing.T) {
	endpoint, secure, err := storageEndpoint(" https://example-account.r2.cloudflarestorage.com/ ", false)
	if err != nil {
		t.Fatalf("expected R2 endpoint URL to normalize: %v", err)
	}
	if endpoint != "example-account.r2.cloudflarestorage.com" || !secure {
		t.Fatalf("unexpected normalized endpoint=%q secure=%v", endpoint, secure)
	}
}

func TestStorageEndpointSupportsHTTPURL(t *testing.T) {
	endpoint, secure, err := storageEndpoint("http://127.0.0.1:9000", true)
	if err != nil {
		t.Fatalf("expected HTTP endpoint URL to normalize: %v", err)
	}
	if endpoint != "127.0.0.1:9000" || secure {
		t.Fatalf("unexpected normalized endpoint=%q secure=%v", endpoint, secure)
	}
}

func TestStorageEndpointUsesFallbackSecureForHostOnlyEndpoint(t *testing.T) {
	endpoint, secure, err := storageEndpoint("minio:9000", true)
	if err != nil {
		t.Fatalf("expected host endpoint to normalize: %v", err)
	}
	if endpoint != "minio:9000" || !secure {
		t.Fatalf("unexpected normalized endpoint=%q secure=%v", endpoint, secure)
	}
}

func TestStorageEndpointRejectsPathsAndUnsupportedSchemes(t *testing.T) {
	for _, raw := range []string{
		"",
		"ftp://example-account.r2.cloudflarestorage.com",
		"https://example-account.r2.cloudflarestorage.com/bucket",
		"minio:9000/bucket",
		"https://example-account.r2.cloudflarestorage.com?bucket=zbt",
	} {
		if _, _, err := storageEndpoint(raw, false); err != ErrInvalidRequest {
			t.Fatalf("expected %q to be rejected, got %v", raw, err)
		}
	}
}

func TestEnsureBucketCanBeDisabledForPrecreatedBuckets(t *testing.T) {
	service := &Service{ensureBucketOnStart: false}

	if err := service.ensureBucket(context.Background()); err != nil {
		t.Fatalf("expected disabled bucket initialization to skip storage calls: %v", err)
	}
}

func TestValidateUploadSizeRejectsEmptyNegativeAndOversizedFiles(t *testing.T) {
	for _, size := range []int64{-1, 0, maxUploadSizeBytes + 1} {
		if err := validateUploadSize(size); err != ErrInvalidRequest {
			t.Fatalf("expected size %d to be rejected, got %v", size, err)
		}
	}

	for _, size := range []int64{1, maxUploadSizeBytes} {
		if err := validateUploadSize(size); err != nil {
			t.Fatalf("expected size %d to be accepted, got %v", size, err)
		}
	}
}

func TestConfirmedUploadSizeRequiresDeclaredSizeMatch(t *testing.T) {
	size, err := confirmedUploadSize(1024, 1024)
	if err != nil {
		t.Fatalf("expected matching upload size to confirm: %v", err)
	}
	if size != 1024 {
		t.Fatalf("unexpected confirmed size: %d", size)
	}

	for _, tc := range []struct {
		name     string
		claimed  int64
		observed int64
		wantErr  error
	}{
		{name: "invalid claimed", claimed: 0, observed: 1024, wantErr: ErrInvalidRequest},
		{name: "invalid observed", claimed: 1024, observed: maxUploadSizeBytes + 1, wantErr: ErrInvalidRequest},
		{name: "smaller observed", claimed: 1024, observed: 512, wantErr: ErrInvalidObjectState},
		{name: "larger observed", claimed: 1024, observed: 2048, wantErr: ErrInvalidObjectState},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := confirmedUploadSize(tc.claimed, tc.observed); err != tc.wantErr {
				t.Fatalf("expected %v for claimed=%d observed=%d, got %v", tc.wantErr, tc.claimed, tc.observed, err)
			}
		})
	}
}
