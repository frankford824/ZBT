package saas

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidRequest = errors.New("invalid request")
)

type Store struct {
	pool *pgxpool.Pool
}

type Tenant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Role struct {
	ID          string                `json:"id"`
	Code        string                `json:"code"`
	Name        string                `json:"name"`
	Permissions map[string]rbac.Level `json:"permissions"`
}

type Member struct {
	ID     string `json:"id"`
	User   User   `json:"user"`
	Status string `json:"status"`
	Roles  []Role `json:"roles"`
}

type UpdateMemberRequest struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	RoleCode  string   `json:"role_code"`
	RoleCodes []string `json:"role_codes"`
}

type Session struct {
	User             User                  `json:"user"`
	Tenant           Tenant                `json:"tenant"`
	Role             Role                  `json:"role"`
	Roles            []Role                `json:"roles"`
	Permissions      map[string]rbac.Level `json:"permissions"`
	SessionRevokedAt *time.Time            `json:"-"`
}

type RegisterRequest struct {
	TenantName string `json:"tenant_name"`
	AdminName  string `json:"admin_name"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

type Notification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	ReadAt    *string   `json:"read_at"`
	CreatedAt time.Time `json:"created_at"`
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Register(ctx context.Context, req RegisterRequest) (Session, error) {
	tenantName := strings.TrimSpace(req.TenantName)
	adminName := strings.TrimSpace(req.AdminName)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	if tenantName == "" || adminName == "" || email == "" || len(password) < 8 {
		return Session{}, ErrInvalidRequest
	}
	tenantID := uuid.NewString()
	var session Session
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from users where lower(email) = lower($1))`, email).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrInvalidRequest
		}
		if _, err := tx.Exec(ctx, `
			insert into tenants (id, name)
			values ($1, $2)
		`, tenantID, tenantName); err != nil {
			return err
		}
		roleID, err := seedDefaultRoles(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		var user User
		if err := tx.QueryRow(ctx, `
			insert into users (email, name, password_hash)
			values ($1, $2, crypt($3, gen_salt('bf')))
			returning id::text, email, name
		`, email, adminName, password).Scan(&user.ID, &user.Email, &user.Name); err != nil {
			return err
		}
		var memberID string
		if err := tx.QueryRow(ctx, `
			insert into tenant_members (tenant_id, user_id, status)
			values ($1, $2, 'active')
			returning id::text
		`, tenantID, user.ID).Scan(&memberID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into tenant_member_roles (tenant_id, tenant_member_id, role_id)
			values ($1, $2, $3)
		`, tenantID, memberID, roleID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into notifications (tenant_id, user_id, title, body)
			values ($1, $2, '企业已创建', '欢迎使用智标通，请先完善团队、知识库和审批链配置。')
		`, tenantID, user.ID); err != nil {
			return err
		}
		role, err := roleByID(ctx, tx, roleID)
		if err != nil {
			return err
		}
		session = Session{
			User:        user,
			Tenant:      Tenant{ID: tenantID, Name: tenantName},
			Role:        role,
			Roles:       []Role{role},
			Permissions: role.Permissions,
		}
		return nil
	})
	return session, normalizeSaaSWriteError(err)
}

func (s *Store) Login(ctx context.Context, tenantID, email, password string) (Session, error) {
	tenantID = strings.TrimSpace(tenantID)
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if tenantID == "" || email == "" || password == "" {
		return Session{}, ErrInvalidRequest
	}
	if _, err := uuid.Parse(tenantID); err != nil {
		return Session{}, ErrInvalidRequest
	}
	var session Session
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var memberID string
		err := tx.QueryRow(ctx, `
			select
				u.id::text, u.email, u.name,
				t.id::text, t.name,
				tm.id::text
			from users u
			join tenant_members tm on tm.user_id = u.id
			join tenants t on t.id = tm.tenant_id
			where tm.tenant_id = $1
				and tm.status = 'active'
				and lower(u.email) = lower($2)
				and u.password_hash = crypt($3, u.password_hash)
			limit 1
		`, tenantID, email, password).Scan(
			&session.User.ID, &session.User.Email, &session.User.Name,
			&session.Tenant.ID, &session.Tenant.Name,
			&memberID,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		roles, permissions, err := loadSessionRoles(ctx, tx, tenantID, memberID)
		if err != nil {
			return err
		}
		if len(roles) == 0 {
			return ErrNotFound
		}
		primaryRole := roles[0]
		primaryRole.Permissions = permissions
		session.Role = primaryRole
		session.Roles = roles
		session.Permissions = permissions
		return nil
	})
	return session, err
}

