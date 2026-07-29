"""Break a chapter into scenes and shots via LLM.

Reads the chapter JSON from MinIO (uses `content` if present, else falls back
to `text`), runs `scene_breakdown`, and writes the resulting shot list to
`results/<project_id>/chapters/<chapter_index>/breakdown.json` plus a
progress channel message for backend.
"""
from __future__ import annotations

import asyncio
import json
import logging
from io import BytesIO

from celery import shared_task

from app.infra.queue.progress import report_progress
from app.infra.queue.notify import notify_complete
from app.infra.storage import get_client
from app.schemas.payloads import SceneBreakdownPayload
from app.services.shot_service import scene_breakdown

log = logging.getLogger(__name__)


@shared_task(name="ai:scene_breakdown", bind=True, max_retries=3, default_retry_delay=10)
def scene_breakdown_task(self, payload: dict) -> dict:
    parsed = SceneBreakdownPayload.model_validate(payload)
    log.info("scene_breakdown start", extra={"chapter_id": parsed.chapter_id})
    report_progress(self.request.id, "running", 0, 3, "fetching chapter", project_id=parsed.project_id)

    text = fetch_chapter_text(parsed.project_id, parsed.chapter_id)
    if not text:
        log.warning("empty chapter text", extra={"chapter_id": parsed.chapter_id})
        return {"status": "ok", "chapter_id": parsed.chapter_id, "shots": []}

    report_progress(self.request.id, "running", 1, 3, "calling LLM", project_id=parsed.project_id)
    shots = breakdown_sync(text, characters=[])

    report_progress(self.request.id, "running", 2, 3, "uploading result", project_id=parsed.project_id)
    key = f"results/{parsed.project_id}/chapters/{parsed.chapter_id}/breakdown.json"
    body = {
        "chapter_id": parsed.chapter_id,
        "shots": [
            {
                "scene_idx": s.scene_idx,
                "shot_idx": s.shot_idx,
                "type": s.type,
                "description": s.description,
                "narration": s.narration,
                "mood": s.mood,
                "duration_sec": s.duration_sec,
                "characters": s.characters,
            }
            for s in shots
        ],
    }
    _upload(key, json.dumps(body, ensure_ascii=False).encode("utf-8"))

    report_progress(self.request.id, "success", 3, 3, "done", project_id=parsed.project_id)
    result = {"status": "ok", "chapter_id": parsed.chapter_id,
              "shot_count": len(shots), "result_key": key}
    notify_complete(self.request.id, {
        "task": self.name,
        "project_id": parsed.project_id,
        "payload": payload,
        "result": result,
    })
    return result


# --- helpers ---------------------------------------------------------------

def fetch_chapter_text(project_id: str, chapter_id: str) -> str:
    """Read a chapter's text. Tries the canonical key first, then any
    `projects/<id>/chapters/*.json` matching the chapter id is acceptable.
    """
    from app.settings import get_settings
    s = get_settings()
    candidates = [
        f"projects/{project_id}/chapters/{chapter_id}.json",
    ]
    for key in candidates:
        try:
            resp = get_client().get_object(s.s3_bucket, key)
        except Exception:
            continue
        try:
            data = resp.read()
        finally:
            resp.close()
            resp.release_conn()
        try:
            obj = json.loads(data.decode("utf-8"))
            return str(obj.get("content") or obj.get("text") or "")
        except Exception:
            return data.decode("utf-8", errors="replace")
    return ""


def _upload(key: str, body: bytes) -> None:
    from app.settings import get_settings
    s = get_settings()
    get_client().put_object(s.s3_bucket, key, BytesIO(body), len(body),
                            content_type="application/json")


def breakdown_sync(text: str, *, characters: list[str]) -> list:
    from app.settings import get_settings
    s = get_settings()
    providers = list(s.llm_providers.keys())
    provider = "doubao" if "doubao" in providers else (providers[0] if providers else "ollama")
    model = "doubao-pro-128k"
    return asyncio.run(scene_breakdown(text, characters=characters, provider=provider, model=model))
