-- +goose Up
-- platform_tenders 与 platform_collector_runs 是本库唯一两张【故意不启用 RLS】的业务表，不是遗漏。
-- 原因：
--   1. 两张表只存公开政府采购公告与采集器运行日志，不含任何租户数据，因此没有 tenant_id 可供隔离；
--   2. 写入方是平台级采集器，走 HMAC 签名的 POST /api/v1/platform/tenders/ingest，
--      该请求不携带租户上下文，无法设置 current_tenant_id()，套 RLS 只会让写入永久失败；
--   3. 租户可见的标讯仍然只落在 tenders（强 RLS），公共池只是匹配任务的输入来源。
-- 若将来这两张表需要承载任何与单个租户相关的数据，必须先补 tenant_id 与 RLS 策略再写入。
create table if not exists platform_tenders (
    id uuid primary key default gen_random_uuid(),
    external_source text not null check (external_source in ('zbcg', 'iccec')),
    external_id text not null,
    title text not null,
    purchaser text not null default '',
    region text not null default '',
    notice_type_name text not null default '',
    publish_date date,
    deadline date,
    source_url text not null default '',
    budget_text text not null default '',
    budget_amount numeric(14, 2),
    raw_content text not null default '',
    requirement_dims jsonb not null default '{}',
    timeline jsonb not null default '{}',
    attachments jsonb not null default '{}',
    review_result jsonb not null default '{}',
    risk_flags text[] not null default '{}',
    status text not null default 'open' check (status in ('open', 'closed', 'awarded', 'cancelled')),
    collected_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (external_source, external_id)
);

create index if not exists idx_platform_tenders_collected on platform_tenders(collected_at desc);
create index if not exists idx_platform_tenders_deadline on platform_tenders(deadline) where status = 'open';

create table if not exists platform_collector_runs (
    id uuid primary key default gen_random_uuid(),
    external_source text not null,
    status text not null check (status in ('ok', 'partial', 'failed', 'blocked')),
    fetched_count integer not null default 0,
    ingested_count integer not null default 0,
    message text not null default '',
    started_at timestamptz not null,
    finished_at timestamptz not null default now()
);

create index if not exists idx_collector_runs_source on platform_collector_runs(external_source, finished_at desc);

alter table tenders add column if not exists platform_tender_id uuid references platform_tenders(id);
alter table tenders add column if not exists match_detail jsonb not null default '{}';

create unique index if not exists uq_tenders_tenant_platform
    on tenders(tenant_id, platform_tender_id) where platform_tender_id is not null;

-- +goose Down
drop index if exists uq_tenders_tenant_platform;
alter table tenders drop column if exists match_detail;
alter table tenders drop column if exists platform_tender_id;
drop table if exists platform_collector_runs;
drop table if exists platform_tenders;
