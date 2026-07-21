"""Typer CLI for ai-engine — debug and batch helpers.

Examples:
    python -m app.cli providers
    python -m app.cli ping
    python -m app.cli enqueue ai:split_chapters --project-id <uuid> --source-key novels/x/source.txt
"""
from __future__ import annotations

import json

import typer
from rich import print

from app.settings import get_settings

cli = typer.Typer(help="novel2av ai-engine CLI")


@cli.command()
def providers() -> None:
    """Print active provider config."""
    s = get_settings()
    print(json.dumps({
        "llm": s.llm_providers,
        "image": s.image_providers,
        "tts": s.tts_providers,
    }, indent=2))


@cli.command()
def ping() -> None:
    """Liveness probe."""
    print("[green]ok[/green]")


@cli.command()
def enqueue(
    task: str = typer.Option(..., "--task", help="ai:split_chapters etc."),
    payload: str = typer.Option("{}", "--payload", help="JSON payload string"),
) -> None:
    """Enqueue a task via Celery (requires running broker)."""
    from app.main import celery_app
    celery_app.send_task(task, kwargs=json.loads(payload))
    print(f"[green]enqueued {task}[/green]")


if __name__ == "__main__":
    cli()
