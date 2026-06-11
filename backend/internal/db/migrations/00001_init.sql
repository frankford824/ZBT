-- +goose Up
create extension if not exists pgcrypto;
create extension if not exists vector;

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
    tenant_id uuid references tenants(id),
    code text not null,
    name text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists permissions (
    code text primary key,
    description text not null
);

create table if not exists role_permissions (
    role_id uuid not null references roles(id),
    permission_code text not null references permissions(code),
    created_at timestamptz not null default now(),
    primary key (role_id, permission_code)
);

create table if not exists module_permissions (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    role_id uuid not null references roles(id),
    module text not null,
    level text not null check (level in ('none', 'read', 'full')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
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

alter table tenant_members enable row level security;
alter table roles enable row level security;
alter table module_permissions enable row level security;
alter table audit_logs enable row level security;
alter table notifications enable row level security;
alter table file_assets enable row level security;
alter table ai_call_logs enable row level security;
alter table projects enable row level security;
alter table bid_documents enable row level security;
alter table knowledge_chunks enable row level security;

create policy tenant_isolation_tenant_members on tenant_members
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_roles on roles
    using (tenant_id is null or tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_module_permissions on module_permissions
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_audit_logs on audit_logs
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_notifications on notifications
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_file_assets on file_assets
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_ai_call_logs on ai_call_logs
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_projects on projects
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_bid_documents on bid_documents
    using (tenant_id::text = current_setting('app.tenant_id', true));
create policy tenant_isolation_knowledge_chunks on knowledge_chunks
    using (tenant_id::text = current_setting('app.tenant_id', true));

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

-- +goose Down
drop table if exists knowledge_chunks;
drop table if exists bid_documents;
drop table if exists projects;
drop table if exists ai_call_logs;
drop table if exists file_assets;
drop table if exists notifications;
drop table if exists audit_logs;
drop table if exists module_permissions;
drop table if exists role_permissions;
drop table if exists permissions;
drop table if exists roles;
drop table if exists tenant_members;
drop table if exists users;
drop table if exists tenants;
