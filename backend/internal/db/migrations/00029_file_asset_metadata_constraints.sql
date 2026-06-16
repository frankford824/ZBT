-- +goose Up
-- +goose StatementBegin
do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'file_assets'::regclass
            and conname = 'file_assets_size_bytes_nonnegative'
    ) then
        alter table file_assets
            add constraint file_assets_size_bytes_nonnegative
            check (size_bytes >= 0) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'file_assets'::regclass
            and conname = 'file_assets_content_type_safe'
    ) then
        alter table file_assets
            add constraint file_assets_content_type_safe
            check (
                length(btrim(content_type)) > 0
                and octet_length(content_type) <= 255
                and content_type !~ '[[:cntrl:]]'
            ) not valid;
    end if;
end $$;
-- +goose StatementEnd

-- +goose Down
alter table file_assets drop constraint if exists file_assets_content_type_safe;
alter table file_assets drop constraint if exists file_assets_size_bytes_nonnegative;
