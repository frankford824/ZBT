-- +goose Up
create table if not exists bid_parts (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    code text not null check (code in ('combined_body', 'tech', 'business', 'boq', 'attachment')),
    title text not null,
    sort_order integer not null default 0,
    status text not null default 'draft',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, bid_document_id, code)
);

create table if not exists bid_chapters (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    bid_part_id uuid not null references bid_parts(id) on delete cascade,
    title text not null,
    content jsonb not null default '{}',
    plain_text text not null default '',
    status text not null default 'generated' check (status in ('pending', 'generating', 'generated', 'accepted', 'edited', 'needs_fix')),
    sort_order integer not null default 0,
    source_refs jsonb not null default '[]',
    needs_human_input jsonb not null default '[]',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists bid_exports (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    bid_part_id uuid references bid_parts(id) on delete set null,
    export_type text not null default 'docx' check (export_type in ('docx', 'pdf', 'zip')),
    part_code text not null default 'combined_body',
    status text not null default 'queued' check (status in ('queued', 'running', 'done', 'failed', 'cancelled')),
    file_asset_id uuid references file_assets(id),
    filename text not null,
    metadata jsonb not null default '{}',
    error_message text,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_bid_parts_tenant_bid on bid_parts(tenant_id, bid_document_id, sort_order);
create index if not exists idx_bid_chapters_tenant_part on bid_chapters(tenant_id, bid_part_id, sort_order);
create index if not exists idx_bid_exports_tenant_bid on bid_exports(tenant_id, bid_document_id, created_at desc);

insert into projects (id, tenant_id, name, status)
values ('40000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000001', '智慧交通综合治理平台建设', 'bidding')
on conflict (id) do update set name = excluded.name, status = excluded.status, updated_at = now();

insert into bid_documents (id, tenant_id, project_id, title, bid_type, status)
values (
    '50000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000001',
    '40000000-0000-4000-8000-000000000001',
    '智慧交通平台分离标书',
    'separated',
    'editing'
)
on conflict (id) do update set title = excluded.title, bid_type = excluded.bid_type, status = excluded.status, updated_at = now();

insert into bid_parts (id, tenant_id, bid_document_id, code, title, sort_order, status)
values
('51000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000001', '50000000-0000-4000-8000-000000000001', 'tech', '技术标', 10, 'generated'),
('51000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000001', '50000000-0000-4000-8000-000000000001', 'business', '商务标', 20, 'generated')
on conflict (tenant_id, bid_document_id, code) do update
set title = excluded.title, sort_order = excluded.sort_order, status = excluded.status, updated_at = now();

insert into bid_chapters (id, tenant_id, bid_document_id, bid_part_id, title, content, plain_text, status, sort_order, source_refs, needs_human_input)
values
(
    '52000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001',
    '51000000-0000-4000-8000-000000000001',
    '一、项目理解',
    '{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"本项目围绕城市交通治理、数据融合和智能研判能力建设展开。"}]}]}',
    '本项目围绕城市交通治理、数据融合和智能研判能力建设展开。',
    'accepted',
    10,
    '[]',
    '[]'
),
(
    '52000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001',
    '51000000-0000-4000-8000-000000000001',
    '二、实施方案',
    '{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"采用分阶段实施策略，覆盖需求调研、平台建设、联调测试、试运行和验收交付。"}]}]}',
    '采用分阶段实施策略，覆盖需求调研、平台建设、联调测试、试运行和验收交付。',
    'generated',
    20,
    '[]',
    '["需补充项目经理证书编号"]'
),
(
    '52000000-0000-4000-8000-000000000003',
    '00000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001',
    '51000000-0000-4000-8000-000000000002',
    '一、投标函',
    '{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"我方已充分理解招标文件要求，并承诺按合同约定完成交付。"}]}]}',
    '我方已充分理解招标文件要求，并承诺按合同约定完成交付。',
    'accepted',
    10,
    '[]',
    '[]'
),
(
    '52000000-0000-4000-8000-000000000004',
    '00000000-0000-4000-8000-000000000001',
    '50000000-0000-4000-8000-000000000001',
    '51000000-0000-4000-8000-000000000002',
    '二、商务响应',
    '{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"商务条款响应以招标文件合同条款为依据，报价与服务范围保持一致。"}]}]}',
    '商务条款响应以招标文件合同条款为依据，报价与服务范围保持一致。',
    'generated',
    20,
    '[]',
    '["需人工确认最终报价金额"]'
)
on conflict (id) do update
set title = excluded.title,
    content = excluded.content,
    plain_text = excluded.plain_text,
    status = excluded.status,
    sort_order = excluded.sort_order,
    source_refs = excluded.source_refs,
    needs_human_input = excluded.needs_human_input,
    updated_at = now();

alter table bid_parts enable row level security;
alter table bid_chapters enable row level security;
alter table bid_exports enable row level security;

alter table bid_parts force row level security;
alter table bid_chapters force row level security;
alter table bid_exports force row level security;

create policy tenant_isolation_bid_parts on bid_parts
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_bid_chapters on bid_chapters
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());
create policy tenant_isolation_bid_exports on bid_exports
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists bid_exports;
drop table if exists bid_chapters;
drop table if exists bid_parts;
