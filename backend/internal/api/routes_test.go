package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRouteSpecsAreAllHandledByRealRoutes(t *testing.T) {
	custom := customRouteSet()
	routeKeys := map[string]bool{}
	missing := []string{}
	for _, spec := range routeSpecs {
		key := spec.Method + " " + spec.Path
		routeKeys[key] = true
		if !custom[key] {
			missing = append(missing, key)
		}
	}
	extra := []string{}
	for key := range custom {
		if !routeKeys[key] {
			extra = append(extra, key)
		}
	}

	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("routeSpecs and real handler set diverged; missing handlers=%v extra handlers=%v", missing, extra)
	}
}

func TestBindOptionalJSONAllowsUnknownLengthEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/optional", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if !bindOptionalJSON(c, &req) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": req.Name})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/optional", strings.NewReader(""))
	request.ContentLength = -1
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected optional empty body to be accepted, got %d", recorder.Code)
	}
}

func TestBindOptionalJSONRejectsMalformedUnknownLengthBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/optional", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if !bindOptionalJSON(c, &req) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": req.Name})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/optional", strings.NewReader("{"))
	request.ContentLength = -1
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed optional body to be rejected, got %d", recorder.Code)
	}
}

func TestLimitRequestBodyRejectsKnownOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	called := false
	router.Use(limitRequestBody(4))
	router.POST("/payload", func(c *gin.Context) {
		called = true
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/payload", strings.NewReader("12345"))
	router.ServeHTTP(recorder, request)

	if called {
		t.Fatal("expected oversized request to abort before handler")
	}
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d", recorder.Code)
	}
}

func TestLimitRequestBodyAllowsSmallBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(8))
	router.POST("/payload", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/payload", strings.NewReader("1234"))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected small request to pass, got %d", recorder.Code)
	}
}

func TestBindJSONMapsUnknownLengthOversizedBodyToPayloadTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(8))
	router.POST("/payload", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if !bindJSON(c, &req) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/payload", strings.NewReader(`{"name":"oversized"}`))
	request.ContentLength = -1
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for unknown-length oversized body, got %d", recorder.Code)
	}
}

func TestRawBodyReadMapsUnknownLengthOversizedBodyToPayloadTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(8))
	router.POST("/payload", func(c *gin.Context) {
		if _, err := c.GetRawData(); err != nil {
			respondBodyReadError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/payload", strings.NewReader("123456789"))
	request.ContentLength = -1
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for unknown-length oversized raw body, got %d", recorder.Code)
	}
}

func TestMaxRequestBodyBytesUsesSafeEnvBounds(t *testing.T) {
	t.Setenv("API_MAX_BODY_BYTES", "2097152")
	if got := maxRequestBodyBytes(); got != 2*1024*1024 {
		t.Fatalf("expected env body limit, got %d", got)
	}

	t.Setenv("API_MAX_BODY_BYTES", "1")
	if got := maxRequestBodyBytes(); got != defaultMaxRequestBodyBytes {
		t.Fatalf("expected tiny env body limit to fall back, got %d", got)
	}

	t.Setenv("API_MAX_BODY_BYTES", "999999999")
	if got := maxRequestBodyBytes(); got != maxMaxRequestBodyBytes {
		t.Fatalf("expected huge env body limit to clamp, got %d", got)
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
