"""Cache-aware LLM call helper.

Wraps `app.infra.llm.gateway.chat` with Redis-backed caching of JSON
responses. Falls back to uncached call when cache isn't reachable.
"""
from __future__ import annotations

import json
import logging
from typing import Any

from app.infra.llm import cache as llm_cache
from app.infra.llm.gateway import ChatMessage, ChatResult, chat

log = logging.getLogger(__name__)


async def chat_cached(
    provider: str,
    model: str,
    messages: list[ChatMessage],
    *,
    response_format_json: bool = False,
    **kwargs: Any,
) -> ChatResult:
    payload = {
        "messages": [{"role": m.role, "content": m.content} for m in messages],
        "response_format_json": response_format_json,
        "kwargs": kwargs,
    }
    cached = llm_cache.get(provider, model, payload)
    if cached:
        log.info("llm cache hit", extra={"provider": provider, "model": model})
        return ChatResult(
            content=cached["content"],
            provider=cached["provider"],
            model=cached["model"],
            input_tokens=cached.get("input_tokens", 0),
            output_tokens=cached.get("output_tokens", 0),
            cost_usd=cached.get("cost_usd", 0.0),
        )
    res = await chat(provider, model, messages,
                     response_format_json=response_format_json, **kwargs)
    llm_cache.put(provider, model, payload, {
        "content": res.content, "provider": res.provider, "model": res.model,
        "input_tokens": res.input_tokens, "output_tokens": res.output_tokens,
        "cost_usd": res.cost_usd,
    })
    return res
