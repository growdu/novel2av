"""FastAPI sidecar for debug + health. Disabled by default in production."""
from __future__ import annotations

from fastapi import FastAPI

from app.infra.metrics import start_sidecar_metrics_server
from app.settings import get_settings

app = FastAPI(title="novel2av-ai-engine sidecar", version="0.1.0")
_settings = get_settings()

# Bind the Prometheus /metrics HTTP server at import time so it shares
# the sidecar process lifecycle. ``start_sidecar_metrics_server`` is
# idempotent so re-imports (tests, reload) keep the original socket.
start_sidecar_metrics_server()


@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok"}


@app.get("/providers")
def providers() -> dict:
    return {
        "llm": list(_settings.llm_providers.keys()),
        "image": list(_settings.image_providers.keys()),
        "tts": list(_settings.tts_providers.keys()),
        "defaults": {
            "image": _settings.default_image_provider,
            "tts": _settings.default_tts_provider,
            "aspect": _settings.default_aspect,
        },
    }
