package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestVectorLiteralFromEmbedding(t *testing.T) {
	values := make([]float64, 1024)
	values[0] = 0.25
	values[1023] = -0.5

	literal, err := vectorLiteralFromEmbedding(values)
	if err != nil {
		t.Fatalf("vectorLiteralFromEmbedding returned error: %v", err)
	}
	if !strings.HasPrefix(literal, "[0.25,") || !strings.HasSuffix(literal, "-0.5]") {
		t.Fatalf("unexpected vector literal: %s", literal)
	}
}

func TestVectorLiteralFromEmbeddingRejectsInvalidValues(t *testing.T) {
	if _, err := vectorLiteralFromEmbedding([]float64{1}); err == nil {
		t.Fatal("expected dimension mismatch")
	}

	values := make([]float64, 1024)
	values[8] = math.NaN()
	if _, err := vectorLiteralFromEmbedding(values); err == nil {
		t.Fatal("expected non-finite value error")
	}
}

func TestSearchCandidateLimit(t *testing.T) {
	if got := searchCandidateLimit(1); got != 30 {
		t.Fatalf("small limits should still retrieve 30 candidates, got %d", got)
	}
	if got := searchCandidateLimit(8); got != 32 {
		t.Fatalf("expected 4x candidate fanout, got %d", got)
	}
	if got := searchCandidateLimit(20); got != 60 {
		t.Fatalf("candidate fanout should cap at 60, got %d", got)
	}
}

func TestApplyKnowledgeRerankUsesProviderOrderAndFillsRemainder(t *testing.T) {
	candidates := []SearchResult{
		{ChunkID: "chunk-a", Title: "A", Score: 0.01},
		{ChunkID: "chunk-b", Title: "B", Score: 0.02},
		{ChunkID: "chunk-c", Title: "C", Score: 0.03},
	}

	reranked := applyKnowledgeRerank(candidates, []rerankResult{
		{ID: "chunk-c", Score: 1},
		{ID: "missing", Score: 0.9},
	}, 3)

	if len(reranked) != 3 {
		t.Fatalf("expected rerank to fill remainder, got %d", len(reranked))
	}
	if reranked[0].ChunkID != "chunk-c" || reranked[0].Score != 1 {
		t.Fatalf("expected provider winner first with rerank score, got %+v", reranked[0])
	}
	if reranked[1].ChunkID != "chunk-a" || reranked[2].ChunkID != "chunk-b" {
		t.Fatalf("expected original order for remainder, got %s then %s", reranked[1].ChunkID, reranked[2].ChunkID)
	}
}

func TestTruncateForRerankPreservesRuneBoundaries(t *testing.T) {
	if got := truncateForRerank("智慧交通ABC", 4); got != "智慧交通" {
		t.Fatalf("unexpected truncation: %q", got)
	}
}

func TestNormalizeKnowledgeSearchQueryTrimsAndCapsRunes(t *testing.T) {
	oversized := strings.Repeat("智", maxKnowledgeSearchQueryChars+5)
	got := normalizeKnowledgeSearchQuery(" \n" + oversized + "\t")

	if count := len([]rune(got)); count != maxKnowledgeSearchQueryChars {
		t.Fatalf("expected %d runes, got %d", maxKnowledgeSearchQueryChars, count)
	}
	if strings.ContainsAny(got, " \n\t") {
		t.Fatalf("expected query whitespace to be trimmed, got %q", got)
	}
}

func TestNormalizeKnowledgeCategoryInputBoundsText(t *testing.T) {
	name, description, err := normalizeKnowledgeCategoryInput(" 资质证书 ", " 企业资质材料 ", false)
	if err != nil {
		t.Fatalf("expected category input to normalize: %v", err)
	}
	if name != "资质证书" || description != "企业资质材料" {
		t.Fatalf("unexpected category normalization: name=%q description=%q", name, description)
	}

	for _, req := range []struct {
		name        string
		description string
		allowBlank  bool
	}{
		{name: " ", description: "", allowBlank: false},
		{name: strings.Repeat("类", maxKnowledgeCategoryNameRunes+1), description: "", allowBlank: false},
		{name: "分类", description: strings.Repeat("说", maxKnowledgeCategoryDescriptionRunes+1), allowBlank: false},
	} {
		if _, _, err := normalizeKnowledgeCategoryInput(req.name, req.description, req.allowBlank); err != ErrInvalidRequest {
			t.Fatalf("expected invalid category input to be rejected, got %v for %+v", err, req)
		}
	}
}

