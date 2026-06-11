-- +goose Up
create table if not exists bid_generation_jobs (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    scope text not null default 'full' check (scope in ('full', 'part', 'chapter')),
    status text not null default 'queued' check (status in ('queued', 'running', 'paused', 'done', 'failed', 'cancelled')),
    progress integer not null default 0 check (progress between 0 and 100),
    total_steps integer not null default 0,
    completed_steps integer not null default 0,
    failed_steps integer not null default 0,
    model_used text not null default '',
    prompt_tokens integer not null default 0,
    completion_tokens integer not null default 0,
    error_message text,
    trace_id text not null default '',
    created_by uuid references users(id),
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists bid_generation_steps (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    job_id uuid not null references bid_generation_jobs(id) on delete cascade,
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    bid_part_id uuid not null references bid_parts(id) on delete cascade,
    chapter_id uuid not null references bid_chapters(id) on delete cascade,
    step_order integer not null default 0,
    status text not null default 'queued' check (status in ('queued', 'running', 'paused', 'done', 'failed', 'cancelled')),
    ai_task_id uuid references ai_tasks(id),
    external_task_id text,
    error_message text,
    metadata jsonb not null default '{}',
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, job_id, chapter_id)
);

create index if not exists idx_bid_generation_jobs_tenant_bid on bid_generation_jobs(tenant_id, bid_document_id, created_at desc);
create index if not exists idx_bid_generation_jobs_tenant_status on bid_generation_jobs(tenant_id, status, updated_at desc);
create index if not exists idx_bid_generation_steps_job on bid_generation_steps(tenant_id, job_id, step_order);
create index if not exists idx_bid_generation_steps_task on bid_generation_steps(tenant_id, ai_task_id);

alter table bid_generation_jobs enable row level security;
alter table bid_generation_steps enable row level security;
alter table bid_generation_jobs force row level security;
alter table bid_generation_steps force row level security;

create policy tenant_isolation_bid_generation_jobs on bid_generation_jobs
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

create policy tenant_isolation_bid_generation_steps on bid_generation_steps
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists bid_generation_steps;
drop table if exists bid_generation_jobs;
