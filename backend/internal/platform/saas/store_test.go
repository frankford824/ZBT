package saas

import (
	"context"
	"errors"
	"testing"

	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCreateRoleRejectsBlankCodeOrName(t *testing.T) {
	store := NewStore(nil)
	for _, req := range []struct {
		code string
		name string
	}{
		{code: "", name: "Manager"},
		{code: "manager", name: ""},
		{code: "   ", name: "Manager"},
		{code: "manager", name: "   "},
	} {
		_, err := store.CreateRole(context.Background(), "tenant-id", req.code, req.name, map[string]rbac.Level{"team": rbac.LevelRead})
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for code=%q name=%q, got %v", req.code, req.name, err)
		}
	}
}

func TestInviteMemberRejectsBlankIdentity(t *testing.T) {
	store := NewStore(nil)
	for _, req := range []struct {
		email    string
		name     string
		password string
	}{
		{email: "", name: "Alice", password: "password1"},
		{email: "alice@example.com", name: "", password: "password1"},
		{email: "   ", name: "Alice", password: "password1"},
		{email: "alice@example.com", name: "   ", password: "password1"},
		{email: "alice@example.com", name: "Alice", password: "short"},
	} {
		_, err := store.InviteMember(context.Background(), "tenant-id", req.email, req.name, "viewer", req.password)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for email=%q name=%q, got %v", req.email, req.name, err)
		}
	}
}

func TestNormalizeSaaSWriteErrorMapsConstraintFailures(t *testing.T) {
	for _, code := range []string{"23503", "23505"} {
		err := normalizeSaaSWriteError(&pgconn.PgError{Code: code})
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected %s to map to ErrInvalidRequest, got %v", code, err)
		}
	}
}
