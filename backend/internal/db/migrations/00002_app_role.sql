-- +goose Up
-- +goose StatementBegin
do $$
begin
    if not exists (select 1 from pg_roles where rolname = 'zbt_app') then
        create role zbt_app login password 'zbt_app';
    end if;
end $$;
-- +goose StatementEnd

grant usage on schema public to zbt_app;
grant select, insert, update, delete on all tables in schema public to zbt_app;
grant usage, select on all sequences in schema public to zbt_app;
grant execute on all functions in schema public to zbt_app;

alter default privileges in schema public grant select, insert, update, delete on tables to zbt_app;
alter default privileges in schema public grant usage, select on sequences to zbt_app;
alter default privileges in schema public grant execute on functions to zbt_app;

-- +goose Down
revoke execute on all functions in schema public from zbt_app;
revoke usage, select on all sequences in schema public from zbt_app;
revoke select, insert, update, delete on all tables in schema public from zbt_app;
revoke usage on schema public from zbt_app;
drop role if exists zbt_app;
