package bid

import "testing"

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
	} {
		if err := validateExportAttachments("tenant-demo", attachments); err == nil {
			t.Fatalf("expected %s to be rejected", name)
		}
	}
}
