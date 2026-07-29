# M8 — 可观测与限流（实施计划）

> 目标：让一个 5 本/天的小流量系统也能在事故时 5 分钟内定位，限额不会被人刷爆。
> 范围：开源的 trace + metrics 体系、per-user 配额、对高频路径的速率限制。
> 不在范围（推到 M9）：K8s 部署、备份、CSP / API Key 加密、真实 auth / magic link。

## 0. 当前基线（2026‑07‑29 摸底）

- `backend/internal/infra/observability/observability.go` 只做了 `slog` JSON handler，**OTel 没初始化**，OTLP endpoint 只挂在 config 上没人用。
- `go.opentelemetry.io/otel v1.31.0` 进了 `go.mod` 间接依赖，没有任何 import。
- 没有 Prometheus 客户端，**没有 `/metrics`**。
- ai-engine 只有 `app/main.py` + `app/sidecar.py`，**没有 OpenTelemetry / Prometheus 客户端依赖**。
- `app/services/_cache.py` 的 chat_cached 已有 `log.info("llm cache hit", ...)` 但**没有指标计数器**。
- backend 队列侧（`internal/infra/queue/asynq.go`）无 Prometheus 队列深度采样。

结论：M8 是真正的"从零搭"，**不存在"接几行就完事"的捷径**。

---

## 1. 三个时间盒

| 框 | 工作量 | 重点 | 阻塞条件 |
|---|---|---|---|
| **A — 本周（5 工日）** | 1 人 | 指标 + 基础 trace 通路 | 无 |
| **B — 接下来 2 周（10 工日）** | 1 人 | 完整 trace 传播、cache 命中率、配额 + 限流 | A 完成 |
| **C — 下个月（20 工日）** | 2 人 | 前端可观测 / 告警 / 接 M7 漏项 + M9 起步 | B 完成 |

---

## 2. A — 本周（地基 + 第一批指标）

### A1. 后端 Prometheus 暴露 + 默认 runtime 指标 — 0.5 d

- 引入 `github.com/prometheus/client_golang/prometheus` + `promhttp`。
- `internal/infra/observability/metrics.go` 新增：
  ```
  var (
    Registry *prometheus.Registry
    HTTPRequests *prometheus.HistogramVec  // method, route, status
    ProcessCollectors []prometheus.Collector  // process_*, go_*
  )
  ```
- 启动时 `observability.Setup(cfg.LogLevel)` 顺手加一个 `SetupMetrics()` 把 `Registry` 初始化好。
- `internal/httpapi/router.go` 公开路由挂一行 `r.Handle("/metrics", promhttp.HandlerFor(Registry, ...))`，应用 chi middleware 抽 `chi.RouteContext` 取路由模板当 label。
- 验收：`curl :8080/metrics | grep go_goroutines` 能看到。

### A2. 后端 HTTP 请求直方图 — 0.5 d

- 自定义 middleware `httpapi/middleware_metrics.go`，对每个请求打：
  - `http_requests_total{method,route,status}` counter
  - `http_request_duration_seconds{method,route}` histogram（bucket 0.01s..10s）
- 排除 `/metrics` 自身（无限递归刷自己）。

### A3. 后端业务基础指标 — 1 d

在 `internal/service/services.go` 旁 `internal/service/metrics.go` 注册：

| 指标 | 类型 | label | 来源 |
|---|---|---|---|
| `n2av_jobs_enqueued_total` | counter | step | pipeline_service.EnqueueStep |
| `n2av_jobs_completed_total` | counter | step, outcome(success/failed) | 内部回调 ingest 路径 |
| `n2av_jobs_duration_seconds` | histogram | step | ingest 时算 now-queued |
| `n2av_providers_failures_total` | counter | kind(llm/image/tts), provider | 失败回调 |
| `n2av_ws_connections_active` | gauge | — | ws.go |
| `n2av_redis_ping_seconds` | histogram | — | svc.Ping |

> 注意：此处不应包含 cost 指标（等 B2 加价格表再说）。

### A4. ai-engine sidecar Prometheus — 0.5 d

- `pyproject.toml` 加 `prometheus-client`。
- `app/sidecar.py` 启动时 `start_http_server(9100)` 在专用端口（不影响主 sidecar API）。
- 加 `app/infra/metrics.py`，导出和后端对齐的指标名（命名空间 `n2av_ai_`，避免两端口拼在一起时冲突）。

### A5. ai-engine Celery worker 指标 — 0.5 d

