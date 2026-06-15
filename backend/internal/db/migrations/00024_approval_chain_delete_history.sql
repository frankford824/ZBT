-- +goose Up
alter table approval_instances drop constraint if exists approval_instances_chain_id_fkey;
alter table approval_instances
    add constraint approval_instances_chain_id_fkey
    foreign key (chain_id) references approval_chains(id) on delete set null;

-- +goose Down
alter table approval_instances drop constraint if exists approval_instances_chain_id_fkey;
alter table approval_instances
    add constraint approval_instances_chain_id_fkey
    foreign key (chain_id) references approval_chains(id);
