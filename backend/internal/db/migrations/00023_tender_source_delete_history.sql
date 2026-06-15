-- +goose Up
alter table tenders drop constraint if exists tenders_source_id_fkey;
alter table tenders
    add constraint tenders_source_id_fkey
    foreign key (source_id) references tender_sources(id) on delete set null;

-- +goose Down
alter table tenders drop constraint if exists tenders_source_id_fkey;
alter table tenders
    add constraint tenders_source_id_fkey
    foreign key (source_id) references tender_sources(id);
