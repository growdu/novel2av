"""Prometheus metrics + Celery signal wiring for the ai-engine.

The ai-engine runs in two process shapes:

* sidecar: a FastAPI process holding the debug API + (optionally) the
  internal callback HTTP server.  Its metrics endpoint listens on
  ``N2AV_SIDECAR_METRICS_PORT`` (default 9100).
* worker: a Celery worker (one or many forked processes).  Each worker
  process starts its own metrics HTTP listener on
  ``N2AV_WORKER_METRICS_PORT`` (default 9101).  In production with
  prefork workers you will want either ``--pool=solo`` for a single
  metrics surface, or ``PROMETHEUS_MULTIPROC_DIR`` aggregation
  (TODO M8).

Metric series are mirrored on both sides:

* ``n2av_ai_jobs_total``                  counter, labels: name, outcome
* ``n2av_ai_jobs_duration_seconds``       histogram, labels: name
* ``n2av_ai_providers_failures_total``    counter, labels: kind, provider
* ``n2av_ai_cache_total``                 counter, labels: op(get|put), result(hit|miss|error)
* ``n2av_ai_cache_duration_seconds``      histogram, labels: op
* ``n2av_ai_process_up``                  gauge (set to 1 after the
                                          metrics server is bound)

Operational ports are env-driven so a sidecar + worker co-located in one
container don't collide.
"""

from __future__ import annotations

import logging
import os
import socket
import time
from typing import Callable

from prometheus_client import (
    CollectorRegistry,
    Counter,
    Gauge,
    Histogram,
    start_http_server,
)

log = logging.getLogger(__name__)


def _new_registry() -> CollectorRegistry:
    """Build a fresh registry. We deliberately do NOT use the default global
    registry so test runs that touch metric state cannot leak samples between
    processes (pytest ordering, repl, etc.)."""
    return CollectorRegistry()


# Series -----------------------------------------------------------------


def _build_jobs_total(reg: CollectorRegistry) -> Counter:
    return Counter(
        "n2av_ai_jobs_total",
        "Celery tasks completed by name + outcome (success|failure|retry).",
        ["name", "outcome"],
        registry=reg,
    )


def _build_jobs_duration(reg: CollectorRegistry) -> Histogram:
    return Histogram(
        "n2av_ai_jobs_duration_seconds",
        "Celery task wall-clock latency in seconds.",
        ["name"],
        registry=reg,
        buckets=(0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600, 1800),
    )


def _build_providers_failures(reg: CollectorRegistry) -> Counter:
    return Counter(
        "n2av_ai_providers_failures_total",
        "Provider calls that returned a 5xx / timeout / non-recoverable error.",
        ["kind", "provider"],
        registry=reg,
    )


def _build_cache_total(reg: CollectorRegistry) -> Counter:
    return Counter(
        "n2av_ai_cache_total",
        "LLM response cache hits/misses/errors.",
        ["op", "result"],
        registry=reg,
    )


def _build_cache_duration(reg: CollectorRegistry) -> Histogram:
    return Histogram(
        "n2av_ai_cache_duration_seconds",
        "LLM response cache Redis get/put round-trip latency.",
        ["op"],
        registry=reg,
        buckets=(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5),
    )


# Module-level registries — one per logical process. They survive across
# import-time metric increments and across Celery signal-driven updates.
SIDECAR_REGISTRY: CollectorRegistry = _new_registry()
WORKER_REGISTRY: CollectorRegistry = _new_registry()

# Counter / Histogram / Gauge instances live on each registry. Tests +
# library users can grab them directly via these names.
S_JOBS_TOTAL = _build_jobs_total(SIDECAR_REGISTRY)
S_JOBS_DURATION = _build_jobs_duration(SIDECAR_REGISTRY)
S_PROVIDERS_FAILURES = _build_providers_failures(SIDECAR_REGISTRY)
S_CACHE_TOTAL = _build_cache_total(SIDECAR_REGISTRY)
S_CACHE_DURATION = _build_cache_duration(SIDECAR_REGISTRY)
S_PROCESS_UP = Gauge(
    "n2av_ai_process_up",
    "Set to 1 once the metrics HTTP server is bound.",
    registry=SIDECAR_REGISTRY,
)

W_JOBS_TOTAL = _build_jobs_total(WORKER_REGISTRY)
W_JOBS_DURATION = _build_jobs_duration(WORKER_REGISTRY)
W_PROVIDERS_FAILURES = _build_providers_failures(WORKER_REGISTRY)
W_CACHE_TOTAL = _build_cache_total(WORKER_REGISTRY)
W_CACHE_DURATION = _build_cache_duration(WORKER_REGISTRY)
W_PROCESS_UP = Gauge(
    "n2av_ai_process_up",
    "Set to 1 once the metrics HTTP server is bound.",
    registry=WORKER_REGISTRY,
)


# Public helpers ---------------------------------------------------------


