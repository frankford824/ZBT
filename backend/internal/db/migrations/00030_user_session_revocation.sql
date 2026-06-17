-- +goose Up
alter table tenant_members
    add column if not exists session_revoked_at timestamptz;

-- +goose Down
alter table tenant_members
    drop column if exists session_revoked_at;
