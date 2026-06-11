-- +goose Up
update module_permissions mp
set tenant_id = r.tenant_id,
    updated_at = now()
from roles r
where mp.role_id = r.id
  and mp.tenant_id <> r.tenant_id;

-- +goose Down
-- Data repair migration; no safe automatic reversal.