func TestNormalizeKnowledgeTagInputBoundsTextAndColor(t *testing.T) {
	name, color, err := normalizeKnowledgeTagInput(" 技术方案 ", " GREEN ", false)
	if err != nil {
		t.Fatalf("expected tag input to normalize: %v", err)
	}
	if name != "技术方案" || color != "green" {
		t.Fatalf("unexpected tag normalization: name=%q color=%q", name, color)
	}
	_, color, err = normalizeKnowledgeTagInput("项目案例", "", false)
	if err != nil || color != "blue" {
		t.Fatalf("expected blank create color to default to blue, got color=%q err=%v", color, err)
	}
	_, color, err = normalizeKnowledgeTagInput("", "", true)
	if err != nil || color != "" {
		t.Fatalf("expected blank update tag fields to preserve existing values, got color=%q err=%v", color, err)
	}

	for _, req := range []struct {
		name       string
		color      string
		allowBlank bool
	}{
		{name: " ", color: "blue", allowBlank: false},
		{name: strings.Repeat("标", maxKnowledgeTagNameRunes+1), color: "blue", allowBlank: false},
		{name: "标签", color: "raw-css-color", allowBlank: false},
	} {
		if _, _, err := normalizeKnowledgeTagInput(req.name, req.color, req.allowBlank); err != ErrInvalidRequest {
			t.Fatalf("expected invalid tag input to be rejected, got %v for %+v", err, req)
		}
	}
}

func TestNormalizeKnowledgeDocumentUpdateBoundsFieldsAndIDs(t *testing.T) {
	categoryID := "00000000-0000-4000-8000-000000000010"
	_, normalized, err := normalizeKnowledgeDocumentUpdate("00000000-0000-4000-8000-000000000001", UpdateDocumentRequest{
		Title:      " 实施方案 ",
		DocType:    " word ",
		CategoryID: &categoryID,
		TagIDs: []string{
			"00000000-0000-4000-8000-000000000101",
			"00000000-0000-4000-8000-000000000101",
			"00000000-0000-4000-8000-000000000102",
		},
		Summary: " 摘要 ",
	})
	if err != nil {
		t.Fatalf("expected document update to normalize: %v", err)
	}
	if normalized.Title != "实施方案" || normalized.DocType != "word" || normalized.Summary != "摘要" {
		t.Fatalf("unexpected document normalization: %+v", normalized)
	}
	if normalized.CategoryID == nil || *normalized.CategoryID != categoryID {
		t.Fatalf("expected category id to normalize, got %+v", normalized.CategoryID)
	}
	if len(normalized.TagIDs) != 2 {
		t.Fatalf("expected duplicate tag ids to be deduped, got %+v", normalized.TagIDs)
	}
}

