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

func TestLoginRejectsInvalidInputBeforeDB(t *testing.T) {
	store := NewStore(nil)
	for _, req := range []struct {
		tenantID string
		email    string
		password string
	}{
		{tenantID: "", email: "admin@example.com", password: "password1"},
		{tenantID: "not-a-uuid", email: "admin@example.com", password: "password1"},
		{tenantID: "00000000-0000-4000-8000-000000000001", email: "", password: "password1"},
		{tenantID: "00000000-0000-4000-8000-000000000001", email: "admin@example.com", password: ""},
	} {
		_, err := store.Login(context.Background(), req.tenantID, req.email, req.password)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for login request=%+v, got %v", req, err)
		}
	}
}

func TestSessionByUserRoleRejectsInvalidClaimsBeforeDB(t *testing.T) {
	store := NewStore(nil)
	validTenantID := "00000000-0000-4000-8000-000000000001"
	validUserID := "00000000-0000-4000-8000-000000000002"
	validRoleID := "00000000-0000-4000-8000-000000000003"
	for _, req := range []struct {
		tenantID string
		userID   string
		roleID   string
	}{
		{tenantID: "", userID: validUserID, roleID: validRoleID},
		{tenantID: "not-a-uuid", userID: validUserID, roleID: validRoleID},
		{tenantID: validTenantID, userID: "", roleID: validRoleID},
		{tenantID: validTenantID, userID: "not-a-uuid", roleID: validRoleID},
		{tenantID: validTenantID, userID: validUserID, roleID: ""},
		{tenantID: validTenantID, userID: validUserID, roleID: "not-a-uuid"},
	} {
		_, err := store.SessionByUserRole(context.Background(), req.tenantID, req.userID, req.roleID)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for session lookup=%+v, got %v", req, err)
		}
	}
}

func TestCreateRoleRejectsInvalidPermissionsBeforeDB(t *testing.T) {
	store := NewStore(nil)
	for _, req := range []struct {
		name        string
		permissions map[string]rbac.Level
	}{
		{name: "unknown module", permissions: map[string]rbac.Level{"admin": rbac.LevelFull}},
		{name: "unknown level", permissions: map[string]rbac.Level{"team": rbac.Level("owner")}},
	} {
		_, err := store.CreateRole(context.Background(), "tenant-id", "manager", req.name, req.permissions)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for %s, got %v", req.name, err)
		}
	}
}

func TestUpdateRoleRejectsInvalidPermissionsBeforeDB(t *testing.T) {
	store := NewStore(nil)
	for _, permissions := range []map[string]rbac.Level{
		{"admin": rbac.LevelFull},
		{"team": rbac.Level("owner")},
	} {
		_, err := store.UpdateRole(context.Background(), "tenant-id", "role-id", "Manager", permissions)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for permissions=%v, got %v", permissions, err)
		}
	}
}

func TestMergeModulePermissionsKeepsStrongestLevel(t *testing.T) {
	permissions := map[string]rbac.Level{
		"dashboard": rbac.LevelRead,
		"cost":      rbac.LevelNone,
	}

	mergeModulePermissions(permissions, map[string]rbac.Level{
		"dashboard":  rbac.LevelFull,
		"cost":       rbac.LevelRead,
		"knowledge":  rbac.LevelRead,
		"compliance": rbac.LevelNone,
	})
	mergeModulePermissions(permissions, map[string]rbac.Level{
		"dashboard": rbac.LevelRead,
		"cost":      rbac.LevelNone,
		"knowledge": rbac.LevelFull,
	})

	if permissions["dashboard"] != rbac.LevelFull {
		t.Fatalf("expected dashboard to keep full permission, got %q", permissions["dashboard"])
	}
	if permissions["cost"] != rbac.LevelRead {
		t.Fatalf("expected cost to keep read permission, got %q", permissions["cost"])
	}
	if permissions["knowledge"] != rbac.LevelFull {
		t.Fatalf("expected knowledge to upgrade to full permission, got %q", permissions["knowledge"])
	}
	if permissions["compliance"] != rbac.LevelNone {
		t.Fatalf("expected compliance to keep none permission, got %q", permissions["compliance"])
	}
}

func TestEnsureTenantManagementAvailableRequiresActiveTeamAdmin(t *testing.T) {
	if err := ensureTenantManagementAvailable(0); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected zero active team admins to be rejected, got %v", err)
	}
	if err := ensureTenantManagementAvailable(-1); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected negative active team admin count to be rejected, got %v", err)
	}
	if err := ensureTenantManagementAvailable(1); err != nil {
		t.Fatalf("expected one active team admin to be accepted: %v", err)
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