func (s *Store) SessionByUserRole(ctx context.Context, tenantID, userID, roleID string) (Session, error) {
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	roleID = strings.TrimSpace(roleID)
	if tenantID == "" || userID == "" || roleID == "" {
		return Session{}, ErrInvalidRequest
	}
	if _, err := uuid.Parse(tenantID); err != nil {
		return Session{}, ErrInvalidRequest
	}
	if _, err := uuid.Parse(userID); err != nil {
		return Session{}, ErrInvalidRequest
	}
	if _, err := uuid.Parse(roleID); err != nil {
		return Session{}, ErrInvalidRequest
	}
	var session Session
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var memberID string
		err := tx.QueryRow(ctx, `
			select
				u.id::text, u.email, u.name,
				t.id::text, t.name,
				tm.id::text,
				tm.session_revoked_at,
				r.id::text, r.code, r.name
			from users u
			join tenant_members tm on tm.user_id = u.id
			join tenants t on t.id = tm.tenant_id
			join tenant_member_roles tmr on tmr.tenant_member_id = tm.id and tmr.tenant_id = tm.tenant_id
			join roles r on r.id = tmr.role_id and r.tenant_id = tm.tenant_id
			where tm.tenant_id = $1
				and u.id = $2
				and r.id = $3
				and tm.status = 'active'
			limit 1
		`, tenantID, userID, roleID).Scan(
			&session.User.ID, &session.User.Email, &session.User.Name,
			&session.Tenant.ID, &session.Tenant.Name,
			&memberID,
			&session.SessionRevokedAt,
			&session.Role.ID, &session.Role.Code, &session.Role.Name,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		roles, permissions, err := loadSessionRoles(ctx, tx, tenantID, memberID)
		if err != nil {
			return err
		}
		if len(roles) == 0 {
			return ErrNotFound
		}
		session.Permissions = permissions
		session.Roles = roles
		session.Role.Permissions = permissions
		return nil
	})
	return session, err
}

func (s *Store) RevokeUserSessions(ctx context.Context, tenantID, userID string) error {
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	if tenantID == "" || userID == "" {
		return ErrInvalidRequest
	}
	if _, err := uuid.Parse(tenantID); err != nil {
		return ErrInvalidRequest
	}
	if _, err := uuid.Parse(userID); err != nil {
		return ErrInvalidRequest
	}
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update tenant_members
			set session_revoked_at = clock_timestamp(), updated_at = now()
			where tenant_id = $1 and user_id = $2
		`, tenantID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) GetTenant(ctx context.Context, tenantID string) (Tenant, error) {
	var tenant Tenant
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `select id::text, name from tenants where id = $1`, tenantID).Scan(&tenant.ID, &tenant.Name)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return tenant, err
}

func (s *Store) UpdateTenant(ctx context.Context, tenantID, name string) (Tenant, error) {
	var tenant Tenant
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			update tenants
			set name = $2, updated_at = now()
			where id = $1
			returning id::text, name
		`, tenantID, name).Scan(&tenant.ID, &tenant.Name)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return tenant, err
}