- 同上，但启动在 `prometheus_client.start_http_server(9101)`（worker 进程），共享同一份 `MetricsRegistry`。
- Celery 自带 `celery.events.state` 可以接到 `worker_ready` / `task_succeeded` / `task_failed` —— 用 `celery.signals` 钩到指标。

### A6. 后端 OTel Tracer 初始化 — 1 d

- `observability.Setup` 增加 `SetupTracing(otlpEndpoint string)`：
  - 用 `sdktrace.NewTracerProvider(WithBatcher(exporter), WithResource(...))`
  - 缺省 endpoint 时退回到 `noop.NewTracerProvider()`（开发态）
- `cmd/api/main.go` 在启动早期调用一次。
- `cmd/cli/main.go` 同样不要漏（CLI 跑任务也得带 trace）。

### A7. 后端 Postgres 慢查询 span — 1 d

- 用 `pgx/v5` 自带的 `tracelog` 或 `otelpgx.Wrap`，包一层 `*pgxpool.Pool`。
- `internal/infra/db/db.go` 在 `NewPool` 时打开。
- 验收：在 ai-engine 实际跑一次 `chapter_split`，后端 OTel UI 能看到 `pg.query` span，sql 文本可见。

**A 盒产出**：`/metrics` 起来，业务指标首批上线（仅 counter / histogram / gauge），OTel trace 后端单进程内可观测。完成后才能进 B。

---

## 3. B — 接下来 2 周（连接 + 配额 + 限流）

### B1. ai-engine OTel 初始化 — 1 d

- `app/infra/observability/tracing.py` 镜像 `metrics.py` 的 SetupTracing 接口，按 `OTEL_EXPORTER_OTLP_ENDPOINT` 启/不启。
- Celery worker 启动时调用一次。
- FastAPI sidecar 同样调一次。

### B2. ai-engine 各 provider span — 1 d

- `app/infra/llm/gateway.py:chat` 包一层 `with tracer.start_as_current_span("llm.chat")`，attributes：
  - `provider`、`model`、`input_tokens`、`output_tokens`、`cache_hit`、`duration_ms`
- `image_provider.generate_image` / `tts_provider.synthesize_speech` 同样包。
- 失败时 `span.record_exception(exc)` + 设置 status。

### B3. 后端 ↔ ai-engine trace 传播 — 2 d

- 后端入队 `ai:split_chapters` 任务 payload 增加 `traceparent` 字段（W3C trace context）。
- ai-engine 在 task 入口处用 `opentelemetry.propagate.extract` 恢复 parent span。
- ai-engine 内子 span 都挂在同一个 trace tree 下。
- 验收：Jaeger / Tempo 上跨进程 trace 是一条线。
- 复杂度点：payload schema 改了要走"6 号文档（data-model）+ 后端 RFC"流程，提交前先开 issue 通告。

### B4. 后端 chi OTel middleware — 0.5 d

- `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` 或 chi 专用 `otelmw`。
- 每个 HTTP 请求起一个 server span。

### B5. WebSocket 链路接入 — 0.5 d

- `infra/queue/ws.go` 升级事件 publish 时同时写入 OTel baggage / extra attributes，便于前端 trace 跟踪。
- 前端 `/lib/ws` 加 `traceparent` 头生成与回传。

### B6. 业务 cache 命中率与延迟指标 — 0.5 d

- `app/infra/llm/cache.py:get/put` 在 Redis 可用时上报指标：
  - `n2av_ai_cache_total{op=get|put,result=hit|miss|error}` counter
  - `n2av_ai_cache_duration_seconds{op=get|put}` histogram

### B7. 错误类型差异化的 Celery 重试（A 漏项） — 1 d

> 这块严格说属 M7 收尾，但需要指标先有 fallback 依据，所以落在 B。

- 自定义 `classify_exception()`：`HTTPError 4xx` 不重试（业务错误），`5xx / Timeout` 重试 3 次指数退避（前 60s、5min、25min）。
- asynq 那边 `asynq.NewTask` 增加 `asynq.IsFailureRetryable` 回调对接到这个分类函数。
- 同步在 ai-engine Celery worker 加对应 signal。

### B8. Per-user 配额表 + middleware — 2 d

> 这是 M8 真正可能做不到位的高风险点——**用户体系现在是 stub**。M8 前半段必须先把 user 表立起来，否则配额挂在一个固定 UUID 上没意义。

