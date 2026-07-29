"""Notify backend after a pipeline task finishes (success or failure).

The backend exposes `POST /api/v1/internal/jobs/{celery_task_id}:complete`
with a small JSON body. The signature is an HMAC over the body using a
shared secret (`AI_BACKEND_HMAC_SECRET`).

Failures to notify are non-fatal: we log and move on. The next retry of
the task will try again.
"""
from __future__ import annotations

import hashlib
import hmac
import json
import logging
import os

import httpx

log = logging.getLogger(__name__)


def _backend_url() -> str | None:
    return os.environ.get("AI_BACKEND_URL")


def _secret() -> str:
    return os.environ.get("AI_BACKEND_HMAC_SECRET", "dev-secret")


def _sign(body: bytes) -> str:
    return hmac.new(_secret().encode(), body, hashlib.sha256).hexdigest()


def notify_complete(
    task_id: str,
    payload: dict,
    *,
    timeout: float = 10.0,
) -> bool:
    url = _backend_url()
    if not url:
        log.debug("backend url not configured; skipping notify")
        return False
    body_bytes = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    headers = {
        "Content-Type": "application/json",
        "X-N2AV-Signature": _sign(body_bytes),
    }
    try:
        with httpx.Client(timeout=timeout) as cli:
            r = cli.post(f"{url.rstrip('/')}/api/v1/internal/jobs/{task_id}:complete",
                         content=body_bytes, headers=headers)
        if r.status_code >= 300:
            log.warning("backend notify non-2xx", extra={"status": r.status_code, "body": r.text[:200]})
            return False
        return True
    except Exception as exc:  # pragma: no cover
        log.warning("backend notify failed", extra={"err": str(exc)})
        return False
