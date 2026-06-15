package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frankford824/ZBT/backend/internal/platform/aicall"
	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
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

func TestAITaskAccessModuleMapsResourceTypes(t *testing.T) {
	cases := []struct {
		resourceType string
		taskType     string
		want         string
	}{
		{resourceType: "knowledge_document", taskType: "knowledge_process", want: "knowledge"},
		{resourceType: "bid_chapter", taskType: "chapter_generate", want: "bid"},
		{resourceType: "bid_export", taskType: "document_export", want: "bid"},
		{resourceType: "cost_project", taskType: "cost_advice", want: "cost"},
		{resourceType: "", taskType: "tender_parse", want: "bid"},
		{resourceType: "", taskType: "unknown", want: "dashboard"},
	}
	for _, tc := range cases {
		if got := aiTaskAccessModule(tc.resourceType, tc.taskType); got != tc.want {
			t.Fatalf("expected %q/%q to map to %q, got %q", tc.resourceType, tc.taskType, tc.want, got)
		}
	}
}

func TestRouteInfosMarksAITaskRouteDynamic(t *testing.T) {
	routes := routeInfos()
	for _, route := range routes {
		if route.Method == http.MethodGet && route.Path == "/ai-tasks/:taskId" {
			if !route.DynamicModule {
				t.Fatal("expected AI task route metadata to mark dynamic module authorization")
			}
			return
		}
	}
	t.Fatal("expected AI task route metadata to be present")
}

func TestRequireAITaskAccessUsesResourceModule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set(rbac.ContextPermissionsKey, map[string]rbac.Level{
		"cost":      rbac.LevelRead,
		"dashboard": rbac.LevelNone,
	})

	if !requireAITaskAccess(context, "cost_project", "cost_advice") {
		t.Fatal("expected cost read permission to allow cost task polling without dashboard permission")
	}
	if context.IsAborted() {
		t.Fatal("expected allowed task access not to abort request")
	}
}

func TestRequireAITaskAccessDeniesMissingResourcePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set(rbac.ContextPermissionsKey, map[string]rbac.Level{
		"bid": rbac.LevelNone,
	})

	if requireAITaskAccess(context, "bid_chapter", "chapter_generate") {
		t.Fatal("expected missing bid read permission to deny bid task polling")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for denied task access, got %d", recorder.Code)
	}
}
