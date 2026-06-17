-- +goose Up
create table if not exists bid_requirement_items (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    parse_result_id uuid references bid_parse_results(id) on delete set null,
    external_id text not null,
    module text not null default '',
    requirement_type text not null default '',
    requirement text not null,
    priority text not null default 'medium' check (priority in ('high', 'medium', 'low')),
    mandatory boolean not null default false,
    score numeric(12, 2),
    expected_response text not null default '',
    coverage_status text not null default 'unmapped' check (coverage_status in ('unmapped', 'planned', 'covered', 'needs_review')),
    source_ref jsonb not null default '{}',
    needs_review boolean not null default false,
    sort_order integer not null default 0,
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, bid_document_id, external_id)
);

create index if not exists idx_bid_requirement_items_tenant_bid
    on bid_requirement_items(tenant_id, bid_document_id, sort_order, created_at);

create index if not exists idx_bid_requirement_items_tenant_status
    on bid_requirement_items(tenant_id, coverage_status, updated_at desc);

create index if not exists idx_bid_requirement_items_tenant_module
    on bid_requirement_items(tenant_id, bid_document_id, module, requirement_type);

alter table bid_requirement_items enable row level security;
alter table bid_requirement_items force row level security;

create policy tenant_isolation_bid_requirement_items on bid_requirement_items
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists bid_requirement_items;
