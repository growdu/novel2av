"""Image generation provider abstraction.

Default provider is `seedream` (Volcengine Ark image endpoint, OpenAI-compatible).
The fallback path simply returns a 1x1 placeholder PNG so the pipeline keeps
running even when no API key is configured (useful in CI / smoke tests).
"""
from __future__ import annotations

import base64
import io
import logging
import os
from dataclasses import dataclass

import httpx

from app.settings import get_settings

log = logging.getLogger(__name__)


@dataclass(slots=True)
class GeneratedImage:
    png_bytes: bytes
    width: int
    height: int
    provider: str
    model: str


async def generate_image(
    prompt: str,
    *,
    provider: str | None = None,
    model: str | None = None,
    width: int = 1024,
    height: int = 1024,
    reference_images: list[bytes] | None = None,
) -> GeneratedImage:
    """Generate a single image for a character / scene.

    The reference_images list (optional) is sent to providers that support
    `image_reference` (e.g. Seedream); other providers ignore it.
    """
    s = get_settings()
    p = provider or s.default_image_provider
    cfg = s.image_providers.get(p, {})
    base_url = cfg.get("base_url", "").rstrip("/")
    api_key = cfg.get("api_key") or os.environ.get("AI_IMAGE_API_KEY") or ""

    if not base_url or not api_key:
        log.warning("image provider not configured; using placeholder", extra={"provider": p})
        return _placeholder(width, height, p)

    payload: dict = {
        "model": model or cfg.get("default_model") or "doubao-seedream-3-0-t2i-250415",
        "prompt": prompt,
        "size": f"{width}x{height}",
        "response_format": "b64_json",
    }
    if reference_images:
        payload["image"] = [base64.b64encode(b).decode("ascii") for b in reference_images]

    headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}
    async with httpx.AsyncClient(timeout=120) as cli:
        r = await cli.post(f"{base_url}/images/generations", json=payload, headers=headers)
        if r.status_code >= 400:
            log.error("image provider error", extra={"status": r.status_code, "body": r.text[:500]})
            return _placeholder(width, height, p)
        body = r.json()
        b64 = body["data"][0].get("b64_json")
        if not b64:
            return _placeholder(width, height, p)
        raw = base64.b64decode(b64)
    return GeneratedImage(png_bytes=raw, width=width, height=height, provider=p,
                          model=payload["model"])


def _placeholder(width: int, height: int, provider: str) -> GeneratedImage:
    """A 1x1 PNG with provider name baked into a tiny tEXt chunk.

    This avoids hard-coding bytes here; Pillow produces it lazily.
    """
    from PIL import Image, PngImagePlugin

    img = Image.new("RGB", (width, height), color=(64, 70, 86))
    meta = PngImagePlugin.PngInfo()
    meta.add_text("provider", provider[:64])
    buf = io.BytesIO()
    img.save(buf, format="PNG", pnginfo=meta)
    return GeneratedImage(png_bytes=buf.getvalue(), width=width, height=height,
                          provider=provider, model="placeholder")
