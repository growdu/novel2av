"""Split raw novel text into chapters using rule + LLM.

Pipeline (best effort, with fallbacks):
  1. Pull source text from MinIO.
  2. Run rule-based split (`rule_split`).
  3. If rule split yields 0 or <2 chapters, fall back to `llm_split`.
  4. Upload each chapter's slice as a JSON file (`projects/<id>/chapters/<n>.json`).
  5. Report progress and return the slice list.

Note: this worker does NOT write to Postgres. The Go backend writes the
canonical `chapters` rows when it consumes the result via a completion
callback (or polls `ai_results:<job_id>`).
"""
from __future__ import annotations

import json
import logging
from io import BytesIO

from celery import shared_task

from app.infra.queue.progress import report_progress
from app.infra.queue.notify import notify_complete
from app.infra.storage import get_client
from app.schemas.payloads import SplitChaptersPayload
from app.services.chapter_service import ChapterSlice, llm_split, rule_split

log = logging.getLogger(__name__)


@shared_task(name="ai:split_chapters", bind=True, max_retries=3, default_retry_delay=10)
def split_chapters(self, payload: dict) -> dict:
    parsed = SplitChaptersPayload.model_validate(payload)
    log.info("split_chapters start", extra={"project_id": parsed.project_id})
    report_progress(self.request.id, "running", 0, 4, "fetching source", project_id=parsed.project_id)

    text = _download_source(parsed.source_key)
    log.info("fetched source", extra={"bytes": len(text)})

    report_progress(self.request.id, "running", 1, 4, "rule split", project_id=parsed.project_id)
    slices = rule_split(text)
    used_llm = False
    if len(slices) < 2:
        # Unusual formatting: ask the LLM to do the heavy lifting.
        used_llm = True
        try:
            slices = llm_split_sync(text)
        except Exception as exc:  # pragma: no cover
            log.warning("llm split failed", extra={"err": str(exc)})
            if not slices:
                # Whole book becomes a single chapter so the user can fix by hand.
                slices = [ChapterSlice(1, "第一章", 0, len(text))]

    report_progress(self.request.id, "running", 2, 4, "uploading chapters", project_id=parsed.project_id)
    for s in slices:
        chunk_key = f"projects/{parsed.project_id}/chapters/{s.index}.json"
        body = json.dumps({
            "index": s.index,
            "title": s.title,
            "start_offset": s.start_offset,
            "end_offset": s.end_offset,
            "word_count": _count_words(text[s.start_offset:s.end_offset]),
            "content": text[s.start_offset:s.end_offset],
        }, ensure_ascii=False).encode("utf-8")
        _upload(chunk_key, body, "application/json")

    report_progress(self.request.id, "running", 3, 4, "storing result", project_id=parsed.project_id)
    result_key = f"results/{parsed.project_id}/split_chapters.json"
    _upload(result_key, json.dumps({
        "chapters": [
            {"index": s.index, "title": s.title,
             "start_offset": s.start_offset, "end_offset": s.end_offset,
             "key": f"projects/{parsed.project_id}/chapters/{s.index}.json"}
            for s in slices
        ],
        "used_llm": used_llm,
    }, ensure_ascii=False).encode("utf-8"), "application/json")

    report_progress(self.request.id, "success", 4, 4, "done", project_id=parsed.project_id)
    result = {"status": "ok", "project_id": parsed.project_id,
              "chapter_count": len(slices), "used_llm": used_llm,
              "result_key": result_key}
    notify_complete(self.request.id, {
        "task": self.name,
        "project_id": parsed.project_id,
        "payload": payload,
        "result": result,
    })
    return result


# --- helpers ---------------------------------------------------------------

def _download_source(key: str) -> str:
    from app.settings import get_settings
    s = get_settings()
    resp = get_client().get_object(s.s3_bucket, key)
    try:
        data = resp.read()
    finally:
        resp.close()
        resp.release_conn()
    return data.decode("utf-8", errors="replace")


def _upload(key: str, body: bytes, content_type: str) -> None:
    from app.settings import get_settings
    s = get_settings()
    get_client().put_object(s.s3_bucket, key, BytesIO(body), len(body),
                            content_type=content_type)


def _count_words(text: str) -> int:
    # Chinese: count non-whitespace runes. Latin: word count. Mixed takes both.
    chinese = sum(1 for ch in text if not ch.isspace() and ord(ch) > 0x4E00)
    latin_words = len([w for w in text.split() if any(c.isalpha() for c in w)])
    return chinese + latin_words


def llm_split_sync(text: str) -> list[ChapterSlice]:
    """Sync wrapper around the async llm_split for Celery workers."""
    import asyncio
    from app.settings import get_settings
    s = get_settings()
    providers = list(s.llm_providers.keys())
    provider = "doubao" if "doubao" in providers else (providers[0] if providers else "ollama")
    model = "doubao-pro-128k"  # operators override via prompt YAML / env
    return asyncio.run(llm_split(text, provider=provider, model=model))
