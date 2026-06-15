-- +goose Up
alter table role_permissions drop constraint if exists role_permissions_role_id_fkey;
alter table role_permissions
    add constraint role_permissions_role_id_fkey
    foreign key (role_id) references roles(id) on delete cascade;

alter table tenant_member_roles drop constraint if exists tenant_member_roles_role_id_fkey;
alter table tenant_member_roles
    add constraint tenant_member_roles_role_id_fkey
    foreign key (role_id) references roles(id) on delete cascade;

alter table module_permissions drop constraint if exists module_permissions_role_id_fkey;
alter table module_permissions
    add constraint module_permissions_role_id_fkey
    foreign key (role_id) references roles(id) on delete cascade;

-- +goose Down
alter table module_permissions drop constraint if exists module_permissions_role_id_fkey;
alter table module_permissions
    add constraint module_permissions_role_id_fkey
    foreign key (role_id) references roles(id);

alter table tenant_member_roles drop constraint if exists tenant_member_roles_role_id_fkey;
alter table tenant_member_roles
    add constraint tenant_member_roles_role_id_fkey
    foreign key (role_id) references roles(id);

alter table role_permissions drop constraint if exists role_permissions_role_id_fkey;
alter table role_permissions
    add constraint role_permissions_role_id_fkey
    foreign key (role_id) references roles(id);