func (s *Store) ListMembers(ctx context.Context, tenantID string) ([]Member, error) {
	var members []Member
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select
				tm.id::text, u.id::text, u.email, u.name, tm.status,
				r.id::text, r.code, r.name
			from tenant_members tm
			join users u on u.id = tm.user_id
			left join tenant_member_roles tmr on tmr.tenant_member_id = tm.id and tmr.tenant_id = tm.tenant_id
			left join roles r on r.id = tmr.role_id and r.tenant_id = tm.tenant_id
			where tm.tenant_id = $1
			order by u.name, r.code
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()

		byID := map[string]*Member{}
		for rows.Next() {
			var memberID, userID, email, name, status string
			var roleID, roleCode, roleName *string
			if err := rows.Scan(&memberID, &userID, &email, &name, &status, &roleID, &roleCode, &roleName); err != nil {
				return err
			}
			member, ok := byID[memberID]
			if !ok {
				members = append(members, Member{
					ID:     memberID,
					User:   User{ID: userID, Email: email, Name: name},
					Status: status,
					Roles:  []Role{},
				})
				member = &members[len(members)-1]
				byID[memberID] = member
			}
			if roleID != nil {
				member.Roles = append(member.Roles, Role{ID: *roleID, Code: *roleCode, Name: *roleName})
			}
		}
		return rows.Err()
	})
	return members, err
}

func (s *Store) InviteMember(ctx context.Context, tenantID, email, name, roleCode, initialPassword string) (Member, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	roleCode = strings.TrimSpace(roleCode)
	initialPassword = strings.TrimSpace(initialPassword)
	if email == "" || name == "" || len(initialPassword) < 8 {
		return Member{}, ErrInvalidRequest
	}
	if roleCode == "" {
		roleCode = "viewer"
	}
	var member Member
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var role Role
		if err := tx.QueryRow(ctx, `select id::text, code, name from roles where tenant_id = $1 and code = $2`, tenantID, roleCode).Scan(&role.ID, &role.Code, &role.Name); err != nil {
			return err
		}
		var user User
		if err := tx.QueryRow(ctx, `
			insert into users (email, name, password_hash)
			values ($1, $2, crypt($3, gen_salt('bf')))
			on conflict (email) do update set name = excluded.name, updated_at = now()
			returning id::text, email, name
		`, email, name, initialPassword).Scan(&user.ID, &user.Email, &user.Name); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			insert into tenant_members (tenant_id, user_id, status)
			values ($1, $2, 'active')
			on conflict (tenant_id, user_id) do update set status = 'active', updated_at = now()
			returning id::text, status
		`, tenantID, user.ID).Scan(&member.ID, &member.Status); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into tenant_member_roles (tenant_id, tenant_member_id, role_id)
			values ($1, $2, $3)
			on conflict do nothing
		`, tenantID, member.ID, role.ID); err != nil {
			return err
		}
		member.User = user
		member.Roles = []Role{role}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrInvalidRequest
	}
	return member, normalizeSaaSWriteError(err)
}

