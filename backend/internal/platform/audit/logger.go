package audit

import (
	"time"

	"github.com/gin-gonic/gin"
)

type Event struct {
	TenantID  string    `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	CreatedAt time.Time `json:"created_at"`
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
