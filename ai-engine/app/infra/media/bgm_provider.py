"""BGM provider abstraction.

Default behaviour: synthesize a soft ambient loop of `duration_sec` using a
deterministic sine-wave generator so the pipeline produces a real BGM track
without external dependencies. If a `musicgen` endpoint is configured via
env (`AI_BGM_ENDPOINT`), the provider POSTs the prompt there and decodes
the response as bytes (audio/mpeg).

Operators can replace the default implementation with Suno/Udio by pointing
at their endpoint and decoding the returned audio bytes.
"""
from __future__ import annotations

import io
import logging
import math
import os
import struct
import wave

import httpx

log = logging.getLogger(__name__)


async def generate_bgm(
    prompt: str,
    *,
    duration_sec: float,
    mood: str = "",
    style: str = "cinematic",
) -> tuple[bytes, str]:
    """Return (audio_bytes, content_type). Falls back to a soft loop on error."""
    s = min(max(duration_sec, 4.0), 180.0)
    endpoint = os.environ.get("AI_BGM_ENDPOINT")
    if endpoint:
        try:
            async with httpx.AsyncClient(timeout=180) as cli:
                r = await cli.post(endpoint, json={
                    "prompt": prompt,
                    "duration_sec": s,
                    "mood": mood,
                    "style": style,
                })
                r.raise_for_status()
                return r.content, r.headers.get("content-type", "audio/wav")
        except Exception as exc:
            log.warning("bgm provider failed; using fallback", extra={"err": str(exc)})
    return soft_loop_wav(s, mood=mood), "audio/wav"


def soft_loop_wav(duration_sec: float, *, mood: str = "", sample_rate: int = 22050) -> bytes:
    """A deterministic soft pad WAV. Pleasant enough for preview, never mistaken
    for real music. Different moods get slightly different chord shapes."""
    seed = sum(ord(c) for c in mood) or 1
    base = 110.0 + (seed % 7) * 4.0  # ~A2 with small offset
    chord = [base, base * 5 / 4, base * 3 / 2]  # root + maj3 + p5
    n_samples = int(sample_rate * duration_sec)
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(sample_rate)
        chunk = bytearray()
        for i in range(n_samples):
            t = i / sample_rate
            env = 0.5 + 0.5 * math.sin(2 * math.pi * 0.1 * t)  # slow swell
            sample = 0.0
            for f in chord:
                sample += math.sin(2 * math.pi * f * t)
            sample = sample / len(chord) * env * 0.18  # quiet
            chunk += struct.pack("<h", int(sample * 32767))
            if len(chunk) > 65536:
                w.writeframes(bytes(chunk))
                chunk = bytearray()
        if chunk:
            w.writeframes(bytes(chunk))
    return buf.getvalue()
