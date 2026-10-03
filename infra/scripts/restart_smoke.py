#!/usr/bin/env python3
"""Restart the AI process during a dedicated fixture task; verify same-ID recovery."""
import json
from pathlib import Path
import subprocess
import time
from chapter_generation_smoke import api_call
from full_bid_smoke import wait_for


def run():
    credentials = json.loads(Path("/opt/zbt-private/access.json").read_text())
    fixture = json.loads(Path("/opt/zbt-private/full-acceptance.json").read_text())
    base = "http://127.0.0.1:18080/api/v1"
    token = api_call(base, "/auth/login", method="POST", body={"email": credentials["smoke_email"], "password": credentials["smoke_password"]})["access_token"]
    # Refuse to modify a bid unless it is one of our archived acceptance fixtures.
    bid = api_call(base, "/bids/" + fixture["bid_id"], token=token)
    if not bid["title"].startswith("ZBT-upload-smoke-") or bid["status"] != "archived":
        raise RuntimeError("restart rehearsal requires the archived acceptance fixture")
    chapter = fixture["chapter_ids"][0]
    task = api_call(base, "/chapters/" + chapter + "/regenerate", method="POST", body={}, token=token)["task"]
    external = task["external_task_id"]
    active_other = subprocess.check_output(["docker", "exec", "zbt-dev-db", "psql", "-U", "zbt", "-d", "zbt", "-At", "-c",
        "select count(*) from ai_tasks where status in ('queued','running') and id <> '" + task["id"] + "'::uuid;"], text=True).strip()
    if active_other != "0":
        raise RuntimeError("other tasks are active; refusing to interrupt user work")
    query = "import sqlite3,os,sys; d=sqlite3.connect(os.environ['AI_DURABLE_QUEUE_PATH']); r=d.execute('select state,attempts from jobs where id=?',(sys.argv[1],)).fetchone(); print(r[0] if r else 'absent')"
    for _ in range(30):
        state = subprocess.check_output(["docker", "exec", "zbt-dev-ai", "python", "-c", query, external], text=True).strip()
        if state == "running":
            break
        if state == "done":
            raise RuntimeError("fixture completed too quickly to exercise interrupted work")
        time.sleep(0.2)
    else:
        raise RuntimeError("fixture did not enter durable running state")
    # SIGKILL represents an abrupt interruption, not a graceful drained restart.
    subprocess.run(["docker", "kill", "--signal=KILL", "zbt-dev-ai"], check=True, stdout=subprocess.DEVNULL)
    subprocess.run(["docker", "start", "zbt-dev-ai"], check=True, stdout=subprocess.DEVNULL)
    completed = wait_for(base, "/ai-tasks/" + task["id"], token, "", timeout=1200)
    if not (completed.get("result") or {}).get("tiptap_json"):
        raise RuntimeError("recovered task has no generated content")
    evidence = {"restart_recovery": "passed", "task_id": task["id"], "external_task_id": external, "same_task_id": True}
    Path("/opt/zbt-private/restart-evidence.json").write_text(json.dumps(evidence, indent=2))
    print(json.dumps(evidence))


if __name__ == "__main__":
    run()
