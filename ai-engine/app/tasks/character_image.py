"""Generate reference images for a character (with variants for selection)."""
from __future__ import annotations

import logging

from celery import shared_task

from app.schemas.payloads import CharacterImagePayload
from app.infra.queue.progress import report_progress

log = logging.getLogger(__name__)


@shared_task(name="ai:character_image", bind=True, max_retries=3, default_retry_delay=10)
def character_image(self, payload: dict) -> dict:
    parsed = CharacterImagePayload.model_validate(payload)
    log.info("character_image start", extra={"project_id": parsed.project_id, "cid": parsed.character_id})
    report_progress(self.request.id, "running", 0, parsed.variants, "generating variants")
    # TODO(app.infra.media.image_provider): call Seedream / SDXL.
    report_progress(self.request.id, "success", parsed.variants, parsed.variants, "done")
    return {"status": "ok", "character_id": parsed.character_id, "variants": []}
