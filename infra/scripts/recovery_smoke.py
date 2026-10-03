#!/usr/bin/env python3
"""Create only an isolated recovery fixture and verify the real timeout sweeper."""
import json
import subprocess
import time
import uuid


def sql(query):
    return subprocess.check_output(["docker", "exec", "-i", "zbt-dev-db", "psql", "-U", "zbt", "-d", "zbt", "-At", "-v", "ON_ERROR_STOP=1"], input=query.encode()).decode().strip()


def run():
    tenant = "00000000-0000-4000-8000-000000000001"
    bid, part, chapter, task = [str(uuid.uuid4()) for _ in range(4)]
    sql(f"""begin;
        insert into bid_documents(id,tenant_id,title,bid_type,status) values('{bid}','{tenant}','ZBT-recovery-smoke','combined','archived');
        insert into bid_parts(id,tenant_id,bid_document_id,code,title) values('{part}','{tenant}','{bid}','combined_body','恢复测试');
        insert into bid_chapters(id,tenant_id,bid_document_id,bid_part_id,title,status) values('{chapter}','{tenant}','{bid}','{part}','超时恢复测试','generating');
        insert into ai_tasks(id,tenant_id,task_type,status,resource_type,resource_id,created_at)
        values('{task}','{tenant}','chapter_generate','queued','bid_chapter','{chapter}',now()-interval '2 hours');
        commit;""")
    deadline = time.monotonic() + 75
    while time.monotonic() < deadline:
        status = sql(f"select status from ai_tasks where id='{task}';")
        if status == "failed":
            if sql(f"select status from bid_chapters where id='{chapter}';") != "needs_fix":
                raise RuntimeError("task ended but chapter remained generating")
            return {"recovery": "passed", "bid_id": bid, "task_id": task, "chapter_status": "needs_fix"}
        time.sleep(2)
    raise RuntimeError("real backend timeout recovery did not settle the fixture")


if __name__ == "__main__":
    print(json.dumps(run()))
