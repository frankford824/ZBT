package bid

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
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

func TestAttachableTenderFileAssetRestrictsBusinessDomain(t *testing.T) {
	bidID := "00000000-0000-4000-8000-000000000001"
	otherBidID := "00000000-0000-4000-8000-000000000002"

	for name, tc := range map[string]struct {
		bizType string
		bizID   sql.NullString
		want    bool
	}{
		"unbound bid tender file": {
			bizType: "bid_tender",
			bizID:   sql.NullString{},
			want:    true,
		},
		"current bid tender file": {
			bizType: "bid_tender",
			bizID:   sql.NullString{String: bidID, Valid: true},
			want:    true,
		},
		"current bid tender file uppercase uuid": {
			bizType: "bid_tender",
			bizID:   sql.NullString{String: strings.ToUpper(bidID), Valid: true},
			want:    true,
		},
		"knowledge file": {
			bizType: "knowledge",
			bizID:   sql.NullString{},
			want:    false,
		},
		"other bid tender file": {
			bizType: "bid_tender",
			bizID:   sql.NullString{String: otherBidID, Valid: true},
			want:    false,
		},
	} {
		if got := attachableTenderFileAsset(tc.bizType, tc.bizID, bidID); got != tc.want {
			t.Fatalf("%s: expected %v, got %v", name, tc.want, got)
		}
	}
}

func TestNormalizeAcceptedTaskSanitizesProviderStatus(t *testing.T) {
	accepted, err := normalizeAcceptedTask(aiTaskAccepted{
		TaskID: " task-123 ",
		Status: " RUNNING ",
	})
	if err != nil {
		t.Fatalf("expected accepted task to normalize: %v", err)
	}
	if accepted.TaskID != "task-123" {
		t.Fatalf("expected task id to be trimmed, got %q", accepted.TaskID)
	}
	if accepted.Status != "running" {
		t.Fatalf("expected status to be normalized, got %q", accepted.Status)
	}
	if accepted.Route == nil {
		t.Fatal("expected nil route to be replaced with an empty object")
	}

	accepted, err = normalizeAcceptedTask(aiTaskAccepted{TaskID: "task-456", Status: "unexpected"})
	if err != nil {
		t.Fatalf("expected invalid provider status to fall back: %v", err)
	}
	if accepted.Status != "queued" {
		t.Fatalf("expected invalid provider status to fall back to queued, got %q", accepted.Status)
	}
}

func TestNormalizeAcceptedTaskRejectsMissingTaskID(t *testing.T) {
	if _, err := normalizeAcceptedTask(aiTaskAccepted{TaskID: "  ", Status: "queued"}); err != ErrInvalidRequest {
		t.Fatalf("expected missing task id to be rejected, got %v", err)
	}
}

func TestConfirmableParseResultStatusOnlyAllowsReadyResults(t *testing.T) {
	for status, want := range map[string]bool{
		"ready":      true,
		" READY ":    true,
		"confirmed":  true,
		"queued":     false,
		"processing": false,
		"failed":     false,
		"":           false,
	} {
		if got := confirmableParseResultStatus(status); got != want {
			t.Fatalf("expected status %q confirmable=%v, got %v", status, want, got)
		}
	}
}

func TestChapterTaskSideEffectsAllowedBlocksCancelledGenerationWork(t *testing.T) {
	for name, tc := range map[string]struct {
		jobStatus  string
		stepStatus string
		linked     bool
		want       bool
	}{
		"standalone chapter task": {
			linked: false,
			want:   true,
		},
		"running generation step": {
			jobStatus:  "running",
			stepStatus: "running",
			linked:     true,
			want:       true,
		},
		"paused generation job keeps current step result": {
			jobStatus:  "paused",
			stepStatus: "running",
			linked:     true,
			want:       true,
		},
		"cancelled generation job blocks late callback": {
			jobStatus:  "cancelled",
			stepStatus: "running",
			linked:     true,
			want:       false,
		},
		"cancelled generation step blocks late callback": {
			jobStatus:  "running",
			stepStatus: "cancelled",
			linked:     true,
			want:       false,
		},
		"status comparison is normalized": {
			jobStatus:  " CANCELLED ",
			stepStatus: "running",
			linked:     true,
			want:       false,
		},
	} {
		if got := chapterTaskSideEffectsAllowed(tc.jobStatus, tc.stepStatus, tc.linked); got != tc.want {
			t.Fatalf("%s: expected %v, got %v", name, tc.want, got)
		}
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
		"query shaped object": {
			{"filename": "bad.txt", "object_key": "tenant-demo/assets/file.txt?download=1"},
		},
		"fragment shaped object": {
			{"filename": "bad.txt", "object_key": "tenant-demo/assets/file.txt#preview"},
		},
		"object with control character": {
			{"filename": "bad.txt", "object_key": "tenant-demo/assets/file\n.txt"},
		},
		"object segment with surrounding whitespace": {
			{"filename": "bad.txt", "object_key": "tenant-demo/assets/ file.txt"},
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

func TestExportCallbackFileResultValidatesSizeAndContentType(t *testing.T) {
	sizeBytes, contentType, err := exportCallbackFileResult("pdf", map[string]any{
		"size_bytes":   float64(128),
		"content_type": "application/pdf",
	})
	if err != nil {
		t.Fatalf("expected valid pdf callback result: %v", err)
	}
	if sizeBytes != 128 || contentType != pdfContentType {
		t.Fatalf("unexpected normalized callback result: size=%d contentType=%q", sizeBytes, contentType)
	}

	sizeBytes, contentType, err = exportCallbackFileResult("zip", map[string]any{
		"size_bytes": json.Number("256"),
	})
	if err != nil {
		t.Fatalf("expected missing content type to default for zip: %v", err)
	}
	if sizeBytes != 256 || contentType != zipContentType {
		t.Fatalf("unexpected zip callback result: size=%d contentType=%q", sizeBytes, contentType)
	}
}

func TestExportCallbackFileResultRejectsUnsafeValues(t *testing.T) {
	for name, result := range map[string]map[string]any{
		"negative size": {
			"size_bytes": float64(-1),
		},
		"fractional size": {
			"size_bytes": float64(1.5),
		},
		"oversized json integer": {
			"size_bytes": float64(maxExactJSONInteger) + 1,
		},
		"wrong content type": {
			"size_bytes":   float64(128),
			"content_type": "text/html",
		},
		"non string content type": {
			"size_bytes":   float64(128),
			"content_type": 42,
		},
	} {
		if _, _, err := exportCallbackFileResult("docx", result); err != ErrInvalidRequest {
			t.Fatalf("expected %s to be rejected, got %v", name, err)
		}
	}
}

func TestTokenUsageSumSQLGuardsMalformedCallbackValues(t *testing.T) {
	expr := tokenUsageSumSQL("input_tokens")
	for _, want := range []string{
		"input_tokens",
		"^[0-9]{1,12}$",
		"::bigint",
		"least(",
		"2147483647",
	} {
		if !strings.Contains(expr, want) {
			t.Fatalf("expected token usage SQL to contain %q, got %s", want, expr)
		}
	}
	if got := tokenUsageSumSQL("bad_field"); got != "0::int" {
		t.Fatalf("expected unsupported token field to be neutralized, got %s", got)
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