func (s *Store) UpdateMember(ctx context.Context, tenantID, memberID string, req UpdateMemberRequest) (Member, error) {
	var member Member
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var userID string
		if err := tx.QueryRow(ctx, `
			select user_id::text
			from tenant_members
			where tenant_id = $1 and id = $2
		`, tenantID, memberID).Scan(&userID); err != nil {
			return err
		}
		if name := strings.TrimSpace(req.Name); name != "" {
			if _, err := tx.Exec(ctx, `
				update users
				set name = $2, updated_at = now()
				where id = $1
			`, userID, name); err != nil {
				return err
			}
		}
		if status := normalizeMemberStatus(req.Status); status != "" {
			if _, err := tx.Exec(ctx, `
				update tenant_members
				set status = $3, updated_at = now()
				where tenant_id = $1 and id = $2
			`, tenantID, memberID, status); err != nil {
				return err
			}
		} else if strings.TrimSpace(req.Status) != "" {
			return ErrInvalidRequest
		}
		roleCodes := normalizeRoleCodes(req.RoleCodes, req.RoleCode)
		if req.RoleCodes != nil || strings.TrimSpace(req.RoleCode) != "" {
			if len(roleCodes) == 0 {
				return ErrInvalidRequest
			}
			rows, err := tx.Query(ctx, `
				select id::text, code, name
				from roles
				where tenant_id = $1 and code = any($2)
			`, tenantID, roleCodes)
			if err != nil {
				return err
			}
			defer rows.Close()
			roles := []Role{}
			for rows.Next() {
				var role Role
				if err := rows.Scan(&role.ID, &role.Code, &role.Name); err != nil {
					return err
				}
				roles = append(roles, role)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(roles) != len(roleCodes) {
				return ErrInvalidRequest
			}
			if _, err := tx.Exec(ctx, `delete from tenant_member_roles where tenant_id = $1 and tenant_member_id = $2`, tenantID, memberID); err != nil {
				return err
			}
			for _, role := range roles {
				if _, err := tx.Exec(ctx, `
					insert into tenant_member_roles (tenant_id, tenant_member_id, role_id)
					values ($1, $2, $3)
					on conflict do nothing
				`, tenantID, memberID, role.ID); err != nil {
					return err
				}
			}
		}
		loaded, err := memberByID(ctx, tx, tenantID, memberID)
		if err != nil {
			return err
		}
		if err := ensureTenantHasActiveTeamAdmin(ctx, tx, tenantID); err != nil {
			return err
		}
		member = loaded
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotFound
	}
	return member, err
}

func (s *Store) DeleteMember(ctx context.Context, tenantID, memberID string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update tenant_members
			set status = 'disabled', updated_at = now()
			where tenant_id = $1 and id = $2
		`, tenantID, memberID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return ensureTenantHasActiveTeamAdmin(ctx, tx, tenantID)
	})
}

func (s *Store) ListRoles(ctx context.Context, tenantID string) ([]Role, error) {
	roles := []Role{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select
				r.id::text, r.code, r.name,
				coalesce(jsonb_object_agg(mp.module, mp.level) filter (where mp.module is not null), '{}'::jsonb)
			from roles r
			left join module_permissions mp on mp.role_id = r.id and mp.tenant_id = r.tenant_id
			where r.tenant_id = $1
			group by r.id, r.code, r.name
			order by r.code
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			role := Role{Permissions: map[string]rbac.Level{}}
			var raw []byte
			if err := rows.Scan(&role.ID, &role.Code, &role.Name, &raw); err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &role.Permissions); err != nil {
				return err
			}
			roles = append(roles, role)
		}
		return rows.Err()
	})
	return roles, err
}

func (s *Store) CreateRole(ctx context.Context, tenantID, code, name string, permissions map[string]rbac.Level) (Role, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return Role{}, ErrInvalidRequest
	}
	if err := validateModulePermissions(permissions); err != nil {
		return Role{}, err
	}
	var role Role
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			insert into roles (tenant_id, code, name)
			values ($1, $2, $3)
			returning id::text, code, name
		`, tenantID, code, name).Scan(&role.ID, &role.Code, &role.Name); err != nil {
			return err
		}
		if err := replaceModulePermissions(ctx, tx, tenantID, role.ID, permissions); err != nil {
			return err
		}
		role.Permissions = permissions
		return nil
	})
	return role, normalizeSaaSWriteError(err)
}

