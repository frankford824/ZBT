-- +goose Up
create table if not exists cost_items (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    cost_project_id uuid not null references cost_projects(id) on delete cascade,
    category text not null default '其他',
    name text not null,
    cost_type text not null default 'other' check (cost_type in ('labor', 'material', 'equipment', 'service', 'other')),
    budget_amount numeric(14, 2) not null default 0,
    actual_amount numeric(14, 2) not null default 0,
    status text not null default 'planned' check (status in ('planned', 'committed', 'actual')),
    vendor text not null default '',
    note text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists cost_reports (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    cost_project_id uuid not null references cost_projects(id) on delete cascade,
    report_type text not null default 'summary',
    status text not null default 'generated' check (status in ('queued', 'generated', 'failed')),
    summary text not null default '',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_cost_items_project on cost_items(tenant_id, cost_project_id, category);
create index if not exists idx_cost_reports_project on cost_reports(tenant_id, cost_project_id, created_at desc);

insert into cost_projects (tenant_id, project_id, name, status, budget_amount, metadata)
select p.tenant_id, p.id, '智慧交通综合治理成本测算', 'active', 12800000.00, '{"source":"seed"}'
from projects p
where p.id = '40000000-0000-4000-8000-000000000001'
on conflict (tenant_id, project_id) do nothing;

insert into cost_items (tenant_id, cost_project_id, category, name, cost_type, budget_amount, actual_amount, status, vendor, note)
select cp.tenant_id, cp.id, item.category, item.name, item.cost_type, item.budget_amount, item.actual_amount, item.status, item.vendor, item.note
from cost_projects cp
cross join (values
    ('人力', '项目经理与实施团队', 'labor', 4200000.00, 3860000.00, 'actual', '内部交付中心', '含项目管理、开发实施和测试验收'),
    ('设备', '服务器与网络设备', 'equipment', 2100000.00, 1980000.00, 'committed', '杭州设备供应商', '含核心服务器、交换设备和安全设备'),
    ('服务', '三年运维服务', 'service', 1800000.00, 1560000.00, 'planned', '运维服务部', '含驻场、巡检、应急响应'),
    ('材料', '第三方软件授权', 'material', 1200000.00, 1320000.00, 'committed', '软件生态伙伴', '部分授权成本高于预算'),
    ('其他', '差旅与会务', 'other', 500000.00, 420000.00, 'actual', '', '项目沟通、培训和验收会议')
) as item(category, name, cost_type, budget_amount, actual_amount, status, vendor, note)
where cp.project_id = '40000000-0000-4000-8000-000000000001'
on conflict do nothing;

alter table cost_items enable row level security;
alter table cost_reports enable row level security;
alter table cost_items force row level security;
alter table cost_reports force row level security;

create policy tenant_isolation_cost_items on cost_items
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_cost_reports on cost_reports
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists cost_reports;
drop table if exists cost_items;
