package taskstatus

import "strings"

func ShouldApplyCallback(current, next string) bool {
	current = strings.ToLower(strings.TrimSpace(current))
	next = strings.ToLower(strings.TrimSpace(next))
	if next == "" {
		return false
	}
	switch current {
	case "done", "failed", "cancelled":
		return false
	case "running":
		return next != "queued"
	case "queued":
		return true
	default:
		return false
	}
}