func (s *Store) UpdateRole(ctx context.Context, tenantID, roleID, name string, permissions map[string]rbac.Level) (Role, error) {
	if permissions != nil {
		if err := validateModulePermissions(permissions); err != nil {
			return Role{}, err
		}
	}
	var role Role
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			update roles
			set name = coalesce(nullif($3, ''), name), updated_at = now()
			where tenant_id = $1 and id = $2
			returning id::text, code, name
		`, tenantID, roleID, name).Scan(&role.ID, &role.Code, &role.Name); err != nil {
			return err
		}
		if permissions != nil {
			if err := replaceModulePermissions(ctx, tx, tenantID, role.ID, permissions); err != nil {
				return err
			}
			if err := ensureTenantHasActiveTeamAdmin(ctx, tx, tenantID); err != nil {
				return err
			}
		}
		loaded, err := loadRolePermissions(ctx, tx, role.ID)
		if err != nil {
			return err
		}
		role.Permissions = loaded
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrNotFound
	}
	return role, err
}

func (s *Store) DeleteRole(ctx context.Context, tenantID, roleID string) error {
	return s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `delete from role_permissions where tenant_id = $1 and role_id = $2`, tenantID, roleID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `delete from module_permissions where tenant_id = $1 and role_id = $2`, tenantID, roleID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `delete from tenant_member_roles where tenant_id = $1 and role_id = $2`, tenantID, roleID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `delete from roles where tenant_id = $1 and id = $2`, tenantID, roleID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return ensureTenantHasActiveTeamAdmin(ctx, tx, tenantID)
	})
}

func (s *Store) ListNotifications(ctx context.Context, tenantID, userID string) ([]Notification, error) {
	notifications := []Notification{}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select id::text, title, body, read_at::text, created_at
			from notifications
			where tenant_id = $1 and (user_id is null or user_id = $2)
			order by created_at desc
			limit 50
		`, tenantID, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var notification Notification
			if err := rows.Scan(&notification.ID, &notification.Title, &notification.Body, &notification.ReadAt, &notification.CreatedAt); err != nil {
				return err
			}
			notifications = append(notifications, notification)
		}
		return rows.Err()
	})
	return notifications, err
}

func (s *Store) MarkNotificationsRead(ctx context.Context, tenantID, userID string, ids []string) (int64, error) {
	var affected int64
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if len(ids) == 0 {
			tag, err := tx.Exec(ctx, `
				update notifications
				set read_at = coalesce(read_at, now()), updated_at = now()
				where tenant_id = $1 and (user_id is null or user_id = $2) and read_at is null
			`, tenantID, userID)
			if err != nil {
				return err
			}
			affected = tag.RowsAffected()
			return nil
		} else {
			tag, err := tx.Exec(ctx, `
				update notifications
				set read_at = coalesce(read_at, now()), updated_at = now()
				where tenant_id = $1 and (user_id is null or user_id = $2) and id::text = any($3)
			`, tenantID, userID, ids)
			if err != nil {
				return err
			}
			affected = tag.RowsAffected()
			return nil
		}
	})
	return affected, err
}

