"""Extract main characters (name, appearance, personality, voice) via LLM.

Reads each chapter's JSON manifest from MinIO, concatenates the `content`
field (truncated per chapter), then asks the LLM. Writes a manifest at
`results/<project_id>/characters.json`.
"""
from __future__ import annotations

import asyncio
import json
import logging
from io import BytesIO

from celery import shared_task

from app.infra.queue.progress import report_progress
from app.infra.storage import get_client
from app.schemas.payloads import ExtractCharactersPayload
from app.services.character_service import CharacterProfile, extract_characters

log = logging.getLogger(__name__)


@shared_task(name="ai:extract_characters", bind=True, max_retries=3, default_retry_delay=10)
def extract_characters_task(self, payload: dict) -> dict:
    parsed = ExtractCharactersPayload.model_validate(payload)
    log.info("extract_characters start", extra={"project_id": parsed.project_id})
    report_progress(self.request.id, "running", 0, 4, "fetching chapters", project_id=parsed.project_id)

    texts: list[str] = []
    for key in parsed.chapter_keys:
        try:
            texts.append(_download_text(key))
        except Exception as exc:
            log.warning("chapter fetch failed", extra={"key": key, "err": str(exc)})

    report_progress(self.request.id, "running", 1, 4, "calling LLM", project_id=parsed.project_id)
    profiles = llm_extract_sync(texts)
    log.info("extracted", extra={"count": len(profiles)})

    report_progress(self.request.id, "running", 2, 4, "uploading manifest", project_id=parsed.project_id)
    manifest = {
        "characters": [p.model_dump() for p in profiles],
    }
    result_key = f"results/{parsed.project_id}/characters.json"
    _upload(result_key, json.dumps(manifest, ensure_ascii=False).encode("utf-8"))

    report_progress(self.request.id, "success", 4, 4, "done", project_id=parsed.project_id)
    return {"status": "ok", "project_id": parsed.project_id,
            "character_count": len(profiles), "result_key": result_key}


# --- helpers ---------------------------------------------------------------

def _download_text(key: str) -> str:
    from app.settings import get_settings
    s = get_settings()
    resp = get_client().get_object(s.s3_bucket, key)
    try:
        data = resp.read()
    finally:
        resp.close()
        resp.release_conn()
    if key.endswith(".json"):
        try:
            obj = json.loads(data.decode("utf-8"))
            return str(obj.get("content", "") or obj.get("text", ""))
        except Exception:
            return data.decode("utf-8", errors="replace")
    return data.decode("utf-8", errors="replace")


def _upload(key: str, body: bytes) -> None:
    from app.settings import get_settings
    s = get_settings()
    get_client().put_object(s.s3_bucket, key, BytesIO(body), len(body),
                            content_type="application/json")


def llm_extract_sync(texts: list[str]) -> list[CharacterProfile]:
    from app.settings import get_settings
    s = get_settings()
    providers = list(s.llm_providers.keys())
    provider = "doubao" if "doubao" in providers else (providers[0] if providers else "ollama")
    model = "doubao-pro-128k"
    return asyncio.run(extract_characters(texts, provider=provider, model=model))