func TestNormalizeKnowledgeDocumentUpdateRejectsInvalidFields(t *testing.T) {
	validID := "00000000-0000-4000-8000-000000000001"
	blankCategoryID := " "
	tooManyTags := make([]string, maxKnowledgeDocumentTagIDs+1)
	for index := range tooManyTags {
		tooManyTags[index] = "00000000-0000-4000-8000-000000000001"
	}
	for name, req := range map[string]struct {
		id      string
		request UpdateDocumentRequest
	}{
		"invalid document id": {
			id: "not-a-uuid",
		},
		"oversized title": {
			id:      validID,
			request: UpdateDocumentRequest{Title: strings.Repeat("题", maxKnowledgeDocumentTitleRunes+1)},
		},
		"invalid doc type": {
			id:      validID,
			request: UpdateDocumentRequest{DocType: "raw_type"},
		},
		"oversized summary": {
			id:      validID,
			request: UpdateDocumentRequest{Summary: strings.Repeat("摘", maxKnowledgeDocumentSummaryRunes+1)},
		},
		"invalid category id": {
			id:      validID,
			request: UpdateDocumentRequest{CategoryID: ptrString("bad-category")},
		},
		"blank category id clears value": {
			id:      validID,
			request: UpdateDocumentRequest{CategoryID: &blankCategoryID},
		},
		"invalid tag id": {
			id:      validID,
			request: UpdateDocumentRequest{TagIDs: []string{"bad-tag"}},
		},
		"too many tag ids": {
			id:      validID,
			request: UpdateDocumentRequest{TagIDs: tooManyTags},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, normalized, err := normalizeKnowledgeDocumentUpdate(req.id, req.request)
			if name == "blank category id clears value" {
				if err != nil || normalized.CategoryID != nil {
					t.Fatalf("expected blank category id to clear value, got category=%+v err=%v", normalized.CategoryID, err)
				}
				return
			}
			if err != ErrInvalidRequest {
				t.Fatalf("expected invalid document update to be rejected, got %v", err)
			}
		})
	}
}

func TestKnowledgeWriteMethodsRejectInvalidInputsBeforeDB(t *testing.T) {
	store := &Store{}

	if _, err := store.CreateCategory(context.Background(), "tenant-id", strings.Repeat("类", maxKnowledgeCategoryNameRunes+1), ""); err != ErrInvalidRequest {
		t.Fatalf("expected oversized category name to be rejected before DB, got %v", err)
	}
	if _, err := store.UpdateCategory(context.Background(), "tenant-id", "bad-id", "分类", ""); err != ErrInvalidRequest {
		t.Fatalf("expected invalid category id to be rejected before DB, got %v", err)
	}
	if _, err := store.CreateTag(context.Background(), "tenant-id", "标签", "raw-css-color"); err != ErrInvalidRequest {
		t.Fatalf("expected invalid tag color to be rejected before DB, got %v", err)
	}
	if _, err := store.UpdateTag(context.Background(), "tenant-id", "bad-id", "标签", "blue"); err != ErrInvalidRequest {
		t.Fatalf("expected invalid tag id to be rejected before DB, got %v", err)
	}
	if _, err := store.UpdateDocument(context.Background(), "tenant-id", "bad-id", UpdateDocumentRequest{}); err != ErrInvalidRequest {
		t.Fatalf("expected invalid document id to be rejected before DB, got %v", err)
	}
}

func TestEstimateTokensForRerankOutputUsesSerializedPayload(t *testing.T) {
	results := []rerankResult{
		{ID: "chunk-a", Index: 0, Score: 1},
		{ID: "chunk-b", Index: 1, Score: 0.5},
	}

	got := estimateTokensForRerankOutput(results)
	want := estimateTokens(`[{"id":"chunk-a","index":0,"score":1},{"id":"chunk-b","index":1,"score":0.5}]`)
	if got != want {
		t.Fatalf("expected serialized rerank output token estimate %d, got %d", want, got)
	}
	if got <= len(results) {
		t.Fatalf("expected token estimate to exceed result count, got %d for %d results", got, len(results))
	}
	if empty := estimateTokensForRerankOutput(nil); empty != 0 {
		t.Fatalf("expected empty rerank output to cost 0 tokens, got %d", empty)
	}
}

func TestResponseTokenOrEstimatePrefersProviderUsage(t *testing.T) {
	if got := responseTokenOrEstimate(map[string]int{"input_tokens": 42}, "input_tokens", 7); got != 42 {
		t.Fatalf("expected provider token usage to win, got %d", got)
	}
	if got := responseTokenOrEstimate(map[string]int{"input_tokens": -1}, "input_tokens", 7); got != 7 {
		t.Fatalf("expected invalid provider token usage to fall back, got %d", got)
	}
	if got := responseTokenOrEstimate(nil, "output_tokens", 0); got != 0 {
		t.Fatalf("expected empty fallback to stay zero, got %d", got)
	}
}

