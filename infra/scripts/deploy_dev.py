#!/usr/bin/env python3
"""Immutable ECS releases, preflight, consistent backup and verified rollback.

Schema changes deliberately require separate migration review: image rollback
must never pretend it can undo an incompatible database migration.
"""
from __future__ import annotations
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.request

from backup_dev import ACTIVE, active_directory, compose, rehearse, snapshot


def execute(args, **kwargs):
    subprocess.run(args, check=True, **kwargs)


def health():
    for port in (18080, 18000, 18001, 8080):
        url = f"http://127.0.0.1:{port}/" + ("healthz" if port != 8080 else "")
        for attempt in range(90):
            try:
                with urllib.request.urlopen(url, timeout=5) as response:
                    if response.status == 200:
                        break
            except Exception:
                if attempt == 89:
                    raise RuntimeError(f"service on port {port} failed health check") from None
                time.sleep(2)


def migrations(directory):
    root = directory / "backend/internal/db/migrations"
    return {path.name: path.read_bytes() for path in root.glob("*.sql")}


# Explicitly reviewed additive, read-only authentication helper. It changes no
# tables, passwords, RLS policies or memberships; old images can ignore it after
# rollback. Any other migration or change to this exact SQL remains blocked.
REVIEWED_ADDITIVE_MIGRATIONS = {
    "00038_authenticated_login_tenants.sql": "4f5f82ed02621f4fee99ce7060cf438d2388e7e15b368b801eaaca3935f10fac",
}


def migration_compatible(previous, candidate):
    return (all(candidate.get(name) == content for name, content in previous.items())
            and all(REVIEWED_ADDITIVE_MIGRATIONS.get(name) == hashlib.sha256(content).hexdigest()
                    for name, content in candidate.items() if name not in previous))


def tester_accounts():
    # No passwords or account data are logged. Exclude only our dedicated
    # release tester, whose bcrypt hash is refreshed by its provisioning step.
    statement = """select coalesce(jsonb_object_agg(u.id::text,
        jsonb_build_object('password_fingerprint', md5(u.password_hash),
            'memberships', (select coalesce(jsonb_agg(jsonb_build_object(
                'tenant', tm.tenant_id, 'status', tm.status) order by tm.tenant_id), '[]')
                from tenant_members tm where tm.user_id=u.id))), '{}')
        from users u where u.email <> 'release-smoke@zbt.local';"""
    return json.loads(subprocess.check_output([
        'docker', 'exec', 'zbt-dev-db', 'psql', '-U', 'zbt', '-d', 'zbt', '-At', '-c', statement], text=True))


