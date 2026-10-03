-- +goose Up
-- The application role must not read arbitrary tenant memberships before login.
-- This bounded, read-only definer function exposes memberships ONLY after
-- verifying the password. Ordinary tenant queries retain their existing RLS.
-- +goose StatementBegin
create function public.authenticated_login_tenants(login_email text, login_password text)
returns table (tenant_id uuid, tenant_name text)
language sql stable security definer
set search_path = pg_catalog, public
as $$
    select t.id, t.name
    from public.users u
    join public.tenant_members tm on tm.user_id = u.id and tm.status = 'active'
    join public.tenants t on t.id = tm.tenant_id
    where lower(u.email) = lower(login_email)
      and u.password_hash = public.crypt(login_password, u.password_hash)
      and exists (
          select 1 from public.tenant_member_roles tmr
          join public.roles r on r.id = tmr.role_id and r.tenant_id = tmr.tenant_id
          where tmr.tenant_member_id = tm.id and tmr.tenant_id = tm.tenant_id
      )
    order by tm.created_at, t.id
    limit 50
$$;
-- +goose StatementEnd
revoke all on function public.authenticated_login_tenants(text, text) from public;
grant execute on function public.authenticated_login_tenants(text, text) to zbt_app;

-- +goose Down
drop function public.authenticated_login_tenants(text, text);
