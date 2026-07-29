"""Provider-agnostic LLM gateway using the OpenAI-compatible HTTP protocol.

Wraps the provider router with health-aware fallback.
"""
from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import Literal

import httpx

from app.infra.llm import router as provider_router
from app.settings import get_settings

Role = Literal["system", "user", "assistant", "tool"]


@dataclass(slots=True)
class ChatMessage:
    role: Role
    content: str


@dataclass(slots=True)
class ChatResult:
    content: str
    provider: str
    model: str
    input_tokens: int
    output_tokens: int
    cost_usd: float


async def chat(
    provider: str,
    model: str,
    messages: list[ChatMessage],
    *,
    api_key: str | None = None,
    temperature: float = 0.7,
    response_format_json: bool = False,
    timeout: float = 60.0,
) -> ChatResult:
    """Chat with health-aware fallback over the configured provider chain."""
    chain = [provider] + [p for p in provider_router.llm_chain() if p != provider]
    last_err: Exception | None = None
    for name in chain:
        try:
            result = await _call(name, model, messages, api_key=api_key,
                                 temperature=temperature,
                                 response_format_json=response_format_json,
                                 timeout=timeout)
            provider_router.record_provider_success(name)
            return result
        except Exception as exc:
            provider_router.record_provider_failure(name)
            last_err = exc
            log.warning("llm provider failed; trying next",
                        extra={"provider": name, "err": str(exc)})
            continue
    raise RuntimeError(f"all llm providers failed: {last_err}")


async def _call(
    name: str,
    model: str,
    messages: list[ChatMessage],
    *,
    api_key: str | None,
    temperature: float,
    response_format_json: bool,
    timeout: float,
) -> ChatResult:
    settings = get_settings()
    cfg = settings.llm_providers.get(name)
    if cfg is None:
        raise ValueError(f"unknown provider: {name}")
    base = cfg.get("base_url", "").rstrip("/")
    key = api_key or cfg.get("api_key") or ""
    payload: dict = {
        "model": model,
        "messages": [{"role": m.role, "content": m.content} for m in messages],
        "temperature": temperature,
    }
    if response_format_json:
        payload["response_format"] = {"type": "json_object"}
    headers = {"Authorization": f"Bearer {key}"} if key else {}

    async with httpx.AsyncClient(timeout=timeout) as cli:
        r = await cli.post(f"{base}/chat/completions", json=payload, headers=headers)
        if r.status_code >= 500:
            raise RuntimeError(f"provider {name} returned {r.status_code}")
        r.raise_for_status()
        body = r.json()

    choice = body["choices"][0]
    usage = body.get("usage", {})
    return ChatResult(
        content=choice["message"]["content"],
        provider=name,
        model=model,
        input_tokens=int(usage.get("prompt_tokens", 0)),
        output_tokens=int(usage.get("completion_tokens", 0)),
        cost_usd=0.0,
    )


async def embed(_provider: str, _model: str, _inputs: list[str]) -> list[list[float]]:
    return [[0.0] for _ in _inputs]


def providers() -> list[str]:
    return list(get_settings().llm_providers.keys())
