-- +goose Up
create index if not exists idx_knowledge_chunks_tenant_document on knowledge_chunks(tenant_id, document_id);
create index if not exists idx_knowledge_chunks_tenant_created on knowledge_chunks(tenant_id, created_at desc);
create index if not exists idx_knowledge_chunks_search on knowledge_chunks
    using gin (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(content, '') || ' ' || coalesce(section_path, '')));

-- +goose Down
drop index if exists idx_knowledge_chunks_search;
drop index if exists idx_knowledge_chunks_tenant_created;
drop index if exists idx_knowledge_chunks_tenant_document;
