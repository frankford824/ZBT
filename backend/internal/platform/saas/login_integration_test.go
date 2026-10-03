package saas

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run only against the disposable regression database, never the live ECS DB.
func TestPublicLoginIntegration(t *testing.T) {
	appURL, adminURL := os.Getenv("ZBT_TEST_APP_DB_URL"), os.Getenv("ZBT_TEST_ADMIN_DB_URL")
	if appURL == "" || adminURL == "" {
		t.Skip("requires isolated migrated test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, appURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	store := NewStore(pool)
	marker := uuid.NewString()
	password := "fixture-password-only"
	a, err := store.Register(ctx, RegisterRequest{TenantName: "Login A", AdminName: "A", Email: marker + "@example.com", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.Register(ctx, RegisterRequest{TenantName: "Login B", AdminName: "B", Email: "b-" + marker + "@example.com", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	login, err := store.Login(ctx, "", a.User.Email, password)
	if err != nil || login.Tenant.ID != a.Tenant.ID {
		t.Fatalf("default login did not resolve registered tenant: %v", err)
	}
	if _, err := store.Login(ctx, "", a.User.Email, "incorrect-password"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong password disclosed membership: %v", err)
	}
	if _, err := store.Login(ctx, b.Tenant.ID, a.User.Email, password); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant login allowed: %v", err)
	}
	if _, err := store.Register(ctx, RegisterRequest{TenantName: "Duplicate", AdminName: "A", Email: a.User.Email, Password: password}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate email not distinguished: %v", err)
	}
	// The helper does not relax ordinary RLS or return all tenant memberships.
	var visible int
	if err := pool.QueryRow(ctx, "select count(*) from tenant_members").Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("ordinary membership RLS bypassed: count=%d err=%v", visible, err)
	}
	if _, err := admin.Exec(ctx, `insert into tenant_members(tenant_id,user_id,status) values($1,$2,'active')`, b.Tenant.ID, a.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `insert into tenant_member_roles(tenant_id,tenant_member_id,role_id)
		select tm.tenant_id,tm.id,r.id from tenant_members tm join roles r on r.tenant_id=tm.tenant_id and r.code='company_admin'
		where tm.tenant_id=$1 and tm.user_id=$2`, b.Tenant.ID, a.User.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.Login(ctx, "", a.User.Email, password)
	var selection *TenantSelectionRequired
	if !errors.As(err, &selection) || len(selection.Tenants) != 2 {
		t.Fatalf("multi-tenant account must choose its own tenant: %v", err)
	}
	if _, err := store.Login(ctx, "", a.User.Email, "incorrect-password"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("multi-tenant choices leaked before authentication: %v", err)
	}
	if _, err := admin.Exec(ctx, `update tenant_members set status='disabled' where user_id=$1`, a.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Login(ctx, "", a.User.Email, password); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled membership logged in: %v", err)
	}
}
