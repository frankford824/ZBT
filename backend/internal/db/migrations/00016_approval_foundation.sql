-- +goose Up
create table if not exists approval_chains (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    name text not null,
    description text not null default '',
    resource_type text not null default 'bid',
    steps jsonb not null default '[]',
    enabled boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists approval_instances (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    chain_id uuid references approval_chains(id),
    bid_document_id uuid references bid_documents(id),
    title text not null,
    status text not null default 'pending' check (status in ('pending', 'approved', 'rejected', 'cancelled')),
    current_step integer not null default 1,
    submitted_by uuid references users(id),
    snapshot jsonb not null default '{}',
    started_at timestamptz not null default now(),
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists approval_actions (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    instance_id uuid not null references approval_instances(id) on delete cascade,
    actor_user_id uuid references users(id),
    action text not null check (action in ('submit', 'approve', 'reject', 'cancel')),
    step_order integer not null default 0,
    comment text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists comments (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    resource_type text not null,
    resource_id uuid not null,
    user_id uuid references users(id),
    body text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_approval_chains_tenant_enabled on approval_chains(tenant_id, enabled, updated_at desc);
create index if not exists idx_approval_instances_tenant_status on approval_instances(tenant_id, status, created_at desc);
create index if not exists idx_approval_instances_bid on approval_instances(tenant_id, bid_document_id, status);
create index if not exists idx_approval_actions_instance on approval_actions(tenant_id, instance_id, created_at);
create index if not exists idx_comments_resource on comments(tenant_id, resource_type, resource_id, created_at desc);

insert into approval_chains (tenant_id, name, description, resource_type, steps, enabled)
select t.id,
       '默认标书审批链',
       '部门主管 -> 项目经理的两级审批链，可在团队协作中调整。',
       'bid',
       '[
         {"order":1,"name":"部门主管审批","role_code":"department_admin","required":true,"condition":""},
         {"order":2,"name":"项目经理审批","role_code":"project_manager","required":true,"condition":""},
         {"order":3,"name":"总经理审批","role_code":"company_admin","required":false,"condition":"金额 > 100 万时启用"}
       ]'::jsonb,
       true
from tenants t
where not exists (
    select 1 from approval_chains ac
    where ac.tenant_id = t.id and ac.resource_type = 'bid'
);

alter table approval_chains enable row level security;
alter table approval_instances enable row level security;
alter table approval_actions enable row level security;
alter table comments enable row level security;
alter table approval_chains force row level security;
alter table approval_instances force row level security;
alter table approval_actions force row level security;
alter table comments force row level security;

create policy tenant_isolation_approval_chains on approval_chains
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_approval_instances on approval_instances
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_approval_actions on approval_actions
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_comments on comments
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists comments;
drop table if exists approval_actions;
drop table if exists approval_instances;
drop table if exists approval_chains;
