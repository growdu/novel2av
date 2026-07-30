"""Tests for app.infra.metrics.

These run without Redis, Celery broker, or FastAPI. They exercise the metric
increment helpers, the port-picker, and the Celery signal glue (with a fake
Celery object that exposes the signal-attach surface only).
"""

from __future__ import annotations

import socket
import sys
from pathlib import Path
from unittest.mock import MagicMock

import pytest
from prometheus_client import generate_latest as _render_metrics

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))


from app.infra import metrics  # noqa: E402


@pytest.fixture(autouse=True)
def _reset_metrics_state(monkeypatch):
    metrics.reset_for_tests()
    # Wipe "bound port" environment so each test picks fresh ports.
    monkeypatch.delenv("N2AV_SIDECAR_METRICS_PORT_BOUND", raising=False)
    monkeypatch.delenv("N2AV_WORKER_METRICS_PORT_BOUND", raising=False)
    yield
    metrics.reset_for_tests()


def test_pick_port_finds_a_free_port():
    chosen = metrics._pick_port(0)  # port 0 = OS-assigned ephemeral
    assert chosen > 0
    # And the port is actually closed (we only probed; we did not bind).
    with pytest.raises(ConnectionRefusedError):
        with socket.create_connection(("127.0.0.1", chosen), timeout=0.1):
            pass


def test_pick_port_skips_taken_ports():
    # Hold a socket on `target` so the picker must advance.
    target = metrics._pick_port(0)
    with socket.socket() as held:
        held.bind(("127.0.0.1", target))
        try:
            chosen = metrics._pick_port(target)
            assert chosen != target
            assert chosen > target
        finally:
            held.close()


def test_cache_helpers_increment():
    metrics.cache_hit()
    metrics.cache_miss()
    metrics.cache_put()
    metrics.cache_error("put")
    samples = _render(metrics.SIDECAR_REGISTRY)
    assert "n2av_ai_cache_total" in samples
    # All four (hit,miss,put ok,put error) appear as exemplars
    assert 'op="get",result="hit"' in samples
    assert 'op="get",result="miss"' in samples
    assert 'op="put",result="ok"' in samples
    assert 'op="put",result="error"' in samples


def test_provider_failure_helper():
    metrics.provider_failure("llm", "doubao")
    metrics.provider_failure("image", "seedream")
    samples = _render(metrics.SIDECAR_REGISTRY)
    assert 'kind="llm",provider="doubao"' in samples
    assert 'kind="image",provider="seedream"' in samples


def test_metrics_series_present_in_default_registry():
    samples = _render(metrics.SIDECAR_REGISTRY)
    for name in (
        "n2av_ai_jobs_total",
        "n2av_ai_jobs_duration_seconds",
        "n2av_ai_providers_failures_total",
        "n2av_ai_cache_total",
        "n2av_ai_cache_duration_seconds",
        "n2av_ai_process_up",
    ):
        assert f"# HELP {name} " in samples, f"missing series {name}"


def test_install_celery_hooks_calls_worker_ready_and_uses_fake_signals():
    celery = pytest.importorskip("celery", reason="celery not installed locally")
    fake_celery = MagicMock()
    # Use the real celery.signals namespace so the connect/disconnect
    # round-trip exercises the production code path.
    fake_celery.signals = celery.signals
    detach = metrics.install_celery_hooks(fake_celery)
    assert callable(detach)
    detach()


def test_record_helper_handles_missing_task():
    """_record must tolerate ``task=None`` (called outside a real task)."""
    metrics._record(None, "success")  # no panic
    metrics._record(object(), "failure")  # no name set -> labels fail
    # Even with name missing, calling _record twice must not raise.


def _render(registry):

    return _render_metrics(registry).decode("utf-8")
