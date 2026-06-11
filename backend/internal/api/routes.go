package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/audit"
	"github.com/frankford824/ZBT/backend/internal/platform/auth"
	"github.com/frankford824/ZBT/backend/internal/platform/bid"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/frankford824/ZBT/backend/internal/platform/knowledge"
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
	cfg            config.Config
	store          *saas.Store
	fileService    *platformfile.Service
	knowledgeStore *knowledge.Store
	bidStore       *bid.Store
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
	{"GET", "/bids/:id/exports", "bid", false},
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

func NewRouter(cfg config.Config, store *saas.Store, fileService *platformfile.Service, knowledgeStore *knowledge.Store, bidStore *bid.Store) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), audit.Middleware())
	s := &server{cfg: cfg, store: store, fileService: fileService, knowledgeStore: knowledgeStore, bidStore: bidStore}

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "zbt-backend",
			"status":  "ok",
			"time":    time.Now().UTC(),
		})
	})

	public := router.Group("/api/v1")
	public.POST("/auth/login", s.login)
	public.POST("/ai/callbacks/tasks", s.aiTaskCallback)

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
	group.GET("/knowledge/categories", rbac.Require("knowledge", rbac.LevelRead), s.listKnowledgeCategories)
	group.POST("/knowledge/categories", rbac.Require("knowledge", rbac.LevelFull), s.createKnowledgeCategory)
	group.PATCH("/knowledge/categories/:id", rbac.Require("knowledge", rbac.LevelFull), s.updateKnowledgeCategory)
	group.DELETE("/knowledge/categories/:id", rbac.Require("knowledge", rbac.LevelFull), s.deleteKnowledgeCategory)
	group.GET("/knowledge/tags", rbac.Require("knowledge", rbac.LevelRead), s.listKnowledgeTags)
	group.POST("/knowledge/tags", rbac.Require("knowledge", rbac.LevelFull), s.createKnowledgeTag)
	group.PATCH("/knowledge/tags/:id", rbac.Require("knowledge", rbac.LevelFull), s.updateKnowledgeTag)
	group.DELETE("/knowledge/tags/:id", rbac.Require("knowledge", rbac.LevelFull), s.deleteKnowledgeTag)
	group.GET("/knowledge/documents", rbac.Require("knowledge", rbac.LevelRead), s.listKnowledgeDocuments)
	group.POST("/knowledge/documents", rbac.Require("knowledge", rbac.LevelFull), s.createKnowledgeDocument)
	group.GET("/knowledge/documents/:id", rbac.Require("knowledge", rbac.LevelRead), s.getKnowledgeDocument)
	group.PATCH("/knowledge/documents/:id", rbac.Require("knowledge", rbac.LevelFull), s.updateKnowledgeDocument)
	group.DELETE("/knowledge/documents/:id", rbac.Require("knowledge", rbac.LevelFull), s.deleteKnowledgeDocument)
	group.POST("/knowledge/documents/:id/process", rbac.Require("knowledge", rbac.LevelFull), s.processKnowledgeDocument)
	group.GET("/knowledge/documents/:id/preview", rbac.Require("knowledge", rbac.LevelRead), s.previewKnowledgeDocument)
	group.GET("/knowledge/documents/:id/references", rbac.Require("knowledge", rbac.LevelRead), s.knowledgeDocumentReferences)
	group.POST("/knowledge/search", rbac.Require("knowledge", rbac.LevelRead), s.searchKnowledge)
	group.GET("/knowledge/stats", rbac.Require("knowledge", rbac.LevelRead), s.knowledgeStats)
	group.GET("/bids", rbac.Require("bid", rbac.LevelRead), s.listBids)
	group.POST("/bids", rbac.Require("bid", rbac.LevelFull), s.createBid)
	group.GET("/bids/:id", rbac.Require("bid", rbac.LevelRead), s.getBid)
	group.GET("/bids/:id/parts", rbac.Require("bid", rbac.LevelRead), s.listBidParts)
	group.GET("/bids/:id/generation/stream", rbac.Require("bid", rbac.LevelRead), s.streamBidGeneration)
	group.GET("/bids/:id/chapters", rbac.Require("bid", rbac.LevelRead), s.listBidChapters)
	group.PATCH("/chapters/:chapterId", rbac.Require("bid", rbac.LevelFull), s.updateChapterContent)
	group.POST("/chapters/:chapterId/accept", rbac.Require("bid", rbac.LevelFull), s.acceptChapter)
	group.POST("/chapters/:chapterId/regenerate", rbac.Require("bid", rbac.LevelFull), s.regenerateChapter)
	group.GET("/chapters/:chapterId/versions", rbac.Require("bid", rbac.LevelRead), s.listChapterVersions)
	group.GET("/chapters/:chapterId/diff", rbac.Require("bid", rbac.LevelRead), s.chapterDiff)
	group.PUT("/chapters/:chapterId/content", rbac.Require("bid", rbac.LevelFull), s.updateChapterContent)
	group.POST("/bids/:id/exports", rbac.Require("bid", rbac.LevelFull), s.createBidExport)
	group.GET("/bids/:id/exports", rbac.Require("bid", rbac.LevelRead), s.listBidExports)
	group.GET("/bid-exports/:exportId", rbac.Require("bid", rbac.LevelRead), s.getBidExport)
	group.POST("/files/presign-upload", rbac.Require("knowledge", rbac.LevelFull), s.presignFileUpload)
	group.POST("/files/:id/confirm", rbac.Require("knowledge", rbac.LevelFull), s.confirmFileUpload)
	group.GET("/files/:id/download-url", rbac.Require("knowledge", rbac.LevelRead), s.fileDownloadURL)
	group.GET("/files/:id/preview-url", rbac.Require("knowledge", rbac.LevelRead), s.filePreviewURL)
	group.GET("/ai-tasks/:taskId", rbac.Require("dashboard", rbac.LevelRead), s.getAITask)
}

