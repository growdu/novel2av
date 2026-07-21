# novel2av-backend

Go platform backend. Owns the project/chapter/character/shot/job state machine
in Postgres, exposes REST + WebSocket under `/api/v1`, and enqueues pipeline
tasks to Redis (asynq) for the Python AI engine to consume.

See [`../docs/03-backend-design.md`](../docs/03-backend-design.md) for the full design.

## Quickstart

```bash
cp .env.example .env
make run-api     # start API on :8080
make run-cli     # CLI: novel2av --help
make test
```

## Layout

```
cmd/api/      HTTP entrypoint
cmd/cli/      Cobra CLI (shares services with API)
internal/
  config/     Viper config loader
  httpapi/    chi router + handlers + ws
  domain/     plain entities + sentinel errors
  service/    use cases (pipeline, asset, event hub)
  infra/
    db/       pgx pool
    storage/  MinIO client
    queue/    asynq client (Go enqueues, Python Celery consumes)
    observability/
migrations/   plain SQL, applied via `novel2av migrate up`
```