- 新表：
  ```sql
  CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT UNIQUE,
    role TEXT NOT NULL DEFAULT 'user',
    plan TEXT NOT NULL DEFAULT 'free',
    monthly_token_quota INT NOT NULL DEFAULT 0,  -- 0=无限
    monthly_video_quota INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE TABLE usage_counters (
    user_id UUID NOT NULL,
    period TEXT NOT NULL,             -- 'YYYY-MM'
    tokens_used INT NOT NULL DEFAULT 0,
    videos_used INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, period)
  );
  ```
- backend service: `internal/service/quota_service.go`，函数 `Reserve(user, kind)` 用 upsert + SELECT FOR UPDATE 防止并发超扣。
- `pipeline_service.EnqueueStep` 入队前调 `quota.Reserve(token)` —— **超配额返回 402 风格的 typed error**（不入队、不计入指标 counter）。

### B9. slowapi (FastAPI sidecar) — 0.5 d

- `app/sidecar.py` 注册 `slowapi.Limiter(key_func=get_remote_address)`。
- 默认 `60/minute`，文件上传接口不限（已走 MinIO presign 上传，限额不在这边）。
- 内部回调 `/jobs/{id}:complete` 用白名单：只有 ai-engine pod IP 才能调（生产再细做，dev 默认全开）。

### B10. Go chi 速率限制 — 0.5 d

- `github.com/go-chi/chi/v5/middleware.RateLimit` 或者更精细的 `github.com/sethvargo/go-limiter`。
- limit 策略写进 `internal/config/config.go`：`RateLimitPublicRPM` / `RateLimitAuthRPM`，env 注入。
- 命中限制 → 429 + `Retry-After`。

### B11. 报警 / 文档 — 1 d

- `infra/grafana/dashboards/` 新增 3 个 dashboard JSON：
  - `pipeline-overview`：job 时长 / 队列深度 / 失败率
  - `provider-health`：各 LLM/image/TTS 提供方 P95 / 错误率 / 缓存命中率
  - `quotas`：每用户用量与限额水位
- `infra/grafana/alerts/*.yaml`：
  - 任一 provider 5xx > 2%/min 持续 5 min → Slack/钉钉
  - 单用户视频配额 > 80% 月用量 → 邮件
  - 队列深度 > 1000 持续 10 min → 升级到 oncall
- `docs/runbook-m8.md`：每个告警对应一页 runbook（oncall 看一眼就懂）。

**B 盒产出**：跨进程 trace 全通、配额生效、限流上线、Grafana 第一版仪表盘 + 告警 runbook。

---

## 4. C — 下个月（接 M7 漏项 + 触 M9）

### C1. M7 漏项 — 任务重跑 UI — 3 d

- 后端：`POST /api/v1/jobs/{id}:replay` 从 Asynq 重新入队同一任务类型 + 新 payload，old job 写 `manual_replay_of=<old_id>`。
- 前端：项目详情页底部"历史任务"表格，"Replay"按钮 + 二次确认。
- 限制：仅 `READY`/`FAILED` 状态可重跑；同一 job 60s 内只能重跑一次（避免点爆重试）。

### C2. Celery 指数退避任务级差异化收尾 — 1 d

- B7 已经在 backend 做了；这里只补 ai-engine 那边对应的 signal 行为（early-abort vs reschedule）。

### C3. cost 价格表落地 — 2 d

- `ai-engine/app/settings.py` 引入 `MODEL_PRICING = { "<model>": (input_per_1k, output_per_1k) }`。
- `gateway.chat` / `image_provider.generate_image` / `tts_provider.synthesize_speech` 计算真实 `cost_usd`，覆盖 `cost_usd=0.0`。
- 后端 `internal/service/metrics.go` 新增 `n2av_estimated_cost_usd_total{kind}` counter，对账用。

### C4. 长文切分（摘要 + 分段） — 3 d

- `services/chapter_service.py` 在切分前先检查 `len(text)`：
  - < 80k 字：现有规则 + LLM
  - 80k~200k：先按 `\n\n` 段落切，每段单独切分再合
  - > 200k：先摘要压缩再交给切分（保留文件名 / 章节号等 metadata）
- 进度上报：分段 `n/m` 通过 `report_progress(... , "splitting", current=i, total=N)`。

### C5. 前端 lint+test+CI — 1 d

- `.eslintrc.cjs` 启用，TS 严格 `strict: true`。
- Vitest 配置 + `*.test.tsx` for `features/project/*` 等。
- GitHub Actions 新增 frontend job：`pnpm lint && pnpm test`。

