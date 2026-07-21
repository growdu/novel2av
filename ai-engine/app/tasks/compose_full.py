"""Concat all chapter videos into a full-book MP4."""
from __future__ import annotations

import logging

from celery import shared_task

from app.schemas.payloads import ComposeFullPayload
from app.infra.queue.progress import report_progress

log = logging.getLogger(__name__)


@shared_task(name="ai:compose_full", bind=True, max_retries=2, default_retry_delay=60)
def compose_full(self, payload: dict) -> dict:
    parsed = ComposeFullPayload.model_validate(payload)
    log.info("compose_full start", extra={"project_id": parsed.project_id})
    report_progress(self.request.id, "running", 0, 1, "concatenating")
    # TODO(app.infra.media.ffmpeg): ffmpeg concat demuxer + chapter titles.
    report_progress(self.request.id, "success", 1, 1, "done")
    return {"status": "ok", "project_id": parsed.project_id, "video_key": ""}