func (s *Store) withTenant(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func loadRolePermissions(ctx context.Context, tx pgx.Tx, roleID string) (map[string]rbac.Level, error) {
	rows, err := tx.Query(ctx, `select module, level from module_permissions where role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := map[string]rbac.Level{}
	for rows.Next() {
		var module string
		var level rbac.Level
		if err := rows.Scan(&module, &level); err != nil {
			return nil, err
		}
		permissions[module] = level
	}
	return permissions, rows.Err()
}

func loadSessionRoles(ctx context.Context, tx pgx.Tx, tenantID, memberID string) ([]Role, map[string]rbac.Level, error) {
	rows, err := tx.Query(ctx, `
		select
			r.id::text, r.code, r.name,
			coalesce(jsonb_object_agg(mp.module, mp.level) filter (where mp.module is not null), '{}'::jsonb)
		from tenant_member_roles tmr
		join roles r on r.id = tmr.role_id and r.tenant_id = tmr.tenant_id
		left join module_permissions mp on mp.role_id = r.id and mp.tenant_id = r.tenant_id
		where tmr.tenant_id = $1 and tmr.tenant_member_id = $2
		group by r.id, r.code, r.name
		order by r.code
	`, tenantID, memberID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	roles := []Role{}
	permissions := map[string]rbac.Level{}
	for rows.Next() {
		role := Role{Permissions: map[string]rbac.Level{}}
		var raw []byte
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &raw); err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal(raw, &role.Permissions); err != nil {
			return nil, nil, err
		}
		mergeModulePermissions(permissions, role.Permissions)
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return roles, permissions, nil
}

func mergeModulePermissions(target, source map[string]rbac.Level) {
	for module, level := range source {
		current, exists := target[module]
		if !exists || levelRank(level) > levelRank(current) {
			target[module] = level
		}
	}
}

func levelRank(level rbac.Level) int {
	switch level {
	case rbac.LevelFull:
		return 2
	case rbac.LevelRead:
		return 1
	default:
		return 0
	}
}

func replaceModulePermissions(ctx context.Context, tx pgx.Tx, tenantID, roleID string, permissions map[string]rbac.Level) error {
	if err := validateModulePermissions(permissions); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from module_permissions where tenant_id = $1 and role_id = $2`, tenantID, roleID); err != nil {
		return err
	}
	for module, level := range permissions {
		if _, err := tx.Exec(ctx, `
			insert into module_permissions (tenant_id, role_id, module, level)
			values ($1, $2, $3, $4)
		`, tenantID, roleID, module, level); err != nil {
			return err
		}
	}
	return nil
}

func ensureTenantHasActiveTeamAdmin(ctx context.Context, tx pgx.Tx, tenantID string) error {
	var count int
	if err := tx.QueryRow(ctx, `
		select count(distinct tm.id)::int
		from tenant_members tm
		join tenant_member_roles tmr on tmr.tenant_member_id = tm.id and tmr.tenant_id = tm.tenant_id
		join module_permissions mp on mp.role_id = tmr.role_id and mp.tenant_id = tmr.tenant_id
		where tm.tenant_id = $1
			and tm.status = 'active'
			and mp.module = 'team'
			and mp.level = 'full'
	`, tenantID).Scan(&count); err != nil {
		return err
	}
	return ensureTenantManagementAvailable(count)
}

func ensureTenantManagementAvailable(activeTeamAdmins int) error {
	if activeTeamAdmins <= 0 {
		return ErrInvalidRequest
	}
	return nil
}

func validateModulePermissions(permissions map[string]rbac.Level) error {
	for module, level := range permissions {
		if !rbac.ValidModule(module) || !validLevel(level) {
			return ErrInvalidRequest
		}
	}
	return nil
}

func validLevel(level rbac.Level) bool {
	return level == rbac.LevelNone || level == rbac.LevelRead || level == rbac.LevelFull
}

func normalizeSaaSWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23505":
			return ErrInvalidRequest
		}
	}
	return err
}

func seedDefaultRoles(ctx context.Context, tx pgx.Tx, tenantID string) (string, error) {
	definitions := []struct {
		Code        string
		Name        string
		Permissions map[string]rbac.Level
	}{
		{Code: "company_admin", Name: "企业管理员", Permissions: fullModulePermissions()},
		{Code: "department_admin", Name: "部门管理员", Permissions: fullModulePermissions()},
		{Code: "project_manager", Name: "项目经理", Permissions: map[string]rbac.Level{
			"dashboard": rbac.LevelRead,
			"tender":    rbac.LevelFull, "bid": rbac.LevelFull, "compliance": rbac.LevelFull, "project": rbac.LevelFull,
			"cost": rbac.LevelRead, "knowledge": rbac.LevelFull, "team": rbac.LevelRead,
		}},
		{Code: "bid_specialist", Name: "投标专员", Permissions: map[string]rbac.Level{
			"dashboard": rbac.LevelRead,
			"tender":    rbac.LevelRead, "bid": rbac.LevelFull, "compliance": rbac.LevelFull, "project": rbac.LevelRead,
			"cost": rbac.LevelNone, "knowledge": rbac.LevelRead, "team": rbac.LevelNone,
		}},
		{Code: "viewer", Name: "查看者", Permissions: map[string]rbac.Level{
			"dashboard": rbac.LevelRead,
			"tender":    rbac.LevelRead, "bid": rbac.LevelRead, "compliance": rbac.LevelRead, "project": rbac.LevelRead,
			"cost": rbac.LevelNone, "knowledge": rbac.LevelRead, "team": rbac.LevelNone,
		}},
	}
	companyAdminRoleID := ""
	for _, definition := range definitions {
		var roleID string
		if err := tx.QueryRow(ctx, `
			insert into roles (tenant_id, code, name)
			values ($1, $2, $3)
			returning id::text
		`, tenantID, definition.Code, definition.Name).Scan(&roleID); err != nil {
			return "", err
		}
		if err := replaceModulePermissions(ctx, tx, tenantID, roleID, definition.Permissions); err != nil {
			return "", err
		}
		if definition.Code == "company_admin" {
			companyAdminRoleID = roleID
		}
	}
	if companyAdminRoleID == "" {
		return "", ErrInvalidRequest
	}
	return companyAdminRoleID, nil
}

