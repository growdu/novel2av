# 03 · 后端底座设计（Go）

后端是 **novel2av-backend**（独立 Go 项目），负责：

1. **HTTP/WebSocket API** 给前端。
2. **Cobra CLI** 给运维与批处理。
3. **任务编排**：把 pipeline 任务投递到 Redis（asynq），由 ai-engine worker 消费。
4. **元数据权威源**：项目/章节/角色/分镜/Job 状态由后端写入 Postgres；ai-engine 只回报事件。

> 设计原则：**API 层只做编排，不做重活**。所有 CPU/IO 密集型工作入队到 ai-engine。

## 1. 进程角色

| 角色 | 入口 | 责任 |
|---|---|---|
| API | `go run ./cmd/api` | 鉴权、参数校验、调度任务、推流进度 |
| CLI | `go run ./cmd/cli <cmd>` | 一次性任务 / 批处理 / 调试 |
| (ai-engine worker 不在本仓库) | — | 见 `05-pipeline-design.md` |

## 2. CLI 设计（Cobra）

```
novel2av
├─ project
│  ├─ create            --input ./novel.txt --title "..."
│  ├─ list
│  ├─ show  <id>
│  └─ delete <id>
├─ pipeline
│  ├─ run     <project_id> --steps split,characters,shots,compose
│  ├─ rerun   <job_id>
│  └─ status  <project_id> [--watch]
├─ chapter
│  ├─ split   <project_id>
│  └─ merge   <project_id> a b
├─ character
│  ├─ extract <project_id>
│  └─ regen   <character_id>
├─ media
│  ├─ compose <project_id> --chapter <n>
│  └─ burn-sub <video> --srt
├─ llm (debug; 转发 ai-engine sidecar)
│  ├─ ping
│  └─ chat "..." --provider doubao
├─ migrate
│  ├─ up
│  └─ down <rev>
└─ dev
   ├─ seed
   └─ token-cost <project_id>
```

CLI 与 API 共享 `internal/domain` 与 `internal/service`，保证逻辑一致。

## 3. API 设计（REST + WebSocket）

> 所有路由前缀 `/api/v1`。OpenAPI 由 `swag` 注释生成。

### 3.1 REST 资源

| Method | Path | 说明 |
|---|---|---|
| POST | `/projects` | 创建项目（multipart 上传 .txt / .md / .epub） |
| GET | `/projects` | 列表（分页、搜索） |
| GET | `/projects/{id}` | 详情（含聚合进度） |
| DELETE | `/projects/{id}` | 删除（清理 MinIO 资产） |
| POST | `/projects/{id}/pipeline:run` | 启动 pipeline |
| POST | `/projects/{id}/pipeline:rerun` | 从某步重跑 |
| GET | `/projects/{id}/chapters` | 章节列表 |
| POST | `/projects/{id}/chapters:split` | 显式触发章节切分 |
| PATCH | `/chapters/{id}` | 改标题/正文 |
| GET | `/projects/{id}/characters` | 角色列表 |
| POST | `/projects/{id}/characters:extract` | 触发角色提取 |
| POST | `/characters/{id}/image:regen` | 重新生成形象图 |
| GET | `/projects/{id}/shots` | 分镜列表 |
| POST | `/shots/{id}/image:regen` | 重新生成分镜图 |
| POST | `/shots/{id}/tts:regen` | 重新生成配音 |
| POST | `/chapters/{id}/video:compose` | 合成章节视频 |
| GET | `/jobs/{id}` | 查询任务状态 |
| GET | `/assets/{id}` | 返回签名 URL |

### 3.2 WebSocket

`/api/v1/ws/projects/{id}`：订阅项目内所有 Job 进度事件。

```jsonc
{ "type": "job.progress",   "job_id": "...", "step": "image",
  "current": 3, "total": 12, "status": "running" }
{ "type": "job.log",        "job_id": "...", "level": "info",
  "msg": "doubao image ok, 12.3s", "ts": 1719000000 }
{ "type": "chapter.ready",  "chapter_id": "...", "video_url": "..." }
{ "type": "job.failed",     "job_id": "...", "error": { "code": "...", "msg": "..." } }
```

