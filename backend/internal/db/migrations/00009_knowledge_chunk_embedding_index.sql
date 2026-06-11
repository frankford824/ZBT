-- +goose Up
create index if not exists idx_knowledge_chunks_embedding_hnsw
    on knowledge_chunks using hnsw (embedding vector_cosine_ops)
    where embedding is not null;

-- +goose Down
drop index if exists idx_knowledge_chunks_embedding_hnsw;
