package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/aicall"
	platformapproval "github.com/frankford824/ZBT/backend/internal/platform/approval"
	"github.com/frankford824/ZBT/backend/internal/platform/auth"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
	"github.com/frankford824/ZBT/backend/internal/platform/saas"
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

func TestRespondStatusMapsApprovalForbiddenToForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	respondStatus(context, http.StatusOK, gin.H{"status": "ok"}, platformapproval.ErrForbidden)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for forbidden approval action, got %d", recorder.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode forbidden response: %v", err)
	}
	if body.Code != "permission_denied" {
		t.Fatalf("expected permission_denied response code, got %q", body.Code)
	}
}

func TestRespondSessionUsesConfiguredJWTAccessTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	respondSession(
		context,
		config.Config{JWTSecret: "secret", JWTAccessTTL: 15 * time.Minute},
		saas.Session{
			User:   saas.User{ID: "user-1", Email: "admin@example.com", Name: "管理员"},
			Tenant: saas.Tenant{ID: "tenant-1", Name: "企业"},
			Role:   saas.Role{ID: "role-1", Code: "admin", Name: "管理员"},
			Roles: []saas.Role{
				{ID: "role-1", Code: "admin", Name: "管理员"},
				{ID: "role-2", Code: "auditor", Name: "审核员"},
			},
			Permissions: map[string]rbac.Level{
				"dashboard": rbac.LevelRead,
			},
		},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected session response, got %d", recorder.Code)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if body.ExpiresIn != 900 {
		t.Fatalf("expected 15 minute expires_in, got %d", body.ExpiresIn)
	}
	claims, err := auth.ParseJWT("secret", body.AccessToken)
	if err != nil {
		t.Fatalf("parse signed token: %v", err)
	}
	remaining := time.Until(time.Unix(claims.ExpiresAt, 0))
	if remaining < 14*time.Minute || remaining > 16*time.Minute {
		t.Fatalf("expected token exp to use configured TTL, remaining %s", remaining)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "admin" || claims.Roles[1] != "auditor" {
		t.Fatalf("expected JWT to include all session roles, got %v", claims.Roles)
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
		wantOK       bool
	}{
		{resourceType: "knowledge_document", taskType: "knowledge_process", want: "knowledge", wantOK: true},
		{resourceType: "bid_chapter", taskType: "chapter_generate", want: "bid", wantOK: true},
		{resourceType: "bid_export", taskType: "document_export", want: "bid", wantOK: true},
		{resourceType: "cost_project", taskType: "cost_advice", want: "cost", wantOK: true},
		{resourceType: "", taskType: "tender_parse", want: "bid", wantOK: true},
		{resourceType: "", taskType: "unknown", want: "", wantOK: false},
	}
	for _, tc := range cases {
		got, ok := aiTaskAccessModule(tc.resourceType, tc.taskType)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("expected %q/%q to map to %q, got %q", tc.resourceType, tc.taskType, tc.want, got)
		}
	}
}

func TestRouteInfosMarksAITaskRouteDynamic(t *testing.T) {
	route, ok := routeInfoByKey(http.MethodGet, "/ai-tasks/:taskId")
	if !ok {
		t.Fatal("expected AI task route metadata to be present")
	}
	if !route.DynamicModule {
		t.Fatal("expected AI task route metadata to mark dynamic module authorization")
	}
}

func TestRouteInfosMarksFileRoutesDynamic(t *testing.T) {
	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/files/presign-upload"},
		{method: http.MethodPost, path: "/files/:id/confirm"},
		{method: http.MethodGet, path: "/files/:id/download-url"},
		{method: http.MethodGet, path: "/files/:id/preview-url"},
	} {
		route, ok := routeInfoByKey(tc.method, tc.path)
		if !ok {
			t.Fatalf("expected file route metadata to be present for %s %s", tc.method, tc.path)
		}
		if !route.DynamicModule {
			t.Fatalf("expected file route metadata to mark dynamic module authorization for %s %s", tc.method, tc.path)
		}
	}
}

func TestRouteInfosAsyncFlagsMatchTaskRoutes(t *testing.T) {
	expected := map[string]bool{
		"POST /bids/:id/upload-tender-file":     true,
		"POST /bids/:id/parse-tender":           true,
		"POST /bids/:id/outline/generate":       true,
		"POST /bids/:id/generate":               true,
		"POST /chapters/:chapterId/regenerate":  true,
		"POST /chapters/:chapterId/ai-action":   true,
		"POST /bids/:id/exports":                true,
		"POST /knowledge/documents/:id/process": true,
		"POST /compliance/checks":               true,
		"POST /compliance/issues/:id/autofix":   true,
		"POST /compliance/checks/:id/report":    true,
		"POST /cost-projects/:id/ai-advice":     true,
		"POST /cost-projects/:id/report":        true,
	}

	for _, route := range routeInfos() {
		key := route.Method + " " + route.Path
		if route.Async && !expected[key] {
			t.Fatalf("route %s is marked async but is not a task-style endpoint", key)
		}
		if expected[key] && !route.Async {
			t.Fatalf("route %s should be marked async", key)
		}
	}
}

func TestRouteInfosExposeAdditionalModuleRequirements(t *testing.T) {
	route, ok := routeInfoByKey(http.MethodPost, "/projects/:id/archive-case")
	if !ok {
		t.Fatal("expected archive-case route metadata to be present")
	}
	if len(route.AdditionalRequirements) != 1 {
		t.Fatalf("expected archive-case to expose one additional requirement, got %d", len(route.AdditionalRequirements))
	}
	requirement := route.AdditionalRequirements[0]
	if requirement.Module != "knowledge" || requirement.Required != rbac.LevelFull {
		t.Fatalf("expected archive-case to require additional knowledge full permission, got %+v", requirement)
	}
}

func routeInfoByKey(method, path string) (routeInfo, bool) {
	for _, route := range routeInfos() {
		if route.Method == method && route.Path == path {
			return route, true
		}
	}
	return routeInfo{}, false
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

func TestRequireAITaskAccessDeniesUnknownTaskType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set(rbac.ContextPermissionsKey, map[string]rbac.Level{
		"dashboard": rbac.LevelFull,
	})

	if requireAITaskAccess(context, "", "future_task") {
		t.Fatal("expected unknown task polling to be denied even with dashboard permission")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unknown task access, got %d", recorder.Code)
	}
}