def deploy(directory, force_failure=False):
    os.umask(0o077)
    private = Path("/opt/zbt-private")
    private.mkdir(mode=0o700, exist_ok=True)
    with (private / "deploy.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        previous = active_directory()
        if not migration_compatible(migrations(previous), migrations(directory)):
            raise RuntimeError("Database migrations changed: review migration/rollback compatibility before deployment")
        new_command = compose(directory)
        old_command = compose(previous)
        execute(new_command + ["config", "--quiet"])
        if not force_failure:
            execute(new_command + ["build", "backend", "ai-service", "frontend"])
            config = json.loads(subprocess.check_output(new_command + ["config", "--format", "json"]))
            # Tests must NEVER mount the live task queue or use live credentials.
            execute(["docker", "run", "--rm", "--network", "none", "--entrypoint", "python",
                     config["services"]["ai-service"]["image"], "-m", "pytest", "-q"])
            # Real-model negative controls run in an isolated candidate container
            # BEFORE stopping public services. No live queue or database mounts.
            evidence_dir = private / "release-evidence"
            evidence_dir.mkdir(mode=0o700, exist_ok=True)
            execute(["docker", "run", "--rm", "--entrypoint", "python",
                     "--env-file", str(directory / ".env"),
                     "-v", str(directory / "ai-service/app/config/model_routing.btjs.yaml") + ":/app/app/config/model_routing.yaml:ro",
                     "-v", str(evidence_dir) + ":/evidence",
                     config["services"]["ai-service"]["image"],
                     "-m", "app.evaluation.evidence_canary_eval", "--output", "/evidence/evidence-canary.json"])
        # For the first move away from the old in-memory worker, refuse to kill
        # active user work. New versions have a persistent queue for restart.
        pending = subprocess.check_output(["docker", "exec", "zbt-dev-db", "psql", "-U", "zbt", "-d", "zbt", "-At", "-c", "select count(*) from ai_tasks where status in ('queued','running');"], text=True).strip()
        if pending != "0":
            raise RuntimeError("AI tasks are still active; let them complete before deployment")
        backup = snapshot(previous)
        existing_accounts = tester_accounts()
        try:
            execute(new_command + ["up", "-d", "--no-build", "--remove-orphans"])
            health()
            if force_failure:
                raise RuntimeError("intentional rollback rehearsal")
            execute(["python3", str(directory / "infra/scripts/protect_dev_accounts.py")])
            credentials = json.loads((private / "access.json").read_text())
            environment = {**os.environ, "ZBT_SMOKE_EMAIL": credentials["smoke_email"], "ZBT_SMOKE_PASSWORD": credentials["smoke_password"]}
            execute(new_command + ["exec", "-T", "ai-service", "python", "-m", "app.evaluation.ocr_bridge_smoke"])
            execute(["python3", str(directory / "infra/scripts/full_bid_smoke.py"), "--base-url", "http://127.0.0.1:8080/api/v1", "--bid-type", "separated"], env=environment)
            execute(["python3", str(directory / "infra/scripts/gray_acceptance.py")], env=environment)
            execute(new_command + ["exec", "-T", "postgres", "pg_isready", "-U", "zbt", "-d", "zbt"])
            execute(new_command + ["exec", "-T", "redis", "redis-cli", "ping"])
            execute(["docker", "volume", "inspect", "zbt-dev_postgres_data", "zbt-dev_minio_data", "zbt-dev_ai_tasks_data"], stdout=subprocess.DEVNULL)
            current_accounts = tester_accounts()
            if any(current_accounts.get(user) != fingerprint for user, fingerprint in existing_accounts.items()):
                raise RuntimeError('Existing tester passwords or memberships changed during acceptance; investigate before release')
            rehearse(backup)
            state = {"directory": str(directory), "pre_deploy_backup": str(backup), "verified_at": time.time(),
                     "preserved_account_count": len(existing_accounts)}
            temporary = ACTIVE.with_suffix(".tmp")
            temporary.write_text(json.dumps(state, indent=2))
            temporary.replace(ACTIVE)
            print(json.dumps({"deployment": "passed", **state}))
        except Exception:
            # Save bounded diagnostics, then restore images/config only. Never
            # overwrite live database writes with a snapshot automatically.
            with (backup / "failed-release.log").open("wb") as output:
                subprocess.run(new_command + ["logs", "--tail", "80", "backend", "ai-service", "ocr-service"], stdout=output, stderr=subprocess.STDOUT)
            execute(old_command + ["up", "-d", "--no-build", "--remove-orphans"])
            health()
            evidence = {"rollback": "passed", "restored_release": str(previous), "candidate": str(directory), "database_restored": False}
            (backup / "rollback-evidence.json").write_text(json.dumps(evidence, indent=2))
            print(json.dumps(evidence))
            if not force_failure:
                raise


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--release-dir", required=True, type=Path)
    parser.add_argument("--rehearse-rollback", action="store_true")
    arguments = parser.parse_args()
    directory = arguments.release_dir.resolve()
    if directory.parent != Path("/opt/zbt/releases") or not (directory / ".env").is_file():
        raise ValueError("release must be a configured child of /opt/zbt/releases")
    deploy(directory, arguments.rehearse_rollback)
