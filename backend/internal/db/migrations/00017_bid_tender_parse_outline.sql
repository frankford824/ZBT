-- +goose Up
create table if not exists bid_tender_files (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    file_asset_id uuid not null references file_assets(id),
    status text not null default 'active' check (status in ('active', 'superseded', 'deleted')),
    created_by uuid references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, bid_document_id, file_asset_id)
);

create table if not exists bid_parse_results (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    file_asset_id uuid references file_assets(id),
    status text not null default 'queued' check (status in ('queued', 'processing', 'ready', 'confirmed', 'failed')),
    structured_result jsonb not null default '{}',
    error_message text,
    confirmed_by uuid references users(id),
    confirmed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, bid_document_id)
);

create table if not exists bid_material_selections (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    selected_refs jsonb not null default '[]',
    notes text not null default '',
    updated_by uuid references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, bid_document_id)
);

create index if not exists idx_bid_tender_files_tenant_bid on bid_tender_files(tenant_id, bid_document_id, created_at desc);
create index if not exists idx_bid_parse_results_tenant_status on bid_parse_results(tenant_id, status, updated_at desc);
create index if not exists idx_bid_material_selections_tenant_bid on bid_material_selections(tenant_id, bid_document_id);

alter table bid_tender_files enable row level security;
alter table bid_parse_results enable row level security;
alter table bid_material_selections enable row level security;

alter table bid_tender_files force row level security;
alter table bid_parse_results force row level security;
alter table bid_material_selections force row level security;

create policy tenant_isolation_bid_tender_files on bid_tender_files
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_bid_parse_results on bid_parse_results
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_bid_material_selections on bid_material_selections
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists bid_material_selections;
drop table if exists bid_parse_results;
drop table if exists bid_tender_files;
