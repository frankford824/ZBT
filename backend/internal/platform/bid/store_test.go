package bid

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestTenderStructuredResultFromCallbackExtractsNestedResult(t *testing.T) {
	structured, ok := tenderStructuredResultFromCallback(map[string]any{
		"structured_result": map[string]any{
			"project_name": "桥梁检查服务",
			"outline": map[string]any{
				"parts": []any{},
			},
		},
	})

	if !ok {
		t.Fatal("expected structured_result to be accepted")
	}
	if structured["project_name"] != "桥梁检查服务" {
		t.Fatalf("unexpected project name: %v", structured["project_name"])
	}
}

func TestTenderStructuredResultFromCallbackRejectsMissingResult(t *testing.T) {
	if _, ok := tenderStructuredResultFromCallback(map[string]any{"summary": "done"}); ok {
		t.Fatal("expected missing structured_result to be rejected")
	}
}

func TestDefaultTenderStructuredResultCarriesSourceObjectKey(t *testing.T) {
	structured := defaultTenderStructuredResult(
		Document{Title: "桥梁检查服务", BidType: "combined"},
		TenderFile{
			FileAssetID: "file-demo",
			ObjectKey:   "tenant/bid_tender/file-demo",
			Filename:    "采购文件.pdf",
			ContentType: "application/pdf",
			SizeBytes:   128,
		},
	)

	source, ok := structured["source_file"].(map[string]any)
	if !ok {
		t.Fatal("expected source_file map")
	}
	if source["object_key"] != "tenant/bid_tender/file-demo" {
		t.Fatalf("expected object key in source_file, got %v", source["object_key"])
	}
}

func TestValidateExportAttachmentsAllowsTenantObjectKeysAndInlineContent(t *testing.T) {
	err := validateExportAttachments(
		"tenant-demo",
		[]map[string]any{
			{"filename": "资质.txt", "object_key": "tenant-demo/assets/qualification.txt"},
			{"filename": "说明.txt", "content_base64": "Y29udGVudA=="},
		},
		[]map[string]any{
			{"filename": "清单.xlsx", "object_key": "tenant-demo/boq/file.xlsx"},
		},
	)
	if err != nil {
		t.Fatalf("expected valid export attachments, got %v", err)
	}
}

func TestValidateExportAttachmentsRejectsUnsafeInputs(t *testing.T) {
	for name, attachments := range map[string][]map[string]any{
		"cross tenant object": {
			{"filename": "secret.txt", "object_key": "other-tenant/assets/secret.txt"},
		},
		"double slash object": {
			{"filename": "bad.txt", "object_key": "tenant-demo/assets//file.txt"},
		},
		"current directory object": {
			{"filename": "bad.txt", "object_key": "tenant-demo/./assets/file.txt"},
		},
		"parent directory object": {
			{"filename": "bad.txt", "object_key": "tenant-demo/../assets/file.txt"},
		},
		"backslash object": {
			{"filename": "bad.txt", "object_key": `tenant-demo\assets\file.txt`},
		},
		"object with surrounding whitespace": {
			{"filename": "bad.txt", "object_key": "tenant-demo/assets/file.txt "},
		},
		"protocol shaped object": {
			{"filename": "bad.txt", "object_key": "http://tenant-demo/assets/file.txt"},
		},
		"local path": {
			{"filename": "secret.txt", "local_path": "/etc/passwd", "content_base64": "YQ=="},
		},
		"missing content": {
			{"filename": "empty.txt"},
		},
		"non string object key": {
			{"filename": "bad.txt", "object_key": 42},
		},
		"non string inline content": {
			{"filename": "bad.txt", "content_base64": 42},
		},
		"missing filename": {
			{"object_key": "tenant-demo/assets/file.txt"},
		},
		"mixed content sources": {
			{"filename": "bad.txt", "object_key": "tenant-demo/assets/file.txt", "content_base64": "YQ=="},
		},
		"invalid inline content": {
			{"filename": "bad.txt", "content_base64": "not-base64"},
		},
		"non string zip path": {
			{"filename": "bad.txt", "content_base64": "YQ==", "zip_path": 42},
		},
	} {
		if err := validateExportAttachments("tenant-demo", attachments); err == nil {
			t.Fatalf("expected %s to be rejected", name)
		}
	}
}

func TestValidateExportAttachmentsRejectsTooManyAttachments(t *testing.T) {
	attachments := make([]map[string]any, 0, maxExportAttachmentCount+1)
	for i := 0; i <= maxExportAttachmentCount; i++ {
		attachments = append(attachments, map[string]any{
			"filename":   "file.txt",
			"object_key": "tenant-demo/assets/file.txt",
		})
	}

	if err := validateExportAttachments("tenant-demo", attachments); err != ErrInvalidRequest {
		t.Fatalf("expected too many attachments to be rejected, got %v", err)
	}
}

