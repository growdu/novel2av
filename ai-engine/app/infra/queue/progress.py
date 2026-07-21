"""Progress reporting helper shared by all tasks.

Reports flow back to backend either via:
  - writing to Redis hash `progress:<job_id>` (backend polls), or
  - publishing to Redis channel `events:project:<id>` (backend subscribes).
"""
from __future__ import annotations

import json
import logging
from typing import Literal

log = logging.getLogger(__name__)


def report_progress(
    job_id: str,
    status: Literal["queued", "running", "success", "failed", "retrying"],
    current: int = 0,
    total: int = 0,
    message: str = "",
) -> None:
    """Stub: in production, publish to Redis; here we just log JSON."""
    payload = {
        "type": "job.progress",
        "job_id": job_id,
        "status": status,
        "current": current,
        "total": total,
        "message": message,
    }
    log.info(json.dumps(payload, ensure_ascii=False))
