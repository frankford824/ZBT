#!/usr/bin/env python3
"""Rotate only default-password accounts; provision a separate release tester.

Run on ECS. Credentials are written to an owner-only file, never stdout.
Existing non-default user passwords are not changed.
"""
import json
import os
from pathlib import Path
import secrets
import subprocess


def sql(statement):
    return subprocess.check_output(
        ["docker", "exec", "-i", "zbt-dev-db", "psql", "-U", "zbt", "-d", "zbt", "-At", "-v", "ON_ERROR_STOP=1"],
        input=statement.encode(),
    ).decode().strip()


def main():
    directory = Path("/opt/zbt-private")
    directory.mkdir(mode=0o700, exist_ok=True)
    os.chmod(directory, 0o700)
    path = directory / "access.json"
    credentials = json.loads(path.read_text()) if path.exists() else {"accounts": {}}
    seed_accounts = "'admin@zbt.local','pm@zbt.local','bidder@zbt.local','viewer@zbt.local','other@zbt.local'"
    emails = sql("select email from users where email in (" + seed_accounts + ") and password_hash=crypt('demo-password',password_hash);").splitlines()
    for email in emails:
        password = secrets.token_urlsafe(32)
        # Email is DB-sourced but still escaped; passwords use a SQL-safe alphabet.
        sql("update users set password_hash=crypt('%s',gen_salt('bf')),updated_at=now() where email='%s' and password_hash=crypt('demo-password',password_hash);" % (password, email.replace("'", "''")))
        credentials["accounts"][email] = password
        # Persist after each rotation so an interruption cannot lose a password.
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        with os.fdopen(descriptor, "w") as output:
            json.dump(credentials, output, indent=2)
            output.flush()
            os.fsync(output.fileno())
    email = "release-smoke@zbt.local"
    password = credentials.get("smoke_password") or secrets.token_urlsafe(32)
    sql("""begin;
        insert into users(email,name,password_hash) values('%s','Release acceptance',crypt('%s',gen_salt('bf')))
        on conflict(email) do update set password_hash=excluded.password_hash;
        insert into tenant_members(tenant_id,user_id,status)
        select '00000000-0000-4000-8000-000000000001',id,'active' from users where email='%s'
        on conflict(tenant_id,user_id) do nothing;
        insert into tenant_member_roles(tenant_id,tenant_member_id,role_id)
        select tm.tenant_id,tm.id,r.id from tenant_members tm join users u on u.id=tm.user_id
        join roles r on r.tenant_id=tm.tenant_id and r.code='company_admin'
        where u.email='%s' and tm.tenant_id='00000000-0000-4000-8000-000000000001'
        on conflict do nothing;
        commit;""" % (email, password, email, email))
    credentials.update(smoke_email=email, smoke_password=password)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(descriptor, "w") as output:
        json.dump(credentials, output, indent=2)
    assert sql("select count(*) from users where email in (" + seed_accounts + ") and password_hash=crypt('demo-password',password_hash);") == "0"
    print(json.dumps({"default_accounts_rotated": len(emails), "smoke_account": email, "credentials_path": str(path)}))


if __name__ == "__main__":
    main()