func TestValidateExportInlineAttachmentContentRejectsOversizedEncodedContent(t *testing.T) {
	encoded := strings.Repeat("A", base64.StdEncoding.EncodedLen(maxExportInlineAttachmentBytes)+1)
	if _, err := validateExportInlineAttachmentContent(encoded); err != ErrInvalidRequest {
		t.Fatalf("expected oversized inline content to be rejected, got %v", err)
	}
}

func TestExportAttachmentObjectKeysDedupesObjectBackedAttachments(t *testing.T) {
	got := exportAttachmentObjectKeys(
		[]map[string]any{
			{"filename": "a.txt", "object_key": "tenant-demo/assets/a.txt"},
			{"filename": "inline.txt", "content_base64": "YQ=="},
		},
		[]map[string]any{
			{"filename": "a-copy.txt", "object_key": "tenant-demo/assets/a.txt"},
			{"filename": "b.txt", "object_key": "tenant-demo/assets/b.txt"},
		},
	)

	want := []string{"tenant-demo/assets/a.txt", "tenant-demo/assets/b.txt"}
	if len(got) != len(want) {
		t.Fatalf("unexpected object keys: got %v want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected object keys: got %v want %v", got, want)
		}
	}
}

func TestNormalizeBidTypeDefaultsOnlyBlankType(t *testing.T) {
	bidType, err := normalizeBidType(" ")
	if err != nil {
		t.Fatalf("expected blank bid type to default: %v", err)
	}
	if bidType != "combined" {
		t.Fatalf("expected blank bid type to default to combined, got %q", bidType)
	}

	bidType, err = normalizeBidType(" SEPARATED ")
	if err != nil {
		t.Fatalf("expected known bid type to normalize: %v", err)
	}
	if bidType != "separated" {
		t.Fatalf("expected known bid type to normalize to separated, got %q", bidType)
	}

	if _, err := normalizeBidType("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported bid type to be rejected, got %v", err)
	}
}

func TestNormalizeDocumentStatusAcceptsOnlyWorkflowStates(t *testing.T) {
	if got := normalizeDocumentStatus(" IN_REVIEW "); got != "in_review" {
		t.Fatalf("expected known document status to normalize, got %q", got)
	}
	if got := normalizeDocumentStatus("submitted"); got != "submitted" {
		t.Fatalf("expected submitted status to be accepted, got %q", got)
	}
	if got := normalizeDocumentStatus("superseded"); got != "" {
		t.Fatalf("expected unsupported document status to be rejected, got %q", got)
	}
}

func TestNormalizeRequestedPartCodeDefaultsOnlyBlankPartCode(t *testing.T) {
	partCode, err := normalizeRequestedPartCode(" ")
	if err != nil {
		t.Fatalf("expected blank part code to default: %v", err)
	}
	if partCode != "combined_body" {
		t.Fatalf("expected blank part code to default to combined_body, got %q", partCode)
	}

	partCode, err = normalizeRequestedPartCode(" TECH ")
	if err != nil {
		t.Fatalf("expected known part code to normalize: %v", err)
	}
	if partCode != "tech" {
		t.Fatalf("expected known part code to normalize to tech, got %q", partCode)
	}

	if _, err := normalizeRequestedPartCode("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported part code to be rejected, got %v", err)
	}
}

func TestNormalizeGenerationInputsRejectUnsupportedValues(t *testing.T) {
	scope, err := normalizeGenerationScope(" ")
	if err != nil {
		t.Fatalf("expected blank generation scope to default: %v", err)
	}
	if scope != "full" {
		t.Fatalf("expected blank generation scope to default to full, got %q", scope)
	}

	scope, err = normalizeGenerationScope(" CHAPTER ")
	if err != nil {
		t.Fatalf("expected known generation scope to normalize: %v", err)
	}
	if scope != "chapter" {
		t.Fatalf("expected known generation scope to normalize to chapter, got %q", scope)
	}

	if _, err := normalizeGenerationScope("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported generation scope to be rejected, got %v", err)
	}

	partCode, err := normalizeGenerationPartCode(" ")
	if err != nil {
		t.Fatalf("expected blank generation part code to be accepted: %v", err)
	}
	if partCode != "" {
		t.Fatalf("expected blank generation part code to stay empty, got %q", partCode)
	}

	partCode, err = normalizeGenerationPartCode(" BUSINESS ")
	if err != nil {
		t.Fatalf("expected known generation part code to normalize: %v", err)
	}
	if partCode != "business" {
		t.Fatalf("expected known generation part code to normalize to business, got %q", partCode)
	}

	if _, err := normalizeGenerationPartCode("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported generation part code to be rejected, got %v", err)
	}
}