### C6. E2E demo — 2 d

- `e2e/` 目录放一份 Playwright 测试 + 后端 fixture，跑一遍：
  1. 创建项目 → 上传 `.md`
  2. 切分 → 看到 5 个章节
  3. 抽角色 → 看到 3 个
  4. 分镜 → 生成一镜 tts + image
  5. 合成 → 下载 mp4
- 跑通即 M8/M9 验收门槛。

### C7. K8s 起步（M9 接触） — 3 d

- `infra/k8s/` 目录：`namespace.yaml` / `api-deployment.yaml` / `worker-deployment.yaml` / `postgres-sts.yaml`（StatefulSet）/ `minio-sts.yaml`。
- 用 `kustomize` 串，不上 Helm（Helm 单独立 M9 后续）。
- CI 加 `kubectl --dry-run=client apply -k infra/k8s/overlays/staging` 校验。
- 注意：M8 阶段不真上 K8s，仅写 manifest 留底。

### C8. 备份脚本（M9 接触） — 1 d

- `infra/backup/postgres.sh`：`pg_dump | gzip | aws s3 cp s3://n2av-backups/pg-$(date).sql.gz`。
- `infra/backup/minio.sh`：`mc mirror source/ dest/n2av-archive/`。
- cron 通过 K8s CronJob 调度（不在 M8 内真部署，但 manifest 写好）。

### C9. CSP / API Key 加密 / Helmet（M9 接触） — 1 d

- `infra/nginx.conf`：加 `add_header Content-Security-Policy "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self' wss:;"`。
- 后端：API Key 走 sealed-secrets / Vault（不在 M8 范围，留 manifest + 文档）。
- `cmd/api/main.go`：`trust proxy headers`、helmet 等价物由 nginx 提供，后端不强加。

**C 盒产出**：M7 完全收口、M9 manifest 就位、demo 可跑。

---

## 5. 风险与先决

| 项 | 风险 | 缓解 |
|---|---|---|
| 用户 stub vs 配额 | 不立 user 表，配额挂空 | B8 第一刀就建表、seed 一个 admin |
| 跨进程 trace 改了 payload 字段 | 旧 worker 收不到 / 反之 | B3 用 `traceparent` 兼容旧包，1 版本内可丢弃 |
| Celery exponential 重试 BUG | 把 4xx 错误放进重试循环 | B7 + 单元测试覆盖错误分类矩阵 |
| 配额表并发竞争 | upsert + SELECT FOR UPDATE 必备 | B8 用 `pg_advisory_xact_lock` 兜底 |
| Grafana 仪表盘跑不起来 | 版本兼容问题 | 锁定 Grafana 11.x + Prometheus 2.5x |

---

## 6. 验证清单（每盒末尾）

**A 盒**：
- [ ] `curl :8080/metrics` 返回 200 + 至少 `go_goroutines` / `n2av_jobs_enqueued_total`
- [ ] 跑一次完整 chapter_split，指标值单调增
- [ ] OTel 后端能 trace 单进程后端

**B 盒**：
- [ ] 跑拆分 + 抽角色，Jaeger / Tempo 上看到完整跨进程 trace
- [ ] 用户配额到上限后入队返回 typed error，前端提示
- [ ] Grafana 三 dashboard 加载、可见
- [ ] 任一 provider 5xx 注入测试 → 告警 5 min 内触发

**C 盒**：
- [ ] 任务重跑 UI 端到端走通一遍
- [ ] cost_usd 对账与 provider 官方价目误差 < 5%
- [ ] 一本 100w 字小说切分成功
- [ ] E2E Playwright 全绿
- [ ] K8s manifests `kubectl apply -k --dry-run` 无错

---

## 7. 不在 M8（明确排除）

- K8s 真部署（放 M9 / infra 自动化阶段）
- 数据库自动备份调度（C8 只产脚本）
- 用户自助注册 / 找回密码（要 auth 才能做，记 M9）
- 个保法合规 / 隐私政策 / GDPR（运营层）
- Webhook 出站（业务扩展，非"事故定位 / 防滥用"主线）
- 前端 RUM（Real User Monitoring）（C5 仅做 dev 环境）

---

## 8. 进度跟踪

- A 盒：5 工日完成阈值 = 本周末
- B 盒：+10 工日 = 下一个周末的二周末
- C 盒：+20 工日 = 月底
- 每周五 5 分钟 demo：起 Prometheus + Grafana + Tempo，给一个真实 split 跑完的截图
