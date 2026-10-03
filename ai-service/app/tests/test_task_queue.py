import pytest
from pydantic import BaseModel

from app.task_queue import DurableQueue


class Payload(BaseModel):
    tenant_id: str = "tenant"
    callback_url: str = "http://backend/callback"


def handler(task_id: str, payload: Payload):
    pass


def test_restart_recovers_claimed_job(tmp_path):
    path = str(tmp_path / "jobs.sqlite")
    q = DurableQueue(path, {"handler": handler}, lambda *_: None)
    q.enqueue("handler", "one", Payload())
    assert q.claim()["id"] == "one"
    recovered = DurableQueue(path, {"handler": handler}, lambda *_: None)
    assert recovered.run_once()
    with recovered.connect() as db:
        assert db.execute("SELECT state FROM jobs").fetchone()[0] == "done"


def test_duplicate_id_is_idempotent_but_different_payload_is_rejected(tmp_path):
    q = DurableQueue(str(tmp_path / "jobs.sqlite"), {"handler": handler}, lambda *_: None)
    q.enqueue("handler", "one", Payload())
    q.enqueue("handler", "one", Payload())
    with pytest.raises(ValueError):
        q.enqueue("handler", "one", Payload(tenant_id="other"))
    assert q.run_once()
    assert not q.run_once()


def test_outbox_survives_restart_and_does_not_regenerate_models(tmp_path):
    path = str(tmp_path / "jobs.sqlite")
    q = DurableQueue(path, {"handler": handler}, lambda *_: None)
    q.enqueue("handler", "one", Payload())
    q.claim()
    q.local.task_id = "one"
    terminal = {
        "task_id": "one",
        "tenant_id": "tenant",
        "status": "done",
        "result": {"content": "saved"},
    }
    q.save_callback("http://backend/callback", terminal)
    delivered = []
    recovered = DurableQueue(path, {}, lambda url, payload: delivered.append(payload))
    assert recovered.run_once()
    assert delivered == [terminal]


def test_failed_callback_remains_durable_until_delivery(tmp_path):
    q = DurableQueue(str(tmp_path / "jobs.sqlite"), {"handler": handler}, lambda *_: None)
    q.enqueue("handler", "one", Payload())
    q.claim()
    q.local.task_id = "one"
    q.save_callback("http://backend/callback", {"task_id": "one", "status": "done"})
    with q.connect() as db:
        db.execute("UPDATE jobs SET state='callback'")

    def unavailable(*_):
        raise OSError("backend unavailable")

    q.deliver = unavailable
    assert q.run_once()
    with q.connect() as db:
        row = db.execute("SELECT * FROM jobs").fetchone()
        assert row["state"] == "callback" and row["attempts"] == 1
        db.execute("UPDATE jobs SET available=0")
    q.deliver = lambda *_: None
    assert q.run_once()


def test_expired_job_reports_terminal_failure_instead_of_running_forever(tmp_path):
    delivered = []
    q = DurableQueue(
        str(tmp_path / "jobs.sqlite"),
        {"handler": handler},
        lambda url, payload: delivered.append(payload),
        max_age=10,
    )
    q.enqueue("handler", "one", Payload())
    with q.connect() as db:
        db.execute("UPDATE jobs SET created=0")
    assert q.run_once()
    assert delivered[0]["status"] == "failed"
    assert delivered[0]["result"]["retryable"] is True
