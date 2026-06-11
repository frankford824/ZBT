-- +goose Up
alter table file_assets add column if not exists status text not null default 'pending';
alter table file_assets add column if not exists confirmed_at timestamptz;

-- +goose StatementBegin
do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'file_assets_status_check'
    ) then
        alter table file_assets
            add constraint file_assets_status_check
            check (status in ('pending', 'ready', 'failed', 'deleted'));
    end if;
end $$;
-- +goose StatementEnd

create unique index if not exists idx_file_assets_object_key on file_assets(object_key);
create index if not exists idx_file_assets_tenant_biz_created on file_assets(tenant_id, biz_type, created_at desc);

-- +goose Down
drop index if exists idx_file_assets_tenant_biz_created;
drop index if exists idx_file_assets_object_key;
alter table file_assets drop constraint if exists file_assets_status_check;
alter table file_assets drop column if exists confirmed_at;
alter table file_assets drop column if exists status;
