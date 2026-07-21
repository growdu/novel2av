"""Break a chapter into scenes and shots via LLM."""
from __future__ import annotations

import logging

from celery import shared_task

from app.schemas.payloads import SceneBreakdownPayload
from app.infra.queue.progress import report_progress

log = logging.getLogger(__name__)


@shared_task(name="ai:scene_breakdown", bind=True, max_retries=3, default_retry_delay=10)
def scene_breakdown(self, payload: dict) -> dict:
    parsed = SceneBreakdownPayload.model_validate(payload)
    log.info("scene_breakdown start", extra={"chapter_id": parsed.chapter_id})
    report_progress(self.request.id, "running", 0, 1, "breaking down")
    # TODO(app.services.shot_service): LLM call → structured shots.
    report_progress(self.request.id, "success", 1, 1, "done")
    return {"status": "ok", "chapter_id": parsed.chapter_id, "shots": []}
