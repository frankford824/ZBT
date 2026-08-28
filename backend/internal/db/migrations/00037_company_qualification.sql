-- +goose Up
-- 企业资质档案。表与字段沿用 docs/ZBT-INTEGRATION-PLAN.md 2.2 节的设计，
-- 数据来源是局域网内的 zizhi-api（NAS 资质库检索服务），见 internal/company/qualification。
--
-- 四张表共用一组来源与确认字段：
--   source_ref         上游来源标识，形如 zizhi:file:122 / zizhi:person:27。
--                      资质原件的权威副本始终在公司 NAS 上，这里只记凭证，不落副本；
--                      file_asset_id 留给用户主动上传到对象存储的那部分。
--   verify_status      默认 confirmed 是给手工录入用的（人录即确认）；
--                      机器抽取路径必须显式写 pending_review。
--   extracted_by       zizhi 表示同步自资质库的 OCR 结果，与本地抽取、人工录入区分开。
--   extract_confidence 只用于待确认队列排序，不改变「必须人工确认」这个前提。
--   extract_evidence   抽取证据（原文件路径、分类、正文片段、警告），审核界面据此定位原件。

create table if not exists company_certificates (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    cert_category text not null default '',     -- 营业执照 / 安全生产许可证 / 建筑业企业资质 / 体系认证
    cert_name text not null,
    cert_level text not null default '',        -- 特级 / 一级 / 二级 / 三级
    -- 招标要求「一级及以上」而企业持「特级」，字符串比不出大小，必须归一化成可比整数。
    -- 映射由服务端确定性维护（qualification.NormalizeLevel），不交给模型判断。
    cert_level_rank integer not null default 0,
    cert_no text not null default '',
    issuer text not null default '',
    issued_at date,
    expires_at date,
    file_asset_id uuid references file_assets(id),
    metadata jsonb not null default '{}',
    source_ref text not null default '',
    verify_status text not null default 'confirmed'
        check (verify_status in ('pending_review', 'confirmed', 'rejected')),
    extracted_by text not null default 'manual'
        check (extracted_by in ('manual', 'ocr_llm', 'zizhi', 'case_archive')),
    extract_confidence numeric(3, 2),
    extract_evidence jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists company_performances (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    project_name text not null,
    owner_name text not null default '',
    project_type text not null default '',
    contract_amount numeric(14, 2),
    signed_at date,
    completed_at date,
    region text not null default '',
    keywords text[] not null default '{}',      -- 用于相似业绩判定
    file_asset_id uuid references file_assets(id),
    -- 中标项目回流时关联，复用已有的 archive-case 链路，业绩库可自动增长
    source_project_id uuid references projects(id),
    metadata jsonb not null default '{}',
    source_ref text not null default '',
    verify_status text not null default 'confirmed'
        check (verify_status in ('pending_review', 'confirmed', 'rejected')),
    extracted_by text not null default 'manual'
        check (extracted_by in ('manual', 'ocr_llm', 'zizhi', 'case_archive')),
    extract_confidence numeric(3, 2),
    extract_evidence jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists company_personnel (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    person_name text not null,
    cert_type text not null default '',         -- 注册建造师 / 专职安全生产管理人员 / 技工证
    cert_level text not null default '',        -- 一级 / 二级
    major text not null default '',             -- 市政公用 / 建筑工程 / 机电
    reg_no text not null default '',
    expires_at date,
    in_service boolean not null default true,
    file_asset_id uuid references file_assets(id),
    metadata jsonb not null default '{}',
    source_ref text not null default '',
    verify_status text not null default 'confirmed'
        check (verify_status in ('pending_review', 'confirmed', 'rejected')),
    extracted_by text not null default 'manual'
        check (extracted_by in ('manual', 'ocr_llm', 'zizhi', 'case_archive')),
    extract_confidence numeric(3, 2),
    extract_evidence jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists company_financials (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    fiscal_year integer not null,
    revenue numeric(16, 2),
    net_assets numeric(16, 2),
    net_profit numeric(16, 2),
    tax_paid numeric(16, 2),
    audit_file_asset_id uuid references file_assets(id),
    metadata jsonb not null default '{}',
    source_ref text not null default '',
    verify_status text not null default 'confirmed'
        check (verify_status in ('pending_review', 'confirmed', 'rejected')),
    extracted_by text not null default 'manual'
        check (extracted_by in ('manual', 'ocr_llm', 'zizhi', 'case_archive')),
    extract_confidence numeric(3, 2),
    extract_evidence jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, fiscal_year)
);

-- 同一份上游材料只应入档一次，否则每次同步都会把台账刷成多条。
-- 谓词必须与 qualification/sync.go 里 `on conflict (tenant_id, source_ref) where source_ref <> ''`
-- 逐字一致，否则 Postgres 匹配不到这个部分索引，upsert 会直接报错。
create unique index if not exists uq_company_certificates_source
    on company_certificates(tenant_id, source_ref) where source_ref <> '';
create unique index if not exists uq_company_performances_source
    on company_performances(tenant_id, source_ref) where source_ref <> '';
create unique index if not exists uq_company_personnel_source
    on company_personnel(tenant_id, source_ref) where source_ref <> '';
create unique index if not exists uq_company_financials_source
    on company_financials(tenant_id, source_ref) where source_ref <> '';

-- 到期预警只看已确认的记录：未经人工确认的抽取结果不该触发预警。
create index if not exists idx_company_certificates_expires
    on company_certificates(tenant_id, expires_at) where verify_status = 'confirmed';
create index if not exists idx_company_personnel_expires
    on company_personnel(tenant_id, expires_at) where verify_status = 'confirmed';

-- 待确认队列。做成部分索引是因为确认完成后这些行就不再被这条路径查询，
-- 全量索引会随台账一起增长却只有一小段被用到。
create index if not exists idx_company_certificates_review
    on company_certificates(tenant_id, verify_status) where verify_status = 'pending_review';
create index if not exists idx_company_performances_review
    on company_performances(tenant_id, verify_status) where verify_status = 'pending_review';
create index if not exists idx_company_personnel_review
    on company_personnel(tenant_id, verify_status) where verify_status = 'pending_review';

alter table company_certificates enable row level security;
alter table company_performances enable row level security;
alter table company_personnel enable row level security;
alter table company_financials enable row level security;
alter table company_certificates force row level security;
alter table company_performances force row level security;
alter table company_personnel force row level security;
alter table company_financials force row level security;

create policy tenant_isolation_company_certificates on company_certificates
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_company_performances on company_performances
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_company_personnel on company_personnel
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_company_financials on company_financials
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists company_financials;
drop table if exists company_personnel;
drop table if exists company_performances;
drop table if exists company_certificates;
