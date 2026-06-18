package bid

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
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

func TestNormalizeAcceptedTaskRejectsOversizedRoute(t *testing.T) {
	_, err := normalizeAcceptedTask(aiTaskAccepted{
		TaskID: "task-bid-00000000-0000-4000-8000-000000000001",
		Status: "running",
		Route: map[string]any{
			"payload": strings.Repeat("路", maxBidTaskRouteJSONBytes),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized accepted route to be rejected, got %v", err)
	}
}

func TestBidCallbackRejectsInvalidResultBeforeDB(t *testing.T) {
	store := &Store{}

	_, err := store.ApplyCallback(context.Background(), CallbackPayload{
		TenantID: "tenant-id",
		TaskID:   "task-bid-00000000-0000-4000-8000-000000000001",
		Status:   "failed",
		Result: map[string]any{
			"invalid": math.NaN(),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid callback result JSON to be rejected before DB, got %v", err)
	}
}

func TestBidCallbackRejectsOversizedResultBeforeDB(t *testing.T) {
	store := &Store{}

	_, err := store.ApplyCallback(context.Background(), CallbackPayload{
		TenantID: "tenant-id",
		TaskID:   "task-bid-00000000-0000-4000-8000-000000000001",
		Status:   "failed",
		Result: map[string]any{
			"payload": strings.Repeat("结", maxBidTaskResultJSONBytes),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized callback result to be rejected before DB, got %v", err)
	}
}

func TestMarshalBidBusinessJSONRejectsInvalidAndOversizedValues(t *testing.T) {
	for name, value := range map[string]any{
		"invalid number": map[string]any{"bad": math.NaN()},
		"unsupported":    map[string]any{"bad": func() {}},
		"oversized":      []any{strings.Repeat("素", maxBidMaterialSelectionJSONBytes)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := marshalBidBusinessJSON(value, maxBidMaterialSelectionJSONBytes); err != ErrInvalidRequest {
				t.Fatalf("expected invalid bid business JSON to be rejected, got %v", err)
			}
		})
	}
}

func TestManualRequirementCoverageRejectsInvalidSourceRefsBeforeDB(t *testing.T) {
	_, _, _, err := manualRequirementCoverageMetadata("requirement-1", "user-1", UpdateRequirementCoverageRequest{
		CoverageStatus: "covered",
		SourceRefs:     []any{map[string]any{"bad": math.NaN()}},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid requirement coverage source refs to be rejected before DB, got %v", err)
	}
}

func TestBatchRequirementCoverageRejectsOversizedMetadataBeforeDB(t *testing.T) {
	_, _, _, err := batchRequirementCoverageMetadata("user-1", BatchUpdateRequirementCoverageRequest{
		CoverageStatus: "covered",
		Evidence:       strings.Repeat("证", maxBidRequirementCoverageJSONBytes),
	}, 1)
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized batch coverage metadata to be rejected before DB, got %v", err)
	}
}

func TestNormalizeBidCallbackBoundsErrorMessage(t *testing.T) {
	_, _, err := normalizeBidCallbackPayload(CallbackPayload{
		TenantID:     "tenant-id",
		TaskID:       "task-bid-00000000-0000-4000-8000-000000000001",
		Status:       "failed",
		ErrorMessage: strings.Repeat("错", maxBidTaskErrorMessageRunes+1),
		Result:       map[string]any{},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized callback error message to be rejected, got %v", err)
	}
}

func TestBindAcceptedTaskRejectsInvalidPayloadBeforeDB(t *testing.T) {
	store := &Store{}

	_, err := store.bindAcceptedTask(
		context.Background(),
		"tenant-id",
		"resource-id",
		"task-id",
		aiTaskAccepted{TaskID: "task-bid-00000000-0000-4000-8000-000000000001", Status: "running"},
		map[string]any{"bad": func() {}},
		nil,
	)
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid accepted payload JSON to be rejected before DB, got %v", err)
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

func TestRequirementRefsFromStructuredResultMatchesChapterTitle(t *testing.T) {
	structured := map[string]any{
		"requirement_items": []any{
			map[string]any{
				"id":          "qualification-001",
				"module":      "qualification",
				"type":        "qualification",
				"requirement": "提供企业资质证书和项目负责人证书",
				"priority":    "high",
				"mandatory":   true,
				"source_ref": map[string]any{
					"source_text": "供应商须提供资质证书",
					"page_start":  float64(12),
				},
			},
			map[string]any{
				"id":          "evaluation-001",
				"module":      "evaluation",
				"type":        "scoring",
				"requirement": "技术方案完整性评分 20 分",
				"priority":    "high",
				"score":       float64(20),
			},
			map[string]any{
				"id":          "annex-001",
				"module":      "annex",
				"type":        "annex",
				"requirement": "按附件格式提交报价表",
				"priority":    "medium",
			},
		},
	}

	qualificationRefs := requirementRefsFromStructuredResult(structured, "二、资格证明文件", 2)
	if len(qualificationRefs) == 0 || qualificationRefs[0].ID != "qualification-001" {
		t.Fatalf("expected qualification requirement first, got %#v", qualificationRefs)
	}
	if qualificationRefs[0].SourceText != "供应商须提供资质证书" {
		t.Fatalf("expected source evidence to be carried, got %#v", qualificationRefs[0])
	}
	if qualificationRefs[0].PageStart == nil || *qualificationRefs[0].PageStart != 12 {
		t.Fatalf("expected page start to be normalized, got %#v", qualificationRefs[0].PageStart)
	}

	evaluationRefs := requirementRefsFromStructuredResult(structured, "二、总体技术方案", 2)
	if len(evaluationRefs) == 0 || evaluationRefs[0].ID != "evaluation-001" {
		t.Fatalf("expected evaluation requirement first, got %#v", evaluationRefs)
	}
}

func TestRequirementRefsFromStructuredResultFallsBackForLegacyParseResult(t *testing.T) {
	structured := map[string]any{
		"qualification_requirements": []any{"营业执照和授权书齐备"},
		"scoring_points":             []any{"服务方案完整性"},
		"invalid_clause_risks":       []any{"签章缺失将导致无效投标"},
	}

	refs := requirementRefsFromStructuredResult(structured, "三、商务偏离表", 2)
	if len(refs) == 0 {
		t.Fatal("expected legacy fields to synthesize requirement refs")
	}
	if refs[0].Module != "invalid_risk" {
		t.Fatalf("expected invalid risk requirement first, got %#v", refs)
	}
	if !refs[0].NeedsReview {
		t.Fatalf("expected legacy synthesized requirement to need review, got %#v", refs[0])
	}
}

func TestRequirementRefsFromStructuredResultFallsBackToHighPriorityWhenNoTitleMatch(t *testing.T) {
	structured := map[string]any{
		"requirement_items": []any{
			map[string]any{
				"id":          "annex-001",
				"module":      "annex",
				"type":        "annex",
				"requirement": "提交报价表格式",
				"priority":    "low",
			},
			map[string]any{
				"id":          "qualification-001",
				"module":      "qualification",
				"type":        "qualification",
				"requirement": "提供营业执照",
				"priority":    "high",
				"mandatory":   true,
			},
		},
	}

	refs := requirementRefsFromStructuredResult(structured, "一、项目概述", 1)
	if len(refs) != 1 || refs[0].ID != "qualification-001" {
		t.Fatalf("expected high priority fallback, got %#v", refs)
	}
}

func TestTenderRequirementTextsKeepsCoverageInstruction(t *testing.T) {
	texts := tenderRequirementTexts([]tenderRequirementRef{
		{
			Module:           "qualification",
			Requirement:      "提供企业资质证书",
			ExpectedResponse: "附证书编号和有效期",
		},
	})

	if len(texts) != 3 {
		t.Fatalf("expected requirement plus two guardrails, got %#v", texts)
	}
	if !strings.Contains(texts[0], "资格要求：提供企业资质证书") || !strings.Contains(texts[0], "响应要点：附证书编号和有效期") {
		t.Fatalf("expected labeled requirement text, got %#v", texts)
	}
	if !strings.Contains(texts[2], "逐条完成自检") {
		t.Fatalf("expected self-check guardrail, got %#v", texts)
	}
}

func TestRequirementItemsFromStructuredResultPreservesSourceAndStatus(t *testing.T) {
	structured := map[string]any{
		"requirement_items": []any{
			map[string]any{
				"id":                "evaluation-001",
				"module":            "evaluation",
				"type":              "scoring",
				"requirement":       "技术方案需响应评分细则",
				"priority":          "HIGH",
				"mandatory":         true,
				"score":             12.5,
				"expected_response": "逐项说明响应内容",
				"status":            "partial",
				"needs_review":      true,
				"source_ref": map[string]any{
					"document_id":  "doc-1",
					"chunk_id":     "chunk-9",
					"source_text":  "评分标准：技术方案需响应评分细则",
					"page_start":   8,
					"page_end":     9,
					"custom_field": "kept",
				},
			},
		},
	}

	items := requirementItemsFromStructuredResult(structured)
	if len(items) != 1 {
		t.Fatalf("expected one item, got %#v", items)
	}
	item := items[0]
	if item.ExternalID != "evaluation-001" || item.Priority != "high" || item.CoverageStatus != "needs_review" {
		t.Fatalf("unexpected normalized item: %#v", item)
	}
	if !item.Mandatory || !item.NeedsReview || item.Score == nil || *item.Score != 12.5 {
		t.Fatalf("expected item flags and score to be retained, got %#v", item)
	}
	if item.SourceRef["chunk_id"] != "chunk-9" || item.SourceRef["custom_field"] != "kept" {
		t.Fatalf("expected source ref to be preserved, got %#v", item.SourceRef)
	}
}

func TestRequirementItemsFromStructuredResultFallsBackFromLegacyFields(t *testing.T) {
	structured := map[string]any{
		"qualification_requirements": []any{"提供营业执照", "提供营业执照"},
		"invalid_clause_risks":       []any{"未盖章将被否决"},
	}

	items := requirementItemsFromStructuredResult(structured)
	if len(items) != 3 {
		t.Fatalf("expected fallback requirement items, got %#v", items)
	}
	if items[0].Module != "qualification" || !items[0].Mandatory || items[0].Priority != "high" {
		t.Fatalf("expected qualification fallback first, got %#v", items[0])
	}
	if items[2].Module != "invalid_risk" || !items[2].Mandatory || items[2].Priority != "high" {
		t.Fatalf("expected invalid risk fallback, got %#v", items[2])
	}
}

func TestNormalizePipelineGateFields(t *testing.T) {
	if got := normalizePipelineStage(" INTERPRET "); got != "interpret" {
		t.Fatalf("expected stage to normalize, got %q", got)
	}
	if got := normalizePipelineStage("unknown"); got != "" {
		t.Fatalf("expected unknown stage to be rejected, got %q", got)
	}
	if got := normalizePipelineGateStatus(" NEEDS_REVIEW "); got != "needs_review" {
		t.Fatalf("expected status to normalize, got %q", got)
	}
	if got := normalizePipelineGateStatus("done"); got != "" {
		t.Fatalf("expected unsupported status to be rejected, got %q", got)
	}
}

func TestPipelineGateStatusForTask(t *testing.T) {
	for status, want := range map[string]string{
		"queued":    "pending",
		"running":   "pending",
		"done":      "passed",
		"failed":    "blocked",
		"cancelled": "blocked",
		"paused":    "pending",
		"unknown":   "",
	} {
		if got := pipelineGateStatusForTask(status); got != want {
			t.Fatalf("expected task status %q to map to %q, got %q", status, want, got)
		}
	}
}

func TestPipelineGateStatusForGenerationJobBlocksPartialCancellation(t *testing.T) {
	if got := pipelineGateStatusForGenerationJob("done", 3, 2, 0, 1); got != "blocked" {
		t.Fatalf("expected partially cancelled job to block gate, got %q", got)
	}
	if got := pipelineGateStatusForGenerationJob("done", 3, 3, 0, 0); got != "passed" {
		t.Fatalf("expected fully done job to pass gate, got %q", got)
	}
	if got := pipelineGateStatusForGenerationJob("running", 3, 1, 0, 0); got != "pending" {
		t.Fatalf("expected running job to keep gate pending, got %q", got)
	}
}

func TestPipelineGateStatusForComplianceResult(t *testing.T) {
	for resultStatus, want := range map[string]string{
		"pass":           "passed",
		"warn":           "needs_review",
		"fail_candidate": "needs_review",
		"fail":           "blocked",
		"bad":            "",
	} {
		if got := pipelineGateStatusForComplianceResult(resultStatus); got != want {
			t.Fatalf("expected compliance result %q to map to %q, got %q", resultStatus, want, got)
		}
	}
}

func TestParseGateMetadataCarriesQualityAndCounts(t *testing.T) {
	metadata := parseGateMetadata("parse-demo", map[string]any{
		"quality_gates": map[string]any{
			"interpret": map[string]any{"status": "pass"},
		},
		"parse_metadata": map[string]any{
			"module_count": 6,
		},
		"requirement_items": []any{
			map[string]any{"id": "qualification-001"},
			map[string]any{"id": "evaluation-001"},
		},
	})

	if metadata["parse_result_id"] != "parse-demo" {
		t.Fatalf("expected parse result id, got %#v", metadata)
	}
	if metadata["requirement_count"] != 2 {
		t.Fatalf("expected requirement count, got %#v", metadata["requirement_count"])
	}
	if _, ok := metadata["quality_gates"].(map[string]any); !ok {
		t.Fatalf("expected quality gates metadata, got %#v", metadata)
	}
	if _, ok := metadata["parse_metadata"].(map[string]any); !ok {
		t.Fatalf("expected parse metadata, got %#v", metadata)
	}
}

func TestChapterVersionModelMetadataCarriesSelfCheckCoverage(t *testing.T) {
	pageStart := 5
	generation := chapterGenerateResponse{
		TraceID: "trace-demo",
		SourceRefs: []sourceRef{
			{
				ChunkID:    "chunk-evaluation-001",
				DocumentID: "doc-evaluation-001",
				Title:      "评分办法",
				PageStart:  &pageStart,
			},
		},
		ModelMetadata: map[string]any{"provider": "real-provider"},
		SelfCheck: map[string]any{
			"status": "needs_review",
			"requirement_coverage": []any{
				map[string]any{
					"requirement_id": "evaluation-001",
					"satisfied":      false,
					"source_refs": []any{
						map[string]any{
							"chunk_id":    "chunk-evaluation-001",
							"document_id": "doc-evaluation-001",
						},
					},
				},
			},
		},
	}

	metadata := chapterVersionModelMetadata(generation)

	if metadata["provider"] != "real-provider" || metadata["trace_id"] != "trace-demo" {
		t.Fatalf("expected model metadata and trace id to be retained, got %#v", metadata)
	}
	selfCheck, ok := metadata["self_check"].(map[string]any)
	if !ok || selfCheck["status"] != "needs_review" {
		t.Fatalf("expected self check to be stored, got %#v", metadata["self_check"])
	}
	if metadata["requirement_coverage_count"] != 1 {
		t.Fatalf("expected requirement coverage count, got %#v", metadata)
	}
	coverage, ok := metadata["requirement_coverage"].([]any)
	if !ok || len(coverage) != 1 {
		t.Fatalf("expected requirement coverage rows, got %#v", metadata["requirement_coverage"])
	}
	coverageRow, ok := coverage[0].(map[string]any)
	if !ok {
		t.Fatalf("expected coverage row map, got %#v", coverage[0])
	}
	sourceRefs := anySlice(coverageRow["source_refs"])
	if len(sourceRefs) != 1 {
		t.Fatalf("expected enriched coverage source refs, got %#v", coverageRow["source_refs"])
	}
	sourceRef, ok := sourceRefs[0].(map[string]any)
	if !ok {
		t.Fatalf("expected source ref map, got %#v", sourceRefs[0])
	}
	if sourceRef["citation_id"] == "" || sourceRef["reference_id"] == "" || sourceRef["source_locator"] == "" || sourceRef["page_start"] != pageStart {
		t.Fatalf("expected coverage source ref to carry traceable citation and location, got %#v", sourceRef)
	}
	generation.ModelMetadata["provider"] = "mutated"
	if metadata["provider"] != "real-provider" {
		t.Fatalf("expected metadata to be copied, got %#v", metadata["provider"])
	}
}

func TestSourceRefsAsAnyAddsTraceableCitationAndLocator(t *testing.T) {
	pageStart := 7
	pageEnd := 9

	refs := sourceRefsAsAny([]sourceRef{
		{
			ChunkID:    "chunk-abc",
			DocumentID: "doc-xyz",
			Title:      "技术方案",
			PageStart:  &pageStart,
			PageEnd:    &pageEnd,
		},
	})

	if len(refs) != 1 {
		t.Fatalf("expected one source ref, got %#v", refs)
	}
	sourceRef, ok := refs[0].(map[string]any)
	if !ok {
		t.Fatalf("expected source ref map, got %#v", refs[0])
	}
	expectedLocator := "chunk:chunk-abc;document:doc-xyz;page:7"
	if sourceRef["citation_id"] != expectedLocator ||
		sourceRef["reference_id"] != expectedLocator ||
		sourceRef["source_locator"] != expectedLocator ||
		sourceRef["locator"] != expectedLocator {
		t.Fatalf("expected stable citation and locator, got %#v", sourceRef)
	}
	if sourceRef["page_start"] != pageStart || sourceRef["page_end"] != pageEnd {
		t.Fatalf("expected page range to be dereferenced, got %#v", sourceRef)
	}
}

func TestRequirementCoverageStatusMapsModelResults(t *testing.T) {
	for name, tc := range map[string]struct {
		item        map[string]any
		wantStatus  string
		wantReview  bool
		wantUpdated bool
	}{
		"covered status": {
			item:        map[string]any{"status": "covered"},
			wantStatus:  "covered",
			wantUpdated: true,
		},
		"satisfied bool": {
			item:        map[string]any{"satisfied": true},
			wantStatus:  "covered",
			wantUpdated: true,
		},
		"needs review wins": {
			item:        map[string]any{"status": "covered", "needs_review": true},
			wantStatus:  "needs_review",
			wantReview:  true,
			wantUpdated: true,
		},
		"unsatisfied becomes review": {
			item:        map[string]any{"satisfied": false},
			wantStatus:  "needs_review",
			wantReview:  true,
			wantUpdated: true,
		},
		"failed status becomes review": {
			item:        map[string]any{"status": "not_covered"},
			wantStatus:  "needs_review",
			wantReview:  true,
			wantUpdated: true,
		},
		"unknown is ignored": {
			item: map[string]any{"evidence": "模型未给状态"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			status, needsReview, updated := requirementCoverageStatus(tc.item)
			if status != tc.wantStatus || needsReview != tc.wantReview || updated != tc.wantUpdated {
				t.Fatalf("unexpected mapping: status=%q needsReview=%v updated=%v", status, needsReview, updated)
			}
		})
	}
}

func TestRequirementCoverageIDAcceptsAutoRFPReferenceFields(t *testing.T) {
	for _, item := range []map[string]any{
		{"requirement_id": "evaluation-001"},
		{"requirementId": "evaluation-001"},
		{"external_id": "evaluation-001"},
		{"reference_id": "evaluation-001"},
		{"referenceId": "evaluation-001"},
		{"id": "evaluation-001"},
	} {
		if got := requirementCoverageID(item); got != "evaluation-001" {
			t.Fatalf("expected coverage id, got %q from %#v", got, item)
		}
	}
}

func TestManualRequirementCoverageMetadataPreservesEvidence(t *testing.T) {
	status, needsReview, metadata, err := manualRequirementCoverageMetadata("evaluation-001", "user-1", UpdateRequirementCoverageRequest{
		CoverageStatus: "covered",
		Evidence:       "已在实施方案章节逐条响应评分点",
		SourceRefs: []any{
			map[string]any{"title": "实施方案", "page": 3},
		},
	})
	if err != nil {
		t.Fatalf("manual coverage metadata: %v", err)
	}
	if status != "covered" || needsReview {
		t.Fatalf("unexpected normalized status: status=%q needsReview=%v", status, needsReview)
	}
	if metadata["source"] != "manual" || metadata["updated_by"] != "user-1" {
		t.Fatalf("expected manual metadata owner, got %#v", metadata)
	}
	if metadata["evidence"] != "已在实施方案章节逐条响应评分点" {
		t.Fatalf("expected evidence to be preserved, got %#v", metadata["evidence"])
	}
	if refs, ok := metadata["source_refs"].([]any); !ok || len(refs) != 1 {
		t.Fatalf("expected source refs to be preserved, got %#v", metadata["source_refs"])
	}
}

func TestManualRequirementCoverageMetadataRejectsInvalidStatus(t *testing.T) {
	if _, _, _, err := manualRequirementCoverageMetadata("evaluation-001", "user-1", UpdateRequirementCoverageRequest{
		CoverageStatus: "done_enough",
	}); err != ErrInvalidRequest {
		t.Fatalf("expected invalid manual coverage status to be rejected, got %v", err)
	}
}

func TestBatchRequirementCoverageMetadataDeduplicatesAndPreservesEvidence(t *testing.T) {
	ids, err := batchRequirementIDsFromRequest(BatchUpdateRequirementCoverageRequest{
		RequirementIDs: []string{" evaluation-001 ", "evaluation-002", "evaluation-001", ""},
	})
	if err != nil {
		t.Fatalf("batch requirement ids: %v", err)
	}
	status, needsReview, metadata, err := batchRequirementCoverageMetadata("user-1", BatchUpdateRequirementCoverageRequest{
		CoverageStatus: "covered",
		Evidence:       "已逐项补充响应依据",
		SourceRefs: []any{
			map[string]any{"title": "响应章节", "page": 8},
		},
	}, len(ids))
	if err != nil {
		t.Fatalf("batch coverage metadata: %v", err)
	}
	if status != "covered" || needsReview {
		t.Fatalf("unexpected normalized status: status=%q needsReview=%v", status, needsReview)
	}
	if len(ids) != 2 || ids[0] != "evaluation-001" || ids[1] != "evaluation-002" {
		t.Fatalf("expected deduplicated requirement ids, got %#v", ids)
	}
	if metadata["action"] != "batch_review" || metadata["batch"] != true || metadata["requirement_count"] != 2 {
		t.Fatalf("expected batch metadata, got %#v", metadata)
	}
	if metadata["batch_scope"] != "selected" {
		t.Fatalf("expected selected batch scope, got %#v", metadata)
	}
	if _, exists := metadata["requirement_id"]; exists {
		t.Fatalf("expected batch metadata to defer requirement id to each item, got %#v", metadata["requirement_id"])
	}
	if metadata["evidence"] != "已逐项补充响应依据" {
		t.Fatalf("expected evidence to be preserved, got %#v", metadata["evidence"])
	}
}

func TestBatchRequirementCoverageMetadataRejectsEmptyOrOversizedBatch(t *testing.T) {
	for name, req := range map[string]BatchUpdateRequirementCoverageRequest{
		"empty": {
			RequirementIDs: []string{" ", ""},
			CoverageStatus: "covered",
		},
		"oversized": {
			RequirementIDs: func() []string {
				ids := make([]string, requirementCoverageBatchLimit+1)
				for index := range ids {
					ids[index] = fmt.Sprintf("evaluation-%03d", index)
				}
				return ids
			}(),
			CoverageStatus: "covered",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := batchRequirementIDsFromRequest(req); err != ErrInvalidRequest {
				t.Fatalf("expected invalid batch to be rejected, got %v", err)
			}
		})
	}
}

func TestBatchRequirementCoverageMetadataTracksFilteredScope(t *testing.T) {
	req := BatchUpdateRequirementCoverageRequest{
		ApplyAll:       true,
		Filter:         "review",
		EvidenceFilter: "missing_source",
		CoverageStatus: "planned",
	}
	if ids, err := batchRequirementIDsFromRequest(req); err != nil || ids != nil {
		t.Fatalf("expected filtered request to defer ids, ids=%#v err=%v", ids, err)
	}
	status, needsReview, metadata, err := batchRequirementCoverageMetadata("user-1", req, 12)
	if err != nil {
		t.Fatalf("batch coverage metadata: %v", err)
	}
	if status != "planned" || needsReview {
		t.Fatalf("unexpected normalized status: status=%q needsReview=%v", status, needsReview)
	}
	if metadata["batch_scope"] != "filtered" || metadata["filter"] != "review" || metadata["evidence_filter"] != "missing_source" {
		t.Fatalf("expected filtered scope metadata, got %#v", metadata)
	}
	if metadata["requirement_count"] != 12 {
		t.Fatalf("expected filtered requirement count, got %#v", metadata["requirement_count"])
	}
}

func TestBatchRequirementCoverageRejectsInvalidFilteredScope(t *testing.T) {
	for name, req := range map[string]BatchUpdateRequirementCoverageRequest{
		"invalid status filter": {
			ApplyAll:       true,
			Filter:         "everything",
			EvidenceFilter: "all",
		},
		"invalid evidence filter": {
			ApplyAll:       true,
			Filter:         "all",
			EvidenceFilter: "nope",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := batchRequirementIDsFromRequest(req); err != ErrInvalidRequest {
				t.Fatalf("expected invalid filtered scope to be rejected, got %v", err)
			}
		})
	}
}

func TestRequirementIDsForBatchFiltersMatchesFrontendFilters(t *testing.T) {
	items := []RequirementItem{
		{
			ID:             "req-covered-complete",
			Mandatory:      true,
			CoverageStatus: "covered",
			Metadata: map[string]any{"latest_coverage": map[string]any{
				"evidence":    "已响应",
				"source_refs": []any{map[string]any{"title": "实施方案"}},
			}},
		},
		{
			ID:             "req-missing-evidence",
			CoverageStatus: "covered",
			Metadata:       map[string]any{"latest_coverage": map[string]any{"source_refs": []any{map[string]any{"title": "实施方案"}}}},
		},
		{
			ID:             "req-missing-source",
			CoverageStatus: "covered",
			Metadata:       map[string]any{"latest_coverage": map[string]any{"evidence": "已响应"}},
		},
		{
			ID:             "req-review",
			CoverageStatus: "needs_review",
			NeedsReview:    true,
			Metadata:       map[string]any{},
		},
	}
	if ids := requirementIDsForBatchFilters(items, "covered", "missing_evidence"); len(ids) != 1 || ids[0] != "req-missing-evidence" {
		t.Fatalf("expected missing evidence item, got %#v", ids)
	}
	if ids := requirementIDsForBatchFilters(items, "covered", "missing_source"); len(ids) != 1 || ids[0] != "req-missing-source" {
		t.Fatalf("expected missing source item, got %#v", ids)
	}
	if ids := requirementIDsForBatchFilters(items, "mandatory", "complete"); len(ids) != 1 || ids[0] != "req-covered-complete" {
		t.Fatalf("expected mandatory complete item, got %#v", ids)
	}
	if ids := requirementIDsForBatchFilters(items, "review", "all"); len(ids) != 1 || ids[0] != "req-review" {
		t.Fatalf("expected review item, got %#v", ids)
	}
}

func TestRequirementCoverageMetadataForItemAddsRequirementID(t *testing.T) {
	base := map[string]any{"action": "batch_review", "batch": true}
	metadata := requirementCoverageMetadataForItem(base, "evaluation-001")
	if metadata["requirement_id"] != "evaluation-001" {
		t.Fatalf("expected item requirement id, got %#v", metadata)
	}
	if _, exists := base["requirement_id"]; exists {
		t.Fatalf("expected base metadata to remain unchanged, got %#v", base)
	}
}

func TestNullableUUIDTextOnlyAllowsValidUUID(t *testing.T) {
	valid := "52000000-0000-4000-8000-000000000001"
	if got := nullableUUIDText(" " + valid + " "); got != valid {
		t.Fatalf("expected valid UUID to be preserved, got %q", got)
	}
	for _, value := range []string{"", "user-1", "not-a-uuid"} {
		if got := nullableUUIDText(value); got != "" {
			t.Fatalf("expected invalid UUID %q to be cleared, got %q", value, got)
		}
	}
}

func TestBuildGenerationCoverageSpecUsesEvaluatorContract(t *testing.T) {
	score := 12.5
	coverage := []any{
		map[string]any{
			"requirement_id": "evaluation-001",
			"satisfied":      true,
			"source_refs": []any{
				map[string]any{
					"chunk_id":     "chunk-1",
					"document_id":  "doc-1",
					"reference_id": "SRC-001",
					"page_start":   3,
				},
			},
		},
	}
	chapter := GenerationCoverageChapter{
		ID:        "chapter-1",
		BidPartID: "part-1",
		PartCode:  "tech",
		PartTitle: "技术标",
		Title:     "实施方案",
		Status:    "generated",
		SourceRefs: []any{map[string]any{
			"chunk_id":    "chunk-1",
			"document_id": "doc-1",
			"citation_id": "SRC-001",
			"page_start":  3,
		}},
		ModelMetadata: map[string]any{
			"self_check": map[string]any{
				"requirement_coverage": coverage,
			},
		},
	}
	chapter.RequirementCoverage = requirementCoverageFromModelMetadata(chapter.ModelMetadata)
	generatedAt := time.Unix(0, 0).UTC()

	spec := buildGenerationCoverageSpec(
		Document{ID: "bid-1", Title: "桥梁检查服务"},
		[]RequirementItem{
			{
				ID:               "requirement-db-1",
				ExternalID:       "evaluation-001",
				Module:           "evaluation",
				Type:             "scoring",
				Requirement:      "技术方案完整",
				Priority:         "high",
				Mandatory:        true,
				Score:            &score,
				ExpectedResponse: "章节需完整响应",
				CoverageStatus:   "covered",
				SourceRef:        map[string]any{"page_start": 3},
				Metadata:         map[string]any{"source": "tender_parse"},
			},
		},
		[]GenerationCoverageChapter{chapter},
		[]GenerationCoverageKnowledgeChunk{{ChunkID: "chunk-1", DocumentID: "doc-1", Title: "知识片段"}},
		generatedAt,
	)

	if spec.Name != "bid-generation-coverage-bid-1" || spec.BidTitle != "桥梁检查服务" {
		t.Fatalf("unexpected spec identity: %#v", spec)
	}
	if len(spec.Requirements) != 1 || spec.Requirements[0].ID != "evaluation-001" {
		t.Fatalf("expected evaluator requirement id to use external id, got %#v", spec.Requirements)
	}
	if spec.Requirements[0].DatabaseID != "requirement-db-1" {
		t.Fatalf("expected database id to be retained, got %#v", spec.Requirements[0])
	}
	if len(spec.Chapters) != 1 || len(spec.Chapters[0].RequirementCoverage) != 1 {
		t.Fatalf("expected chapter coverage to be exposed at top level, got %#v", spec.Chapters)
	}
	if !spec.RequireSourceRefs ||
		spec.Thresholds["min_mandatory_coverage_ratio"] != 1 ||
		spec.Thresholds["min_source_ref_resolution_ratio"] != 0.95 ||
		spec.Thresholds["min_source_ref_reference_id_ratio"] != 1 ||
		spec.Thresholds["min_source_ref_location_ratio"] != 1 {
		t.Fatalf("expected evaluator thresholds and source requirement, got %#v", spec)
	}
	sourceRef, ok := spec.Chapters[0].SourceRefs[0].(map[string]any)
	if !ok || sourceRef["citation_id"] != "SRC-001" || sourceRef["page_start"] != 3 {
		t.Fatalf("expected generation source refs to retain citation and location, got %#v", spec.Chapters[0].SourceRefs)
	}
	if !spec.GeneratedAt.Equal(generatedAt) {
		t.Fatalf("expected generated_at to be retained, got %v", spec.GeneratedAt)
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

func TestExportFilenameSanitizesUnsafeTitleCharacters(t *testing.T) {
	got := exportFilename(" ..桥梁\n检查:服务? ", "tech", "docx")
	if got != "桥梁检查-服务-技术标.docx" {
		t.Fatalf("unexpected sanitized export filename: %q", got)
	}
	if strings.ContainsAny(got, "\r\n\t/\\:*?\"<>|") {
		t.Fatalf("expected export filename to remove unsafe characters, got %q", got)
	}
}

func TestExportFilenameCapsLongTitleAndPreservesSuffix(t *testing.T) {
	got := exportFilename(strings.Repeat("标", maxExportFilenameRunes+50), "combined_body", "pdf")
	if runeCount := len([]rune(got)); runeCount > maxExportFilenameRunes {
		t.Fatalf("expected export filename to stay within %d runes, got %d", maxExportFilenameRunes, runeCount)
	}
	if !strings.HasSuffix(got, "-综合标书.pdf") {
		t.Fatalf("expected export filename to preserve business suffix, got %q", got)
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
