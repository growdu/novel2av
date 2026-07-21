# novel2av — 小说转视频平台

> 一个基于大模型的小说转视频 Web 平台。按章节切分小说 → 提取主要角色 → 生成分镜 → 生成配音配乐 → 合成带字幕的短视频。

本仓库是 **monorepo**，包含三个独立子项目 + 文档：

```
novel2av/
├─ backend/    # Go 平台底座 (HTTP + WebSocket + CLI + 队列编排)
├─ ai-engine/  # Python AI 引擎 (Celery worker + FastAPI sidecar)
├─ frontend/   # Vite + React + TypeScript SPA
├─ infra/      # docker-compose / 部署脚本
└─ docs/       # 技术选型、架构、概要设计
```

## 技术栈一览

| 项目 | 技术 |
|---|---|
| backend | Go 1.22+, chi, pgx, asynq, minio-go, gorilla/websocket, Cobra, Viper, OpenTelemetry |
| ai-engine | Python 3.11+, Celery, Redis, FastAPI, pydantic, httpx, ffmpeg, Pillow |
| frontend | Vite 5, React 18, TypeScript 5, TanStack Query, Zustand, React Router, Tailwind |
| 基础设施 | Postgres 16, Redis 7, MinIO, Nginx, Prometheus/Grafana, OpenTelemetry |

## 快速开始（docker-compose）

```bash
make -C infra up
# 等待 Postgres / Redis / MinIO healthy
make -C infra migrate           # 跑 SQL 迁移
make -C infra seed              # 灌示例数据（可选）

# 入口
#   Web:        http://localhost:5173
#   Backend:    http://localhost:8080
#   AI sidecar: http://localhost:8000
#   MinIO UI:   http://localhost:9001
```

手动开发（不依赖 Docker）见各子项目 `README.md`。

## 架构一句话

> 前端 → Go 后端（API / 状态机 / 入队） → Redis → Python AI 引擎（执行 LLM / 图像 / 语音 / 视频合成）。

详细分层、数据流、状态机与降级策略见 `docs/`。

## 文档索引

| 文档 | 内容 |
|---|---|
| [`docs/00-overview.md`](docs/00-overview.md) | 需求、目标、术语 |
| [`docs/01-tech-selection.md`](docs/01-tech-selection.md) | 技术选型（含替代方案） |
| [`docs/02-architecture.md`](docs/02-architecture.md) | 系统架构、进程拓扑、数据流 |
| [`docs/03-backend-design.md`](docs/03-backend-design.md) | 后端（Go）概要设计 |
| [`docs/04-frontend-design.md`](docs/04-frontend-design.md) | 前端概要设计 |
| [`docs/05-pipeline-design.md`](docs/05-pipeline-design.md) | AI 流水线（任务契约 / 状态机） |
| [`docs/06-data-model.md`](docs/06-data-model.md) | 数据模型与 API 契约 |
| [`docs/07-roadmap.md`](docs/07-roadmap.md) | 实施路线与里程碑 |

## 边界规则（记住这几条就能避免走偏）

1. 前端 **不直连** 任何外部 AI API；只调 `backend-api`。
2. `backend-api` **不允许** 在请求路径里调 LLM / 图像 / TTS / FFmpeg。
3. 所有 AI 重活必须由 `backend-api` **入队** → `ai-engine` worker **消费**。
4. 项目状态机的 **权威源是 Postgres**（由 backend 写入）；ai-engine 只回报事件。
5. 任何资源 URL 返回 **签名 URL**（TTL 1h），不暴露 MinIO 直链。

## 路线图（简版）

- **M0** 骨架（已就位：本仓库）
- **M1** 项目管理（上传 / 列表 / 删除）
- **M2** 章节切分
- **M3** 角色提取 + 形象图
- **M4** 分镜 + 镜头图 + 配音
- **M5** BGM + 字幕 + 合成
- **M6** 全本合并
- **M7** 稳定性与降级
- **M8** 可观测与限流
- **M9** 生产化（K8s / 备份 / 安全）

详见 `docs/07-roadmap.md`。

## 许可证

Internal — TBD.
