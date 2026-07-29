"""LLM response cache backed by Redis.

Key shape: `llm_cache:v1:<sha256(prompt+model+provider)>[:response_format]`
TTL: 30 days (configurable).
"""
from __future__ import annotations

import hashlib
import json
import logging
import os
from typing import Any

log = logging.getLogger(__name__)

CACHE_TTL_SECONDS = 30 * 24 * 3600


def _redis():
    url = os.environ.get("AI_REDIS_URL")
    if not url:
        return None
    try:
        import redis  # type: ignore
    except Exception:
        return None
    try:
        return redis.Redis.from_url(url)
    except Exception:
        return None


def _key(provider: str, model: str, payload: dict) -> str:
    body = json.dumps(payload, sort_keys=True, ensure_ascii=False).encode("utf-8")
    h = hashlib.sha256()
    h.update(provider.encode())
    h.update(b"|")
    h.update(model.encode())
    h.update(b"|")
    h.update(body)
    return f"llm_cache:v1:{h.hexdigest()}"


def get(provider: str, model: str, payload: dict) -> dict | None:
    r = _redis()
    if r is None:
        return None
    try:
        raw = r.get(_key(provider, model, payload))
    except Exception as exc:  # pragma: no cover
        log.warning("llm cache get failed", extra={"err": str(exc)})
        return None
    if not raw:
        return None
    try:
        return json.loads(raw)
    except Exception:
        return None


def put(provider: str, model: str, payload: dict, result: dict) -> None:
    r = _redis()
    if r is None:
        return
    try:
        r.set(_key(provider, model, payload), json.dumps(result, ensure_ascii=False),
              ex=CACHE_TTL_SECONDS)
    except Exception as exc:  # pragma: no cover
        log.warning("llm cache put failed", extra={"err": str(exc)})
