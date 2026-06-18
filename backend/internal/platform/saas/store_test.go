package saas

import (
	"context"
	"errors"
	"strings"
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
		{tenantID: "00000000-0000-4000-8000-000000000001", email: "not-an-email", password: "password1"},
		{tenantID: "00000000-0000-4000-8000-000000000001", email: "admin@example.com", password: ""},
		{tenantID: "00000000-0000-4000-8000-000000000001", email: "admin@example.com", password: strings.Repeat("p", maxSaaSPasswordBytes+1)},
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
	roleID := "00000000-0000-4000-8000-000000000003"
	for _, permissions := range []map[string]rbac.Level{
		{"admin": rbac.LevelFull},
		{"team": rbac.Level("owner")},
	} {
		_, err := store.UpdateRole(context.Background(), "tenant-id", roleID, "Manager", permissions)
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

func TestRegisterRejectsOversizedIdentityBeforeDB(t *testing.T) {
	store := NewStore(nil)
	valid := RegisterRequest{
		TenantName: "智标通",
		AdminName:  "管理员",
		Email:      "admin@example.com",
		Password:   "password1",
	}
	for _, req := range []RegisterRequest{
		{TenantName: strings.Repeat("租", maxSaaSTenantNameRunes+1), AdminName: valid.AdminName, Email: valid.Email, Password: valid.Password},
		{TenantName: valid.TenantName, AdminName: strings.Repeat("管", maxSaaSUserNameRunes+1), Email: valid.Email, Password: valid.Password},
		{TenantName: valid.TenantName, AdminName: valid.AdminName, Email: "not-an-email", Password: valid.Password},
		{TenantName: valid.TenantName, AdminName: valid.AdminName, Email: valid.Email, Password: strings.Repeat("p", maxSaaSPasswordBytes+1)},
		{TenantName: valid.TenantName, AdminName: valid.AdminName, Email: valid.Email, Password: "short"},
	} {
		_, err := store.Register(context.Background(), req)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for register request=%+v, got %v", req, err)
		}
	}
}

func TestNormalizeEmailLowercasesAndRejectsInvalidAddresses(t *testing.T) {
	email, err := normalizeEmail(" ADMIN@Example.COM ")
	if err != nil {
		t.Fatalf("expected bounded email to normalize: %v", err)
	}
	if email != "admin@example.com" {
		t.Fatalf("expected lowercase normalized email, got %q", email)
	}
	for _, value := range []string{
		"not-an-email",
		"Name <admin@example.com>",
		"admin @example.com",
		strings.Repeat("a", maxSaaSEmailRunes) + "@example.com",
	} {
		if _, err := normalizeEmail(value); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected invalid email %q to be rejected, got %v", value, err)
		}
	}
}

func TestNormalizePasswordBoundsBcryptInput(t *testing.T) {
	for _, value := range []string{
		strings.Repeat("p", minSaaSPasswordBytes),
		strings.Repeat("p", maxSaaSPasswordBytes),
	} {
		password, err := normalizePassword(value)
		if err != nil {
			t.Fatalf("expected password length %d to be accepted: %v", len(value), err)
		}
		if password != value {
			t.Fatalf("expected password to remain unchanged")
		}
	}
	for _, value := range []string{
		strings.Repeat("p", minSaaSPasswordBytes-1),
		strings.Repeat("p", maxSaaSPasswordBytes+1),
	} {
		if _, err := normalizePassword(value); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected password length %d to be rejected, got %v", len(value), err)
		}
	}
}

func TestInviteMemberRejectsOversizedIdentityBeforeDB(t *testing.T) {
	store := NewStore(nil)
	for _, req := range []struct {
		email    string
		name     string
		roleCode string
		password string
	}{
		{email: "not-an-email", name: "Alice", roleCode: "viewer", password: "password1"},
		{email: "alice@example.com", name: strings.Repeat("人", maxSaaSUserNameRunes+1), roleCode: "viewer", password: "password1"},
		{email: "alice@example.com", name: "Alice", roleCode: strings.Repeat("r", maxSaaSRoleCodeRunes+1), password: "password1"},
		{email: "alice@example.com", name: "Alice", roleCode: "viewer", password: strings.Repeat("p", maxSaaSPasswordBytes+1)},
	} {
		_, err := store.InviteMember(context.Background(), "tenant-id", req.email, req.name, req.roleCode, req.password)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for invite request=%+v, got %v", req, err)
		}
	}
}

