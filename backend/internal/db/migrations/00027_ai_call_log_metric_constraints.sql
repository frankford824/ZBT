-- +goose Up
-- +goose StatementBegin
do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'ai_call_logs'::regclass
            and conname = 'ai_call_logs_input_tokens_nonnegative'
    ) then
        alter table ai_call_logs
            add constraint ai_call_logs_input_tokens_nonnegative
            check (input_tokens >= 0) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'ai_call_logs'::regclass
            and conname = 'ai_call_logs_output_tokens_nonnegative'
    ) then
        alter table ai_call_logs
            add constraint ai_call_logs_output_tokens_nonnegative
            check (output_tokens >= 0) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'ai_call_logs'::regclass
            and conname = 'ai_call_logs_estimated_cost_nonnegative'
    ) then
        alter table ai_call_logs
            add constraint ai_call_logs_estimated_cost_nonnegative
            check (estimated_cost >= 0) not valid;
    end if;

    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'ai_call_logs'::regclass
            and conname = 'ai_call_logs_latency_ms_nonnegative'
    ) then
        alter table ai_call_logs
            add constraint ai_call_logs_latency_ms_nonnegative
            check (latency_ms >= 0) not valid;
    end if;
end $$;
-- +goose StatementEnd

-- +goose Down
alter table ai_call_logs drop constraint if exists ai_call_logs_latency_ms_nonnegative;
alter table ai_call_logs drop constraint if exists ai_call_logs_estimated_cost_nonnegative;
alter table ai_call_logs drop constraint if exists ai_call_logs_output_tokens_nonnegative;
alter table ai_call_logs drop constraint if exists ai_call_logs_input_tokens_nonnegative;
