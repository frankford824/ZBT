-- +goose Up
create table if not exists project_milestones (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    project_id uuid not null references projects(id) on delete cascade,
    title text not null,
    status text not null default 'pending' check (status in ('pending', 'done')),
    due_date date,
    completed_at timestamptz,
    sort_order integer not null default 0,
    note text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists project_members (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    project_id uuid not null references projects(id) on delete cascade,
    user_id uuid not null references users(id),
    role text not null default 'member',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, project_id, user_id)
);

create table if not exists project_logs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    project_id uuid not null references projects(id) on delete cascade,
    actor_user_id uuid references users(id),
    action text not null,
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now()
);

create table if not exists cost_projects (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    project_id uuid not null references projects(id),
    name text not null,
    status text not null default 'draft' check (status in ('draft', 'active', 'closed')),
    budget_amount numeric(14, 2),
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, project_id)
);

create index if not exists idx_project_milestones_project on project_milestones(tenant_id, project_id, sort_order, due_date);
create index if not exists idx_project_members_project on project_members(tenant_id, project_id);
create index if not exists idx_project_logs_project on project_logs(tenant_id, project_id, created_at desc);
create index if not exists idx_cost_projects_tenant_created on cost_projects(tenant_id, created_at desc);

insert into project_members (tenant_id, project_id, user_id, role)
select p.tenant_id, p.id, '10000000-0000-4000-8000-000000000002'::uuid, 'owner'
from projects p
where p.tenant_id = '00000000-0000-4000-8000-000000000001'
on conflict (tenant_id, project_id, user_id) do nothing;

insert into project_milestones (tenant_id, project_id, title, status, due_date, sort_order, note)
select p.tenant_id, p.id, milestone.title, milestone.status, milestone.due_date::date, milestone.sort_order, milestone.note
from projects p
cross join (values
    ('项目创建', 'done', '2026-06-01', 10, '项目机会已登记'),
    ('标书制作', 'pending', '2026-06-12', 20, '完成技术标和商务标初稿'),
    ('提交投标', 'pending', '2026-06-18', 30, '完成投标文件提交')
) as milestone(title, status, due_date, sort_order, note)
where p.id = '40000000-0000-4000-8000-000000000001'
on conflict do nothing;

insert into project_logs (tenant_id, project_id, actor_user_id, action, metadata)
select p.tenant_id, p.id, '10000000-0000-4000-8000-000000000001'::uuid, 'project.seeded', '{"source":"migration"}'
from projects p
where p.id = '40000000-0000-4000-8000-000000000001'
on conflict do nothing;

alter table project_milestones enable row level security;
alter table project_members enable row level security;
alter table project_logs enable row level security;
alter table cost_projects enable row level security;
alter table project_milestones force row level security;
alter table project_members force row level security;
alter table project_logs force row level security;
alter table cost_projects force row level security;

create policy tenant_isolation_project_milestones on project_milestones
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_project_members on project_members
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_project_logs on project_logs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_cost_projects on cost_projects
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists cost_projects;
drop table if exists project_logs;
drop table if exists project_members;
drop table if exists project_milestones;
