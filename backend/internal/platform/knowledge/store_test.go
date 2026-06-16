package knowledge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
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
