# 01 · 技术选型

## 1. 选型原则

1. **职责清晰**：Go 做平台底座（API、编排、CLI、调度），Python 做 AI 引擎（LLM/图像/语音/视频重活）。
2. **可替换**：每个外部依赖（LLM/图像/TTS/视频）都封装在 Gateway 之后，可热插拔。
3. **前后端解耦**：仅通过 HTTP/WebSocket + OpenAPI 契约通信。
4. **本地可跑**：开发期不强依赖任何付费 API，关键能力有开源回退。

## 2. 后端底座（Go）

| 维度 | 选择 | 理由 |
|---|---|---|
| 语言 | **Go 1.22+** | 高并发、低资源、原生 CLI 二进制；与 Python 配合可各自扬长 |
| Web 框架 | **chi**（轻量） 或 **gin** | 二选一，本骨架使用 chi；中间件生态成熟 |
| 数据库驱动 | **pgx** + sqlc 或 sqlx | 高性能、原生类型；本骨架用 pgx + 手写 SQL |
| ORM/查询 | 原生 SQL + 仓储层 | 避免重型 ORM |
| 任务队列 | **asynq**（Redis） | Go 原生，延迟队列、速率限制、可视化看板 |
| 实时通道 | **gorilla/websocket** + Redis Pub/Sub | 给前端推送 Job 进度 |
| 对象存储 | **aws-sdk-go-v2**（S3 兼容，MinIO） | 官方 SDK |
| 缓存/队列 | **Redis 7** | 同 ai-engine 共享 |
| CLI | **Cobra** + **Viper** | 配置 + 子命令完整方案 |
| 配置 | **Viper** + 环境变量 | 12-factor |
| 日志 | **slog**（标准库） + 结构化字段 | 直接 JSON |
| 追踪 | **OpenTelemetry-Go** → OTLP | 跨服务 trace |
| 测试 | 标准 `testing` + testify | 集成 testcontainers-go |

### 与 AI 引擎的边界

- 后端 **不调** 任何 LLM/图像/TTS API。
- 后端通过 **asynq 队列** 把 `pipeline:split / :characters / :shots / :compose` 等任务投递到 Redis。
- ai-engine 侧的 Celery/内置 worker 消费（详见 §3）。
- 同步控制面（如「查询模型列表、ping provider」）走 **gRPC**（可关停，HTTP 也行）。

## 3. AI 引擎（Python）

| 维度 | 选择 | 理由 |
|---|---|---|
| 语言 | **Python 3.11** | LLM/图像/语音/FFmpeg 生态最丰富 |
| 任务队列 | **Celery 5 + Redis**（与 Go asynq 同 Redis 实例，分 db 隔离） | 成熟、便于调用 Python 库 |
| 控制面 HTTP | **FastAPI**（sidecar） | 模型元数据、健康检查、调试接口 |
| LLM 抽象 | 自研 `llm_gateway`（OpenAI 兼容接口） | 见 §4 |
| FFmpeg | `ffmpeg` 6.x + `ffprobe` | 视频合成 |
| 图像/语音 | Pillow / pydub 等 | 处理 + 转码 |
| 配置 | **pydantic-settings** | 与 Go 配置保持同等结构 |
| 日志 | **structlog** | JSON 日志 |
| 追踪 | OpenTelemetry-Python | 与 Go 串联 |
| CLI | **Typer** + **Rich** | 调试与批处理 |

> ai-engine 既可作为「Celery worker」跑，也可独立暴露 FastAPI sidecar 给本地调试。

## 4. 前端选型

| 维度 | 选择 | 理由 |
|---|---|---|
| 构建 | **Vite 5** | 快、ESM 原生 |
| 框架 | **React 18 + TypeScript 5** | 生态最成熟 |
| 路由 | **React Router 6** | 数据路由 |
| 状态（服务端态） | **TanStack Query 5** | 缓存、重试、订阅进度 |
| 状态（客户端态） | **Zustand** | 轻量 |
| 表单 | **React Hook Form + Zod** | 与后端契约对齐 |
| UI | **Tailwind CSS 3 + shadcn/ui** | 可定制 |
| 实时 | **WebSocket** + reconnect 封装 | 接收 Job 进度 |
| 测试 | Vitest + Testing Library + Playwright | 单测 + E2E |
| 代码质量 | ESLint + Prettier + tsc | CI 卡口 |

