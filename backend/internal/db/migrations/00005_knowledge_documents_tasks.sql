-- +goose Up
create table if not exists knowledge_categories (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    parent_id uuid references knowledge_categories(id),
    name text not null,
    description text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, name)
);

create table if not exists knowledge_tags (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    name text not null,
    color text not null default 'blue',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, name)
);

create table if not exists knowledge_documents (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    file_asset_id uuid not null references file_assets(id),
    category_id uuid references knowledge_categories(id),
    title text not null,
    doc_type text not null default 'general',
    parse_status text not null default 'ready' check (parse_status in ('ready', 'queued', 'processing', 'processed', 'failed')),
    summary text not null default '',
    metadata jsonb not null default '{}',
    processed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, file_asset_id)
);

create table if not exists knowledge_document_tags (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    document_id uuid not null references knowledge_documents(id) on delete cascade,
    tag_id uuid not null references knowledge_tags(id) on delete cascade,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, document_id, tag_id)
);

create table if not exists knowledge_references (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    source_document_id uuid not null references knowledge_documents(id),
    bid_document_id uuid,
    chapter_id uuid,
    chunk_id uuid,
    title text not null default '',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists ai_tasks (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    user_id uuid references users(id),
    task_type text not null,
    status text not null default 'queued' check (status in ('queued', 'running', 'done', 'failed', 'cancelled')),
    external_task_id text,
    resource_type text not null,
    resource_id uuid not null,
    payload jsonb not null default '{}',
    route jsonb not null default '{}',
    result jsonb not null default '{}',
    error_message text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_knowledge_documents_tenant_created on knowledge_documents(tenant_id, created_at desc);
create index if not exists idx_knowledge_documents_tenant_status on knowledge_documents(tenant_id, parse_status);
create index if not exists idx_ai_tasks_tenant_resource on ai_tasks(tenant_id, resource_type, resource_id);
create unique index if not exists idx_ai_tasks_tenant_external on ai_tasks(tenant_id, external_task_id) where external_task_id is not null;

insert into knowledge_categories (tenant_id, name, description)
select t.id, category.name, category.description
from tenants t
cross join (values
    ('资质证书', '企业资质、许可、认证与有效期材料'),
    ('项目案例', '历史项目、中标案例和复用业绩'),
    ('技术方案', '技术架构、实施方案和解决方案素材'),
    ('合同模板', '合同、承诺函和商务模板')
) as category(name, description)
on conflict (tenant_id, name) do nothing;

insert into knowledge_tags (tenant_id, name, color)
select t.id, tag.name, tag.color
from tenants t
cross join (values
    ('资质证书', 'blue'),
    ('技术方案', 'green'),
    ('项目案例', 'orange')
) as tag(name, color)
on conflict (tenant_id, name) do nothing;

alter table knowledge_categories enable row level security;
alter table knowledge_tags enable row level security;
alter table knowledge_documents enable row level security;
alter table knowledge_document_tags enable row level security;
alter table knowledge_references enable row level security;
alter table ai_tasks enable row level security;

alter table knowledge_categories force row level security;
alter table knowledge_tags force row level security;
alter table knowledge_documents force row level security;
alter table knowledge_document_tags force row level security;
alter table knowledge_references force row level security;
alter table ai_tasks force row level security;

create policy tenant_isolation_knowledge_categories on knowledge_categories
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_knowledge_tags on knowledge_tags
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_knowledge_documents on knowledge_documents
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_knowledge_document_tags on knowledge_document_tags
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_knowledge_references on knowledge_references
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_ai_tasks on ai_tasks
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists ai_tasks;
drop table if exists knowledge_references;
drop table if exists knowledge_document_tags;
drop table if exists knowledge_documents;
drop table if exists knowledge_tags;
drop table if exists knowledge_categories;
