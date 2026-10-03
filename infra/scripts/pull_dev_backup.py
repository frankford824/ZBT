#!/usr/bin/env python3
"""Pull the latest complete ECS snapshot off-host and verify every checksum."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess


def pull(host, destination):
    os.umask(0o077)
    ssh = ["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=15", host]
    latest = json.loads(subprocess.check_output(ssh + ["python3 -c 'import json; print(open(\"/opt/zbt-backups/latest.json\").read())'"], text=True, timeout=30))
    remote = str(latest["path"])
    if not re.fullmatch(r"/opt/zbt-backups/\d{8}T\d{6}Z-[0-9a-f]{8}", remote):
        raise ValueError("unexpected remote backup path")
    destination.mkdir(mode=0o700, parents=True, exist_ok=True)
    target = destination / Path(remote).name
    target.mkdir(mode=0o700, exist_ok=True)
    subprocess.run(["rsync", "-a", "-e", "ssh -o BatchMode=yes -o ConnectTimeout=15", host + ":" + remote + "/", str(target) + "/"], check=True, timeout=600)
    for member in [target, *target.rglob("*")]:
        if member.is_symlink():
            raise ValueError("unexpected symlink in backup")
        os.chmod(member, 0o700 if member.is_dir() else 0o600)
    manifest = json.loads((target / "manifest.json").read_text())
    for name, expected in manifest["sha256"].items():
        if Path(name).name != name:
            raise ValueError("invalid backup member name")
        digest = hashlib.sha256()
        with (target / name).open("rb") as source:
            for block in iter(lambda: source.read(1024 * 1024), b""):
                digest.update(block)
        if digest.hexdigest() != expected:
            raise RuntimeError("backup checksum mismatch: " + name)
    print(json.dumps({"status": "passed", "off_host_backup": str(target), "files_verified": len(manifest["sha256"])}))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default="ecs-test")
    parser.add_argument("--destination", type=Path, default=Path.home() / ".codex/private/zbt-ecs-backups")
    args = parser.parse_args()
    pull(args.host, args.destination)
