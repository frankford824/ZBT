-- name: GetTenant :one
select id, name, created_at, updated_at
from tenants
where id = $1;

-- name: ListPermissions :many
select code, description
from permissions
order by code;
