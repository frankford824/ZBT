package bid

import "testing"

func TestOverviewIncludesCriticalRisksWithoutTitleKeywordMatch(t *testing.T) {
	structured := map[string]any{"requirement_items": []any{
		map[string]any{"id": "risk-1", "module": "invalid_risk", "type": "risk", "requirement": "逾期送达将被拒收", "mandatory": true, "source_text": "逾期送达将被拒收"},
		map[string]any{"id": "score-1", "module": "evaluation", "type": "score", "requirement": "技术方案40分", "score": float64(40), "source_text": "技术方案40分"},
	}}
	refs := requirementRefsFromStructuredResult(structured, "一、项目理解", 48)
	found := map[string]bool{}
	for _, ref := range refs {
		found[ref.ID] = true
	}
	if !found["risk-1"] || !found["score-1"] {
		t.Fatalf("critical requirements omitted: %#v", refs)
	}
}