func registerStubs(group *gin.RouterGroup) {
	custom := map[string]bool{
		"GET /me":                                 true,
		"GET /meta/routes":                        true,
		"GET /tenant":                             true,
		"PATCH /tenant":                           true,
		"GET /tenant/members":                     true,
		"POST /tenant/members/invite":             true,
		"GET /roles":                              true,
		"POST /roles":                             true,
		"PATCH /roles/:id":                        true,
		"DELETE /roles/:id":                       true,
		"GET /notifications":                      true,
		"GET /knowledge/categories":               true,
		"POST /knowledge/categories":              true,
		"PATCH /knowledge/categories/:id":         true,
		"DELETE /knowledge/categories/:id":        true,
		"GET /knowledge/tags":                     true,
		"POST /knowledge/tags":                    true,
		"PATCH /knowledge/tags/:id":               true,
		"DELETE /knowledge/tags/:id":              true,
		"GET /knowledge/documents":                true,
		"POST /knowledge/documents":               true,
		"GET /knowledge/documents/:id":            true,
		"PATCH /knowledge/documents/:id":          true,
		"DELETE /knowledge/documents/:id":         true,
		"POST /knowledge/documents/:id/process":   true,
		"GET /knowledge/documents/:id/preview":    true,
		"GET /knowledge/documents/:id/references": true,
		"POST /knowledge/search":                  true,
		"GET /knowledge/stats":                    true,
		"GET /bids":                               true,
		"POST /bids":                              true,
		"GET /bids/:id":                           true,
		"GET /bids/:id/parts":                     true,
		"GET /bids/:id/generation/stream":         true,
		"GET /bids/:id/chapters":                  true,
		"PATCH /chapters/:chapterId":              true,
		"POST /chapters/:chapterId/accept":        true,
		"POST /chapters/:chapterId/regenerate":    true,
		"GET /chapters/:chapterId/versions":       true,
		"GET /chapters/:chapterId/diff":           true,
		"PUT /chapters/:chapterId/content":        true,
		"POST /bids/:id/exports":                  true,
		"GET /bids/:id/exports":                   true,
		"GET /bid-exports/:exportId":              true,
		"POST /files/presign-upload":              true,
		"POST /files/:id/confirm":                 true,
		"GET /files/:id/download-url":             true,
		"GET /files/:id/preview-url":              true,
		"GET /ai-tasks/:taskId":                   true,
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

func (s *server) listKnowledgeCategories(c *gin.Context) {
	result, err := s.knowledgeStore.ListCategories(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createKnowledgeCategory(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.knowledgeStore.CreateCategory(c.Request.Context(), tenant.FromContext(c.Request.Context()), req.Name, req.Description)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateKnowledgeCategory(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.knowledgeStore.UpdateCategory(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req.Name, req.Description)
	respond(c, result, err)
}

func (s *server) deleteKnowledgeCategory(c *gin.Context) {
	err := s.knowledgeStore.DeleteCategory(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) listKnowledgeTags(c *gin.Context) {
	result, err := s.knowledgeStore.ListTags(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createKnowledgeTag(c *gin.Context) {
	var req struct {
		Name  string `json:"name" binding:"required"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.knowledgeStore.CreateTag(c.Request.Context(), tenant.FromContext(c.Request.Context()), req.Name, req.Color)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateKnowledgeTag(c *gin.Context) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.knowledgeStore.UpdateTag(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req.Name, req.Color)
	respond(c, result, err)
}

func (s *server) deleteKnowledgeTag(c *gin.Context) {
	err := s.knowledgeStore.DeleteTag(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) listKnowledgeDocuments(c *gin.Context) {
	result, err := s.knowledgeStore.ListDocuments(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createKnowledgeDocument(c *gin.Context) {
	var req struct {
		FileID string `json:"file_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.knowledgeStore.EnsureDocumentForFile(c.Request.Context(), tenant.FromContext(c.Request.Context()), req.FileID)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) getKnowledgeDocument(c *gin.Context) {
	result, err := s.knowledgeStore.GetDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) updateKnowledgeDocument(c *gin.Context) {
	var req knowledge.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.knowledgeStore.UpdateDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteKnowledgeDocument(c *gin.Context) {
	err := s.knowledgeStore.DeleteDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) processKnowledgeDocument(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.knowledgeStore.ProcessDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) previewKnowledgeDocument(c *gin.Context) {
	document, err := s.knowledgeStore.GetDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	result, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), document.File.ID, true)
	respond(c, result, err)
}

func (s *server) knowledgeDocumentReferences(c *gin.Context) {
	result, err := s.knowledgeStore.DocumentReferences(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) searchKnowledge(c *gin.Context) {
	var req knowledge.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	results, err := s.knowledgeStore.Search(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	if err != nil {
		respond(c, nil, err)
		return
	}
	sourceRefs := make([]knowledge.SourceRef, 0, len(results))
	for _, result := range results {
		sourceRefs = append(sourceRefs, result.SourceRef)
	}
	respond(c, gin.H{"items": results, "source_refs": sourceRefs}, nil)
}

func (s *server) knowledgeStats(c *gin.Context) {
	result, err := s.knowledgeStore.Stats(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, result, err)
}

func (s *server) listBids(c *gin.Context) {
	result, err := s.bidStore.ListDocuments(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createBid(c *gin.Context) {
	var req bid.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.bidStore.CreateDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) getBid(c *gin.Context) {
	result, err := s.bidStore.GetDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) listBidParts(c *gin.Context) {
	result, err := s.bidStore.ListParts(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) listBidChapters(c *gin.Context) {
	result, err := s.bidStore.ListChapters(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) streamBidGeneration(c *gin.Context) {
	tenantID := tenant.FromContext(c.Request.Context())
	bidID := c.Param("id")
	snapshot, err := s.bidStore.GenerationSnapshot(c.Request.Context(), tenantID, bidID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	lastFingerprint := generationSnapshotFingerprint(snapshot)
	if err := writeSSE(c, flusher, "generation", snapshot); err != nil {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			next, err := s.bidStore.GenerationSnapshot(c.Request.Context(), tenantID, bidID)
			if err != nil {
				_ = writeSSE(c, flusher, "error", gin.H{"error": err.Error()})
				return
			}
			fingerprint := generationSnapshotFingerprint(next)
			if fingerprint == lastFingerprint {
				continue
			}
			lastFingerprint = fingerprint
			if err := writeSSE(c, flusher, "generation", next); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *server) updateChapterContent(c *gin.Context) {
	var req bid.UpdateChapterContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.UpdateChapterContent(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("chapterId"), req)
	respond(c, result, err)
}

func (s *server) acceptChapter(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.AcceptChapter(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("chapterId"))
	respond(c, result, err)
}

func (s *server) regenerateChapter(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.RegenerateChapter(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("chapterId"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) listChapterVersions(c *gin.Context) {
	result, err := s.bidStore.ListChapterVersions(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("chapterId"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) chapterDiff(c *gin.Context) {
	result, err := s.bidStore.ChapterDiff(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("chapterId"))
	respond(c, result, err)
}

func (s *server) createBidExport(c *gin.Context) {
	var req bid.CreateExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.CreateExport(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) listBidExports(c *gin.Context) {
	result, err := s.bidStore.ListExports(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) getBidExport(c *gin.Context) {
	result, err := s.bidStore.GetExport(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("exportId"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	payload := gin.H{"export": result}
	if result.Status == "done" && result.FileAssetID != nil {
		download, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), *result.FileAssetID, false)
		if err != nil {
			respond(c, nil, err)
			return
		}
		payload["download"] = download
	}
	respond(c, payload, nil)
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
	if err != nil {
		respond(c, nil, err)
		return
	}
	if result.BizType == "knowledge" {
		document, err := s.knowledgeStore.EnsureDocumentForFile(c.Request.Context(), tenant.FromContext(c.Request.Context()), result.ID)
		respond(c, gin.H{"file": result, "document": document}, err)
		return
	}
	respond(c, result, nil)
}

func (s *server) fileDownloadURL(c *gin.Context) {
	result, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), false)
	respond(c, result, err)
}

func (s *server) filePreviewURL(c *gin.Context) {
	result, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), true)
	respond(c, result, err)
}

func (s *server) getAITask(c *gin.Context) {
	result, err := s.knowledgeStore.GetTask(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("taskId"))
	respond(c, result, err)
}

func (s *server) aiTaskCallback(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !s.verifyCallbackSignature(c.GetHeader("X-ZBT-Timestamp"), c.GetHeader("X-ZBT-Signature"), body) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid callback signature"})
		return
	}
	var payload knowledge.CallbackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task, err := s.knowledgeStore.GetTaskByExternalID(c.Request.Context(), payload.TenantID, payload.TaskID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	switch task.ResourceType {
	case "knowledge_document":
		result, err := s.knowledgeStore.ApplyCallback(c.Request.Context(), payload)
		respond(c, result, err)
	case "bid_export", "bid_chapter":
		result, err := s.bidStore.ApplyCallback(c.Request.Context(), bid.CallbackPayload{
			TenantID:     payload.TenantID,
			TaskID:       payload.TaskID,
			Status:       payload.Status,
			Result:       payload.Result,
			ErrorMessage: payload.ErrorMessage,
		})
		respond(c, result, err)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported callback resource type"})
	}
}

func (s *server) verifyCallbackSignature(timestampHeader, signatureHeader string, body []byte) bool {
	if s.cfg.AIServiceHMACSecret == "" || timestampHeader == "" || signatureHeader == "" {
		return false
	}
	timestamp, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return false
	}
	if time.Since(time.Unix(timestamp, 0)).Abs() > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.AIServiceHMACSecret))
	mac.Write([]byte(timestampHeader))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

func requiredLevel(spec routeSpec) rbac.Level {
	if spec.Method == http.MethodGet {
		return rbac.LevelRead
	}
	return rbac.LevelFull
}

func writeSSE(c *gin.Context, flusher http.Flusher, event string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, body); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func generationSnapshotFingerprint(snapshot bid.GenerationSnapshot) string {
	snapshot.GeneratedAt = time.Time{}
	body, _ := json.Marshal(snapshot)
	return string(body)
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
	if errors.Is(err, saas.ErrNotFound) || errors.Is(err, platformfile.ErrNotFound) || errors.Is(err, knowledge.ErrNotFound) || errors.Is(err, bid.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if errors.Is(err, platformfile.ErrInvalidRequest) || errors.Is(err, knowledge.ErrInvalidRequest) || errors.Is(err, bid.ErrInvalidRequest) {
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
