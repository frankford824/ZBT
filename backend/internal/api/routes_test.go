package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frankford824/ZBT/backend/internal/platform/aicall"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
	"github.com/gin-gonic/gin"
)

func TestFileAccessModuleMapsSupportedFileTypes(t *testing.T) {
	for _, tc := range []struct {
		bizType string
		module  string
	}{
		{bizType: "", module: "knowledge"},
		{bizType: "knowledge", module: "knowledge"},
		{bizType: "knowledge_case", module: "knowledge"},
		{bizType: "bid_tender", module: "bid"},
		{bizType: "bid_export", module: "bid"},
	} {
		module, ok := platformfile.AccessModuleForBizType(tc.bizType)
		if !ok || module != tc.module {
			t.Fatalf("expected %q to use %q permission, got module=%q ok=%v", tc.bizType, tc.module, module, ok)
		}
	}
}

func TestFileAccessModuleRejectsUnknownFileTypes(t *testing.T) {
	if _, ok := platformfile.AccessModuleForBizType(" BID_ATTACHMENT "); ok {
		t.Fatal("expected unsupported file biz type to be rejected")
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
