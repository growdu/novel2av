"""Generate per-shot image + tts + bgm in parallel (sequential stub here)."""
from __future__ import annotations

import logging

from celery import shared_task

from app.schemas.payloads import GenerateShotPayload
from app.infra.queue.progress import report_progress

log = logging.getLogger(__name__)


@shared_task(name="ai:generate_shot", bind=True, max_retries=3, default_retry_delay=10)
def generate_shot(self, payload: dict) -> dict:
    parsed = GenerateShotPayload.model_validate(payload)
    log.info("generate_shot start", extra={"shot_id": parsed.shot_id})
    report_progress(self.request.id, "running", 1, 3, "image")
    # TODO(image_provider): Seedream / SDXL.
    report_progress(self.request.id, "running", 2, 3, "tts")
    # TODO(tts_provider): Doubao / Edge-TTS.
    report_progress(self.request.id, "running", 3, 3, "bgm")
    # TODO(bgm_provider): MusicGen / Suno.
    report_progress(self.request.id, "success", 3, 3, "done")
    return {"status": "ok", "shot_id": parsed.shot_id}
