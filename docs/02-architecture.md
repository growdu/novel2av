# 02 · 系统架构

## 1. 全景视图

```
┌─────────────────────────────────────────────────────────────────────┐
│                          用户浏览器 (SPA)                           │
│   Vite + React + TS + Tailwind + TanStack Query + WebSocket         │
└──────────────────────────────────┬──────────────────────────────────┘
                                   │ HTTPS (OpenAPI / WS)
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Nginx / API 网关                            │
│         /api/*  → backend (Go)   /ws/*  → backend (Go)   /*  → SPA  │
└──────────────────────────────────┬──────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│        后端底座  backend/  (Go)  —  novel2av-backend                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │ Projects │  │ Chapters │  │ Assets   │  │  Jobs    │  ...        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘             │
│       └──────────────┴─────────────┴─────────────┘                  │
│                       Domain Services                               │
│        ┌─────────────────┬─────────────────────┐                    │
│        │ Queue Client    │ Object Storage      │   gRPC (可选)      │
│        │ (asynq/Redis)   │ (MinIO S3)          │ ──► ai-engine      │
│        └────────┬────────┴─────────┬───────────┘   sidecar          │
└─────────────────┼──────────────────┼────────────────────────────────┘
                  │                  │
        ┌─────────▼─────────┐  ┌─────▼──────┐  ┌────────────────────┐
        │ asynq (Redis)     │  │ Postgres   │  │ MinIO              │
        │  + Celery broker  │  │            │  │                    │
        └─────────┬─────────┘  └────────────┘  └────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│        AI 引擎  ai-engine/  (Python)  —  novel2av-ai-engine         │
│   Celery worker  ──►  tasks: split / extract / breakdown /          │
│                              image / tts / bgm / compose            │
│   FastAPI sidecar  ──►  /healthz / providers / debug                │
└──────────────┬──────────────────────────────────────────────────────┘
               │
   ┌───────────┼────────────┬─────────────┐
   ▼           ▼            ▼             ▼
 豆包 Ark    Seedream     豆包 TTS       FFmpeg
 (LLM)      (图像)       (语音)         (合成)
   │           │            │
   └─ 回退 ────┴─ DeepSeek / Ollama / Edge-TTS / ComfyUI
```

## 2. 分层

### 2.1 backend (Go)

| 层 | 责任 |
|---|---|
| transport/http | chi 路由、参数校验、OpenAPI |
| transport/ws | WebSocket 进度推送 |
| domain | 项目/章节/角色/分镜/任务的业务规则 |
| infra/queue | asynq 任务投递 |
| infra/storage | MinIO / S3 |
| infra/db | pgx 仓储 |
| cmd/api | HTTP 服务入口 |
| cmd/cli | Cobra CLI |

### 2.2 ai-engine (Python)

| 层 | 责任 |
|---|---|
| tasks | Celery 任务：每个 pipeline 步骤 |
| services | 业务逻辑（章节切分、角色提取、合成） |
| infra/llm | LLM Gateway |
| infra/media | ffmpeg 封装 |
| sidecar | FastAPI 调试接口 |
| cli | Typer 调试 CLI |

## 3. 进程拓扑

```
                 ┌──────────────────────┐
                 │  backend-api (Go)    │  ← 1..N 副本
                 └──────────┬───────────┘
                            │
                            │ asynq enqueue
                            ▼
                  ┌────────────────────┐
                  │  Redis (db0/1/2)   │   队列：db0=Go, db1=Python
                  └─────────┬──────────┘
                            │
                            ▼
                 ┌──────────────────────┐
                 │ ai-engine worker     │  ← Celery
                 │ ai-engine sidecar    │  ← FastAPI (可选)
                 └──────────────────────┘

                 ┌──────────────────────┐
                 │ Postgres             │  ← backend 写入元数据
                 │ MinIO                │  ← 媒体资产
                 └──────────────────────┘
```

- **backend-api**：无状态 Go 服务，水平扩展。
- **ai-engine worker**：CPU/IO 混合密集，独立伸缩。
- **ai-engine sidecar**：仅调试/健康，默认不开。

## 4. 模块清单

### 4.1 backend/

```
backend/
├─ cmd/
│  ├─ api/main.go            # HTTP + WS 入口
│  └─ cli/main.go            # Cobra CLI
├─ internal/
│  ├─ config/                # Viper 配置
│  ├─ httpapi/
│  │  ├─ router.go           # chi 路由
│  │  ├─ projects.go
│  │  ├─ chapters.go
│  │  ├─ characters.go
│  │  ├─ shots.go
│  │  ├─ jobs.go
│  │  ├─ assets.go
│  │  └─ ws.go
│  ├─ domain/                # 实体 + 业务规则
│  │  ├─ project.go
│  │  ├─ chapter.go
│  │  ├─ character.go
│  │  ├─ shot.go
│  │  └─ job.go
│  ├─ service/               # 跨实体用例
│  │  ├─ pipeline_service.go
│  │  ├─ asset_service.go
│  │  └─ event_bus.go        # Pub/Sub
│  ├─ infra/
│  │  ├─ db/                 # pgx 连接池、迁移
│  │  ├─ storage/            # MinIO 客户端
│  │  ├─ queue/              # asynq client + handler 注册
│  │  └─ observability/      # OTel + slog
│  └─ api/                   # 生成的 OpenAPI / DTO
├─ migrations/               # SQL 迁移
├─ gen/                      # oapi-codegen / sqlc 输出
├─ go.mod
├─ go.sum
├─ Dockerfile
└─ Makefile
```

