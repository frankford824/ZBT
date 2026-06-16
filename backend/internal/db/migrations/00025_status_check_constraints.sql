-- +goose Up
-- +goose StatementBegin
do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'tenant_members'::regclass
            and conname = 'tenant_members_status_check'
    ) then
        alter table tenant_members
            add constraint tenant_members_status_check
            check (status in ('active', 'invited', 'disabled')) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'ai_call_logs'::regclass
            and conname = 'ai_call_logs_status_check'
    ) then
        alter table ai_call_logs
            add constraint ai_call_logs_status_check
            check (status in ('queued', 'running', 'done', 'failed', 'cancelled')) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'bid_documents'::regclass
            and conname = 'bid_documents_status_check'
    ) then
        alter table bid_documents
            add constraint bid_documents_status_check
            check (status in ('draft', 'generating', 'editing', 'in_review', 'approved', 'submitted', 'archived')) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'bid_parts'::regclass
            and conname = 'bid_parts_status_check'
    ) then
        alter table bid_parts
            add constraint bid_parts_status_check
            check (status in ('draft', 'generated')) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'bid_chapter_versions'::regclass
            and conname = 'bid_chapter_versions_status_check'
    ) then
        alter table bid_chapter_versions
            add constraint bid_chapter_versions_status_check
            check (status in ('pending', 'generating', 'generated', 'accepted', 'edited', 'needs_fix')) not valid;
    end if;
end $$;
-- +goose StatementEnd

-- +goose Down
alter table bid_chapter_versions drop constraint if exists bid_chapter_versions_status_check;
alter table bid_parts drop constraint if exists bid_parts_status_check;
alter table bid_documents drop constraint if exists bid_documents_status_check;
alter table ai_call_logs drop constraint if exists ai_call_logs_status_check;
alter table tenant_members drop constraint if exists tenant_members_status_check;
