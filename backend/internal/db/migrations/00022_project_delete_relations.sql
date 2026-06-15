-- +goose Up
alter table bid_documents drop constraint if exists bid_documents_project_id_fkey;
alter table bid_documents
    add constraint bid_documents_project_id_fkey
    foreign key (project_id) references projects(id) on delete set null;

alter table cost_projects drop constraint if exists cost_projects_project_id_fkey;
alter table cost_projects
    add constraint cost_projects_project_id_fkey
    foreign key (project_id) references projects(id) on delete cascade;

-- +goose Down
alter table cost_projects drop constraint if exists cost_projects_project_id_fkey;
alter table cost_projects
    add constraint cost_projects_project_id_fkey
    foreign key (project_id) references projects(id);

alter table bid_documents drop constraint if exists bid_documents_project_id_fkey;
alter table bid_documents
    add constraint bid_documents_project_id_fkey
    foreign key (project_id) references projects(id);
