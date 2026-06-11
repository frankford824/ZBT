-- +goose Up
create table if not exists compliance_rules (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    code text not null,
    name text not null,
    category text not null,
    level text not null check (level in ('L1', 'L2', 'L3', 'L4')),
    severity text not null check (severity in ('pass', 'warn', 'fail_candidate', 'fail')),
    description text not null default '',
    enabled boolean not null default true,
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, code)
);

create table if not exists compliance_checks (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid references bid_documents(id),
    name text not null,
    status text not null default 'queued' check (status in ('queued', 'running', 'done', 'failed')),
    result_status text not null default 'pass' check (result_status in ('pass', 'warn', 'fail_candidate', 'fail')),
    score integer not null default 100 check (score between 0 and 100),
    config jsonb not null default '{}',
    task_id text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists compliance_issues (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    check_id uuid not null references compliance_checks(id) on delete cascade,
    rule_id uuid references compliance_rules(id),
    category text not null,
    severity text not null check (severity in ('pass', 'warn', 'fail_candidate', 'fail')),
    status text not null default 'open' check (status in ('open', 'fixed', 'ignored', 'confirmed_fail')),
    title text not null,
    evidence text not null default '',
    suggestion text not null default '',
    location jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists compliance_reports (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    check_id uuid not null references compliance_checks(id) on delete cascade,
    status text not null default 'generated' check (status in ('queued', 'generated', 'failed')),
    summary text not null default '',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists compliance_fix_logs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    issue_id uuid not null references compliance_issues(id) on delete cascade,
    action text not null,
    status text not null default 'done' check (status in ('queued', 'done', 'failed')),
    message text not null default '',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now()
);

create index if not exists idx_compliance_checks_tenant_created on compliance_checks(tenant_id, created_at desc);
create index if not exists idx_compliance_issues_check on compliance_issues(tenant_id, check_id, severity);
create index if not exists idx_compliance_reports_check on compliance_reports(tenant_id, check_id, created_at desc);
create index if not exists idx_compliance_fix_logs_issue on compliance_fix_logs(tenant_id, issue_id, created_at desc);

insert into compliance_rules (tenant_id, code, name, category, level, severity, description, metadata)
select t.id, rule.code, rule.name, rule.category, rule.level, rule.severity, rule.description, rule.metadata::jsonb
from tenants t
cross join (values
    ('signature_required', '签章完整性', '签章', 'L1', 'fail', '法定代表人签章、单位盖章缺失会导致确定性废标风险。', '{"deterministic":true}'),
    ('validity_days', '投标有效期', '废标条款', 'L3', 'fail', '投标有效期必须满足招标文件要求。', '{"deterministic":true,"minimum_days":90}'),
    ('response_time_semantic', '服务响应承诺一致性', '语义一致性', 'L2', 'fail_candidate', '服务响应时间等承诺与招标要求不一致时需人工确认。', '{"deterministic":false}'),
    ('catalog_page_number', '目录页码格式', '格式规范', 'L1', 'warn', '目录、页码、标题层级等格式问题需要在导出前确认。', '{"deterministic":true}'),
    ('score_optimization', '评分点优化建议', '评分标准', 'L4', 'warn', '评分优化默认 beta，可辅助补强方案亮点。', '{"beta":true}')
) as rule(code, name, category, level, severity, description, metadata)
on conflict (tenant_id, code) do nothing;

alter table compliance_rules enable row level security;
alter table compliance_checks enable row level security;
alter table compliance_issues enable row level security;
alter table compliance_reports enable row level security;
alter table compliance_fix_logs enable row level security;
alter table compliance_rules force row level security;
alter table compliance_checks force row level security;
alter table compliance_issues force row level security;
alter table compliance_reports force row level security;
alter table compliance_fix_logs force row level security;

create policy tenant_isolation_compliance_rules on compliance_rules
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_compliance_checks on compliance_checks
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_compliance_issues on compliance_issues
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_compliance_reports on compliance_reports
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_compliance_fix_logs on compliance_fix_logs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists compliance_fix_logs;
drop table if exists compliance_reports;
drop table if exists compliance_issues;
drop table if exists compliance_checks;
drop table if exists compliance_rules;
