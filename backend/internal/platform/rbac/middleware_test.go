package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireDeniesWhenPermissionsContextMissing(t *testing.T) {
	router := testRouter()
	router.GET("/cost-projects", Require("cost", LevelRead), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	status := performRequest(router, http.MethodGet, "/cost-projects")
	if status != http.StatusForbidden {
		t.Fatalf("expected missing permissions context to return 403, got %d", status)
	}
}

func TestRequireAllowsReadPermissionOnlyForReads(t *testing.T) {
	router := testRouter()
	router.Use(func(c *gin.Context) {
		c.Set(ContextPermissionsKey, map[string]Level{"cost": LevelRead})
		c.Next()
	})
	router.GET("/cost-projects", Require("cost", LevelRead), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.POST("/cost-projects", Require("cost", LevelFull), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	if status := performRequest(router, http.MethodGet, "/cost-projects"); status != http.StatusNoContent {
		t.Fatalf("expected read permission to allow GET, got %d", status)
	}
	if status := performRequest(router, http.MethodPost, "/cost-projects"); status != http.StatusForbidden {
		t.Fatalf("expected read permission to deny POST, got %d", status)
	}
}

func TestRequireAllowsFullPermissionForWrites(t *testing.T) {
	router := testRouter()
	router.Use(func(c *gin.Context) {
		c.Set(ContextPermissionsKey, map[string]Level{"cost": LevelFull})
		c.Next()
	})
	router.POST("/cost-projects", Require("cost", LevelFull), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	if status := performRequest(router, http.MethodPost, "/cost-projects"); status != http.StatusNoContent {
		t.Fatalf("expected full permission to allow POST, got %d", status)
	}
}

func testRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func performRequest(router *gin.Engine, method, path string) int {
	req := httptest.NewRequest(method, path, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp.Code
}
