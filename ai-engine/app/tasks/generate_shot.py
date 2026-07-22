"""Generate per-shot image + tts + bgm in parallel.

Image generation uses `infra.media.image_provider.generate_image`.
TTS uses `infra.media.tts_provider.synthesize_speech`.
Bgm is a placeholder silent track of the same duration as the shot (real
music generation lands in M5 with MusicGen/Suno).

Writes per-shot assets to `shots/<project_id>/<chapter_id>/<shot_id>/{image,tts}.{png,wav}`
and a `summary.json` so backend can ingest them.
"""
from __future__ import annotations

import asyncio
import json
import logging
import math
import os
from io import BytesIO

from celery import shared_task

from app.infra.media.image_provider import generate_image
from app.infra.media.tts_provider import silence_wav, synthesize_speech, voice_for
from app.infra.queue.progress import report_progress
from app.infra.storage import get_client
from app.schemas.payloads import GenerateShotPayload

log = logging.getLogger(__name__)


@shared_task(name="ai:generate_shot", bind=True, max_retries=3, default_retry_delay=10)
def generate_shot(self, payload: dict) -> dict:
    parsed = GenerateShotPayload.model_validate(payload)
    log.info("generate_shot start", extra={"shot_id": parsed.shot_id})
    total = 3
    report_progress(self.request.id, "running", 1, total, "image", project_id=parsed.project_id)

    # Load character refs from MinIO (if provided as content_keys).
    refs = _load_ref_bytes(parsed.character_refs)

    # Image.
    image_png = generate_shot_image_sync(
        parsed.description, parsed.style, parsed.aspect, refs,
    )
    image_key = f"shots/{parsed.project_id}/{parsed.chapter_id}/{parsed.shot_id}/image.png"
    _upload(image_key, image_png, "image/png")

    report_progress(self.request.id, "running", 2, total, "tts", project_id=parsed.project_id)
    tts_bytes, _ = asyncio.run(synthesize_speech(parsed.narration, voice_id=voice_for(parsed.style)))
    tts_key = f"shots/{parsed.project_id}/{parsed.chapter_id}/{parsed.shot_id}/narration.wav"
    _upload(tts_key, tts_bytes, "audio/wav")

    report_progress(self.request.id, "running", 3, total, "bgm", project_id=parsed.project_id)
    bgm_bytes = silence_wav(parsed.narration)  # M5: replace with real BGM provider
    bgm_key = f"shots/{parsed.project_id}/{parsed.chapter_id}/{parsed.shot_id}/bgm.wav"
    _upload(bgm_key, bgm_bytes, "audio/wav")

    summary = {
        "shot_id": parsed.shot_id,
        "image_key": image_key,
        "tts_key": tts_key,
        "bgm_key": bgm_key,
        "duration_sec": parsed.duration_sec,
    }
    summary_key = f"shots/{parsed.project_id}/{parsed.chapter_id}/{parsed.shot_id}/summary.json"
    _upload(summary_key, json.dumps(summary, ensure_ascii=False).encode("utf-8"), "application/json")

    report_progress(self.request.id, "success", total, total, "done", project_id=parsed.project_id)
    return summary


# --- helpers ---------------------------------------------------------------

def _load_ref_bytes(keys: list[str]) -> list[bytes]:
    from app.settings import get_settings
    s = get_settings()
    out: list[bytes] = []
    for k in keys:
        if not k:
            continue
        try:
            resp = get_client().get_object(s.s3_bucket, k)
        except Exception:
            continue
        try:
            data = resp.read()
        finally:
            resp.close()
            resp.release_conn()
        if data:
            out.append(data)
    return out


def _upload(key: str, body: bytes, content_type: str) -> None:
    from app.settings import get_settings
    s = get_settings()
    get_client().put_object(s.s3_bucket, key, BytesIO(body), len(body), content_type=content_type)


def generate_shot_image_sync(description: str, style: str, aspect: str, refs: list[bytes]) -> bytes:
    """Compose a shot image prompt and call the provider."""
    prompt = (
        f"{description}\n"
        f"cinematic still, {style} style, {aspect}, professional lighting, no text."
    )
    width, height = _aspect_to_size(aspect)
    img = asyncio.run(generate_image(
        prompt, width=width, height=height, reference_images=refs or None,
    ))
    return img.png_bytes


def _aspect_to_size(aspect: str) -> tuple[int, int]:
    presets = {
        "9:16": (1080, 1920),
        "16:9": (1920, 1080),
        "1:1":  (1024, 1024),
    }
    return presets.get(aspect, (1080, 1920))
