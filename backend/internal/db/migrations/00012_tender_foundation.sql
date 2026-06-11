-- +goose Up
create table if not exists tender_sources (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    name text not null,
    source_type text not null default '政府采购',
    url text not null,
    status text not null default 'active' check (status in ('active', 'inactive', 'failed')),
    last_verified_at timestamptz,
    last_verify_status text,
    last_verify_message text not null default '',
    config jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, name)
);

create table if not exists tenders (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    source_id uuid references tender_sources(id),
    title text not null,
    purchaser text not null default '',
    region text not null default '',
    budget_amount numeric(14, 2),
    budget_text text not null default '',
    publish_date date,
    deadline date,
    status text not null default 'open' check (status in ('open', 'closed', 'awarded', 'cancelled')),
    match_score integer not null default 0 check (match_score between 0 and 100),
    summary text not null default '',
    requirements text[] not null default '{}',
    risk_flags text[] not null default '{}',
    source_url text not null default '',
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists tender_user_states (
    tenant_id uuid not null references tenants(id),
    tender_id uuid not null references tenders(id) on delete cascade,
    user_id uuid not null references users(id),
    favorite boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (tenant_id, tender_id, user_id)
);

create table if not exists tender_parse_results (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    tender_id uuid not null references tenders(id) on delete cascade,
    status text not null default 'ready' check (status in ('queued', 'processing', 'ready', 'failed')),
    result jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_tenders_tenant_created on tenders(tenant_id, created_at desc);
create index if not exists idx_tenders_tenant_deadline on tenders(tenant_id, deadline);
create index if not exists idx_tenders_tenant_region on tenders(tenant_id, region);
create index if not exists idx_tender_user_states_favorite on tender_user_states(tenant_id, user_id, favorite);
create index if not exists idx_tender_sources_tenant_created on tender_sources(tenant_id, created_at desc);

insert into tender_sources (tenant_id, name, source_type, url, status, last_verify_status, last_verify_message, config)
select t.id, source.name, source.source_type, source.url, 'active', 'ok', 'seeded', source.config::jsonb
from tenants t
cross join (values
    ('中国招标投标公共服务平台', '公共平台', 'https://www.cebpubservice.com/', '{"keywords":["智慧城市","系统集成","运维服务"]}'),
    ('浙江政府采购网', '政府采购', 'https://zfcg.czt.zj.gov.cn/', '{"keywords":["交通治理","数据治理","软件平台"]}'),
    ('江苏省公共资源交易平台', '公共资源', 'https://jsggzy.jszwfw.gov.cn/', '{"keywords":["能耗监测","园区运维","工程建设"]}')
) as source(name, source_type, url, config)
on conflict (tenant_id, name) do nothing;

insert into tenders (
    tenant_id, source_id, title, purchaser, region, budget_amount, budget_text,
    publish_date, deadline, status, match_score, summary, requirements, risk_flags, source_url, metadata
)
select t.id, ts.id, tender.title, tender.purchaser, tender.region, tender.budget_amount, tender.budget_text,
    tender.publish_date::date, tender.deadline::date, 'open', tender.match_score, tender.summary,
    tender.requirements::text[], tender.risk_flags::text[], tender.source_url, tender.metadata::jsonb
from tenants t
cross join (values
    ('中国招标投标公共服务平台', '某市智慧交通综合治理平台建设项目', '某市交通运输局', '浙江', 12800000.00, '1,280万', '2026-06-01', '2026-06-18', 91, '建设智慧交通综合治理平台，覆盖数据接入、运行监测、事件处置和运维服务。', array['具备信息系统建设相关经验','技术方案覆盖数据安全与实施计划','提供三年运维服务承诺'], array['法定代表人签章缺失将导致废标','投标有效期不足将导致废标'], 'https://www.cebpubservice.com/tender/zbt-demo-traffic', '{"industry":"智慧交通"}'),
    ('浙江政府采购网', '县域政务数据治理与共享交换平台采购', '某县数据资源管理局', '浙江', 8600000.00, '860万', '2026-06-03', '2026-06-24', 88, '围绕政务数据目录、共享交换、数据质量和安全审计建设数据治理平台。', array['支持国产化部署','需提供数据治理方法论','需提交安全保障方案'], array['未提供同类案例可能影响评分'], 'https://zfcg.czt.zj.gov.cn/tender/zbt-demo-data', '{"industry":"数据治理"}'),
    ('江苏省公共资源交易平台', '园区能耗监测与运维服务采购', '苏州某产业园管理委员会', '江苏', 6400000.00, '640万', '2026-06-05', '2026-06-22', 86, '采购园区能耗监测、告警处置、报表分析和年度运维服务。', array['支持多能源计量接入','提供运维驻场服务','支持月度能耗分析报告'], array['报价明细不完整可能导致无效响应'], 'https://jsggzy.jszwfw.gov.cn/tender/zbt-demo-energy', '{"industry":"能源运维"}')
) as tender(source_name, title, purchaser, region, budget_amount, budget_text, publish_date, deadline, match_score, summary, requirements, risk_flags, source_url, metadata)
join tender_sources ts on ts.tenant_id = t.id and ts.name = tender.source_name
on conflict do nothing;

alter table tender_sources enable row level security;
alter table tenders enable row level security;
alter table tender_user_states enable row level security;
alter table tender_parse_results enable row level security;
alter table tender_sources force row level security;
alter table tenders force row level security;
alter table tender_user_states force row level security;
alter table tender_parse_results force row level security;

create policy tenant_isolation_tender_sources on tender_sources
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_tenders on tenders
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_tender_user_states on tender_user_states
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_tender_parse_results on tender_parse_results
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists tender_parse_results;
drop table if exists tender_user_states;
drop table if exists tenders;
drop table if exists tender_sources;
