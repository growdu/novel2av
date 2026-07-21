# novel2av-ai-engine

Python AI engine. Consumes pipeline tasks enqueued by the Go backend (Redis),
calls LLM / image / TTS / BGM providers, and writes media back to MinIO.

Two processes:

- **Celery worker** — does the work: `celery -A app.main.celery_app worker -Q ai -l info`
- **FastAPI sidecar** — debug/health: `uvicorn app.sidecar:app --port 8000`

## Layout

```
app/
  main.py            Celery app + task wiring
  sidecar.py         FastAPI debug server
  settings.py        pydantic-settings
  cli.py             Typer debug CLI
  api/               sidecar routes
  services/          business services (chapter split, characters, shots, compose)
  tasks/             one Celery task per pipeline step
  infra/
    llm/             OpenAI-compatible LLM gateway
    media/           ffmpeg wrappers
    storage/         MinIO client
    queue/           progress reporting
  prompts/           versioned YAML prompts
  schemas/           Pydantic payload models
```

## Provider conventions

All providers speak OpenAI's `/chat/completions` protocol. To add one:

1. Add a config block under `AI_LLM_PROVIDERS__<NAME>__...` env vars.
2. Ensure `base_url` points to the OpenAI-compatible endpoint.
3. Set `api_key`.

Image providers follow the same OpenAI image API where applicable; for
provider-specific protocols (Seedream, ComfyUI), wrap in `infra/media`.