func TestPositiveFiniteCostRejectsUnsafeValues(t *testing.T) {
	if got := positiveFiniteCost(0.125); got != 0.125 {
		t.Fatalf("expected positive cost to pass through, got %.6f", got)
	}
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := positiveFiniteCost(value); got != 0 {
			t.Fatalf("expected unsafe cost %v to be zero, got %.6f", value, got)
		}
	}
}

func TestValidateKnowledgeChunksRejectsUnsafeCallbackChunks(t *testing.T) {
	for name, chunks := range map[string][]ChunkInput{
		"empty list": {},
		"empty content": {
			{Title: "空片段", Content: "   "},
		},
		"too many chunks": func() []ChunkInput {
			items := make([]ChunkInput, maxKnowledgeCallbackChunks+1)
			for index := range items {
				items[index] = ChunkInput{Title: "片段", Content: "内容"}
			}
			return items
		}(),
		"oversized content": {
			{Title: "片段", Content: strings.Repeat("内", maxKnowledgeChunkContentChars+1)},
		},
		"oversized title": {
			{Title: strings.Repeat("题", maxKnowledgeChunkTitleChars+1), Content: "内容"},
		},
		"oversized section": {
			{Title: "片段", SectionPath: strings.Repeat("章", maxKnowledgeChunkSectionChars+1), Content: "内容"},
		},
	} {
		if err := validateKnowledgeChunks(chunks); err != ErrInvalidRequest {
			t.Fatalf("expected %s to be rejected, got %v", name, err)
		}
	}
}

func TestValidateKnowledgeChunksAllowsBoundedCallbackChunks(t *testing.T) {
	chunks := []ChunkInput{
		{Title: "片段", SectionPath: "章节", Content: strings.Repeat("内", maxKnowledgeChunkContentChars)},
	}

	if err := validateKnowledgeChunks(chunks); err != nil {
		t.Fatalf("expected bounded chunk to be accepted, got %v", err)
	}
}

func TestKnowledgeChunkMetadataJSONAddsIndexWithoutMutatingInput(t *testing.T) {
	metadata := map[string]any{"source": "parser"}
	encoded, err := knowledgeChunkMetadataJSON(metadata, 3)
	if err != nil {
		t.Fatalf("expected metadata json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid metadata json: %v", err)
	}
	if decoded["chunk_index"] != float64(3) || decoded["source"] != "parser" {
		t.Fatalf("unexpected metadata json: %v", decoded)
	}
	if _, ok := metadata["chunk_index"]; ok {
		t.Fatal("expected input metadata map not to be mutated")
	}
}

func TestKnowledgeChunkMetadataJSONRejectsOversizedMetadata(t *testing.T) {
	_, err := knowledgeChunkMetadataJSON(
		map[string]any{"memo": strings.Repeat("x", maxKnowledgeChunkMetadataBytes+1)},
		1,
	)
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized metadata to be rejected, got %v", err)
	}
}

func TestNormalizeDocumentTemplateRequestTrimsDefaultsAndBoundsContent(t *testing.T) {
	normalized, contentJSON, err := normalizeDocumentTemplateRequest(CreateDocumentTemplateRequest{
		Name:        " 项目实施方案 ",
		Description: " 模板说明 ",
		Content: map[string]any{
			"sections": []any{"项目理解", "实施计划"},
		},
	})
	if err != nil {
		t.Fatalf("expected template request to normalize: %v", err)
	}
	if normalized.Name != "项目实施方案" || normalized.Category != "通用模板" || normalized.Version != "v1.0" {
		t.Fatalf("unexpected normalized template identity: %+v", normalized)
	}
	if normalized.Description != "模板说明" {
		t.Fatalf("expected description to be trimmed, got %q", normalized.Description)
	}
	var decoded map[string]any
	if err := json.Unmarshal(contentJSON, &decoded); err != nil {
		t.Fatalf("expected valid content json: %v", err)
	}
	if _, ok := decoded["sections"]; !ok {
		t.Fatalf("expected content to be preserved, got %#v", decoded)
	}
}

