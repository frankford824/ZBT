package api

import (
	"net/http"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/audit"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
	"github.com/frankford824/ZBT/backend/internal/platform/tenant"
	"github.com/gin-gonic/gin"
)

type routeSpec struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Module string `json:"module"`
	Async  bool   `json:"async"`
}

var routeSpecs = []routeSpec{
	{"GET", "/me", "dashboard", false},
	{"GET", "/meta/routes", "dashboard", false},
	{"GET", "/tenant", "team", false},
	{"PATCH", "/tenant", "team", false},
	{"GET", "/tenant/members", "team", false},
	{"POST", "/tenant/members/invite", "team", false},
	{"PATCH", "/tenant/members/:id", "team", false},
	{"DELETE", "/tenant/members/:id", "team", false},
	{"GET", "/roles", "team", false},
	{"POST", "/roles", "team", false},
	{"PATCH", "/roles/:id", "team", false},
	{"DELETE", "/roles/:id", "team", false},
	{"GET", "/tenders", "tender", false},
	{"POST", "/tenders", "tender", false},
	{"GET", "/tenders/:id", "tender", false},
	{"PATCH", "/tenders/:id", "tender", false},
	{"POST", "/tenders/:id/favorite", "tender", false},
	{"DELETE", "/tenders/:id/favorite", "tender", false},
	{"POST", "/tenders/:id/create-project", "tender", false},
	{"POST", "/tenders/:id/create-bid", "tender", false},
	{"GET", "/tender-sources", "tender", false},
	{"POST", "/tender-sources", "tender", false},
	{"PATCH", "/tender-sources/:id", "tender", false},
	{"DELETE", "/tender-sources/:id", "tender", false},
	{"POST", "/tender-sources/:id/verify", "tender", true},
	{"GET", "/projects", "project", false},
	{"POST", "/projects", "project", false},
	{"GET", "/projects/:id", "project", false},
	{"PATCH", "/projects/:id", "project", false},
	{"DELETE", "/projects/:id", "project", false},
	{"POST", "/projects/:id/transition", "project", false},
	{"GET", "/projects/:id/milestones", "project", false},
	{"POST", "/projects/:id/milestones", "project", false},
	{"PATCH", "/projects/:id/milestones/:milestoneId", "project", false},
	{"DELETE", "/projects/:id/milestones/:milestoneId", "project", false},
	{"POST", "/projects/:id/members", "project", false},
	{"DELETE", "/projects/:id/members/:memberId", "project", false},
	{"POST", "/projects/:id/create-cost-project", "project", false},
	{"GET", "/projects/:id/activities", "project", false},
	{"GET", "/bids", "bid", false},
	{"POST", "/bids", "bid", false},
	{"GET", "/bids/:id", "bid", false},
	{"PATCH", "/bids/:id", "bid", false},
	{"DELETE", "/bids/:id", "bid", false},
	{"POST", "/bids/:id/upload-tender-file", "bid", true},
	{"POST", "/bids/:id/parse-tender", "bid", true},
	{"GET", "/bids/:id/parse-result", "bid", false},
	{"PUT", "/bids/:id/parse-result", "bid", false},
	{"POST", "/bids/:id/outline/generate", "bid", true},
	{"GET", "/bids/:id/parts", "bid", false},
	{"GET", "/bids/:id/parts/:partId/outline", "bid", false},
	{"PUT", "/bids/:id/parts/:partId/outline", "bid", false},
	{"GET", "/bids/:id/material-selection", "bid", false},
	{"PUT", "/bids/:id/material-selection", "bid", false},
	{"POST", "/bids/:id/generate", "bid", true},
	{"GET", "/bids/:id/generation-jobs", "bid", false},
	{"GET", "/generation-jobs/:jobId", "bid", false},
	{"POST", "/generation-jobs/:jobId/pause", "bid", false},
	{"POST", "/generation-jobs/:jobId/resume", "bid", false},
	{"POST", "/generation-jobs/:jobId/cancel", "bid", false},
	{"GET", "/bids/:id/generation/stream", "bid", false},
	{"GET", "/bids/:id/chapters", "bid", false},
	{"PATCH", "/chapters/:chapterId", "bid", false},
	{"POST", "/chapters/:chapterId/accept", "bid", false},
	{"POST", "/chapters/:chapterId/regenerate", "bid", true},
	{"GET", "/chapters/:chapterId/versions", "bid", false},
	{"GET", "/chapters/:chapterId/diff", "bid", false},
	{"PUT", "/chapters/:chapterId/content", "bid", false},
	{"POST", "/chapters/:chapterId/ai-action", "bid", true},
	{"POST", "/bids/:id/exports", "bid", true},
	{"GET", "/bid-exports/:exportId", "bid", false},
	{"GET", "/bid-templates", "bid", false},
	{"POST", "/bid-templates/:templateId/use", "bid", false},
	{"GET", "/knowledge", "knowledge", false},
	{"GET", "/knowledge/categories", "knowledge", false},
	{"POST", "/knowledge/categories", "knowledge", false},
	{"PATCH", "/knowledge/categories/:id", "knowledge", false},
	{"DELETE", "/knowledge/categories/:id", "knowledge", false},
	{"GET", "/knowledge/tags", "knowledge", false},
	{"POST", "/knowledge/tags", "knowledge", false},
	{"PATCH", "/knowledge/tags/:id", "knowledge", false},
	{"DELETE", "/knowledge/tags/:id", "knowledge", false},
	{"GET", "/knowledge/documents", "knowledge", false},
	{"POST", "/knowledge/documents", "knowledge", false},
	{"GET", "/knowledge/documents/:id", "knowledge", false},
	{"PATCH", "/knowledge/documents/:id", "knowledge", false},
	{"DELETE", "/knowledge/documents/:id", "knowledge", false},
	{"POST", "/knowledge/documents/:id/process", "knowledge", true},
	{"GET", "/knowledge/documents/:id/preview", "knowledge", false},
	{"GET", "/knowledge/documents/:id/references", "knowledge", false},
	{"POST", "/knowledge/search", "knowledge", true},
	{"GET", "/knowledge/templates", "knowledge", false},
	{"POST", "/knowledge/templates", "knowledge", false},
	{"GET", "/knowledge/stats", "knowledge", false},
	{"POST", "/compliance/checks", "compliance", true},
	{"GET", "/compliance/checks", "compliance", false},
	{"GET", "/compliance/checks/:id", "compliance", false},
	{"GET", "/compliance/checks/:id/issues", "compliance", false},
	{"GET", "/compliance/checks/:id/stream", "compliance", false},
	{"POST", "/compliance/issues/:id/autofix", "compliance", true},
	{"POST", "/compliance/issues/:id/ignore", "compliance", false},
	{"POST", "/compliance/issues/:id/confirm-fail", "compliance", false},
	{"POST", "/compliance/checks/:id/report", "compliance", true},
	{"GET", "/compliance/rules", "compliance", false},
	{"POST", "/compliance/rules", "compliance", false},
	{"PATCH", "/compliance/rules/:id", "compliance", false},
	{"DELETE", "/compliance/rules/:id", "compliance", false},
	{"GET", "/cost-projects", "cost", false},
	{"POST", "/cost-projects", "cost", false},
	{"GET", "/cost-projects/:id", "cost", false},
	{"PATCH", "/cost-projects/:id", "cost", false},
	{"GET", "/cost-projects/:id/items", "cost", false},
	{"POST", "/cost-projects/:id/items", "cost", false},
	{"PATCH", "/cost-items/:id", "cost", false},
	{"DELETE", "/cost-items/:id", "cost", false},
	{"GET", "/cost-projects/:id/analysis", "cost", false},
	{"POST", "/cost-projects/:id/ai-advice", "cost", true},
	{"POST", "/cost-projects/:id/report", "cost", true},
	{"GET", "/approval-chains", "team", false},
	{"POST", "/approval-chains", "team", false},
	{"PATCH", "/approval-chains/:id", "team", false},
	{"DELETE", "/approval-chains/:id", "team", false},
	{"POST", "/bids/:id/submit-for-approval", "bid", false},
	{"GET", "/approvals", "team", false},
	{"GET", "/approvals/:id", "team", false},
	{"POST", "/approvals/:id/approve", "team", false},
	{"POST", "/approvals/:id/reject", "team", false},
	{"GET", "/notifications", "team", false},
	{"POST", "/notifications/read", "team", false},
	{"GET", "/notifications/stream", "team", false},
	{"POST", "/files/presign-upload", "knowledge", false},
	{"POST", "/files/:id/confirm", "knowledge", false},
	{"GET", "/files/:id/download-url", "knowledge", false},
	{"GET", "/files/:id/preview-url", "knowledge", false},
	{"GET", "/ai-tasks/:taskId", "dashboard", false},
}

