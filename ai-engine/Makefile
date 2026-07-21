.PHONY: install run-worker run-sidecar lint test

install:
	pip install -e ".[dev]"

run-worker:
	celery -A app.main.celery_app worker -Q ai -l info --concurrency=4

run-sidecar:
	uvicorn app.sidecar:app --host 0.0.0.0 --port 8000 --reload

lint:
	ruff check app

test:
	pytest -q
