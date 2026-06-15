-- +goose Up
alter table knowledge_categories drop constraint if exists knowledge_categories_parent_id_fkey;
alter table knowledge_categories
    add constraint knowledge_categories_parent_id_fkey
    foreign key (parent_id) references knowledge_categories(id) on delete set null;

alter table knowledge_documents drop constraint if exists knowledge_documents_category_id_fkey;
alter table knowledge_documents
    add constraint knowledge_documents_category_id_fkey
    foreign key (category_id) references knowledge_categories(id) on delete set null;

alter table knowledge_references drop constraint if exists knowledge_references_source_document_id_fkey;
alter table knowledge_references
    add constraint knowledge_references_source_document_id_fkey
    foreign key (source_document_id) references knowledge_documents(id) on delete set null;

-- +goose Down
alter table knowledge_references drop constraint if exists knowledge_references_source_document_id_fkey;
alter table knowledge_references
    add constraint knowledge_references_source_document_id_fkey
    foreign key (source_document_id) references knowledge_documents(id);

alter table knowledge_documents drop constraint if exists knowledge_documents_category_id_fkey;
alter table knowledge_documents
    add constraint knowledge_documents_category_id_fkey
    foreign key (category_id) references knowledge_categories(id);

alter table knowledge_categories drop constraint if exists knowledge_categories_parent_id_fkey;
alter table knowledge_categories
    add constraint knowledge_categories_parent_id_fkey
    foreign key (parent_id) references knowledge_categories(id);
