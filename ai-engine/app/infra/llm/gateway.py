"""Provider-agnostic LLM gateway using the OpenAI-compatible HTTP protocol."""
from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

import httpx

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


def providers() -> list[str]:
    return list(get_settings().llm_providers.keys())


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
    """Issue a chat completion against the requested provider.

    The provider must speak OpenAI's `/chat/completions` shape.
    """
    settings = get_settings()
    cfg = settings.llm_providers.get(provider)
    if cfg is None:
        raise ValueError(f"unknown provider: {provider}")
    base = cfg["base_url"].rstrip("/")
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
        r.raise_for_status()
        body = r.json()

    choice = body["choices"][0]
    usage = body.get("usage", {})
    return ChatResult(
        content=choice["message"]["content"],
        provider=provider,
        model=model,
        input_tokens=int(usage.get("prompt_tokens", 0)),
        output_tokens=int(usage.get("completion_tokens", 0)),
        cost_usd=0.0,  # TODO: price table per provider/model
    )


async def embed(_provider: str, _model: str, _inputs: list[str]) -> list[list[float]]:
    """Stub embedding call. Will use the same gateway shape."""
    return [[0.0] for _ in _inputs]
