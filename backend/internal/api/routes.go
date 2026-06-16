package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/aicall"
	platformapproval "github.com/frankford824/ZBT/backend/internal/platform/approval"
	"github.com/frankford824/ZBT/backend/internal/platform/audit"
	"github.com/frankford824/ZBT/backend/internal/platform/auth"
	"github.com/frankford824/ZBT/backend/internal/platform/bid"
	platformcompliance "github.com/frankford824/ZBT/backend/internal/platform/compliance"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	platformcost "github.com/frankford824/ZBT/backend/internal/platform/cost"
	platformdashboard "github.com/frankford824/ZBT/backend/internal/platform/dashboard"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/frankford824/ZBT/backend/internal/platform/knowledge"
	platformproject "github.com/frankford824/ZBT/backend/internal/platform/project"
	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
	"github.com/frankford824/ZBT/backend/internal/platform/saas"
	"github.com/frankford824/ZBT/backend/internal/platform/tenant"
	platformtender "github.com/frankford824/ZBT/backend/internal/platform/tender"
	"github.com/gin-gonic/gin"
)

type routeSpec struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Module string `json:"module"`
	Async  bool   `json:"async"`
}

type routeInfo struct {
	Method        string     `json:"method"`
	Path          string     `json:"path"`
	Module        string     `json:"module"`
	Required      rbac.Level `json:"required"`
	Async         bool       `json:"async"`
	DynamicModule bool       `json:"dynamic_module,omitempty"`
}

type server struct {
	cfg             config.Config
	store           *saas.Store
	fileService     *platformfile.Service
	knowledgeStore  *knowledge.Store
	bidStore        *bid.Store
	tenderStore     *platformtender.Store
	projectStore    *platformproject.Store
	costStore       *platformcost.Store
	complianceStore *platformcompliance.Store
	approvalStore   *platformapproval.Store
	dashboardStore  *platformdashboard.Store
	aiCallStore     *aicall.Store
}

var routeSpecs = []routeSpec{
	{"GET", "/me", "dashboard", false},
	{"GET", "/meta/routes", "dashboard", false},
	{"GET", "/dashboard/summary", "dashboard", false},
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
	{"POST", "/projects/:id/archive-case", "project", false},
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
	{"GET", "/ai-call-logs", "team", false},
	{"POST", "/files/presign-upload", "knowledge", false},
	{"POST", "/files/:id/confirm", "knowledge", false},
	{"GET", "/files/:id/download-url", "knowledge", false},
	{"GET", "/files/:id/preview-url", "knowledge", false},
	{"GET", "/ai-tasks/:taskId", "dashboard", false},
}

var routeLevelOverrides = map[string]rbac.Level{
	"POST /tenders/:id/favorite":   rbac.LevelRead,
	"DELETE /tenders/:id/favorite": rbac.LevelRead,
	"POST /knowledge/search":       rbac.LevelRead,
	"POST /notifications/read":     rbac.LevelRead,
}

func NewRouter(cfg config.Config, store *saas.Store, fileService *platformfile.Service, knowledgeStore *knowledge.Store, bidStore *bid.Store, tenderStore *platformtender.Store, projectStore *platformproject.Store, costStore *platformcost.Store, complianceStore *platformcompliance.Store, approvalStore *platformapproval.Store, dashboardStore *platformdashboard.Store, aiCallStore *aicall.Store) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), audit.Middleware(), limitRequestBody(maxRequestBodyBytes()))
	s := &server{cfg: cfg, store: store, fileService: fileService, knowledgeStore: knowledgeStore, bidStore: bidStore, tenderStore: tenderStore, projectStore: projectStore, costStore: costStore, complianceStore: complianceStore, approvalStore: approvalStore, dashboardStore: dashboardStore, aiCallStore: aiCallStore}

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "zbt-backend",
			"status":  "ok",
			"time":    time.Now().UTC(),
		})
	})

	public := router.Group("/api/v1")
	public.POST("/auth/register", s.register)
	public.POST("/auth/login", s.login)
	public.POST("/auth/refresh", s.refresh)
	public.POST("/ai/callbacks/tasks", s.aiTaskCallback)

	api := router.Group("/api/v1", s.authenticate(), tenant.Middleware())
	api.POST("/auth/logout", s.logout)
	api.GET("/me", s.currentUser)
	api.GET("/meta/routes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"routes": routeInfos()})
	})
	api.GET("/dashboard/summary", rbac.Require("dashboard", rbac.LevelRead), s.dashboardSummary)
	s.registerSaaSRoutes(api)
	registerStubs(api)
	return router
}

const (
	defaultMaxRequestBodyBytes = int64(96 * 1024 * 1024)
	minMaxRequestBodyBytes     = int64(1024 * 1024)
	maxMaxRequestBodyBytes     = int64(256 * 1024 * 1024)
)

