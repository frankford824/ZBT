package bid

import "testing"

func TestManualStatusCannotForgeWorkflowProgress(t *testing.T) {
	for _, current := range []string{"draft", "editing", "archived", "generating"} {
		for _, target := range []string{"in_review", "approved", "submitted"} {
			if manualDocumentStatusAllowed(current, target) {
				t.Fatalf("status bypass allowed: %s -> %s", current, target)
			}
		}
	}
	if manualDocumentStatusAllowed("draft", "generating") || manualDocumentStatusAllowed("in_review", "editing") || manualDocumentStatusAllowed("approved", "editing") {
		t.Fatal("dedicated workflow can be bypassed")
	}
	for _, transition := range [][2]string{{"draft", "editing"}, {"editing", "draft"}, {"approved", "submitted"}, {"submitted", "archived"}, {"draft", ""}, {"approved", "approved"}} {
		if !manualDocumentStatusAllowed(transition[0], transition[1]) {
			t.Fatalf("legitimate transition denied: %v", transition)
		}
	}
}
