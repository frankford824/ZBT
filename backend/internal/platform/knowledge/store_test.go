package knowledge

import (
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
