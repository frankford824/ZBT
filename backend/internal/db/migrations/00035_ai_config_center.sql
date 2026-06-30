-- +goose Up
create table if not exists ai_model_configs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    enabled boolean not null default false,
    llm_provider text not null default 'openai_compatible_primary',
    llm_model text not null default 'gpt-4o-mini',
    embedding_provider text not null default 'openai_compatible_primary',
    embedding_model text not null default 'text-embedding-3-large',
    rerank_provider text not null default 'openai_compatible_primary',
    rerank_model text not null default 'gpt-4o-mini',
    ocr_provider text not null default 'http_ocr',
    ocr_endpoint text not null default '',
    monthly_budget numeric(12, 4) not null default 0,
    pricing jsonb not null default '{}',
    mock_fallback_allowed boolean not null default false,
    metadata jsonb not null default '{}',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id),
    check (monthly_budget >= 0),
    check (llm_provider in ('openai_compatible_primary', 'cloudflare_ai_gateway', 'deepseek', 'dashscope', 'mock', 'local')),
    check (embedding_provider in ('openai_compatible_primary', 'cloudflare_ai_gateway', 'deepseek', 'dashscope', 'mock', 'local')),
    check (rerank_provider in ('openai_compatible_primary', 'cloudflare_ai_gateway', 'deepseek', 'dashscope', 'mock', 'local')),
    check (ocr_provider in ('http_ocr', 'mineru', 'paddleocr', 'local', 'openai_compatible_primary', 'cloudflare_ai_gateway'))
);

create index if not exists idx_ai_model_configs_tenant
    on ai_model_configs(tenant_id);

alter table ai_model_configs enable row level security;
alter table ai_model_configs force row level security;

create policy tenant_isolation_ai_model_configs on ai_model_configs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists ai_model_configs;
