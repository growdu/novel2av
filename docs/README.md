# Novel2AV — 设计文档

本目录是 **小说转视频** Web 平台的技术选型、架构设计与概要设计文档。

> **项目状态**：设计阶段。本仓库目前仅包含文档；后续会拆出两个独立子项目：
>
> - `novel2av-backend/`（Python / FastAPI / Celery）
> - `novel2av-frontend/`（Vite / React / TypeScript）

## 文档索引

1. [00-overview.md](./00-overview.md) — 需求、目标、术语
2. [01-tech-selection.md](./01-tech-selection.md) — 技术选型
3. [02-architecture.md](./02-architecture.md) — 系统架构
4. [03-backend-design.md](./03-backend-design.md) — 后端概要设计
5. [04-frontend-design.md](./04-frontend-design.md) — 前端概要设计
6. [05-pipeline-design.md](./05-pipeline-design.md) — AI 流水线设计
7. [06-data-model.md](./06-data-model.md) — 数据模型与接口契约
8. [07-roadmap.md](./07-roadmap.md) — 实施路线与里程碑

## 一句话技术栈

- **后端**：Python 3.11 + FastAPI + Celery + Redis + PostgreSQL + MinIO + FFmpeg
- **前端**：Vite + React 18 + TypeScript + TanStack Query + Tailwind + shadcn/ui
- **AI**：豆包（LLM/图像/TTS）为默认，统一 LLM Gateway 抽象，可热插拔；本地 Ollama / ComfyUI / Edge-TTS 作为回退
