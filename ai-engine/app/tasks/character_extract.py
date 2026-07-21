"""Extract main characters (name, appearance, personality, voice) via LLM."""
from __future__ import annotations

import logging

from celery import shared_task

from app.schemas.payloads import ExtractCharactersPayload
from app.infra.queue.progress import report_progress

log = logging.getLogger(__name__)


@shared_task(name="ai:extract_characters", bind=True, max_retries=3, default_retry_delay=10)
def extract_characters(self, payload: dict) -> dict:
    parsed = ExtractCharactersPayload.model_validate(payload)
    log.info("extract_characters start", extra={"project_id": parsed.project_id})
    report_progress(self.request.id, "running", 0, 1, "extracting")
    # TODO(app.services.character_service): LLM call + validation.
    report_progress(self.request.id, "success", 1, 1, "done")
    return {"status": "ok", "project_id": parsed.project_id, "characters": []}