func TestNormalizeDocumentTemplateRequestRejectsOversizedFieldsAndContent(t *testing.T) {
	for name, req := range map[string]CreateDocumentTemplateRequest{
		"blank name": {
			Name: " ",
		},
		"oversized name": {
			Name: strings.Repeat("模", maxKnowledgeTemplateNameRunes+1),
		},
		"oversized category": {
			Name:     "模板",
			Category: strings.Repeat("类", maxKnowledgeTemplateCategoryRunes+1),
		},
		"oversized description": {
			Name:        "模板",
			Description: strings.Repeat("说", maxKnowledgeTemplateDescriptionRunes+1),
		},
		"oversized version": {
			Name:    "模板",
			Version: strings.Repeat("v", maxKnowledgeTemplateVersionRunes+1),
		},
		"oversized content": {
			Name:    "模板",
			Content: map[string]any{"payload": strings.Repeat("内", maxKnowledgeTemplateContentJSONBytes)},
		},
		"non json content": {
			Name:    "模板",
			Content: map[string]any{"bad": func() {}},
		},
		"invalid number content": {
			Name:    "模板",
			Content: map[string]any{"bad": math.Inf(1)},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := normalizeDocumentTemplateRequest(req); err != ErrInvalidRequest {
				t.Fatalf("expected invalid template request, got %v", err)
			}
		})
	}
}