### 与后端解耦

- 唯一耦合点是 **OpenAPI 文档**：由后端 Go（`swag` 或 `oapi-codegen`）生成，前端用 `openapi-typescript` 生成类型。

## 5. LLM / 多模态服务选型

抽象为统一 `LLM Gateway`（在 ai-engine 内），所有上游以 OpenAI 兼容协议暴露：

```
                ┌──────────────┐
                │ LLM Gateway  │  ← OpenAI /chat/completions 兼容
                └──────┬───────┘
   ┌───────────┬───────┼───────┬────────────┐
   │           │       │       │            │
 豆包 Ark   DeepSeek  OpenAI  Claude  Ollama(本地)
 (Seedream   (文本)   (备)   (备)    (Qwen2.5/llama3
  文/图/语音)                              )
```

| 能力 | 默认 | 回退 | 备注 |
|---|---|---|---|
| 长文理解 / 结构化输出 | **豆包 Doubao-pro-128k** | DeepSeek-V3 / Qwen2.5-72B (Ollama) | 章节切分、角色提取、分镜脚本 |
| 角色形象图 | **豆包 Seedream** | ComfyUI + SDXL (本地) | 角色 LoRA 训练可选 |
| 场景图 | **豆包 Seedream** | SDXL + ControlNet | 用角色参考图做一致性 |
| 旁白 TTS | **豆包 TTS** | Edge-TTS / ChatTTS | 多音色 |
| 配乐 | **Suno / Udio** 或自建库 | 本地 MusicGen | BGM 按情绪生成 |
| 视频 API（可选升级） | 可灵 / Pika | — | 默认不启用 |

### 关键设计：Prompt 与结构化输出

- 所有 LLM 任务统一用 **JSON Schema / function calling** 强制结构化输出，后端用 Pydantic 直接校验。
- Prompt 模板与少量样本（few-shot）放在 `ai-engine/app/prompts/`，版本化管理。

## 6. 视频合成选型

- **默认方案**：图片 + 配音 + BGM → **FFmpeg** 合成（在 ai-engine 中）。
- **可选升级**：调用视频生成 API（可灵/Pika）生成动态片段。
- **工具链**：`ffmpeg` 6.x + `ffprobe`，封装为 `media.ffmpeg` 服务。

## 7. 中间件与基础设施

| 组件 | 选择 | 用途 |
|---|---|---|
| 容器 | Docker + docker-compose | 开发与单机部署 |
| 编排（可选） | Kubernetes + Helm | 规模化 |
| 反向代理 | Nginx / Caddy | 前端 SPA + API 网关 |
| 对象存储 | MinIO | 媒体资产 |
| 数据库 | PostgreSQL 16 | 元数据 |
| 缓存/队列 | Redis 7 | Go asynq + Python Celery 共享，按 db 隔离 |
| 监控 | Prometheus + Grafana | 业务指标 |
| 日志 | Loki / ELK | 日志聚合 |
| 追踪 | Tempo / Jaeger | 链路追踪 |
| CI | GitHub Actions | 三项目独立流水线 |

## 8. 安全与合规

- API Key 走环境变量 + Secret Manager（生产用 Vault / 阿里云 KMS）。
- 对象存储签名 URL，前端不可直传原始 bucket。
- 用户上传文本做长度 / 编码 / 敏感词三重校验。
- 输出内容做水印。

## 9. 替代方案速查

| 决策 | 默认 | 备选 |
|---|---|---|
| 后端底座语言 | Go | Rust (axum) / Node (NestJS) |
| Go Web 框架 | chi | gin / echo / fiber |
| Go 队列 | asynq | machinery / 直接 Redis |
| Python 队列 | Celery | Dramatiq / RQ / Temporal |
| 前端框架 | React | Vue3 / SvelteKit |
| 视频合成 | FFmpeg | MoviePy / Remotion |
| 对象存储 | MinIO | 阿里云 OSS / AWS S3 |
