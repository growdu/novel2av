"""Generate reference images for a character (with variants for selection)."""
from __future__ import annotations

import asyncio
import json
import logging
from io import BytesIO

from celery import shared_task

from app.infra.media.image_provider import generate_image
from app.infra.queue.progress import report_progress
from app.infra.queue.notify import notify_complete
from app.infra.storage import get_client
from app.schemas.payloads import CharacterImagePayload

log = logging.getLogger(__name__)


@shared_task(name="ai:character_image", bind=True, max_retries=3, default_retry_delay=10)
def character_image(self, payload: dict) -> dict:
    parsed = CharacterImagePayload.model_validate(payload)
    log.info("character_image start",
             extra={"project_id": parsed.project_id, "cid": parsed.character_id})
    report_progress(self.request.id, "running", 0, parsed.variants, "preparing prompt", project_id=parsed.project_id)

    name, appearance, style = _load_character(parsed.project_id, parsed.character_id)
    prompt = _build_prompt(name, appearance, style)

    refs: list[bytes] = _maybe_load_refs(parsed.project_id, parsed.character_id)
    report_progress(self.request.id, "running", 1, parsed.variants, "generating", project_id=parsed.project_id)

    variants = generate_variants_sync(prompt, n=parsed.variants, reference_images=refs)

    report_progress(self.request.id, "running", parsed.variants - 1, parsed.variants, "uploading")
    keys: list[str] = []
    for i, png in enumerate(variants, start=1):
        key = f"characters/{parsed.project_id}/{parsed.character_id}/variants/v{i}.png"
        _upload(key, png, "image/png")
        keys.append(key)

    # The first variant is the default ref image.
    if variants:
        ref_key = f"characters/{parsed.project_id}/{parsed.character_id}/ref_image.png"
        _upload(ref_key, variants[0], "image/png")

    report_progress(self.request.id, "success", parsed.variants, parsed.variants, "done", project_id=parsed.project_id)
    result = {"status": "ok", "character_id": parsed.character_id,
              "ref_image_key": f"characters/{parsed.project_id}/{parsed.character_id}/ref_image.png",
              "variants": keys}
    notify_complete(self.request.id, {
        "task": self.name,
        "project_id": parsed.project_id,
        "payload": payload,
        "result": result,
    })
    return result


# --- helpers ---------------------------------------------------------------

def _load_character(project_id: str, character_id: str) -> tuple[str, str, str]:
    """Pull the character profile from the project manifest on MinIO."""
    from app.settings import get_settings
    s = get_settings()
    key = f"results/{project_id}/characters.json"
    try:
        resp = get_client().get_object(s.s3_bucket, key)
    except Exception:
        return character_id, "", "cinematic"
    try:
        body = resp.read()
    finally:
        resp.close()
        resp.release_conn()
    try:
        manifest = json.loads(body.decode("utf-8"))
    except Exception:
        return character_id, "", "cinematic"
    for ch in manifest.get("characters", []):
        if ch.get("id") == character_id or ch.get("name") == character_id:
            return (ch.get("name", character_id), ch.get("appearance", ""),
                    ch.get("style", "cinematic"))
    return character_id, "", "cinematic"


def _maybe_load_refs(project_id: str, character_id: str) -> list[bytes]:
    """If a previous ref image exists, load it so providers can keep the face consistent."""
    from app.settings import get_settings
    s = get_settings()
    key = f"characters/{project_id}/{character_id}/ref_image.png"
    try:
        resp = get_client().get_object(s.s3_bucket, key)
    except Exception:
        return []
    try:
        data = resp.read()
    finally:
        resp.close()
        resp.release_conn()
    return [data] if data else []


def _build_prompt(name: str, appearance: str, style: str) -> str:
    try:
        import yaml
        from pathlib import Path
        p = Path(__file__).resolve().parents[1] / "prompts" / "character_image.yaml"
        with p.open("r", encoding="utf-8") as f:
            tmpl = yaml.safe_load(f)
        return (tmpl.get("user_template", "")
                .replace("{{name}}", name)
                .replace("{{role}}", "supporting")
                .replace("{{appearance}}", appearance or "")
                .replace("{{style}}", style or "cinematic"))
    except Exception:
        return (
            f"{name}: {appearance}. Portrait, upper body, neutral background, "
            f"soft lighting, {style} style, high detail."
        )


def _upload(key: str, body: bytes, content_type: str) -> None:
    from app.settings import get_settings
    s = get_settings()
    get_client().put_object(s.s3_bucket, key, BytesIO(body), len(body), content_type=content_type)


def generate_variants_sync(prompt: str, *, n: int, reference_images: list[bytes]) -> list[bytes]:
    async def _run() -> list[bytes]:
        out: list[bytes] = []
        for _ in range(n):
            img = await generate_image(prompt, reference_images=reference_images or None)
            out.append(img.png_bytes)
        return out

    return asyncio.run(_run())