func fullModulePermissions() map[string]rbac.Level {
	return map[string]rbac.Level{
		"dashboard":  rbac.LevelFull,
		"tender":     rbac.LevelFull,
		"bid":        rbac.LevelFull,
		"compliance": rbac.LevelFull,
		"project":    rbac.LevelFull,
		"cost":       rbac.LevelFull,
		"knowledge":  rbac.LevelFull,
		"team":       rbac.LevelFull,
	}
}

func roleByID(ctx context.Context, tx pgx.Tx, roleID string) (Role, error) {
	var role Role
	if err := tx.QueryRow(ctx, `
		select id::text, code, name
		from roles
		where id = $1
	`, roleID).Scan(&role.ID, &role.Code, &role.Name); err != nil {
		return Role{}, err
	}
	permissions, err := loadRolePermissions(ctx, tx, role.ID)
	if err != nil {
		return Role{}, err
	}
	role.Permissions = permissions
	return role, nil
}

func memberByID(ctx context.Context, tx pgx.Tx, tenantID, memberID string) (Member, error) {
	rows, err := tx.Query(ctx, `
		select
			tm.id::text, u.id::text, u.email, u.name, tm.status,
			r.id::text, r.code, r.name
		from tenant_members tm
		join users u on u.id = tm.user_id
		left join tenant_member_roles tmr on tmr.tenant_member_id = tm.id and tmr.tenant_id = tm.tenant_id
		left join roles r on r.id = tmr.role_id and r.tenant_id = tm.tenant_id
		where tm.tenant_id = $1 and tm.id = $2
		order by r.code
	`, tenantID, memberID)
	if err != nil {
		return Member{}, err
	}
	defer rows.Close()
	var member *Member
	for rows.Next() {
		var memberID, userID, email, name, status string
		var roleID, roleCode, roleName *string
		if err := rows.Scan(&memberID, &userID, &email, &name, &status, &roleID, &roleCode, &roleName); err != nil {
			return Member{}, err
		}
		if member == nil {
			member = &Member{
				ID:     memberID,
				User:   User{ID: userID, Email: email, Name: name},
				Status: status,
				Roles:  []Role{},
			}
		}
		if roleID != nil {
			member.Roles = append(member.Roles, Role{ID: *roleID, Code: *roleCode, Name: *roleName})
		}
	}
	if err := rows.Err(); err != nil {
		return Member{}, err
	}
	if member == nil {
		return Member{}, pgx.ErrNoRows
	}
	return *member, nil
}

func normalizeMemberStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "active", "invited", "disabled":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeRoleCodes(values []string, single string) []string {
	seen := map[string]bool{}
	codes := []string{}
	for _, value := range append(values, single) {
		code := strings.TrimSpace(value)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		codes = append(codes, code)
	}
	return codes
}
