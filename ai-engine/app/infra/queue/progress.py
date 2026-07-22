"""Progress reporting helper used by Celery tasks.

Writes to two places:
  - structured log (always)
  - Redis Pub/Sub channel `events:project:<project_id>` if available

The Go backend subscribes to those channels and fans them out to WebSocket
clients.
"""
from __future__ import annotations

import json
import logging
import os
from typing import Literal

log = logging.getLogger(__name__)


def _redis():
    """Lazily build a Redis client; None if REDIS_URL not set."""
    url = os.environ.get("AI_REDIS_URL")
    if not url:
        return None
    try:
        import redis  # type: ignore
    except Exception:
        return None
    try:
        return redis.Redis.from_url(url)
    except Exception:
        return None


def report_progress(
    job_id: str,
    status: Literal["queued", "running", "success", "failed", "retrying"],
    current: int = 0,
    total: int = 0,
    message: str = "",
    *,
    project_id: str | None = None,
    chapter_id: str | None = None,
    shot_id: str | None = None,
) -> None:
    payload = {
        "type": "job.progress",
        "job_id": job_id,
        "project_id": project_id or "",
        "chapter_id": chapter_id or "",
        "shot_id": shot_id or "",
        "step": "",
        "status": status,
        "current": current,
        "total": total,
        "message": message,
    }
    line = json.dumps(payload, ensure_ascii=False)
    log.info(line)

    r = _redis()
    if r is None or not project_id:
        return
    try:
        r.publish(f"events:project:{project_id}", line)
    except Exception as exc:  # pragma: no cover
        log.warning("redis publish failed", extra={"err": str(exc)})
