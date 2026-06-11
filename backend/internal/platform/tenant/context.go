package tenant

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	ContextKey      contextKey = "tenant_id"
	Header                     = "X-Tenant-ID"
	DefaultTenantID            = "00000000-0000-4000-8000-000000000001"
)

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader(Header)
		if value, exists := c.Get("tenant_id"); exists {
			if current, ok := value.(string); ok && current != "" {
				tenantID = current
			}
		}
		if tenantID == "" {
			tenantID = DefaultTenantID
		}
		ctx := context.WithValue(c.Request.Context(), ContextKey, tenantID)
		c.Request = c.Request.WithContext(ctx)
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

func FromContext(ctx context.Context) string {
	value, _ := ctx.Value(ContextKey).(string)
	if value == "" {
		return DefaultTenantID
	}
	return value
}

func Require(c *gin.Context) string {
	tenantID := FromContext(c.Request.Context())
	if tenantID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return ""
	}
	return tenantID
}
