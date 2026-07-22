"""TTS provider abstraction.

Default provider is `doubao` (Volcengine TTS). The fallback path generates a
silent WAV of `duration_sec` so the pipeline keeps running even when no API
key is configured (CI / smoke tests).
"""
from __future__ import annotations

import asyncio
import base64
import io
import logging
import os
import struct
import wave

import httpx

from app.settings import get_settings

log = logging.getLogger(__name__)


async def synthesize_speech(
    text: str,
    *,
    provider: str | None = None,
    voice_id: str | None = None,
    speed: float = 1.0,
) -> tuple[bytes, str]:
    """Return (wav_bytes, provider). Falls back to silence on error."""
    s = get_settings()
    p = provider or s.default_tts_provider
    cfg = s.tts_providers.get(p, {})
    base_url = cfg.get("base_url", "").rstrip("/")
    api_key = cfg.get("api_key") or os.environ.get("AI_TTS_API_KEY") or ""

    if not base_url or not api_key or not text.strip():
        log.warning("tts provider not configured; returning silence", extra={"provider": p})
        return silence_wav(text), p

    payload = {
        "model": cfg.get("default_model") or "doubao-tts",
        "input": text,
        "voice": voice_id or cfg.get("default_voice") or "zh_female_warm",
        "speed": speed,
        "response_format": "wav",
    }
    headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}
    try:
        async with httpx.AsyncClient(timeout=60) as cli:
            r = await cli.post(f"{base_url}/audio/speech", json=payload, headers=headers)
            if r.status_code >= 400:
                log.warning("tts provider error; falling back to silence",
                            extra={"status": r.status_code})
                return silence_wav(text), p
            return r.content, p
    except Exception as exc:  # pragma: no cover
        log.warning("tts call failed", extra={"err": str(exc)})
        return silence_wav(text), p


def silence_wav(text: str, sample_rate: int = 22050) -> bytes:
    """A WAV file containing roughly the duration needed to read `text` aloud.

    Duration estimate: 3.5 chars/sec for CJK + 2.2 words/sec for latin.
    """
    chinese = sum(1 for ch in text if not ch.isspace() and ord(ch) > 0x4E00)
    latin = len([w for w in text.split() if any(c.isalpha() for c in w)])
    seconds = max(1.5, chinese / 3.5 + latin / 2.2)
    n_samples = int(sample_rate * seconds)
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(sample_rate)
        w.writeframes(b"\x00\x00" * n_samples)
    return buf.getvalue()


def voice_for(profile: str, provider: str | None = None) -> str | None:
    """Resolve a voice id from a profile name (e.g. "male_adult_calm").

    Falls back to provider default when no mapping exists.
    """
    s = get_settings()
    p = provider or s.default_tts_provider
    try:
        import yaml
        from pathlib import Path
        path = Path(__file__).resolve().parents[2] / "prompts" / "tts_voice_profile.yaml"
        with path.open("r", encoding="utf-8") as f:
            cfg = yaml.safe_load(f)
        profiles = cfg.get("profiles", {}) if isinstance(cfg, dict) else {}
        entry = profiles.get(profile)
        if isinstance(entry, dict):
            return entry.get(p)
    except Exception:
        return None
    return None
