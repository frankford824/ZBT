-- +goose Up
create table if not exists bid_requirement_coverage_events (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    requirement_item_id uuid not null references bid_requirement_items(id) on delete cascade,
    requirement_external_id text not null default '',
    chapter_id uuid references bid_chapters(id) on delete set null,
    actor_user_id uuid references users(id) on delete set null,
    source text not null default 'system' check (source in ('model', 'manual', 'system')),
    coverage_status text not null check (coverage_status in ('unmapped', 'planned', 'covered', 'needs_review')),
    needs_review boolean not null default false,
    evidence text not null default '',
    source_refs jsonb not null default '[]',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now()
);

create index if not exists idx_bid_requirement_coverage_events_tenant_requirement
    on bid_requirement_coverage_events(tenant_id, bid_document_id, requirement_item_id, created_at desc);

create index if not exists idx_bid_requirement_coverage_events_tenant_status
    on bid_requirement_coverage_events(tenant_id, coverage_status, created_at desc);

alter table bid_requirement_coverage_events enable row level security;
alter table bid_requirement_coverage_events force row level security;

create policy tenant_isolation_bid_requirement_coverage_events on bid_requirement_coverage_events
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists bid_requirement_coverage_events;
