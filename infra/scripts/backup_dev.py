#!/usr/bin/env python3
"""Consistent single-host snapshot and isolated restore rehearsal for personal ECS.

Snapshots contain credentials: owner-only directories, never commit them. This
does NOT claim that a backup on the ECS disk is an off-site backup.
"""
from __future__ import annotations
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
import shutil
from pathlib import Path
import subprocess
import time
import uuid

ROOT = Path("/opt/zbt-backups")
ACTIVE = Path("/opt/zbt/deployment-state.json")


def run(args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def compose(directory):
    return ["docker", "compose", "-p", "zbt-dev", "--project-directory", str(directory),
            "-f", str(directory / "docker-compose.yml"), "-f", str(directory / "docker-compose.ecs.yml")]


def active_directory():
    return Path(json.loads(ACTIVE.read_text())["directory"]) if ACTIVE.exists() else Path("/opt/zbt")


def checksum(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def snapshot(directory=None):
    os.umask(0o077)
    ROOT.mkdir(mode=0o700, exist_ok=True)
    if shutil.disk_usage(ROOT).free < 5 * 1024 ** 3:
        raise RuntimeError("less than 5 GiB free: snapshot refused before stopping services")
    path = ROOT / (datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex[:8])
    path.mkdir(mode=0o700)
    directory = directory or active_directory()
    command = compose(directory)
    services = ["frontend", "backend", "ai-service", "ocr-service", "minio"]
    # Quiesce all writers to keep DB references, objects and queue in one snapshot.
    run(command + ["stop", "-t", "30", *services])
    try:
        for name, args in [
            ("postgres.dump", ["pg_dump", "-U", "zbt", "-d", "zbt", "-Fc"]),
            ("roles.sql", ["pg_dumpall", "-U", "zbt", "--globals-only"]),
        ]:
            with (path / name).open("wb") as output:
                run(["docker", "exec", "zbt-dev-db", *args], stdout=output)
        frontend_image = subprocess.check_output(["docker", "inspect", "-f", "{{.Config.Image}}", "zbt-dev-frontend"], text=True).strip()
        volumes = ["minio_data"]
        exists = subprocess.run(["docker", "volume", "inspect", "zbt-dev_ai_tasks_data"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0
        if exists:
            volumes.append("ai_tasks_data")
        for name in volumes:
            run(["docker", "run", "--rm", "--entrypoint", "tar", "-v", "zbt-dev_" + name + ":/data:ro", "-v", str(path) + ":/backup", frontend_image, "-cf", "/backup/" + name + ".tar", "-C", "/data", "."])
        for source, target in [(directory / ".env", "environment.env"),
                               (directory / "docker-compose.yml", "docker-compose.yml"),
                               (directory / "docker-compose.ecs.yml", "docker-compose.ecs.yml"),
                               (directory / "ai-service/app/config/model_routing.btjs.yaml", "model_routing.yaml"),
                               (Path("/opt/zbt-private/access.json"), "access.json")]:
            if source.is_file():
                shutil.copyfile(source, path / target)
        counts = subprocess.check_output(["docker", "exec", "zbt-dev-db", "psql", "-U", "zbt", "-d", "zbt", "-At", "-c", "select json_build_object('bids',(select count(*) from bid_documents),'files',(select count(*) from file_assets),'chapters',(select count(*) from bid_chapters));"], text=True)
        manifest = {"created_at": datetime.now(timezone.utc).isoformat(), "release_directory": str(directory), "counts": json.loads(counts),
                    "sha256": {file.name: checksum(file) for file in path.iterdir() if file.is_file()}}
        (path / "manifest.json").write_text(json.dumps(manifest, indent=2))
        (ROOT / "latest.json").write_text(json.dumps({"path": str(path)}))
    finally:
        run(command + ["start", *services])
    print(json.dumps({"backup": str(path), "consistent": True}))
    return path


def rehearse(path):
    path = path.resolve()
    if path.parent != ROOT or not (path / "manifest.json").is_file():
        raise ValueError("restore rehearsal accepts only a complete ZBT backup directory")
    manifest = json.loads((path / "manifest.json").read_text())
    for name, expected in manifest["sha256"].items():
        if checksum(path / name) != expected:
            raise RuntimeError("backup checksum mismatch: " + name)
    suffix = uuid.uuid4().hex[:12]
    container = "zbt-restore-db-" + suffix
    image = subprocess.check_output(["docker", "inspect", "-f", "{{.Config.Image}}", "zbt-dev-db"], text=True).strip()
    # No host port/network and no production volume. Container writable layer is disposable.
    run(["docker", "run", "-d", "--name", container, "--network", "none", "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-e", "POSTGRES_USER=zbt", "-e", "POSTGRES_DB=zbt", image], stdout=subprocess.DEVNULL)
    try:
        for _ in range(60):
            if subprocess.run(["docker", "exec", container, "pg_isready", "-U", "zbt"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
                break
            time.sleep(1)
        # RLS policies reference these roles; do not import live superuser passwords.
        run(["docker", "exec", container, "psql", "-U", "zbt", "-d", "zbt", "-c", "create role zbt_app;"], stdout=subprocess.DEVNULL)
        with (path / "postgres.dump").open("rb") as source:
            run(["docker", "exec", "-i", container, "pg_restore", "-U", "zbt", "-d", "zbt", "--no-owner", "--no-acl", "--exit-on-error"], stdin=source)
        result = subprocess.check_output(["docker", "exec", container, "psql", "-U", "zbt", "-d", "zbt", "-At", "-c", "select json_build_object('bids',(select count(*) from bid_documents),'files',(select count(*) from file_assets),'chapters',(select count(*) from bid_chapters));"], text=True)
        if json.loads(result) != manifest["counts"]:
            raise RuntimeError("restored database counts differ from backup")
        # Extract into a unique disposable volume, then independently archive it;
        # compare file contents, not tar headers/timestamps, with Python tarfile.
        import tarfile
        volume = "zbt-restore-objects-" + suffix
        frontend = subprocess.check_output(["docker", "inspect", "-f", "{{.Config.Image}}", "zbt-dev-frontend"], text=True).strip()
        run(["docker", "volume", "create", volume], stdout=subprocess.DEVNULL)
        try:
            run(["docker", "run", "--rm", "--network", "none", "--entrypoint", "tar", "-v", volume + ":/data", "-v", str(path) + ":/backup:ro", frontend, "-xf", "/backup/minio_data.tar", "-C", "/data"])
            restored = subprocess.check_output(["docker", "run", "--rm", "--network", "none", "--entrypoint", "tar", "-v", volume + ":/data:ro", frontend, "-cf", "-", "-C", "/data", "."])
            import io
            def contents(archive):
                return {member.name: hashlib.sha256(archive.extractfile(member).read()).hexdigest()
                        for member in archive.getmembers() if member.isfile()}
            with tarfile.open(path / "minio_data.tar") as original, tarfile.open(fileobj=io.BytesIO(restored)) as restored_archive:
                if contents(original) != contents(restored_archive):
                    raise RuntimeError("restored object volume differs from backup")
        finally:
            run(["docker", "volume", "rm", volume], stdout=subprocess.DEVNULL)
        evidence = {"status": "passed", "database": json.loads(result), "object_volume": "byte-for-byte restored", "live_data_modified": False}
        if (path / "ai_tasks_data.tar").is_file():
            import sqlite3
            import tempfile
            with tempfile.TemporaryDirectory(prefix="zbt-queue-restore-") as temporary:
                with tarfile.open(path / "ai_tasks_data.tar") as archive:
                    for member in archive.getmembers():
                        name = member.name.removeprefix("./")
                        if name in {"tasks.sqlite3", "tasks.sqlite3-wal", "tasks.sqlite3-shm"} and member.isfile():
                            (Path(temporary) / name).write_bytes(archive.extractfile(member).read())
                with sqlite3.connect(str(Path(temporary) / "tasks.sqlite3")) as database:
                    if database.execute("pragma integrity_check").fetchone()[0] != "ok":
                        raise RuntimeError("restored AI queue failed integrity check")
                    evidence["durable_queue_jobs"] = database.execute("select count(*) from jobs").fetchone()[0]
        (path / "restore-evidence.json").write_text(json.dumps(evidence, indent=2))
        print(json.dumps(evidence))
    finally:
        run(["docker", "rm", "-f", "-v", container], stdout=subprocess.DEVNULL)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rehearse", type=Path)
    args = parser.parse_args()
    if args.rehearse:
        rehearse(args.rehearse)
    else:
        import fcntl
        Path("/opt/zbt-private").mkdir(mode=0o700, exist_ok=True)
        with open("/opt/zbt-private/deploy.lock", "a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            snapshot()