进度来源：
1. ai-engine 通过 Redis 写 `progress:<job_id>`（hash）。
2. backend-api 用 `EVENT.SUB` 订阅 channel `events:project:<id>`，在 WS 推送。
3. 同时落库 `Job.meta`，供前端 REST 兜底查询。

## 4. 鉴权 / 多租户

- v0.1：单用户本地账号 + API Key。
- 预留 JWT / OAuth2 接入位。
- 资源访问按 `user_id` 隔离。

## 5. 配置（Viper + 环境变量）

```
APP_ENV=dev|prod
HTTP_ADDR=:8080

DB_URL=postgres://novel2av:novel2av@postgres:5432/novel2av?sslmode=disable
REDIS_URL=redis://redis:6379/0
REDIS_AI_URL=redis://redis:6379/1          # ai-engine 队列 db
S3_ENDPOINT=minio:9000
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
S3_BUCKET=novel2av
S3_REGION=us-east-1

OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
LOG_LEVEL=info
```

## 6. 错误模型

统一 `apperr.Error{code, message, http_status, details}`：

| code | http | 含义 |
|---|---|---|
| `invalid_input` | 400 | 校验失败 |
| `not_found` | 404 | 资源不存在 |
| `conflict` | 409 | 状态机冲突 |
| `upstream_error` | 502 | ai-engine 上报失败 |
| `rate_limited` | 429 | 配额 |
| `internal` | 500 | 未捕获 |

chi `ErrorHandler` 统一转 JSON，前端按 code 分支处理。

## 7. 任务编排（asynq）

```go
// internal/service/pipeline_service.go
func (s *PipelineService) RunFull(ctx context.Context, projectID string, steps []string) error {
    payload, _ := json.Marshal(PipelineJobPayload{ProjectID: projectID, Steps: steps})
    t := asynq.NewTask("pipeline:run", payload,
        asynq.Queue("default"),
        asynq.MaxRetry(3),
        asynq.Timeout(30 * time.Minute),
    )
    return s.queue.Enqueue(ctx, t)
}
```

- `pipeline:run` 任务作为「协调器」在 Go 中调度一组子任务入队（chain 到 ai-engine 队列）。
- 子任务类型与 ai-engine 侧对齐：`ai:split_chapters / ai:extract_characters / ai:scene_breakdown / ai:generate_shot / ai:compose_chapter`。

## 8. 存储布局（与 ai-engine 共享 MinIO bucket）

```
s3://novel2av/
├─ novels/{project_id}/source.txt
├─ projects/{project_id}/chapters/{n}.json
├─ characters/{project_id}/{character_id}/
│    ├─ profile.json
│    ├─ ref_image.png
│    └─ variants/*.png
├─ shots/{project_id}/{chapter_id}/{shot_id}/
│    ├─ image.png
│    ├─ narration.wav
│    ├─ bgm.wav
│    └─ subtitle.srt
└─ videos/{project_id}/{chapter_id}.mp4
```

## 9. 健康检查 / 启动

- `/healthz`：进程存活。
- `/readyz`：DB + Redis + MinIO 连通；ai-engine sidecar ping（可选）。
- 启动命令：`novel2av migrate up && go run ./cmd/api`。

## 10. 性能与限额

- API：单实例数千 RPS（静态 IO）。
- 限流：`golang.org/x/time/rate` 按用户/IP token bucket。
- 配额：每用户每月生成分钟数 + token 限额，统计在 Postgres。

## 11. 测试策略

- **单测**：domain + service（mock infra）。
- **集成**：testcontainers-go 起 postgres + redis + minio。
- **契约**：从 swag 注释生成 OpenAPI，前端用 `openapi-typescript` 生成类型；CI 检查一致性。
- **E2E**：CLI 跑一个真实小样本（≤ 5 章）作为冒烟。

## 12. 关键依赖（go.mod）

```
github.com/go-chi/chi/v5
github.com/spf13/cobra
github.com/spf13/viper
github.com/jackc/pgx/v5
github.com/hibiken/asynq
github.com/minio/minio-go/v7
github.com/gorilla/websocket
github.com/redis/go-redis/v9
go.opentelemetry.io/otel
github.com/swaggo/swag
github.com/stretchr/testify
golang.org/x/time
```
