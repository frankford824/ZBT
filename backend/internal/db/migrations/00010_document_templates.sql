-- +goose Up
create table if not exists document_templates (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    name text not null,
    category text not null default '通用模板',
    description text not null default '',
    version text not null default 'v1.0',
    content jsonb not null default '{}',
    usage_count integer not null default 0,
    status text not null default 'active' check (status in ('active', 'archived')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, name, version)
);

create index if not exists idx_document_templates_tenant_created on document_templates(tenant_id, created_at desc);
create index if not exists idx_document_templates_tenant_category on document_templates(tenant_id, category);

insert into document_templates (tenant_id, name, category, description, version, content, usage_count)
select t.id, template.name, template.category, template.description, template.version, template.content::jsonb, template.usage_count
from tenants t
cross join (values
    ('项目实施方案模板.docx', '方案模板', '技术标项目实施方案结构模板，含项目理解、总体架构、实施计划和保障措施。', 'v3.0', '{"sections":["项目理解","总体架构","实施计划","质量保障"]}', 89),
    ('售后服务承诺模板.docx', '服务模板', '商务标售后服务承诺与响应时效模板。', 'v2.1', '{"sections":["服务承诺","响应机制","人员安排","备件保障"]}', 42),
    ('数据安全响应模板.docx', '制度模板', '数据安全、权限控制、日志审计和应急响应模板。', 'v1.4', '{"sections":["安全目标","权限模型","日志审计","应急响应"]}', 37)
) as template(name, category, description, version, content, usage_count)
on conflict (tenant_id, name, version) do nothing;

alter table document_templates enable row level security;
alter table document_templates force row level security;

create policy tenant_isolation_document_templates on document_templates
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists document_templates;