def _pick_port(preferred: int, span: int = 20) -> int:
    """Find an unused TCP port starting at ``preferred``. A preferred of 0
    asks the OS for an ephemeral port. We scan ``[preferred, preferred+span)``
    so a Celery worker that preforked to several children can each own a
    distinct port (9101, 9102, ...). On a host that has no free port in
    range we raise so the caller can decide whether to keep going."""
    for off in range(span):
        port = preferred + off
        try:
            with socket.socket() as s:
                s.bind(("", port))
                return s.getsockname()[1]
        except OSError:
            continue
    raise RuntimeError(f"no free port in [{preferred}, {preferred + span})")


_started: dict[str, bool] = {"sidecar": False, "worker": False}


def start_sidecar_metrics_server() -> int:
    """Idempotently start the sidecar /metrics endpoint. Returns the
    actual port that ended up bound."""
    if _started["sidecar"]:
        return int(os.environ.get("N2AV_SIDECAR_METRICS_PORT_BOUND", "0"))
    preferred = int(os.environ.get("N2AV_SIDECAR_METRICS_PORT", "9100"))
    chosen = _pick_port(preferred)
    start_http_server(chosen, registry=SIDECAR_REGISTRY)
    S_PROCESS_UP.set(1)
    os.environ["N2AV_SIDECAR_METRICS_PORT_BOUND"] = str(chosen)
    _started["sidecar"] = True
    log.info("sidecar metrics listening", extra={"port": chosen})
    return chosen


def start_worker_metrics_server() -> int:
    """Idempotently start the worker /metrics endpoint. With Celery's prefork
    pool each child process calls this; the second and later calls find
    the port already bound and use the next one in the window."""
    if _started["worker"]:
        return int(os.environ.get("N2AV_WORKER_METRICS_PORT_BOUND", "0"))
    preferred = int(os.environ.get("N2AV_WORKER_METRICS_PORT", "9101"))
    try:
        chosen = _pick_port(preferred)
    except RuntimeError as exc:
        log.warning("worker metrics server unavailable: %s", exc)
        return 0
    start_http_server(chosen, registry=WORKER_REGISTRY)
    W_PROCESS_UP.set(1)
    os.environ["N2AV_WORKER_METRICS_PORT_BOUND"] = str(chosen)
    _started["worker"] = True
    log.info("worker metrics listening", extra={"port": chosen})
    return chosen


def reset_for_tests() -> None:
    """Tests use this between cases so module-global state doesn't bleed."""
    for started_key in _started:
        _started[started_key] = False


# Convenience wrappers so the cache + provider instrumentation stays a
# one-liner at each call site.


def cache_hit(provider: str = "", model: str = "") -> None:
    S_CACHE_TOTAL.labels("get", "hit").inc()
    W_CACHE_TOTAL.labels("get", "hit").inc()


def cache_miss(provider: str = "", model: str = "") -> None:
    S_CACHE_TOTAL.labels("get", "miss").inc()
    W_CACHE_TOTAL.labels("get", "miss").inc()


def cache_put(provider: str = "", model: str = "") -> None:
    S_CACHE_TOTAL.labels("put", "ok").inc()
    W_CACHE_TOTAL.labels("put", "ok").inc()


def cache_error(op: str = "get") -> None:
    S_CACHE_TOTAL.labels(op, "error").inc()
    W_CACHE_TOTAL.labels(op, "error").inc()


def provider_failure(kind: str, provider: str) -> None:
    S_PROVIDERS_FAILURES.labels(kind, provider).inc()
    W_PROVIDERS_FAILURES.labels(kind, provider).inc()


# Celery signal glue -----------------------------------------------------


def install_celery_hooks(celery_app) -> Callable[[], None]:
    """Attach worker_ready + task_prerun + task_postrun handlers to the
    given Celery app. Returns the disconnect function for tests that want
    to clean up after themselves."""
    from celery import signals

    start_worker_metrics_server()

    def on_worker_ready(**_):
        log.info("celery worker ready", extra={"pid": os.getpid()})

    def on_task_prerun(sender=None, task=None, **_):
        setattr(task, "_n2av_start_time", time.monotonic())

    def on_task_success(sender=None, result=None, task=None, **_):
        _record(task, "success")

    def on_task_failure(sender=None, task=None, exception=None, **_):
        _record(task, "failure")

    signals.worker_ready.connect(on_worker_ready)
    signals.task_prerun.connect(on_task_prerun)
    signals.task_success.connect(on_task_success)
    signals.task_failure.connect(on_task_failure)

    def disconnect() -> None:
        signals.worker_ready.disconnect(on_worker_ready)
        signals.task_prerun.disconnect(on_task_prerun)
        signals.task_success.disconnect(on_task_success)
        signals.task_failure.disconnect(on_task_failure)

    return disconnect


def _record(task, outcome: str) -> None:
    if task is None:
        return
    name = getattr(task, "name", None) or "unknown"
    elapsed = getattr(task, "_n2av_start_time", None)
    W_JOBS_TOTAL.labels(name, outcome).inc()
    if elapsed is not None:
        W_JOBS_DURATION.labels(name).observe(time.monotonic() - elapsed)