func maxRequestBodyBytes() int64 {
	raw := strings.TrimSpace(os.Getenv("API_MAX_BODY_BYTES"))
	if raw == "" {
		return defaultMaxRequestBodyBytes
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < minMaxRequestBodyBytes {
		return defaultMaxRequestBodyBytes
	}
	if value > maxMaxRequestBodyBytes {
		return maxMaxRequestBodyBytes
	}
	return value
}

func limitRequestBody(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = defaultMaxRequestBodyBytes
	}
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, apiError("payload_too_large", "请求内容过大"))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func (s *server) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
		TenantID string `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = s.cfg.DefaultTenantID
	}
	session, err := s.store.Login(c.Request.Context(), tenantID, req.Email, req.Password)
	if errors.Is(err, saas.ErrNotFound) {
		c.JSON(http.StatusUnauthorized, apiError("invalid_credentials", "账号或密码不正确"))
		return
	}
	if errors.Is(err, saas.ErrInvalidRequest) {
		respondBadRequest(c)
		return
	}
	if err != nil {
		respondInternal(c)
		return
	}
	respondSession(c, s.cfg, session)
}

func (s *server) register(c *gin.Context) {
	var req saas.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	session, err := s.store.Register(c.Request.Context(), req)
	if errors.Is(err, saas.ErrInvalidRequest) {
		respondBadRequest(c)
		return
	}
	if err != nil {
		respondInternal(c)
		return
	}
	respondSession(c, s.cfg, session)
}

func (s *server) refresh(c *gin.Context) {
	claims, err := auth.ParseJWT(s.cfg.JWTSecret, bearerToken(c.GetHeader("Authorization")))
	if err != nil {
		respondUnauthorized(c)
		return
	}
	session, err := s.store.SessionByUserRole(c.Request.Context(), claims.TenantID, claims.UserID, claims.RoleID)
	if errors.Is(err, saas.ErrNotFound) {
		respondUnauthorized(c)
		return
	}
	if err != nil {
		respondInternal(c)
		return
	}
	respondSession(c, s.cfg, session)
}

func (s *server) logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func respondSession(c *gin.Context, cfg config.Config, session saas.Session) {
	token, err := auth.SignJWT(cfg.JWTSecret, auth.Claims{
		UserID:    session.User.ID,
		TenantID:  session.Tenant.ID,
		RoleID:    session.Role.ID,
		RoleCode:  session.Role.Code,
		Roles:     []string{session.Role.Code},
		ExpiresAt: time.Now().Add(8 * time.Hour).Unix(),
	})
	if err != nil {
		respondInternal(c)
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiError("unauthorized", "登录状态已过期，请重新登录"))
			return
		}
		claims, err := auth.ParseJWT(s.cfg.JWTSecret, token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiError("unauthorized", "登录状态已过期，请重新登录"))
			return
		}
		session, err := s.store.SessionByUserRole(c.Request.Context(), claims.TenantID, claims.UserID, claims.RoleID)
		if errors.Is(err, saas.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiError("unauthorized", "登录状态已过期，请重新登录"))
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, apiError("internal_error", "服务暂时不可用，请稍后重试"))
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

func (s *server) dashboardSummary(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.dashboardStore.Summary(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string))
	respond(c, result, err)
}

func (s *server) registerSaaSRoutes(group *gin.RouterGroup) {
	group.GET("/tenders", rbac.Require("tender", rbac.LevelRead), s.listTenders)
	group.POST("/tenders", rbac.Require("tender", rbac.LevelFull), s.createTender)
	group.GET("/tenders/:id", rbac.Require("tender", rbac.LevelRead), s.getTender)
	group.PATCH("/tenders/:id", rbac.Require("tender", rbac.LevelFull), s.updateTender)
	group.POST("/tenders/:id/favorite", rbac.Require("tender", rbac.LevelRead), s.favoriteTender)
	group.DELETE("/tenders/:id/favorite", rbac.Require("tender", rbac.LevelRead), s.unfavoriteTender)
	group.POST("/tenders/:id/create-project", rbac.Require("tender", rbac.LevelFull), s.createProjectFromTender)
	group.POST("/tenders/:id/create-bid", rbac.Require("tender", rbac.LevelFull), s.createBidFromTender)
	group.GET("/tender-sources", rbac.Require("tender", rbac.LevelRead), s.listTenderSources)
	group.POST("/tender-sources", rbac.Require("tender", rbac.LevelFull), s.createTenderSource)
	group.PATCH("/tender-sources/:id", rbac.Require("tender", rbac.LevelFull), s.updateTenderSource)
	group.DELETE("/tender-sources/:id", rbac.Require("tender", rbac.LevelFull), s.deleteTenderSource)
	group.POST("/tender-sources/:id/verify", rbac.Require("tender", rbac.LevelFull), s.verifyTenderSource)
	group.GET("/projects", rbac.Require("project", rbac.LevelRead), s.listProjects)
	group.POST("/projects", rbac.Require("project", rbac.LevelFull), s.createProject)
	group.GET("/projects/:id", rbac.Require("project", rbac.LevelRead), s.getProject)
	group.PATCH("/projects/:id", rbac.Require("project", rbac.LevelFull), s.updateProject)
	group.DELETE("/projects/:id", rbac.Require("project", rbac.LevelFull), s.deleteProject)
	group.POST("/projects/:id/transition", rbac.Require("project", rbac.LevelFull), s.transitionProject)
	group.GET("/projects/:id/milestones", rbac.Require("project", rbac.LevelRead), s.listProjectMilestones)
	group.POST("/projects/:id/milestones", rbac.Require("project", rbac.LevelFull), s.createProjectMilestone)
	group.PATCH("/projects/:id/milestones/:milestoneId", rbac.Require("project", rbac.LevelFull), s.updateProjectMilestone)
	group.DELETE("/projects/:id/milestones/:milestoneId", rbac.Require("project", rbac.LevelFull), s.deleteProjectMilestone)
	group.POST("/projects/:id/members", rbac.Require("project", rbac.LevelFull), s.addProjectMember)
	group.DELETE("/projects/:id/members/:memberId", rbac.Require("project", rbac.LevelFull), s.deleteProjectMember)
	group.POST("/projects/:id/create-cost-project", rbac.Require("project", rbac.LevelFull), s.createCostProjectFromProject)
	group.POST("/projects/:id/archive-case", rbac.Require("project", rbac.LevelFull), rbac.Require("knowledge", rbac.LevelFull), s.archiveProjectCase)
	group.GET("/projects/:id/activities", rbac.Require("project", rbac.LevelRead), s.projectActivities)
	group.GET("/cost-projects", rbac.Require("cost", rbac.LevelRead), s.listCostProjects)
	group.POST("/cost-projects", rbac.Require("cost", rbac.LevelFull), s.createCostProject)
	group.GET("/cost-projects/:id", rbac.Require("cost", rbac.LevelRead), s.getCostProject)
	group.PATCH("/cost-projects/:id", rbac.Require("cost", rbac.LevelFull), s.updateCostProject)
	group.GET("/cost-projects/:id/items", rbac.Require("cost", rbac.LevelRead), s.listCostItems)
	group.POST("/cost-projects/:id/items", rbac.Require("cost", rbac.LevelFull), s.createCostItem)
	group.PATCH("/cost-items/:id", rbac.Require("cost", rbac.LevelFull), s.updateCostItem)
	group.DELETE("/cost-items/:id", rbac.Require("cost", rbac.LevelFull), s.deleteCostItem)
	group.GET("/cost-projects/:id/analysis", rbac.Require("cost", rbac.LevelRead), s.costAnalysis)
	group.POST("/cost-projects/:id/ai-advice", rbac.Require("cost", rbac.LevelFull), s.costAdvice)
	group.POST("/cost-projects/:id/report", rbac.Require("cost", rbac.LevelFull), s.createCostReport)
	group.POST("/compliance/checks", rbac.Require("compliance", rbac.LevelFull), s.createComplianceCheck)
	group.GET("/compliance/checks", rbac.Require("compliance", rbac.LevelRead), s.listComplianceChecks)
	group.GET("/compliance/checks/:id", rbac.Require("compliance", rbac.LevelRead), s.getComplianceCheck)
	group.GET("/compliance/checks/:id/issues", rbac.Require("compliance", rbac.LevelRead), s.listComplianceIssues)
	group.GET("/compliance/checks/:id/stream", rbac.Require("compliance", rbac.LevelRead), s.streamComplianceCheck)
	group.POST("/compliance/issues/:id/autofix", rbac.Require("compliance", rbac.LevelFull), s.autofixComplianceIssue)
	group.POST("/compliance/issues/:id/ignore", rbac.Require("compliance", rbac.LevelFull), s.ignoreComplianceIssue)
	group.POST("/compliance/issues/:id/confirm-fail", rbac.Require("compliance", rbac.LevelFull), s.confirmFailComplianceIssue)
	group.POST("/compliance/checks/:id/report", rbac.Require("compliance", rbac.LevelFull), s.createComplianceReport)
	group.GET("/compliance/rules", rbac.Require("compliance", rbac.LevelRead), s.listComplianceRules)
	group.POST("/compliance/rules", rbac.Require("compliance", rbac.LevelFull), s.createComplianceRule)
	group.PATCH("/compliance/rules/:id", rbac.Require("compliance", rbac.LevelFull), s.updateComplianceRule)
	group.DELETE("/compliance/rules/:id", rbac.Require("compliance", rbac.LevelFull), s.deleteComplianceRule)
	group.GET("/tenant", rbac.Require("team", rbac.LevelRead), s.getTenant)
	group.PATCH("/tenant", rbac.Require("team", rbac.LevelFull), s.updateTenant)
	group.GET("/tenant/members", rbac.Require("team", rbac.LevelRead), s.listMembers)
	group.POST("/tenant/members/invite", rbac.Require("team", rbac.LevelFull), s.inviteMember)
	group.PATCH("/tenant/members/:id", rbac.Require("team", rbac.LevelFull), s.updateMember)
	group.DELETE("/tenant/members/:id", rbac.Require("team", rbac.LevelFull), s.deleteMember)
	group.GET("/roles", rbac.Require("team", rbac.LevelRead), s.listRoles)
	group.POST("/roles", rbac.Require("team", rbac.LevelFull), s.createRole)
	group.PATCH("/roles/:id", rbac.Require("team", rbac.LevelFull), s.updateRole)
	group.DELETE("/roles/:id", rbac.Require("team", rbac.LevelFull), s.deleteRole)
	group.GET("/notifications", rbac.Require("team", rbac.LevelRead), s.listNotifications)
	group.GET("/knowledge", rbac.Require("knowledge", rbac.LevelRead), s.knowledgeHome)
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
	group.GET("/knowledge/templates", rbac.Require("knowledge", rbac.LevelRead), s.listKnowledgeTemplates)
	group.POST("/knowledge/templates", rbac.Require("knowledge", rbac.LevelFull), s.createKnowledgeTemplate)
	group.GET("/knowledge/stats", rbac.Require("knowledge", rbac.LevelRead), s.knowledgeStats)
	group.GET("/bid-templates", rbac.Require("bid", rbac.LevelRead), s.listBidTemplates)
	group.POST("/bid-templates/:templateId/use", rbac.Require("bid", rbac.LevelFull), s.useBidTemplate)
	group.GET("/bids", rbac.Require("bid", rbac.LevelRead), s.listBids)
	group.POST("/bids", rbac.Require("bid", rbac.LevelFull), s.createBid)
	group.GET("/bids/:id", rbac.Require("bid", rbac.LevelRead), s.getBid)
	group.PATCH("/bids/:id", rbac.Require("bid", rbac.LevelFull), s.updateBid)
	group.DELETE("/bids/:id", rbac.Require("bid", rbac.LevelFull), s.deleteBid)
	group.POST("/bids/:id/upload-tender-file", rbac.Require("bid", rbac.LevelFull), s.uploadBidTenderFile)
	group.POST("/bids/:id/parse-tender", rbac.Require("bid", rbac.LevelFull), s.parseBidTender)
	group.GET("/bids/:id/parse-result", rbac.Require("bid", rbac.LevelRead), s.getBidParseResult)
	group.PUT("/bids/:id/parse-result", rbac.Require("bid", rbac.LevelFull), s.confirmBidParseResult)
	group.POST("/bids/:id/outline/generate", rbac.Require("bid", rbac.LevelFull), s.generateBidOutline)
	group.GET("/bids/:id/parts", rbac.Require("bid", rbac.LevelRead), s.listBidParts)
	group.GET("/bids/:id/parts/:partId/outline", rbac.Require("bid", rbac.LevelRead), s.getBidPartOutline)
	group.PUT("/bids/:id/parts/:partId/outline", rbac.Require("bid", rbac.LevelFull), s.updateBidPartOutline)
	group.GET("/bids/:id/material-selection", rbac.Require("bid", rbac.LevelRead), s.getBidMaterialSelection)
	group.PUT("/bids/:id/material-selection", rbac.Require("bid", rbac.LevelFull), s.updateBidMaterialSelection)
	group.POST("/bids/:id/generate", rbac.Require("bid", rbac.LevelFull), s.generateBid)
	group.GET("/bids/:id/generation-jobs", rbac.Require("bid", rbac.LevelRead), s.listBidGenerationJobs)
	group.GET("/generation-jobs/:jobId", rbac.Require("bid", rbac.LevelRead), s.getBidGenerationJob)
	group.POST("/generation-jobs/:jobId/pause", rbac.Require("bid", rbac.LevelFull), s.pauseBidGenerationJob)
	group.POST("/generation-jobs/:jobId/resume", rbac.Require("bid", rbac.LevelFull), s.resumeBidGenerationJob)
	group.POST("/generation-jobs/:jobId/cancel", rbac.Require("bid", rbac.LevelFull), s.cancelBidGenerationJob)
	group.GET("/bids/:id/generation/stream", rbac.Require("bid", rbac.LevelRead), s.streamBidGeneration)
	group.GET("/bids/:id/chapters", rbac.Require("bid", rbac.LevelRead), s.listBidChapters)
	group.PATCH("/chapters/:chapterId", rbac.Require("bid", rbac.LevelFull), s.updateChapterContent)
	group.POST("/chapters/:chapterId/accept", rbac.Require("bid", rbac.LevelFull), s.acceptChapter)
	group.POST("/chapters/:chapterId/regenerate", rbac.Require("bid", rbac.LevelFull), s.regenerateChapter)
	group.GET("/chapters/:chapterId/versions", rbac.Require("bid", rbac.LevelRead), s.listChapterVersions)
	group.GET("/chapters/:chapterId/diff", rbac.Require("bid", rbac.LevelRead), s.chapterDiff)
	group.PUT("/chapters/:chapterId/content", rbac.Require("bid", rbac.LevelFull), s.updateChapterContent)
	group.POST("/chapters/:chapterId/ai-action", rbac.Require("bid", rbac.LevelFull), s.chapterAIAction)
	group.POST("/bids/:id/exports", rbac.Require("bid", rbac.LevelFull), s.createBidExport)
	group.GET("/bids/:id/exports", rbac.Require("bid", rbac.LevelRead), s.listBidExports)
	group.GET("/bid-exports/:exportId", rbac.Require("bid", rbac.LevelRead), s.getBidExport)
	group.POST("/bids/:id/submit-for-approval", rbac.Require("bid", rbac.LevelFull), s.submitBidForApproval)
	group.POST("/files/presign-upload", s.presignFileUpload)
	group.POST("/files/:id/confirm", s.confirmFileUpload)
	group.GET("/files/:id/download-url", s.fileDownloadURL)
	group.GET("/files/:id/preview-url", s.filePreviewURL)
	group.GET("/ai-tasks/:taskId", s.getAITask)
	group.GET("/approval-chains", rbac.Require("team", rbac.LevelRead), s.listApprovalChains)
	group.POST("/approval-chains", rbac.Require("team", rbac.LevelFull), s.createApprovalChain)
	group.PATCH("/approval-chains/:id", rbac.Require("team", rbac.LevelFull), s.updateApprovalChain)
	group.DELETE("/approval-chains/:id", rbac.Require("team", rbac.LevelFull), s.deleteApprovalChain)
	group.GET("/approvals", rbac.Require("team", rbac.LevelRead), s.listApprovals)
	group.GET("/approvals/:id", rbac.Require("team", rbac.LevelRead), s.getApproval)
	group.POST("/approvals/:id/approve", rbac.Require("team", rbac.LevelFull), s.approveApproval)
	group.POST("/approvals/:id/reject", rbac.Require("team", rbac.LevelFull), s.rejectApproval)
	group.POST("/notifications/read", rbac.Require("team", rbac.LevelRead), s.markNotificationsRead)
	group.GET("/notifications/stream", rbac.Require("team", rbac.LevelRead), s.streamNotifications)
	group.GET("/ai-call-logs", rbac.Require("team", rbac.LevelRead), s.listAICallLogs)
}

func registerStubs(group *gin.RouterGroup) {
	custom := map[string]bool{
		"GET /me":                                      true,
		"GET /meta/routes":                             true,
		"GET /dashboard/summary":                       true,
		"GET /tenant":                                  true,
		"PATCH /tenant":                                true,
		"GET /tenant/members":                          true,
		"POST /tenant/members/invite":                  true,
		"PATCH /tenant/members/:id":                    true,
		"DELETE /tenant/members/:id":                   true,
		"GET /roles":                                   true,
		"POST /roles":                                  true,
		"PATCH /roles/:id":                             true,
		"DELETE /roles/:id":                            true,
		"GET /notifications":                           true,
		"GET /tenders":                                 true,
		"POST /tenders":                                true,
		"GET /tenders/:id":                             true,
		"PATCH /tenders/:id":                           true,
		"POST /tenders/:id/favorite":                   true,
		"DELETE /tenders/:id/favorite":                 true,
		"POST /tenders/:id/create-project":             true,
		"POST /tenders/:id/create-bid":                 true,
		"GET /tender-sources":                          true,
		"POST /tender-sources":                         true,
		"PATCH /tender-sources/:id":                    true,
		"DELETE /tender-sources/:id":                   true,
		"POST /tender-sources/:id/verify":              true,
		"GET /projects":                                true,
		"POST /projects":                               true,
		"GET /projects/:id":                            true,
		"PATCH /projects/:id":                          true,
		"DELETE /projects/:id":                         true,
		"POST /projects/:id/transition":                true,
		"GET /projects/:id/milestones":                 true,
		"POST /projects/:id/milestones":                true,
		"PATCH /projects/:id/milestones/:milestoneId":  true,
		"DELETE /projects/:id/milestones/:milestoneId": true,
		"POST /projects/:id/members":                   true,
		"DELETE /projects/:id/members/:memberId":       true,
		"POST /projects/:id/create-cost-project":       true,
		"POST /projects/:id/archive-case":              true,
		"GET /projects/:id/activities":                 true,
		"GET /cost-projects":                           true,
		"POST /cost-projects":                          true,
		"GET /cost-projects/:id":                       true,
		"PATCH /cost-projects/:id":                     true,
		"GET /cost-projects/:id/items":                 true,
		"POST /cost-projects/:id/items":                true,
		"PATCH /cost-items/:id":                        true,
		"DELETE /cost-items/:id":                       true,
		"GET /cost-projects/:id/analysis":              true,
		"POST /cost-projects/:id/ai-advice":            true,
		"POST /cost-projects/:id/report":               true,
		"POST /compliance/checks":                      true,
		"GET /compliance/checks":                       true,
		"GET /compliance/checks/:id":                   true,
		"GET /compliance/checks/:id/issues":            true,
		"GET /compliance/checks/:id/stream":            true,
		"POST /compliance/issues/:id/autofix":          true,
		"POST /compliance/issues/:id/ignore":           true,
		"POST /compliance/issues/:id/confirm-fail":     true,
		"POST /compliance/checks/:id/report":           true,
		"GET /compliance/rules":                        true,
		"POST /compliance/rules":                       true,
		"PATCH /compliance/rules/:id":                  true,
		"DELETE /compliance/rules/:id":                 true,
		"GET /knowledge":                               true,
		"GET /knowledge/categories":                    true,
		"POST /knowledge/categories":                   true,
		"PATCH /knowledge/categories/:id":              true,
		"DELETE /knowledge/categories/:id":             true,
		"GET /knowledge/tags":                          true,
		"POST /knowledge/tags":                         true,
		"PATCH /knowledge/tags/:id":                    true,
		"DELETE /knowledge/tags/:id":                   true,
		"GET /knowledge/documents":                     true,
		"POST /knowledge/documents":                    true,
		"GET /knowledge/documents/:id":                 true,
		"PATCH /knowledge/documents/:id":               true,
		"DELETE /knowledge/documents/:id":              true,
		"POST /knowledge/documents/:id/process":        true,
		"GET /knowledge/documents/:id/preview":         true,
		"GET /knowledge/documents/:id/references":      true,
		"POST /knowledge/search":                       true,
		"GET /knowledge/templates":                     true,
		"POST /knowledge/templates":                    true,
		"GET /knowledge/stats":                         true,
		"GET /bid-templates":                           true,
		"POST /bid-templates/:templateId/use":          true,
		"GET /bids":                                    true,
		"POST /bids":                                   true,
		"GET /bids/:id":                                true,
		"PATCH /bids/:id":                              true,
		"DELETE /bids/:id":                             true,
		"POST /bids/:id/upload-tender-file":            true,
		"POST /bids/:id/parse-tender":                  true,
		"GET /bids/:id/parse-result":                   true,
		"PUT /bids/:id/parse-result":                   true,
		"POST /bids/:id/outline/generate":              true,
		"GET /bids/:id/parts":                          true,
		"GET /bids/:id/parts/:partId/outline":          true,
		"PUT /bids/:id/parts/:partId/outline":          true,
		"GET /bids/:id/material-selection":             true,
		"PUT /bids/:id/material-selection":             true,
		"POST /bids/:id/generate":                      true,
		"GET /bids/:id/generation-jobs":                true,
		"GET /generation-jobs/:jobId":                  true,
		"POST /generation-jobs/:jobId/pause":           true,
		"POST /generation-jobs/:jobId/resume":          true,
		"POST /generation-jobs/:jobId/cancel":          true,
		"GET /bids/:id/generation/stream":              true,
		"GET /bids/:id/chapters":                       true,
		"PATCH /chapters/:chapterId":                   true,
		"POST /chapters/:chapterId/accept":             true,
		"POST /chapters/:chapterId/regenerate":         true,
		"GET /chapters/:chapterId/versions":            true,
		"GET /chapters/:chapterId/diff":                true,
		"PUT /chapters/:chapterId/content":             true,
		"POST /chapters/:chapterId/ai-action":          true,
		"POST /bids/:id/exports":                       true,
		"GET /bids/:id/exports":                        true,
		"GET /bid-exports/:exportId":                   true,
		"POST /bids/:id/submit-for-approval":           true,
		"POST /files/presign-upload":                   true,
		"POST /files/:id/confirm":                      true,
		"GET /files/:id/download-url":                  true,
		"GET /files/:id/preview-url":                   true,
		"GET /ai-tasks/:taskId":                        true,
		"GET /approval-chains":                         true,
		"POST /approval-chains":                        true,
		"PATCH /approval-chains/:id":                   true,
		"DELETE /approval-chains/:id":                  true,
		"GET /approvals":                               true,
		"GET /approvals/:id":                           true,
		"POST /approvals/:id/approve":                  true,
		"POST /approvals/:id/reject":                   true,
		"POST /notifications/read":                     true,
		"GET /notifications/stream":                    true,
		"GET /ai-call-logs":                            true,
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

func (s *server) listTenders(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.tenderStore.List(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), platformtender.ListFilter{
		Search:       c.Query("q"),
		Region:       c.Query("region"),
		Status:       c.Query("status"),
		SourceID:     c.Query("source_id"),
		FavoriteOnly: c.Query("favorite") == "true",
		Recommended:  c.Query("recommended") == "true",
	})
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createTender(c *gin.Context) {
	var req platformtender.CreateTenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.tenderStore.Create(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) getTender(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.tenderStore.Get(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	respond(c, result, err)
}

func (s *server) updateTender(c *gin.Context) {
	var req platformtender.UpdateTenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.tenderStore.Update(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) favoriteTender(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.tenderStore.SetFavorite(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), true)
	respond(c, result, err)
}

func (s *server) unfavoriteTender(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.tenderStore.SetFavorite(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), false)
	respond(c, result, err)
}

func (s *server) createProjectFromTender(c *gin.Context) {
	result, err := s.tenderStore.CreateProject(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) createBidFromTender(c *gin.Context) {
	userID, _ := c.Get("user_id")
	found, err := s.tenderStore.Get(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	result, err := s.bidStore.CreateDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), bid.CreateDocumentRequest{
		Title:       found.Title + "投标文件",
		ProjectName: found.Title,
		BidType:     "combined",
	})
	respondStatus(c, http.StatusCreated, gin.H{"tender": found, "bid": result}, err)
}

func (s *server) listTenderSources(c *gin.Context) {
	result, err := s.tenderStore.ListSources(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createTenderSource(c *gin.Context) {
	var req platformtender.CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.tenderStore.CreateSource(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateTenderSource(c *gin.Context) {
	var req platformtender.UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.tenderStore.UpdateSource(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteTenderSource(c *gin.Context) {
	err := s.tenderStore.DeleteSource(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) verifyTenderSource(c *gin.Context) {
	result, err := s.tenderStore.VerifySource(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) listProjects(c *gin.Context) {
	result, err := s.projectStore.List(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Query("status"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createProject(c *gin.Context) {
	var req platformproject.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.projectStore.Create(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) getProject(c *gin.Context) {
	result, err := s.projectStore.Get(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) updateProject(c *gin.Context) {
	var req platformproject.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.projectStore.Update(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteProject(c *gin.Context) {
	err := s.projectStore.Delete(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) transitionProject(c *gin.Context) {
	var req platformproject.TransitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.projectStore.Transition(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) listProjectMilestones(c *gin.Context) {
	result, err := s.projectStore.ListMilestones(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createProjectMilestone(c *gin.Context) {
	var req platformproject.CreateMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.projectStore.CreateMilestone(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateProjectMilestone(c *gin.Context) {
	var req platformproject.UpdateMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.projectStore.UpdateMilestone(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), c.Param("milestoneId"), req)
	respond(c, result, err)
}

func (s *server) deleteProjectMilestone(c *gin.Context) {
	userID, _ := c.Get("user_id")
	err := s.projectStore.DeleteMilestone(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), c.Param("milestoneId"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) addProjectMember(c *gin.Context) {
	var req platformproject.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.projectStore.AddMember(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) deleteProjectMember(c *gin.Context) {
	userID, _ := c.Get("user_id")
	err := s.projectStore.DeleteMember(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), c.Param("memberId"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) createCostProjectFromProject(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.projectStore.CreateCostProject(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) archiveProjectCase(c *gin.Context) {
	tenantID := tenant.FromContext(c.Request.Context())
	userID, _ := c.Get("user_id")
	draft, err := s.projectStore.BuildWonCaseDraft(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	fileAsset, err := s.fileService.CreateGeneratedAsset(c.Request.Context(), tenantID, userID.(string), platformfile.GeneratedAssetRequest{
		Filename:    draft.Filename,
		ContentType: "text/markdown; charset=utf-8",
		Content:     []byte(draft.Content),
		BizType:     "knowledge_case",
		BizID:       c.Param("id"),
	})
	if err != nil {
		respond(c, nil, err)
		return
	}
	result, err := s.projectStore.ArchiveWonCase(c.Request.Context(), tenantID, userID.(string), c.Param("id"), fileAsset.ID, draft)
	respondStatus(c, http.StatusCreated, gin.H{"case": result, "file": fileAsset}, err)
}

func (s *server) projectActivities(c *gin.Context) {
	result, err := s.projectStore.ListActivities(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) listCostProjects(c *gin.Context) {
	result, err := s.costStore.ListProjects(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createCostProject(c *gin.Context) {
	var req platformcost.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.costStore.CreateProject(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) getCostProject(c *gin.Context) {
	result, err := s.costStore.GetProject(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) updateCostProject(c *gin.Context) {
	var req platformcost.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.costStore.UpdateProject(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) listCostItems(c *gin.Context) {
	result, err := s.costStore.ListItems(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createCostItem(c *gin.Context) {
	var req platformcost.CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.costStore.CreateItem(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateCostItem(c *gin.Context) {
	var req platformcost.UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.costStore.UpdateItem(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteCostItem(c *gin.Context) {
	err := s.costStore.DeleteItem(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) costAnalysis(c *gin.Context) {
	result, err := s.costStore.Analysis(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) costAdvice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.costStore.Advice(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) createCostReport(c *gin.Context) {
	result, err := s.costStore.CreateReport(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) createComplianceCheck(c *gin.Context) {
	var req platformcompliance.CreateCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.complianceStore.CreateCheck(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) listComplianceChecks(c *gin.Context) {
	result, err := s.complianceStore.ListChecks(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) getComplianceCheck(c *gin.Context) {
	result, err := s.complianceStore.GetCheck(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) listComplianceIssues(c *gin.Context) {
	result, err := s.complianceStore.ListIssues(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) streamComplianceCheck(c *gin.Context) {
	result, err := s.complianceStore.Snapshot(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		respondInternal(c)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	_ = writeSSE(c, flusher, "compliance", result)
}

func (s *server) autofixComplianceIssue(c *gin.Context) {
	result, err := s.complianceStore.AutofixIssue(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) ignoreComplianceIssue(c *gin.Context) {
	result, err := s.complianceStore.IgnoreIssue(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) confirmFailComplianceIssue(c *gin.Context) {
	result, err := s.complianceStore.ConfirmFailIssue(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) createComplianceReport(c *gin.Context) {
	result, err := s.complianceStore.CreateReport(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) listComplianceRules(c *gin.Context) {
	result, err := s.complianceStore.ListRules(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createComplianceRule(c *gin.Context) {
	var req platformcompliance.CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.complianceStore.CreateRule(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateComplianceRule(c *gin.Context) {
	var req platformcompliance.UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.complianceStore.UpdateRule(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteComplianceRule(c *gin.Context) {
	err := s.complianceStore.DeleteRule(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
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
		respondBadRequest(c)
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
		Email           string `json:"email" binding:"required"`
		Name            string `json:"name" binding:"required"`
		RoleCode        string `json:"role_code"`
		InitialPassword string `json:"initial_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.store.InviteMember(c.Request.Context(), tenant.FromContext(c.Request.Context()), req.Email, req.Name, req.RoleCode, req.InitialPassword)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateMember(c *gin.Context) {
	var req saas.UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.store.UpdateMember(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteMember(c *gin.Context) {
	err := s.store.DeleteMember(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
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
		respondBadRequest(c)
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
		respondBadRequest(c)
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

func (s *server) markNotificationsRead(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequest(c)
			return
		}
	}
	userID, _ := c.Get("user_id")
	updated, err := s.store.MarkNotificationsRead(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), req.IDs)
	respond(c, gin.H{"updated": updated}, err)
}