### 4.2 ai-engine/

```
ai-engine/
├─ app/
│  ├─ main.py                # Celery app 入口（worker）
│  ├─ sidecar.py             # FastAPI sidecar
│  ├─ settings.py            # pydantic-settings
│  ├─ api/                   # sidecar 路由
│  ├─ services/              # 业务服务
│  ├─ tasks/                 # Celery 任务（pipeline 步骤）
│  │  ├─ chapter_split.py
│  │  ├─ character_extract.py
│  │  ├─ scene_breakdown.py
│  │  ├─ image_generate.py
│  │  ├─ tts_generate.py
│  │  ├─ bgm_generate.py
│  │  └─ video_compose.py
│  ├─ infra/
│  │  ├─ llm/                # LLM Gateway
│  │  ├─ media/              # ffmpeg 封装
│  │  ├─ storage/            # MinIO 客户端（与 Go 共享凭证）
│  │  └─ queue/              # Celery app
│  ├─ prompts/               # 提示词模板（YAML）
│  ├─ schemas/               # Pydantic
│  └─ cli.py                 # Typer CLI
├─ pyproject.toml
├─ Dockerfile
└─ Makefile
```

### 4.3 frontend/

```
frontend/
├─ src/
│  ├─ main.tsx
│  ├─ app/                   # 路由 + Provider
│  ├─ pages/
│  ├─ features/
│  ├─ components/
│  ├─ lib/api/               # openapi-fetch
│  ├─ lib/ws/
│  ├─ stores/
│  └─ styles/
├─ index.html
├─ package.json
├─ vite.config.ts
├─ tailwind.config.ts
└─ Dockerfile
```

## 5. 关键数据流

### 5.1 创建项目并触发完整 pipeline

```
Browser              backend-api (Go)              DB           Redis            ai-engine worker
  │  POST /projects ──►│                              │              │                    │
  │                    │ insert project               │              │                    │
  │  ◄── 201 project ─│                              │              │                    │
  │  POST /projects/{id}/pipeline:run ─►              │              │                    │
  │                    │ enqueue asynq                │              │                    │
  │                    │ ──── split_chapters ──────────────────────►│ (Celery) consume   │
  │                    │ update DB → status RUNNING   │              │ split_chapters ───►│
  │                    │                              │              │ publish progress   │
  │  WS progress ◄─────│ subscribe ──────────────────►│              │                    │
  │                                                  │              │ extract_characters ─►
  │                                                  │              │ scene_breakdown ─────►
  │                                                  │              │ per-shot img+tts+bgm ►
  │                                                  │              │ compose_chapter_video►
  │                                                  │              │ publish chapter.ready
```

### 5.2 失败重试

- 单步失败 → asynq `MaxRetry` + 指数退避。
- 超过阈值 → 标 `FAILED`，前端可一键「从此步重跑」。
- Job DAG 用 `parent_job_id` 记录，前端按子图选择重跑范围。

## 6. 部署拓扑

**开发（docker-compose）**
```
backend-api + ai-engine + worker + sidecar + postgres + redis + minio + web
```

**生产（最小可用）**
- backend-api / ai-engine worker / ai-engine sidecar：K8s Deployment，独立 HPA。
- Postgres / Redis：托管或自管主从。
- MinIO：分布式或切换 OSS/S3。
- 前端：Nginx 静态托管或 CDN。

## 7. 可观测性

- **指标**：每个 pipeline 步骤的成功率、P50/P95 耗时、token 消耗、媒体字节数。
- **日志**：所有日志带 `project_id / chapter_id / job_id`。
- **追踪**：Go OTel + Python OTel → 同一 OTLP collector（Jaeger/Tempo）。
- **告警**：队列积压、连续失败率、外部 API 5xx。

## 8. 关键不变量 / 边界规则

- 前端**不直接**调任何外部 AI API，只调 backend-api。
- backend-api **不允许**在请求路径里调 LLM/图像/TTS/FFmpeg。
- 所有 AI 重活必须由 backend-api **入队** → ai-engine worker **消费**。
- 所有生成的资产 URL 都是 **签名 URL**，TTL 1h。
- 项目所有资源路径必须可由 `project_id` 推导，便于清理。
- ai-engine 不直接改写 backend 拥有的状态机，只通过事件 / 队列回报结果；DB 写操作集中在 backend。
