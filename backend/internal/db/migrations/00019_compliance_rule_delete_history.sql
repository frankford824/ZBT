-- +goose Up
alter table compliance_issues drop constraint if exists compliance_issues_rule_id_fkey;
alter table compliance_issues
    add constraint compliance_issues_rule_id_fkey
    foreign key (rule_id) references compliance_rules(id) on delete set null;

-- +goose Down
alter table compliance_issues drop constraint if exists compliance_issues_rule_id_fkey;
alter table compliance_issues
    add constraint compliance_issues_rule_id_fkey
    foreign key (rule_id) references compliance_rules(id);
