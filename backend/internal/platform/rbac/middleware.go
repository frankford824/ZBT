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

var demoPermissions = map[string]Level{
	"dashboard":  LevelFull,
	"tender":     LevelFull,
	"bid":        LevelFull,
	"compliance": LevelFull,
	"project":    LevelFull,
	"cost":       LevelFull,
	"knowledge":  LevelFull,
	"team":       LevelFull,
}

func Require(module string, level Level) gin.HandlerFunc {
	return func(c *gin.Context) {
		if allows(demoPermissions[module], level) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "permission denied", "module": module})
	}
}

func allows(actual, required Level) bool {
	if required == LevelNone {
		return true
	}
	if actual == LevelFull {
		return true
	}
	return actual == LevelRead && required == LevelRead
}

func DemoPermissions() map[string]Level {
	copied := make(map[string]Level, len(demoPermissions))
	for key, value := range demoPermissions {
		copied[key] = value
	}
	return copied
}
