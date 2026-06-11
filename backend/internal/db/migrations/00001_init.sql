-- +goose Up
create extension if not exists pgcrypto;
create extension if not exists vector;

create or replace function current_tenant_id() returns uuid
language sql
stable
as $$
    select nullif(current_setting('app.tenant_id', true), '')::uuid
$$;

create table if not exists tenants (
    id uuid primary key default gen_random_uuid(),
    name text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists users (
    id uuid primary key default gen_random_uuid(),
    email text not null unique,
    name text not null,
    password_hash text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists tenant_members (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    user_id uuid not null references users(id),
    status text not null default 'active',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, user_id)
);

create table if not exists roles (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    code text not null,
    name text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, code)
);

create table if not exists permissions (
    code text primary key,
    description text not null
);

create table if not exists role_permissions (
    tenant_id uuid not null references tenants(id),
    role_id uuid not null references roles(id),
    permission_code text not null references permissions(code),
    created_at timestamptz not null default now(),
    primary key (tenant_id, role_id, permission_code)
);

create table if not exists tenant_member_roles (
    tenant_id uuid not null references tenants(id),
    tenant_member_id uuid not null references tenant_members(id) on delete cascade,
    role_id uuid not null references roles(id),
    created_at timestamptz not null default now(),
    primary key (tenant_id, tenant_member_id, role_id)
);

create table if not exists module_permissions (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    role_id uuid not null references roles(id),
    module text not null,
    level text not null check (level in ('none', 'read', 'full')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, role_id, module)
);

create table if not exists audit_logs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    user_id uuid references users(id),
    action text not null,
    resource_type text not null,
    resource_id uuid,
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now()
);

