"""Celery app entrypoint for ai-engine.

Run with:
    celery -A app.main.celery_app worker -Q ai -l info

All heavy pipeline steps live in app.tasks.* and are registered below.
"""
from __future__ import annotations

from celery import Celery

from app.settings import get_settings

settings = get_settings()

celery_app = Celery(
    "novel2av_ai",
    broker=settings.redis_url,
    backend=settings.redis_url,
    include=[
        "app.tasks.chapter_split",
        "app.tasks.character_extract",
        "app.tasks.character_image",
        "app.tasks.scene_breakdown",
        "app.tasks.generate_shot",
        "app.tasks.compose_chapter",
        "app.tasks.compose_full",
    ],
)

celery_app.conf.update(
    task_default_queue="ai",
    task_acks_late=True,
    task_reject_on_worker_lost=True,
    worker_prefetch_multiplier=1,
    task_time_limit=60 * 60,         # 1h hard limit per task
    task_soft_time_limit=50 * 60,
    broker_connection_retry_on_startup=True,
)