func TestUpdateMemberRejectsInvalidRoleCodesBeforeDB(t *testing.T) {
	store := NewStore(nil)
	tenantID := "tenant-id"
	memberID := "00000000-0000-4000-8000-000000000002"

	_, err := store.UpdateMember(context.Background(), tenantID, "not-a-uuid", UpdateMemberRequest{Name: "Alice"})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid member id to be rejected, got %v", err)
	}

	tooManyRoles := make([]string, maxSaaSRoleCodesPerMember+1)
	for i := range tooManyRoles {
		tooManyRoles[i] = "viewer"
	}
	for _, req := range []UpdateMemberRequest{
		{RoleCodes: tooManyRoles},
		{RoleCode: strings.Repeat("r", maxSaaSRoleCodeRunes+1)},
		{Name: strings.Repeat("人", maxSaaSUserNameRunes+1)},
	} {
		_, err := store.UpdateMember(context.Background(), tenantID, memberID, req)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for member update=%+v, got %v", req, err)
		}
	}
}

func TestNormalizeRoleCodesDedupesAndBoundsValues(t *testing.T) {
	codes, err := normalizeRoleCodes([]string{" viewer ", "viewer"}, "company_admin")
	if err != nil {
		t.Fatalf("expected bounded role codes to normalize: %v", err)
	}
	if got, want := strings.Join(codes, ","), "viewer,company_admin"; got != want {
		t.Fatalf("expected normalized role codes %q, got %q", want, got)
	}
	if _, err := normalizeRoleCodes([]string{strings.Repeat("r", maxSaaSRoleCodeRunes+1)}, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected oversized role code to be rejected, got %v", err)
	}
}

func TestCreateAndUpdateRoleRejectOversizedFieldsBeforeDB(t *testing.T) {
	store := NewStore(nil)
	validPermissions := map[string]rbac.Level{"team": rbac.LevelRead}
	roleID := "00000000-0000-4000-8000-000000000003"
	for _, req := range []struct {
		code string
		name string
	}{
		{code: strings.Repeat("r", maxSaaSRoleCodeRunes+1), name: "Manager"},
		{code: "manager", name: strings.Repeat("角", maxSaaSRoleNameRunes+1)},
	} {
		_, err := store.CreateRole(context.Background(), "tenant-id", req.code, req.name, validPermissions)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for role create=%+v, got %v", req, err)
		}
	}

	_, err := store.UpdateRole(context.Background(), "tenant-id", "not-a-uuid", "Manager", validPermissions)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid role id to be rejected, got %v", err)
	}
	_, err = store.UpdateRole(context.Background(), "tenant-id", roleID, strings.Repeat("角", maxSaaSRoleNameRunes+1), validPermissions)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected oversized role name to be rejected, got %v", err)
	}
}

func TestMarkNotificationsReadRejectsInvalidIDsBeforeDB(t *testing.T) {
	store := NewStore(nil)
	tenantID := "tenant-id"
	userID := "00000000-0000-4000-8000-000000000002"

	tooManyIDs := make([]string, maxSaaSNotificationReadIDs+1)
	for i := range tooManyIDs {
		tooManyIDs[i] = "00000000-0000-4000-8000-000000000001"
	}
	for _, ids := range [][]string{
		{"not-a-uuid"},
		tooManyIDs,
		{" ", "\t"},
	} {
		if _, err := store.MarkNotificationsRead(context.Background(), tenantID, userID, ids); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest for notification ids=%v, got %v", ids, err)
		}
	}
}

func TestNormalizeNotificationIDsDedupesAndCanonicalizes(t *testing.T) {
	ids, err := normalizeNotificationIDs([]string{
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000002",
	})
	if err != nil {
		t.Fatalf("expected bounded notification ids to normalize: %v", err)
	}
	if got, want := strings.Join(ids, ","), "00000000-0000-4000-8000-000000000001,00000000-0000-4000-8000-000000000002"; got != want {
		t.Fatalf("expected normalized notification ids %q, got %q", want, got)
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
