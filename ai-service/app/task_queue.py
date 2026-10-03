"""Single-host durable AI jobs and callback outbox (SQLite on a named volume).

Delivery is at least once; stable task IDs and backend terminal-state guards
make callback replay safe. Never deserialize executable objects from storage.
"""

from __future__ import annotations

import json
import logging
import sqlite3
import threading
import time
from collections.abc import Callable
from contextlib import contextmanager
from pathlib import Path
from typing import get_type_hints

log = logging.getLogger(__name__)


class DurableQueue:
    def __init__(
        self,
        path: str,
        handlers: dict[str, Callable],
        deliver: Callable,
        workers: int = 2,
        max_age: int = 3600,
    ):
        self.path, self.handlers, self.deliver = path, handlers, deliver
        self.workers, self.max_age = workers, max_age
        self.stop_event = threading.Event()
        self.local = threading.local()
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        with self.connect() as db:
            db.execute("PRAGMA journal_mode=WAL")
            db.execute("""CREATE TABLE IF NOT EXISTS jobs (
                id TEXT PRIMARY KEY, handler TEXT NOT NULL, payload TEXT NOT NULL,
                extra TEXT NOT NULL, state TEXT NOT NULL DEFAULT 'queued',
                created REAL NOT NULL, updated REAL NOT NULL, attempts INTEGER NOT NULL DEFAULT 0,
                available REAL NOT NULL DEFAULT 0, callback_url TEXT, callback TEXT,
                error TEXT NOT NULL DEFAULT '')""")
            # A new process owns this DB exclusively. Interrupted work is replayed;
            # saved callbacks are delivered without rerunning expensive models.
            db.execute(
                "UPDATE jobs SET state=CASE WHEN callback IS NULL THEN 'queued' ELSE 'callback' END, attempts=attempts+1 WHERE state='running'"
            )
        self.threads: list[threading.Thread] = []

    @contextmanager
    def connect(self):
        db = sqlite3.connect(self.path, timeout=30)
        db.row_factory = sqlite3.Row
        db.execute("PRAGMA synchronous=FULL")
        try:
            with db:
                yield db
        finally:
            db.close()

    def enqueue(self, handler: str, task_id: str, payload, extra=()):
        if handler not in self.handlers:
            raise ValueError("unknown durable handler")
        encoded = json.dumps(payload.model_dump(mode="json"), ensure_ascii=False)
        extras = json.dumps(extra)
        with self.connect() as db:
            existing = db.execute(
                "SELECT handler,payload,extra FROM jobs WHERE id=?", (task_id,)
            ).fetchone()
            if existing and (existing["handler"], existing["payload"], existing["extra"]) != (
                handler,
                encoded,
                extras,
            ):
                raise ValueError("task ID cannot be reused for a different request")
            db.execute(
                "INSERT OR IGNORE INTO jobs(id,handler,payload,extra,created,updated) VALUES(?,?,?,?,?,?)",
                (task_id, handler, encoded, extras, time.time(), time.time()),
            )

    def save_callback(self, url: str, payload: dict):
        task_id = getattr(self.local, "task_id", None)
        if not task_id:
            return False
        if payload.get("task_id") != task_id:
            raise ValueError("callback does not belong to the active durable job")
        with self.connect() as db:
            db.execute(
                "UPDATE jobs SET callback_url=?,callback=?,updated=? WHERE id=?",
                (url, json.dumps(payload, ensure_ascii=False), time.time(), task_id),
            )
        return True

    def start(self):
        for _ in range(self.workers):
            thread = threading.Thread(target=self.loop, daemon=True)
            thread.start()
            self.threads.append(thread)

    def close(self):
        self.stop_event.set()
        # Long-running work remains recoverable on disk, not dropped on shutdown.
        for thread in self.threads:
            thread.join(timeout=1)

    def claim(self):
        with self.connect() as db:
            db.execute("BEGIN IMMEDIATE")
            row = db.execute(
                "SELECT * FROM jobs WHERE state IN ('queued','callback') AND available<=? ORDER BY created LIMIT 1",
                (time.time(),),
            ).fetchone()
            if row:
                db.execute(
                    "UPDATE jobs SET state='running',updated=? WHERE id=?", (time.time(), row["id"])
                )
            db.execute(
                "DELETE FROM jobs WHERE state='done' AND updated<?", (time.time() - 7 * 86400,)
            )
            return row

    def run_once(self):
        row = self.claim()
        if row is None:
            return False
        task_id = row["id"]
        self.local.task_id = task_id
        try:
            if row["callback"]:
                self.deliver(row["callback_url"], json.loads(row["callback"]))
            elif time.time() - row["created"] > self.max_age or row["attempts"] >= 3:
                original = json.loads(row["payload"])
                failure = {
                    "tenant_id": original["tenant_id"],
                    "task_id": task_id,
                    "status": "failed",
                    "error_message": "任务执行超时或多次中断，请重试",
                    "result": {"error_code": "task_recovery_exhausted", "retryable": True},
                }
                self.save_callback(original.get("callback_url", ""), failure)
                if original.get("callback_url"):
                    self.deliver(original["callback_url"], failure)
            else:
                handler = self.handlers[row["handler"]]
                schema = get_type_hints(handler)["payload"]
                payload = schema.model_validate_json(row["payload"])
                handler(task_id, payload, *json.loads(row["extra"]))
            with self.connect() as db:
                db.execute(
                    "UPDATE jobs SET state='done',updated=?,error='' WHERE id=?",
                    (time.time(), task_id),
                )
        except Exception as exc:
            # Keep outbox failures pending indefinitely with bounded backoff:
            # a healthy backend can still receive the terminal callback later.
            log.warning(
                "Durable AI task retry: task_id=%s error_type=%s", task_id, type(exc).__name__
            )
            with self.connect() as db:
                db.execute(
                    """UPDATE jobs SET state=CASE WHEN callback IS NULL THEN 'queued' ELSE 'callback' END,
                           attempts=attempts+1,available=?,updated=?,error=? WHERE id=?""",
                    (
                        time.time() + min(60, 2 ** min(row["attempts"], 6)),
                        time.time(),
                        type(exc).__name__,
                        task_id,
                    ),
                )
        finally:
            self.local.task_id = None
        return True

    def loop(self):
        while not self.stop_event.is_set():
            try:
                if not self.run_once():
                    self.stop_event.wait(0.5)
            except Exception:
                log.exception("Durable queue worker failed")
                self.stop_event.wait(2)
