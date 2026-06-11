package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/audit"
	"github.com/frankford824/ZBT/backend/internal/platform/auth"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
	"github.com/frankford824/ZBT/backend/internal/platform/saas"
	"github.com/frankford824/ZBT/backend/internal/platform/tenant"
	"github.com/gin-gonic/gin"
)

type routeSpec struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Module string `json:"module"`
	Async  bool   `json:"async"`
}

type server struct {
	cfg         config.Config
	store       *saas.Store
	fileService *platformfile.Service
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

func NewRouter(cfg config.Config, store *saas.Store, fileService *platformfile.Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), audit.Middleware())
	s := &server{cfg: cfg, store: store, fileService: fileService}

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "zbt-backend",
			"status":  "ok",
			"time":    time.Now().UTC(),
		})
	})

	public := router.Group("/api/v1")
	public.POST("/auth/login", s.login)

	api := router.Group("/api/v1", s.authenticate(), tenant.Middleware())
	api.GET("/me", s.currentUser)
	api.GET("/meta/routes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"routes": routeSpecs, "ai_service_url": cfg.AIServiceURL})
	})
	s.registerSaaSRoutes(api)
	registerStubs(api)
	return router
}

func (s *server) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		TenantID string `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = s.cfg.DefaultTenantID
	}
	session, err := s.store.Login(c.Request.Context(), tenantID, req.Email, req.Password)
	if errors.Is(err, saas.ErrNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := auth.SignJWT(s.cfg.JWTSecret, auth.Claims{
		UserID:    session.User.ID,
		TenantID:  session.Tenant.ID,
		RoleID:    session.Role.ID,
		RoleCode:  session.Role.Code,
		Roles:     []string{session.Role.Code},
		ExpiresAt: time.Now().Add(8 * time.Hour).Unix(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   28800,
		"session":      session,
	})
}

func (s *server) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := auth.ParseJWT(s.cfg.JWTSecret, token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		session, err := s.store.SessionByUserRole(c.Request.Context(), claims.TenantID, claims.UserID, claims.RoleID)
		if errors.Is(err, saas.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session not found"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Set("session", session)
		c.Set("tenant_id", session.Tenant.ID)
		c.Set("user_id", session.User.ID)
		c.Set("role_code", session.Role.Code)
		c.Set(rbac.ContextPermissionsKey, session.Permissions)
		c.Next()
	}
}

func (s *server) currentUser(c *gin.Context) {
	session, _ := c.Get("session")
	c.JSON(http.StatusOK, session)
}

func (s *server) registerSaaSRoutes(group *gin.RouterGroup) {
	group.GET("/tenant", rbac.Require("team", rbac.LevelRead), s.getTenant)
	group.PATCH("/tenant", rbac.Require("team", rbac.LevelFull), s.updateTenant)
	group.GET("/tenant/members", rbac.Require("team", rbac.LevelRead), s.listMembers)
	group.POST("/tenant/members/invite", rbac.Require("team", rbac.LevelFull), s.inviteMember)
	group.GET("/roles", rbac.Require("team", rbac.LevelRead), s.listRoles)
	group.POST("/roles", rbac.Require("team", rbac.LevelFull), s.createRole)
	group.PATCH("/roles/:id", rbac.Require("team", rbac.LevelFull), s.updateRole)
	group.DELETE("/roles/:id", rbac.Require("team", rbac.LevelFull), s.deleteRole)
	group.GET("/notifications", s.listNotifications)
	group.GET("/knowledge/documents", rbac.Require("knowledge", rbac.LevelRead), s.listKnowledgeDocuments)
	group.POST("/files/presign-upload", rbac.Require("knowledge", rbac.LevelFull), s.presignFileUpload)
	group.POST("/files/:id/confirm", rbac.Require("knowledge", rbac.LevelFull), s.confirmFileUpload)
	group.GET("/files/:id/download-url", rbac.Require("knowledge", rbac.LevelRead), s.fileDownloadURL)
	group.GET("/files/:id/preview-url", rbac.Require("knowledge", rbac.LevelRead), s.filePreviewURL)
}

func registerStubs(group *gin.RouterGroup) {
	custom := map[string]bool{
		"GET /me":                     true,
		"GET /meta/routes":            true,
		"GET /tenant":                 true,
		"PATCH /tenant":               true,
		"GET /tenant/members":         true,
		"POST /tenant/members/invite": true,
		"GET /roles":                  true,
		"POST /roles":                 true,
		"PATCH /roles/:id":            true,
		"DELETE /roles/:id":           true,
		"GET /notifications":          true,
		"GET /knowledge/documents":    true,
		"POST /files/presign-upload":  true,
		"POST /files/:id/confirm":     true,
		"GET /files/:id/download-url": true,
		"GET /files/:id/preview-url":  true,
	}
	for _, spec := range routeSpecs {
		if custom[spec.Method+" "+spec.Path] {
			continue
		}
		spec := spec
		handlers := []gin.HandlerFunc{rbac.Require(spec.Module, requiredLevel(spec)), stub(spec)}
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

func (s *server) getTenant(c *gin.Context) {
	result, err := s.store.GetTenant(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, result, err)
}

func (s *server) updateTenant(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.store.UpdateTenant(c.Request.Context(), tenant.FromContext(c.Request.Context()), req.Name)
	respond(c, result, err)
}

func (s *server) listMembers(c *gin.Context) {
	result, err := s.store.ListMembers(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) inviteMember(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Name     string `json:"name" binding:"required"`
		RoleCode string `json:"role_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.store.InviteMember(c.Request.Context(), tenant.FromContext(c.Request.Context()), req.Email, req.Name, req.RoleCode)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) listRoles(c *gin.Context) {
	result, err := s.store.ListRoles(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createRole(c *gin.Context) {
	var req struct {
		Code        string                `json:"code" binding:"required"`
		Name        string                `json:"name" binding:"required"`
		Permissions map[string]rbac.Level `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.store.CreateRole(c.Request.Context(), tenant.FromContext(c.Request.Context()), req.Code, req.Name, req.Permissions)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateRole(c *gin.Context) {
	var req struct {
		Name        string                `json:"name"`
		Permissions map[string]rbac.Level `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.store.UpdateRole(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req.Name, req.Permissions)
	respond(c, result, err)
}

func (s *server) deleteRole(c *gin.Context) {
	err := s.store.DeleteRole(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) listNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.store.ListNotifications(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) listKnowledgeDocuments(c *gin.Context) {
	result, err := s.fileService.ListAssets(c.Request.Context(), tenant.FromContext(c.Request.Context()), "knowledge")
	respond(c, gin.H{"items": result}, err)
}

func (s *server) presignFileUpload(c *gin.Context) {
	var req platformfile.PresignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.fileService.PresignUpload(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) confirmFileUpload(c *gin.Context) {
	result, err := s.fileService.ConfirmUpload(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) fileDownloadURL(c *gin.Context) {
	result, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), false)
	respond(c, result, err)
}

func (s *server) filePreviewURL(c *gin.Context) {
	result, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), true)
	respond(c, result, err)
}

func requiredLevel(spec routeSpec) rbac.Level {
	if spec.Method == http.MethodGet {
		return rbac.LevelRead
	}
	return rbac.LevelFull
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func respond(c *gin.Context, payload any, err error) {
	respondStatus(c, http.StatusOK, payload, err)
}

func respondStatus(c *gin.Context, status int, payload any, err error) {
	if errors.Is(err, saas.ErrNotFound) || errors.Is(err, platformfile.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if errors.Is(err, platformfile.ErrInvalidRequest) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, platformfile.ErrObjectNotUploaded) || errors.Is(err, platformfile.ErrInvalidObjectState) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(status, payload)
}
