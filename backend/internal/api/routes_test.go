package api

import "testing"

func TestFileAccessModuleMapsBidFilesToBidPermission(t *testing.T) {
	for _, bizType := range []string{"bid_tender", "bid_export", " BID_ATTACHMENT "} {
		if got := fileAccessModule(bizType); got != "bid" {
			t.Fatalf("expected %q to use bid permission, got %q", bizType, got)
		}
	}
}

func TestFileAccessModuleDefaultsToKnowledgePermission(t *testing.T) {
	for _, bizType := range []string{"", "knowledge", "knowledge_case"} {
		if got := fileAccessModule(bizType); got != "knowledge" {
			t.Fatalf("expected %q to use knowledge permission, got %q", bizType, got)
		}
	}
}
