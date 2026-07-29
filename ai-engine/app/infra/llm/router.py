"""Provider router for LLM / Image / TTS.

Goals:
  - health-aware fallback: if the primary provider returns 5xx or times out,
    transparently try the next one in the chain.
  - capability check: provider must expose `/chat/completions`; image via
    `/images/generations` (OpenAI-compatible).
  - tiny circuit breaker: consecutive failures within a short window
    temporarily demote the provider.

Order is configured via env:
  AI_LLM_PROVIDER_ORDER=doubao,deepseek,ollama
  AI_IMAGE_PROVIDER_ORDER=seedream,comfyui
  AI_TTS_PROVIDER_ORDER=doubao,edge

Health state is in-process (per worker); a Redis-shared view would be the
next iteration (M8+).
"""
from __future__ import annotations

import logging
import os
import time
from dataclasses import dataclass

log = logging.getLogger(__name__)


@dataclass
class CircuitState:
    failures: int = 0
    open_until: float = 0.0


_CIRCUIT: dict[str, CircuitState] = {}
CIRCUIT_THRESHOLD = 3
CIRCUIT_COOLDOWN = 30.0  # seconds


def _chain(kind: str, default: list[str]) -> list[str]:
    raw = os.environ.get(f"AI_{kind}_PROVIDER_ORDER", "")
    chain = [p.strip() for p in raw.split(",") if p.strip()]
    if not chain:
        chain = default
    return chain


def _is_open(name: str) -> bool:
    cs = _CIRCUIT.get(name)
    if not cs:
        return False
    return cs.open_until > time.monotonic()


def _record_failure(name: str) -> None:
    cs = _CIRCUIT.setdefault(name, CircuitState())
    cs.failures += 1
    if cs.failures >= CIRCUIT_THRESHOLD:
        cs.open_until = time.monotonic() + CIRCUIT_COOLDOWN
        log.warning("circuit opened", extra={"provider": name, "cooldown": CIRCUIT_COOLDOWN})


def _record_success(name: str) -> None:
    cs = _CIRCUIT.get(name)
    if not cs:
        return
    cs.failures = 0
    cs.open_until = 0.0


# --- selection -------------------------------------------------------------

_LLM_DEFAULT = ["doubao", "deepseek", "ollama"]
_IMAGE_DEFAULT = ["seedream", "comfyui"]
_TTS_DEFAULT = ["doubao", "edge"]


def llm_chain() -> list[str]:
    chain = _chain("LLM", _LLM_DEFAULT)
    return [p for p in chain if not _is_open(p)]


def image_chain() -> list[str]:
    chain = _chain("IMAGE", _IMAGE_DEFAULT)
    return [p for p in chain if not _is_open(p)]


def tts_chain() -> list[str]:
    chain = _chain("TTS", _TTS_DEFAULT)
    return [p for p in chain if not _is_open(p)]


def record_provider_failure(name: str) -> None:
    _record_failure(name)


def record_provider_success(name: str) -> None:
    _record_success(name)


# --- helpers ---------------------------------------------------------------

def health_summary() -> dict:
    now = time.monotonic()
    return {
        name: {
            "failures": cs.failures,
            "circuit_open_until": max(0.0, cs.open_until - now),
        }
        for name, cs in _CIRCUIT.items()
    }
