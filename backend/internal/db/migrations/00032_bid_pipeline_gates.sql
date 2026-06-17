-- +goose Up
create table if not exists bid_pipeline_gates (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    stage text not null check (stage in ('interpret', 'plan', 'generate', 'check', 'format')),
    status text not null default 'pending' check (status in ('pending', 'needs_review', 'passed', 'blocked')),
    reviewed_by uuid references users(id) on delete set null,
    reviewed_at timestamptz,
    reason text not null default '',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, bid_document_id, stage)
);

create index if not exists idx_bid_pipeline_gates_bid on bid_pipeline_gates(tenant_id, bid_document_id, stage);
create index if not exists idx_bid_pipeline_gates_status on bid_pipeline_gates(tenant_id, status, updated_at desc);

alter table bid_pipeline_gates enable row level security;
alter table bid_pipeline_gates force row level security;

drop policy if exists tenant_isolation_bid_pipeline_gates on bid_pipeline_gates;
create policy tenant_isolation_bid_pipeline_gates on bid_pipeline_gates
    using (tenant_id = current_setting('app.tenant_id', true)::uuid)
    with check (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose Down
drop table if exists bid_pipeline_gates;
