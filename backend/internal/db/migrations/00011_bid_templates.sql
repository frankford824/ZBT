-- +goose Up
create table if not exists bid_templates (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    name text not null,
    bid_type text not null check (bid_type in ('combined', 'separated', 'custom')),
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

create index if not exists idx_bid_templates_tenant_created on bid_templates(tenant_id, created_at desc);
create index if not exists idx_bid_templates_tenant_type on bid_templates(tenant_id, bid_type);

insert into bid_templates (tenant_id, name, bid_type, category, description, version, content, usage_count)
select t.id, template.name, template.bid_type, template.category, template.description, template.version, template.content::jsonb, template.usage_count
from tenants t
cross join (values
    ('智慧交通技术标模板', 'separated', '技术标', '面向智慧交通、系统集成和软件平台项目的技术标结构模板。', 'v2.4', '{"sections":["项目理解","总体技术路线","系统架构设计","实施计划","质量保障","售后服务"],"default_parts":["技术标","商务标"]}', 126),
    ('商务响应标书模板', 'separated', '商务标', '覆盖资格证明、商务偏离、服务承诺和报价说明的商务响应模板。', 'v1.8', '{"sections":["投标函","资格证明","商务偏离表","服务承诺","报价说明"],"default_parts":["技术标","商务标"]}', 84),
    ('综合项目投标模板', 'combined', '综合标', '适用于不分册项目的综合投标文件，包含技术、商务和服务章节。', 'v3.1', '{"sections":["项目概况","响应方案","项目管理","商务条款","交付与验收"],"default_parts":["综合标书主体"]}', 158),
    ('定制化方案标书模板', 'custom', '定制模板', '为复杂项目预留自定义结构，便于后续扩展章节和材料引用。', 'v1.2', '{"sections":["需求澄清","定制方案","风险控制","附录材料"],"default_parts":["综合标书主体"]}', 31)
) as template(name, bid_type, category, description, version, content, usage_count)
on conflict (tenant_id, name, version) do nothing;

alter table bid_templates enable row level security;
alter table bid_templates force row level security;

create policy tenant_isolation_bid_templates on bid_templates
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists bid_templates;