create table if not exists notifications (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    user_id uuid references users(id),
    title text not null,
    body text not null,
    read_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists file_assets (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    owner_user_id uuid references users(id),
    biz_type text not null,
    biz_id uuid,
    object_key text not null,
    filename text not null,
    content_type text not null,
    size_bytes bigint not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists ai_call_logs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    user_id uuid references users(id),
    trace_id text not null,
    task_type text not null,
    provider text not null,
    model text not null,
    input_tokens integer not null default 0,
    output_tokens integer not null default 0,
    estimated_cost numeric(12, 4) not null default 0,
    latency_ms integer not null default 0,
    status text not null,
    error_message text,
    fallback_from text,
    biz_ref jsonb not null default '{}',
    created_at timestamptz not null default now()
);

create table if not exists projects (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    name text not null,
    status text not null check (status in ('opportunity', 'bidding', 'compliance_review', 'submitted', 'closed')),
    result text check (result in ('won', 'lost', 'pending')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists bid_documents (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    project_id uuid references projects(id),
    title text not null,
    bid_type text not null check (bid_type in ('combined', 'separated', 'custom')),
    status text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists knowledge_chunks (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    document_id uuid,
    title text not null,
    content text not null,
    section_path text not null,
    page_start integer,
    page_end integer,
    metadata jsonb not null default '{}',
    embedding vector(1024),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

insert into permissions (code, description) values
('dashboard.view', '查看工作台'),
('tender.view', '查看标讯'),
('tender.manage', '管理标讯'),
('bid.view', '查看标书'),
('bid.create', '创建标书'),
('bid.edit', '编辑标书'),
('bid.delete', '删除标书'),
('bid.export', '导出标书'),
('compliance.view', '查看合规检查'),
('compliance.run', '运行合规检查'),
('project.view', '查看项目'),
('project.manage', '管理项目'),
('cost.view', '查看成本'),
('cost.manage', '管理成本'),
('knowledge.view', '查看知识库'),
('knowledge.manage', '管理知识库'),
('team.view', '查看团队'),
('team.manage', '管理团队'),
('approval.manage', '管理审批'),
('file.upload', '上传文件'),
('file.download', '下载文件'),
('ai.run', '运行 AI 任务')
on conflict (code) do nothing;

insert into tenants (id, name) values
('00000000-0000-4000-8000-000000000001', '杭州智建科技有限公司'),
('00000000-0000-4000-8000-000000000002', '上海样板建设集团')
on conflict (id) do update set name = excluded.name, updated_at = now();

insert into users (id, email, name, password_hash) values
('10000000-0000-4000-8000-000000000001', 'admin@zbt.local', '陈思远', crypt('demo-password', gen_salt('bf'))),
('10000000-0000-4000-8000-000000000002', 'pm@zbt.local', '林悦', crypt('demo-password', gen_salt('bf'))),
('10000000-0000-4000-8000-000000000003', 'bidder@zbt.local', '赵宁', crypt('demo-password', gen_salt('bf'))),
('10000000-0000-4000-8000-000000000004', 'viewer@zbt.local', '周明', crypt('demo-password', gen_salt('bf'))),
('10000000-0000-4000-8000-000000000005', 'other@zbt.local', '王远', crypt('demo-password', gen_salt('bf')))
on conflict (email) do update set name = excluded.name, updated_at = now();

insert into roles (id, tenant_id, code, name) values
('20000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000001', 'super_admin', '超级管理员'),
('20000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000001', 'company_admin', '企业管理员'),
('20000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000001', 'department_admin', '部门管理员'),
('20000000-0000-4000-8000-000000000004', '00000000-0000-4000-8000-000000000001', 'project_manager', '项目经理'),
('20000000-0000-4000-8000-000000000005', '00000000-0000-4000-8000-000000000001', 'bid_specialist', '投标专员'),
('20000000-0000-4000-8000-000000000006', '00000000-0000-4000-8000-000000000001', 'viewer', '查看者'),
('20000000-0000-4000-8000-000000000007', '00000000-0000-4000-8000-000000000002', 'company_admin', '企业管理员')
on conflict (tenant_id, code) do update set name = excluded.name, updated_at = now();

insert into module_permissions (tenant_id, role_id, module, level)
select '00000000-0000-4000-8000-000000000001'::uuid, role_id, module, level
from (values
('20000000-0000-4000-8000-000000000001'::uuid, 'dashboard', 'full'), ('20000000-0000-4000-8000-000000000001'::uuid, 'tender', 'full'), ('20000000-0000-4000-8000-000000000001'::uuid, 'bid', 'full'), ('20000000-0000-4000-8000-000000000001'::uuid, 'compliance', 'full'), ('20000000-0000-4000-8000-000000000001'::uuid, 'project', 'full'), ('20000000-0000-4000-8000-000000000001'::uuid, 'cost', 'full'), ('20000000-0000-4000-8000-000000000001'::uuid, 'knowledge', 'full'), ('20000000-0000-4000-8000-000000000001'::uuid, 'team', 'full'),
('20000000-0000-4000-8000-000000000002'::uuid, 'dashboard', 'full'), ('20000000-0000-4000-8000-000000000002'::uuid, 'tender', 'full'), ('20000000-0000-4000-8000-000000000002'::uuid, 'bid', 'full'), ('20000000-0000-4000-8000-000000000002'::uuid, 'compliance', 'full'), ('20000000-0000-4000-8000-000000000002'::uuid, 'project', 'full'), ('20000000-0000-4000-8000-000000000002'::uuid, 'cost', 'full'), ('20000000-0000-4000-8000-000000000002'::uuid, 'knowledge', 'full'), ('20000000-0000-4000-8000-000000000002'::uuid, 'team', 'full'),
('20000000-0000-4000-8000-000000000003'::uuid, 'dashboard', 'read'), ('20000000-0000-4000-8000-000000000003'::uuid, 'tender', 'full'), ('20000000-0000-4000-8000-000000000003'::uuid, 'bid', 'full'), ('20000000-0000-4000-8000-000000000003'::uuid, 'compliance', 'full'), ('20000000-0000-4000-8000-000000000003'::uuid, 'project', 'full'), ('20000000-0000-4000-8000-000000000003'::uuid, 'cost', 'read'), ('20000000-0000-4000-8000-000000000003'::uuid, 'knowledge', 'full'), ('20000000-0000-4000-8000-000000000003'::uuid, 'team', 'read'),
('20000000-0000-4000-8000-000000000004'::uuid, 'dashboard', 'read'), ('20000000-0000-4000-8000-000000000004'::uuid, 'tender', 'full'), ('20000000-0000-4000-8000-000000000004'::uuid, 'bid', 'full'), ('20000000-0000-4000-8000-000000000004'::uuid, 'compliance', 'full'), ('20000000-0000-4000-8000-000000000004'::uuid, 'project', 'full'), ('20000000-0000-4000-8000-000000000004'::uuid, 'cost', 'none'), ('20000000-0000-4000-8000-000000000004'::uuid, 'knowledge', 'read'), ('20000000-0000-4000-8000-000000000004'::uuid, 'team', 'none'),
('20000000-0000-4000-8000-000000000005'::uuid, 'dashboard', 'read'), ('20000000-0000-4000-8000-000000000005'::uuid, 'tender', 'read'), ('20000000-0000-4000-8000-000000000005'::uuid, 'bid', 'full'), ('20000000-0000-4000-8000-000000000005'::uuid, 'compliance', 'full'), ('20000000-0000-4000-8000-000000000005'::uuid, 'project', 'read'), ('20000000-0000-4000-8000-000000000005'::uuid, 'cost', 'none'), ('20000000-0000-4000-8000-000000000005'::uuid, 'knowledge', 'read'), ('20000000-0000-4000-8000-000000000005'::uuid, 'team', 'none'),
('20000000-0000-4000-8000-000000000006'::uuid, 'dashboard', 'read'), ('20000000-0000-4000-8000-000000000006'::uuid, 'tender', 'read'), ('20000000-0000-4000-8000-000000000006'::uuid, 'bid', 'read'), ('20000000-0000-4000-8000-000000000006'::uuid, 'compliance', 'read'), ('20000000-0000-4000-8000-000000000006'::uuid, 'project', 'read'), ('20000000-0000-4000-8000-000000000006'::uuid, 'cost', 'none'), ('20000000-0000-4000-8000-000000000006'::uuid, 'knowledge', 'read'), ('20000000-0000-4000-8000-000000000006'::uuid, 'team', 'none'),
('20000000-0000-4000-8000-000000000007'::uuid, 'dashboard', 'full'), ('20000000-0000-4000-8000-000000000007'::uuid, 'tender', 'full'), ('20000000-0000-4000-8000-000000000007'::uuid, 'bid', 'full'), ('20000000-0000-4000-8000-000000000007'::uuid, 'compliance', 'full'), ('20000000-0000-4000-8000-000000000007'::uuid, 'project', 'full'), ('20000000-0000-4000-8000-000000000007'::uuid, 'cost', 'full'), ('20000000-0000-4000-8000-000000000007'::uuid, 'knowledge', 'full'), ('20000000-0000-4000-8000-000000000007'::uuid, 'team', 'full')
) as matrix(role_id, module, level)
on conflict do nothing;

insert into tenant_members (id, tenant_id, user_id, status) values
('30000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', 'active'),
('30000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000002', 'active'),
('30000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000003', 'active'),
('30000000-0000-4000-8000-000000000004', '00000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000004', 'active'),
('30000000-0000-4000-8000-000000000005', '00000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000005', 'active')
on conflict (tenant_id, user_id) do update set status = excluded.status, updated_at = now();

insert into tenant_member_roles (tenant_id, tenant_member_id, role_id) values
('00000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000001', '20000000-0000-4000-8000-000000000002'),
('00000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000002', '20000000-0000-4000-8000-000000000004'),
('00000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000003', '20000000-0000-4000-8000-000000000005'),
('00000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000004', '20000000-0000-4000-8000-000000000006'),
('00000000-0000-4000-8000-000000000002', '30000000-0000-4000-8000-000000000005', '20000000-0000-4000-8000-000000000007')
on conflict do nothing;

insert into notifications (tenant_id, user_id, title, body) values
('00000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', '审批待处理', '智慧交通项目标书等待审批'),
('00000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000003', '合规检查完成', '技术标合规检查已生成待确认问题')
on conflict do nothing;

alter table tenants enable row level security;
alter table tenant_members enable row level security;
alter table roles enable row level security;
alter table role_permissions enable row level security;
alter table tenant_member_roles enable row level security;
alter table module_permissions enable row level security;
alter table audit_logs enable row level security;
alter table notifications enable row level security;
alter table file_assets enable row level security;
alter table ai_call_logs enable row level security;
alter table projects enable row level security;
alter table bid_documents enable row level security;
alter table knowledge_chunks enable row level security;

alter table tenants force row level security;
alter table tenant_members force row level security;
alter table roles force row level security;
alter table role_permissions force row level security;
alter table tenant_member_roles force row level security;
alter table module_permissions force row level security;
alter table audit_logs force row level security;
alter table notifications force row level security;
alter table file_assets force row level security;
alter table ai_call_logs force row level security;
alter table projects force row level security;
alter table bid_documents force row level security;
alter table knowledge_chunks force row level security;

create policy tenant_isolation_tenants on tenants
    using (id = current_tenant_id())
    with check (id = current_tenant_id());
create policy tenant_isolation_tenant_members on tenant_members
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_roles on roles
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_role_permissions on role_permissions
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_tenant_member_roles on tenant_member_roles
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_module_permissions on module_permissions
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_audit_logs on audit_logs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_notifications on notifications
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_file_assets on file_assets
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_ai_call_logs on ai_call_logs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_projects on projects
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_bid_documents on bid_documents
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_knowledge_chunks on knowledge_chunks
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists knowledge_chunks;
drop table if exists bid_documents;
drop table if exists projects;
drop table if exists ai_call_logs;
drop table if exists file_assets;
drop table if exists notifications;
drop table if exists audit_logs;
drop table if exists module_permissions;
drop table if exists tenant_member_roles;
drop table if exists role_permissions;
drop table if exists permissions;
drop table if exists roles;
drop table if exists tenant_members;
drop table if exists users;
drop table if exists tenants;
drop function if exists current_tenant_id();