func TestCreateDocumentTemplateRejectsInvalidRequestBeforeDB(t *testing.T) {
	store := &Store{}

	_, err := store.CreateDocumentTemplate(context.Background(), "tenant-id", CreateDocumentTemplateRequest{
		Name:    "模板",
		Content: map[string]any{"bad": func() {}},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid template content to be rejected before DB, got %v", err)
	}
}

func TestKnowledgeCallbackRejectsInvalidResultBeforeDB(t *testing.T) {
	store := &Store{}

	_, err := store.ApplyCallback(context.Background(), CallbackPayload{
		TenantID: "tenant-id",
		TaskID:   "task-knowledge-00000000-0000-4000-8000-000000000001",
		Status:   "failed",
		Result: map[string]any{
			"invalid": math.NaN(),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid callback result JSON to be rejected before DB, got %v", err)
	}
}

func TestKnowledgeCallbackRejectsOversizedResultBeforeDB(t *testing.T) {
	store := &Store{}

	_, err := store.ApplyCallback(context.Background(), CallbackPayload{
		TenantID: "tenant-id",
		TaskID:   "task-knowledge-00000000-0000-4000-8000-000000000001",
		Status:   "failed",
		Result: map[string]any{
			"payload": strings.Repeat("结", maxKnowledgeTaskResultJSONBytes),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized callback result to be rejected before DB, got %v", err)
	}
}

func TestScanTaskRejectsInvalidStoredJSONFields(t *testing.T) {
	for name, row := range map[string]knowledgeTaskScanRow{
		"invalid payload object": {
			payloadRaw: []byte(`[{"bad":"shape"}]`),
			routeRaw:   []byte(`{}`),
			resultRaw:  []byte(`{}`),
		},
		"invalid route json": {
			payloadRaw: []byte(`{}`),
			routeRaw:   []byte(`{"route":`),
			resultRaw:  []byte(`{}`),
		},
		"oversized result": {
			payloadRaw: []byte(`{}`),
			routeRaw:   []byte(`{}`),
			resultRaw:  []byte(`{"payload":"` + strings.Repeat("结", maxKnowledgeTaskResultJSONBytes) + `"}`),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := scanTask(row); err == nil {
				t.Fatal("expected invalid stored task JSON to be rejected")
			}
		})
	}
}

func TestScanTaskNormalizesEmptyStoredJSONFields(t *testing.T) {
	task, err := scanTask(knowledgeTaskScanRow{
		payloadRaw: nil,
		routeRaw:   []byte(`null`),
		resultRaw:  []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("expected empty stored task JSON fields to normalize, got %v", err)
	}
	if task.Payload == nil || len(task.Payload) != 0 || task.Route == nil || len(task.Route) != 0 || task.Result == nil || len(task.Result) != 0 {
		t.Fatalf("expected empty task JSON maps, got payload=%#v route=%#v result=%#v", task.Payload, task.Route, task.Result)
	}
}

func TestNormalizeKnowledgeCallbackBoundsDocumentFields(t *testing.T) {
	for name, payload := range map[string]CallbackPayload{
		"oversized processed title": {
			TenantID:       "tenant-id",
			TaskID:         "task-knowledge-00000000-0000-4000-8000-000000000001",
			Status:         "failed",
			ProcessedTitle: strings.Repeat("题", maxKnowledgeDocumentTitleRunes+1),
		},
		"oversized explicit summary": {
			TenantID: "tenant-id",
			TaskID:   "task-knowledge-00000000-0000-4000-8000-000000000001",
			Status:   "failed",
			Summary:  strings.Repeat("摘", maxKnowledgeDocumentSummaryRunes+1),
		},
		"oversized result summary": {
			TenantID: "tenant-id",
			TaskID:   "task-knowledge-00000000-0000-4000-8000-000000000001",
			Status:   "failed",
			Result: map[string]any{
				"summary": strings.Repeat("摘", maxKnowledgeDocumentSummaryRunes+1),
			},
		},
		"oversized error message": {
			TenantID:     "tenant-id",
			TaskID:       "task-knowledge-00000000-0000-4000-8000-000000000001",
			Status:       "failed",
			ErrorMessage: strings.Repeat("错", maxKnowledgeTaskErrorMessageRunes+1),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := normalizeKnowledgeCallbackPayload(payload); err != ErrInvalidRequest {
				t.Fatalf("expected invalid callback document fields to be rejected, got %v", err)
			}
		})
	}
}

func TestKnowledgeCallbackRejectsInvalidDoneChunksBeforeDB(t *testing.T) {
	store := &Store{}

	_, err := store.ApplyCallback(context.Background(), CallbackPayload{
		TenantID: "tenant-id",
		TaskID:   "task-knowledge-00000000-0000-4000-8000-000000000001",
		Status:   "done",
		Result:   map[string]any{},
		Chunks: []ChunkInput{
			{Title: "空片段", Content: "   "},
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid done chunks to be rejected before DB, got %v", err)
	}
}

func TestNormalizeAcceptedKnowledgeTaskRejectsOversizedRoute(t *testing.T) {
	_, _, err := normalizeAcceptedKnowledgeTask(aiTaskAccepted{
		TaskID: "task-knowledge-00000000-0000-4000-8000-000000000001",
		Status: "running",
		Route: map[string]any{
			"payload": strings.Repeat("路", maxKnowledgeTaskRouteJSONBytes),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized accepted route to be rejected, got %v", err)
	}
}

func ptrString(value string) *string {
	return &value
}

type knowledgeTaskScanRow struct {
	payloadRaw []byte
	routeRaw   []byte
	resultRaw  []byte
}

func (row knowledgeTaskScanRow) Scan(dest ...any) error {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	*(dest[0].(*string)) = "task-1"
	*(dest[1].(*string)) = "knowledge_process"
	*(dest[2].(*string)) = "done"
	*(dest[3].(*sql.NullString)) = sql.NullString{}
	*(dest[4].(*string)) = "knowledge_document"
	*(dest[5].(*string)) = "00000000-0000-4000-8000-000000000001"
	*(dest[6].(*[]byte)) = row.payloadRaw
	*(dest[7].(*[]byte)) = row.routeRaw
	*(dest[8].(*[]byte)) = row.resultRaw
	*(dest[9].(*sql.NullString)) = sql.NullString{}
	*(dest[10].(*sql.NullTime)) = sql.NullTime{}
	*(dest[11].(*sql.NullTime)) = sql.NullTime{}
	*(dest[12].(*time.Time)) = now
	*(dest[13].(*time.Time)) = now
	return nil
}
