-- +goose Up
-- +goose StatementBegin
do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'cost_projects'::regclass
            and conname = 'cost_projects_budget_amount_nonnegative'
    ) then
        alter table cost_projects
            add constraint cost_projects_budget_amount_nonnegative
            check (budget_amount is null or budget_amount >= 0) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'cost_items'::regclass
            and conname = 'cost_items_budget_amount_nonnegative'
    ) then
        alter table cost_items
            add constraint cost_items_budget_amount_nonnegative
            check (budget_amount >= 0) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'cost_items'::regclass
            and conname = 'cost_items_actual_amount_nonnegative'
    ) then
        alter table cost_items
            add constraint cost_items_actual_amount_nonnegative
            check (actual_amount >= 0) not valid;
    end if;
end $$;
-- +goose StatementEnd

-- +goose Down
alter table cost_items drop constraint if exists cost_items_actual_amount_nonnegative;
alter table cost_items drop constraint if exists cost_items_budget_amount_nonnegative;
alter table cost_projects drop constraint if exists cost_projects_budget_amount_nonnegative;