func (s *server) streamNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.store.ListNotifications(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string))
	if err != nil {
		respond(c, nil, err)
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		respondInternal(c)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	_ = writeSSE(c, flusher, "notifications", gin.H{"items": result, "updated_at": time.Now()})
}

func (s *server) knowledgeHome(c *gin.Context) {
	tenantID := tenant.FromContext(c.Request.Context())
	stats, err := s.knowledgeStore.Stats(c.Request.Context(), tenantID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	categories, err := s.knowledgeStore.ListCategories(c.Request.Context(), tenantID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	tags, err := s.knowledgeStore.ListTags(c.Request.Context(), tenantID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	documents, err := s.knowledgeStore.ListDocuments(c.Request.Context(), tenantID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	templates, err := s.knowledgeStore.ListDocumentTemplates(c.Request.Context(), tenantID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	if len(documents) > 8 {
		documents = documents[:8]
	}
	if len(templates) > 8 {
		templates = templates[:8]
	}
	respond(c, gin.H{
		"stats":            stats,
		"categories":       categories,
		"tags":             tags,
		"recent_documents": documents,
		"templates":        templates,
	}, nil)
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
		respondBadRequest(c)
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
		respondBadRequest(c)
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
		respondBadRequest(c)
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
		respondBadRequest(c)
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
		respondBadRequest(c)
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
		respondBadRequest(c)
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
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	results, err := s.knowledgeStore.Search(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), req)
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

func (s *server) listAICallLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	result, err := s.aiCallStore.List(c.Request.Context(), tenant.FromContext(c.Request.Context()), limit)
	respond(c, gin.H{"items": result}, err)
}

func (s *server) listKnowledgeTemplates(c *gin.Context) {
	result, err := s.knowledgeStore.ListDocumentTemplates(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createKnowledgeTemplate(c *gin.Context) {
	var req knowledge.CreateDocumentTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.knowledgeStore.CreateDocumentTemplate(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) knowledgeStats(c *gin.Context) {
	result, err := s.knowledgeStore.Stats(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, result, err)
}

func (s *server) listBidTemplates(c *gin.Context) {
	result, err := s.bidStore.ListTemplates(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) useBidTemplate(c *gin.Context) {
	var req bid.UseTemplateRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequest(c)
			return
		}
	}
	result, err := s.bidStore.UseTemplate(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("templateId"), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) listBids(c *gin.Context) {
	result, err := s.bidStore.ListDocuments(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createBid(c *gin.Context) {
	var req bid.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.bidStore.CreateDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) getBid(c *gin.Context) {
	result, err := s.bidStore.GetDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) updateBid(c *gin.Context) {
	var req bid.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.bidStore.UpdateDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteBid(c *gin.Context) {
	err := s.bidStore.DeleteDocument(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) uploadBidTenderFile(c *gin.Context) {
	var req bid.UploadTenderFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.UploadTenderFile(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) parseBidTender(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.ParseTender(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) getBidParseResult(c *gin.Context) {
	result, err := s.bidStore.GetParseResult(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) confirmBidParseResult(c *gin.Context) {
	var req bid.ConfirmParseResultRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequest(c)
			return
		}
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.ConfirmParseResult(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) generateBidOutline(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.GenerateOutline(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) listBidParts(c *gin.Context) {
	result, err := s.bidStore.ListParts(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) getBidPartOutline(c *gin.Context) {
	result, err := s.bidStore.GetPartOutline(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), c.Param("partId"))
	respond(c, result, err)
}

func (s *server) updateBidPartOutline(c *gin.Context) {
	var req bid.UpdatePartOutlineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.UpdatePartOutline(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), c.Param("partId"), req)
	respond(c, result, err)
}

func (s *server) getBidMaterialSelection(c *gin.Context) {
	result, err := s.bidStore.GetMaterialSelection(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) updateBidMaterialSelection(c *gin.Context) {
	var req bid.UpdateMaterialSelectionRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequest(c)
			return
		}
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.UpdateMaterialSelection(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) generateBid(c *gin.Context) {
	var req bid.GenerateBidRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequest(c)
			return
		}
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.GenerateBid(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respondStatus(c, http.StatusAccepted, result, err)
}

func (s *server) listBidGenerationJobs(c *gin.Context) {
	result, err := s.bidStore.ListGenerationJobs(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) getBidGenerationJob(c *gin.Context) {
	result, err := s.bidStore.GetGenerationJob(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("jobId"))
	respond(c, result, err)
}

func (s *server) pauseBidGenerationJob(c *gin.Context) {
	result, err := s.bidStore.PauseGenerationJob(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("jobId"))
	respond(c, result, err)
}

func (s *server) resumeBidGenerationJob(c *gin.Context) {
	result, err := s.bidStore.ResumeGenerationJob(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("jobId"))
	respond(c, result, err)
}

func (s *server) cancelBidGenerationJob(c *gin.Context) {
	result, err := s.bidStore.CancelGenerationJob(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("jobId"))
	respond(c, result, err)
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
		respondInternal(c)
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
				_ = writeSSE(c, flusher, "error", apiError("stream_unavailable", "实时更新暂时不可用，请稍后重试"))
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
		respondBadRequest(c)
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

func (s *server) chapterAIAction(c *gin.Context) {
	var req bid.ChapterAIActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.bidStore.ChapterAIAction(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("chapterId"), req)
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
		respondBadRequest(c)
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

func (s *server) submitBidForApproval(c *gin.Context) {
	userID, _ := c.Get("user_id")
	result, err := s.approvalStore.SubmitBid(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"))
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) listApprovalChains(c *gin.Context) {
	result, err := s.approvalStore.ListChains(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) createApprovalChain(c *gin.Context) {
	var req platformapproval.CreateChainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.approvalStore.CreateChain(c.Request.Context(), tenant.FromContext(c.Request.Context()), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) updateApprovalChain(c *gin.Context) {
	var req platformapproval.UpdateChainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	result, err := s.approvalStore.UpdateChain(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) deleteApprovalChain(c *gin.Context) {
	err := s.approvalStore.DeleteChain(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *server) listApprovals(c *gin.Context) {
	result, err := s.approvalStore.ListInstances(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Query("status"))
	respond(c, gin.H{"items": result}, err)
}

func (s *server) getApproval(c *gin.Context) {
	result, err := s.approvalStore.GetInstance(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	respond(c, result, err)
}

func (s *server) approveApproval(c *gin.Context) {
	var req platformapproval.DecisionRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequest(c)
			return
		}
	}
	userID, _ := c.Get("user_id")
	result, err := s.approvalStore.Approve(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) rejectApproval(c *gin.Context) {
	var req platformapproval.DecisionRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequest(c)
			return
		}
	}
	userID, _ := c.Get("user_id")
	result, err := s.approvalStore.Reject(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) presignFileUpload(c *gin.Context) {
	var req platformfile.PresignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c)
		return
	}
	if !requireFileAccess(c, req.BizType, rbac.LevelFull) {
		return
	}
	userID, _ := c.Get("user_id")
	result, err := s.fileService.PresignUpload(c.Request.Context(), tenant.FromContext(c.Request.Context()), userID.(string), req)
	respondStatus(c, http.StatusCreated, result, err)
}

func (s *server) confirmFileUpload(c *gin.Context) {
	asset, err := s.fileService.GetAsset(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	if !requireFileAccess(c, asset.BizType, rbac.LevelFull) {
		return
	}
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
	if !s.requireExistingFileAccess(c, rbac.LevelRead) {
		return
	}
	result, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), false)
	respond(c, result, err)
}

func (s *server) filePreviewURL(c *gin.Context) {
	if !s.requireExistingFileAccess(c, rbac.LevelRead) {
		return
	}
	result, err := s.fileService.DownloadURL(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"), true)
	respond(c, result, err)
}

func (s *server) requireExistingFileAccess(c *gin.Context, level rbac.Level) bool {
	asset, err := s.fileService.GetAsset(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("id"))
	if err != nil {
		respond(c, nil, err)
		return false
	}
	return requireFileAccess(c, asset.BizType, level)
}

func requireFileAccess(c *gin.Context, bizType string, level rbac.Level) bool {
	module, ok := platformfile.AccessModuleForBizType(bizType)
	if !ok {
		respondBadRequest(c)
		return false
	}
	if rbac.Allows(rbac.PermissionsFromContext(c)[module], level) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "permission_denied", "error": "当前账号没有此操作权限", "module": module})
	return false
}

func (s *server) getAITask(c *gin.Context) {
	result, err := s.knowledgeStore.GetTask(c.Request.Context(), tenant.FromContext(c.Request.Context()), c.Param("taskId"))
	if err != nil {
		respond(c, result, err)
		return
	}
	if !requireAITaskAccess(c, result.ResourceType, result.TaskType) {
		return
	}
	respond(c, result, err)
}

func requireAITaskAccess(c *gin.Context, resourceType, taskType string) bool {
	module := aiTaskAccessModule(resourceType, taskType)
	if rbac.Allows(rbac.PermissionsFromContext(c)[module], rbac.LevelRead) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "permission_denied", "error": "当前账号没有此操作权限", "module": module})
	return false
}

func aiTaskAccessModule(resourceType, taskType string) string {
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	taskType = strings.ToLower(strings.TrimSpace(taskType))
	switch {
	case strings.HasPrefix(resourceType, "knowledge"):
		return "knowledge"
	case strings.HasPrefix(resourceType, "bid"):
		return "bid"
	case strings.HasPrefix(resourceType, "cost"):
		return "cost"
	case strings.HasPrefix(resourceType, "compliance"):
		return "compliance"
	}
	switch taskType {
	case "knowledge_process", "knowledge_embedding", "knowledge_rerank":
		return "knowledge"
	case "tender_parse", "outline_generate", "chapter_generate", "chapter_ai_action", "document_export":
		return "bid"
	case "cost_advice":
		return "cost"
	case "compliance_check":
		return "compliance"
	default:
		return "dashboard"
	}
}

func (s *server) aiTaskCallback(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		respondBadRequest(c)
		return
	}
	if !s.verifyCallbackSignature(c.GetHeader("X-ZBT-Timestamp"), c.GetHeader("X-ZBT-Signature"), body) {
		c.JSON(http.StatusUnauthorized, apiError("invalid_signature", "回调签名校验失败"))
		return
	}
	var payload knowledge.CallbackPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		respondBadRequest(c)
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
		if err == nil {
			err = s.recordTaskCallback(c, payload.TenantID, payload.TaskID, result.Result, result.Status, result.ErrorMessage)
		}
		respond(c, result, err)
	case "bid_export", "bid_chapter", "bid_parse_result":
		result, err := s.bidStore.ApplyCallback(c.Request.Context(), bid.CallbackPayload{
			TenantID:     payload.TenantID,
			TaskID:       payload.TaskID,
			Status:       payload.Status,
			Result:       payload.Result,
			ErrorMessage: payload.ErrorMessage,
		})
		if err == nil {
			err = s.recordTaskCallback(c, payload.TenantID, payload.TaskID, result.Result, result.Status, result.ErrorMessage)
		}
		respond(c, result, err)
	case "cost_project":
		result, err := s.costStore.ApplyAdviceCallback(c.Request.Context(), platformcost.CallbackPayload{
			TenantID:     payload.TenantID,
			TaskID:       payload.TaskID,
			Status:       payload.Status,
			Result:       payload.Result,
			ErrorMessage: payload.ErrorMessage,
		})
		if err == nil {
			err = s.recordTaskCallback(c, payload.TenantID, payload.TaskID, result.Result, result.Status, result.ErrorMessage)
		}
		respond(c, result, err)
	default:
		c.JSON(http.StatusBadRequest, apiError("unsupported_callback_resource", "回调资源类型不支持"))
	}
}

func (s *server) recordTaskCallback(c *gin.Context, tenantID, taskID string, result map[string]any, status string, errorMessage *string) error {
	message := ""
	if errorMessage != nil {
		message = *errorMessage
	}
	_, err := s.aiCallStore.RecordTaskCallback(c.Request.Context(), tenantID, taskID, result, status, message)
	return err
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
	if level, ok := routeLevelOverrides[routeKey(spec)]; ok {
		return level
	}
	if spec.Method == http.MethodGet {
		return rbac.LevelRead
	}
	return rbac.LevelFull
}

func routeInfos() []routeInfo {
	routes := make([]routeInfo, 0, len(routeSpecs))
	for _, spec := range routeSpecs {
		routes = append(routes, routeInfo{
			Method:        spec.Method,
			Path:          spec.Path,
			Module:        spec.Module,
			Required:      requiredLevel(spec),
			Async:         spec.Async,
			DynamicModule: isDynamicModuleRoute(spec),
		})
	}
	return routes
}

func isDynamicModuleRoute(spec routeSpec) bool {
	return spec.Method == http.MethodGet && spec.Path == "/ai-tasks/:taskId"
}

func routeKey(spec routeSpec) string {
	return spec.Method + " " + spec.Path
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

func apiError(code, message string) gin.H {
	return gin.H{"code": code, "error": message}
}

func respondBadRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, apiError("bad_request", "请求内容不完整或格式不正确"))
}

func respondUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, apiError("unauthorized", "登录状态已过期，请重新登录"))
}

func respondInternal(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, apiError("internal_error", "服务暂时不可用，请稍后重试"))
}

func respondStatus(c *gin.Context, status int, payload any, err error) {
	if errors.Is(err, saas.ErrNotFound) || errors.Is(err, platformfile.ErrNotFound) || errors.Is(err, knowledge.ErrNotFound) || errors.Is(err, bid.ErrNotFound) || errors.Is(err, platformtender.ErrNotFound) || errors.Is(err, platformproject.ErrNotFound) || errors.Is(err, platformcost.ErrNotFound) || errors.Is(err, platformcompliance.ErrNotFound) || errors.Is(err, platformapproval.ErrNotFound) {
		c.JSON(http.StatusNotFound, apiError("not_found", "资源不存在"))
		return
	}
	if errors.Is(err, saas.ErrInvalidRequest) || errors.Is(err, platformfile.ErrInvalidRequest) || errors.Is(err, knowledge.ErrInvalidRequest) || errors.Is(err, bid.ErrInvalidRequest) || errors.Is(err, platformtender.ErrInvalidRequest) || errors.Is(err, platformproject.ErrInvalidRequest) || errors.Is(err, platformcost.ErrInvalidRequest) || errors.Is(err, platformcompliance.ErrInvalidRequest) || errors.Is(err, platformapproval.ErrInvalidRequest) || errors.Is(err, aicall.ErrInvalidRequest) {
		respondBadRequest(c)
		return
	}
	if errors.Is(err, platformfile.ErrObjectNotUploaded) || errors.Is(err, platformfile.ErrInvalidObjectState) {
		c.JSON(http.StatusConflict, apiError("file_not_ready", "文件尚未上传完成或状态不可用"))
		return
	}
	if err != nil {
		respondInternal(c)
		return
	}
	c.JSON(status, payload)
}
