package api

import (
	"net/http"
	"strconv"

	"github.com/frankford824/ZBT/backend/internal/company/qualification"
	"github.com/frankford824/ZBT/backend/internal/platform/tenant"
	"github.com/gin-gonic/gin"
)

// qualificationFilter 解析资质档案列表的公共查询参数。
func qualificationFilter(c *gin.Context) (qualification.ListFilter, bool) {
	limit, ok := boundedQueryLimit(c, 50, 200)
	if !ok {
		return qualification.ListFilter{}, false
	}
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, apiError("invalid_offset", "offset 必须是非负整数"))
			return qualification.ListFilter{}, false
		}
		offset = v
	}
	return qualification.ListFilter{
		VerifyStatus: c.Query("verify_status"),
		Limit:        limit,
		Offset:       offset,
	}, true
}

func (s *server) listCompanyCertificates(c *gin.Context) {
	filter, ok := qualificationFilter(c)
	if !ok {
		return
	}
	result, err := s.qualificationStore.ListCertificates(c.Request.Context(),
		tenant.FromContext(c.Request.Context()), filter)
	respond(c, gin.H{
		"items": result.Items,
		"total": result.Total,
		"limit": filter.Limit,
	}, err)
}

func (s *server) listCompanyPersonnel(c *gin.Context) {
	filter, ok := qualificationFilter(c)
	if !ok {
		return
	}
	result, err := s.qualificationStore.ListPersonnel(c.Request.Context(),
		tenant.FromContext(c.Request.Context()), filter)
	respond(c, gin.H{
		"items": result.Items,
		"total": result.Total,
		"limit": filter.Limit,
	}, err)
}

// qualificationSourceStatus 报告资质库（zizhi-api）的连通性与规模。
// 前端用它决定「同步」按钮是否可点，以及展示上游还有多少文件没进档案。
func (s *server) qualificationSourceStatus(c *gin.Context) {
	if s.zizhiClient == nil {
		c.JSON(http.StatusOK, gin.H{
			"configured": false,
			"reason":     "未配置 ZIZHI_API_URL / ZIZHI_API_KEY",
		})
		return
	}
	health, err := s.zizhiClient.Health(c.Request.Context())
	if err != nil {
		// 上游不可达不是本服务的错误，返回 200 让前端正常渲染「暂时不可用」，
		// 而不是弹一个 5xx 报错遮住整个资质页面。
		c.JSON(http.StatusOK, gin.H{
			"configured": true,
			"reachable":  false,
			"reason":     "资质库暂时不可达",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"configured":    true,
		"reachable":     true,
		"total_files":   health.Count,
		"by_category":   health.ByCategory,
		"expired_files": health.Expired,
		"people":        health.People,
		"indexing":      health.Indexing,
		"extracting":    health.Extracting,
	})
}

// 审核抽取结果。只有走过这一步、变成 confirmed 的记录才允许作为投标资格依据，
// 所以这里同时接受字段修正——绝大多数记录的证号与有效期需要人对着原件补。
func (s *server) reviewCompanyCertificate(c *gin.Context) {
	var req qualification.CertificateReview
	if !bindJSON(c, &req) {
		return
	}
	result, err := s.qualificationStore.ReviewCertificate(c.Request.Context(),
		tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) reviewCompanyPersonnel(c *gin.Context) {
	var req qualification.PersonnelReview
	if !bindJSON(c, &req) {
		return
	}
	result, err := s.qualificationStore.ReviewPersonnel(c.Request.Context(),
		tenant.FromContext(c.Request.Context()), c.Param("id"), req)
	respond(c, result, err)
}

func (s *server) syncQualificationFromZizhi(c *gin.Context) {
	if s.zizhiClient == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable,
			apiError("zizhi_not_configured", "资质库未接入，请先配置 ZIZHI_API_URL 与 ZIZHI_API_KEY"))
		return
	}
	result, err := s.qualificationStore.SyncFromZizhi(c.Request.Context(),
		tenant.FromContext(c.Request.Context()), s.zizhiClient)
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
