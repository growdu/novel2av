"""Split raw novel text into chapters using LLM + heuristics."""
from __future__ import annotations

import logging

from celery import shared_task

from app.schemas.payloads import SplitChaptersPayload
from app.infra.queue.progress import report_progress

log = logging.getLogger(__name__)


@shared_task(name="ai:split_chapters", bind=True, max_retries=3, default_retry_delay=10)
def split_chapters(self, payload: dict) -> dict:
    parsed = SplitChaptersPayload.model_validate(payload)
    log.info("split_chapters start", extra={"project_id": parsed.project_id})
    report_progress(self.request.id, "running", 0, 1, "fetching source")
    # TODO(app.services.chapter_service): actual split via LLM gateway.
    report_progress(self.request.id, "success", 1, 1, "done")
    return {"status": "ok", "project_id": parsed.project_id, "chapters": []}
