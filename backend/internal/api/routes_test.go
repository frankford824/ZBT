package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frankford824/ZBT/backend/internal/platform/aicall"
	"github.com/gin-gonic/gin"
)

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

func TestRespondStatusMapsAICallInvalidRequestToBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	respondStatus(context, http.StatusOK, gin.H{"status": "ok"}, aicall.ErrInvalidRequest)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid AI call request, got %d", recorder.Code)
	}
}