func NewRouter(cfg config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), audit.Middleware())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "zbt-backend",
			"status":  "ok",
			"time":    time.Now().UTC(),
		})
	})

	api := router.Group("/api/v1", tenant.Middleware())
	api.GET("/me", currentUser)
	api.GET("/meta/routes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"routes": routeSpecs, "ai_service_url": cfg.AIServiceURL})
	})
	registerStubs(api)
	return router
}

func currentUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"user":        gin.H{"id": "user-demo", "name": "陈思远", "role": "企业管理员"},
		"tenant":      gin.H{"id": tenant.FromContext(c.Request.Context()), "name": "杭州智建科技有限公司"},
		"permissions": rbac.DemoPermissions(),
	})
}

func registerStubs(group *gin.RouterGroup) {
	for _, spec := range routeSpecs {
		if spec.Path == "/me" || spec.Path == "/meta/routes" {
			continue
		}
		spec := spec
		handlers := []gin.HandlerFunc{rbac.Require(spec.Module, rbac.LevelRead), stub(spec)}
		group.Handle(spec.Method, spec.Path, handlers...)
	}
}

func stub(spec routeSpec) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload := gin.H{
			"module":    spec.Module,
			"method":    spec.Method,
			"path":      spec.Path,
			"tenant_id": tenant.FromContext(c.Request.Context()),
			"params":    c.Params,
		}
		if spec.Async {
			payload["task_id"] = "task-demo"
			payload["status"] = "queued"
			c.JSON(http.StatusAccepted, payload)
			return
		}
		payload["status"] = "ok"
		c.JSON(http.StatusOK, payload)
	}
}
