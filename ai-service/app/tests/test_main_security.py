import hashlib
import hmac

from app.main import DEFAULT_AI_HMAC_SECRET, ai_service_hmac_secret, safe_output_filename, verify_request_signature


def test_safe_output_filename_keeps_task_output_in_temp_directory() -> None:
    assert safe_output_filename("/etc/passwd", "pdf") == "passwd.pdf"
    assert safe_output_filename("..\\..\\投标文件?.docx", "docx") == "投标文件.docx"
    assert safe_output_filename("", "zip") == "export.zip"


def test_safe_output_filename_preserves_suffix_when_truncated() -> None:
    filename = safe_output_filename("a" * 200, "pdf")

    assert len(filename) == 120
    assert filename.endswith(".pdf")


def test_verify_request_signature_accepts_valid_body_signature() -> None:
    body = b'{"task":"demo"}'
    timestamp = "1800000000"
    signature = hmac.new(b"secret", timestamp.encode() + b"." + body, hashlib.sha256).hexdigest()

    assert verify_request_signature(timestamp, signature, body, "secret", now=1800000000)


def test_verify_request_signature_rejects_invalid_or_expired_signature() -> None:
    body = b'{"task":"demo"}'
    timestamp = "1800000000"

    assert not verify_request_signature(timestamp, "bad", body, "secret", now=1800000000)
    assert not verify_request_signature(timestamp, "bad", body, "secret", now=1800000400)


def test_ai_service_hmac_secret_has_development_default(monkeypatch) -> None:
    monkeypatch.delenv("AI_SERVICE_HMAC_SECRET", raising=False)

    assert ai_service_hmac_secret() == DEFAULT_AI_HMAC_SECRET


def test_ai_service_hmac_secret_treats_empty_value_as_unset(monkeypatch) -> None:
    monkeypatch.setenv("AI_SERVICE_HMAC_SECRET", "")

    assert ai_service_hmac_secret() == DEFAULT_AI_HMAC_SECRET


def test_ai_service_hmac_secret_allows_override(monkeypatch) -> None:
    monkeypatch.setenv("AI_SERVICE_HMAC_SECRET", "custom-secret")

    assert ai_service_hmac_secret() == "custom-secret"
