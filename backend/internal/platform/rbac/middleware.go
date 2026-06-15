package rbac

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Level string

const (
	LevelNone Level = "none"
	LevelRead Level = "read"
	LevelFull Level = "full"
)

const ContextPermissionsKey = "module_permissions"

var supportedModules = map[string]struct{}{
	"dashboard":  {},
	"tender":     {},
	"bid":        {},
	"compliance": {},
	"project":    {},
	"cost":       {},
	"knowledge":  {},
	"team":       {},
}

func Require(module string, level Level) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions := PermissionsFromContext(c)
		if Allows(permissions[module], level) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "permission_denied", "error": "当前账号没有此操作权限", "module": module})
	}
}

func Allows(actual, required Level) bool {
	if required == LevelNone {
		return true
	}
	if actual == LevelFull {
		return true
	}
	return actual == LevelRead && required == LevelRead
}

func PermissionsFromContext(c *gin.Context) map[string]Level {
	if value, exists := c.Get(ContextPermissionsKey); exists {
		if permissions, ok := value.(map[string]Level); ok {
			return permissions
		}
	}
	return map[string]Level{}
}

func ValidModule(module string) bool {
	_, ok := supportedModules[module]
	return ok
}
