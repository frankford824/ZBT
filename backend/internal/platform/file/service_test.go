package file

import (
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
