-- +goose Up
create table if not exists external_tool_configs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    provider_key text not null,
    name text not null,
    transport text not null default 'streamable_http' check (transport in ('streamable_http')),
    endpoint text not null default '',
    command text not null default '',
    enabled boolean not null default false,
    allowed_tools text[] not null default '{}',
    timeout_ms integer not null default 5000 check (timeout_ms between 500 and 60000),
    monthly_budget numeric(12, 4) not null default 0,
    redaction_policy text not null default 'summary_only' check (redaction_policy in ('summary_only', 'no_sensitive', 'disabled')),
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, provider_key)
);

create table if not exists external_tool_audit_logs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    user_id uuid references users(id) on delete set null,
    config_id uuid references external_tool_configs(id) on delete set null,
    tool_provider text not null,
    tool_name text not null,
    request_hash text not null,
    request_summary text not null default '',
    response_summary text not null default '',
    latency_ms integer not null default 0,
    status text not null check (status in ('success', 'failed', 'blocked')),
    error_message text not null default '',
    estimated_cost numeric(12, 4) not null default 0,
    resource_type text not null default '',
    resource_id uuid,
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now()
);

create index if not exists idx_external_tool_configs_tenant_provider
    on external_tool_configs(tenant_id, provider_key);
create index if not exists idx_external_tool_audit_logs_tenant_created
    on external_tool_audit_logs(tenant_id, created_at desc);
create index if not exists idx_external_tool_audit_logs_tenant_provider
    on external_tool_audit_logs(tenant_id, tool_provider, tool_name, created_at desc);

alter table external_tool_configs enable row level security;
alter table external_tool_configs force row level security;
alter table external_tool_audit_logs enable row level security;
alter table external_tool_audit_logs force row level security;

create policy tenant_isolation_external_tool_configs on external_tool_configs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_external_tool_audit_logs on external_tool_audit_logs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists external_tool_audit_logs;
drop table if exists external_tool_configs;
