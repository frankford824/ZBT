-- +goose Up
update project_members
set role = 'member', updated_at = now()
where role not in ('owner', 'member', 'viewer');

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'project_members'::regclass
            and conname = 'project_members_role_check'
    ) then
        alter table project_members
            add constraint project_members_role_check
            check (role in ('owner', 'member', 'viewer'));
    end if;
end $$;

-- +goose Down
alter table project_members drop constraint if exists project_members_role_check;
