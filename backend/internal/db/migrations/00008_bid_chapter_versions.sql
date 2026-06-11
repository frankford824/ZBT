-- +goose Up
alter table knowledge_references alter column source_document_id drop not null;

create table if not exists bid_chapter_versions (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    chapter_id uuid not null references bid_chapters(id) on delete cascade,
    bid_document_id uuid not null references bid_documents(id) on delete cascade,
    bid_part_id uuid not null references bid_parts(id) on delete cascade,
    version_no integer not null,
    title text not null,
    content jsonb not null default '{}',
    plain_text text not null default '',
    status text not null,
    source_refs jsonb not null default '[]',
    needs_human_input jsonb not null default '[]',
    change_reason text not null default '',
    model_metadata jsonb not null default '{}',
    token_usage jsonb not null default '{}',
    created_by uuid references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (tenant_id, chapter_id, version_no)
);

create index if not exists idx_bid_chapter_versions_tenant_chapter on bid_chapter_versions(tenant_id, chapter_id, version_no desc);
create index if not exists idx_knowledge_references_tenant_chapter on knowledge_references(tenant_id, chapter_id, created_at desc);

alter table bid_chapter_versions enable row level security;
alter table bid_chapter_versions force row level security;

create policy tenant_isolation_bid_chapter_versions on bid_chapter_versions
    using (tenant_id = current_tenant_id())
    with check (tenant_id = current_tenant_id());

-- +goose Down
drop table if exists bid_chapter_versions;
delete from knowledge_references where source_document_id is null;
alter table knowledge_references alter column source_document_id set not null;
