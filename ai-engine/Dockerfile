FROM python:3.11-slim AS base
ENV PIP_NO_CACHE_DIR=1 PYTHONUNBUFFERED=1
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY pyproject.toml ./
RUN pip install --upgrade pip && pip install -e .
COPY app ./app
EXPOSE 8000
CMD ["celery", "-A", "app.main.celery_app", "worker", "-Q", "ai", "-l", "info", "--concurrency=4"]
