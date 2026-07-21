"""Compose a chapter video from shot assets using ffmpeg."""
from __future__ import annotations

import logging

from celery import shared_task

from app.schemas.payloads import ComposeChapterPayload
from app.infra.queue.progress import report_progress

log = logging.getLogger(__name__)


@shared_task(name="ai:compose_chapter", bind=True, max_retries=3, default_retry_delay=30)
def compose_chapter(self, payload: dict) -> dict:
    parsed = ComposeChapterPayload.model_validate(payload)
    log.info("compose_chapter start", extra={"chapter_id": parsed.chapter_id})
    report_progress(self.request.id, "running", 0, 1, "composing")
    # TODO(app.infra.media.ffmpeg): concat shots + subtitles.
    report_progress(self.request.id, "success", 1, 1, "done")
    return {"status": "ok", "chapter_id": parsed.chapter_id, "video_key": ""}
