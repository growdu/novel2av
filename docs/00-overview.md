# Novel2AV — 概览与目标

> 小说 → 视频（Novel-to-Audio-Visual）的 Web 平台。本目录为 **技术选型、架构设计与概要设计** 的统一文档集，面向研发交付。

## 1. 业务目标

用户上传一部长篇或中篇小说文本，平台在最少人工干预的情况下：

1. 自动 **章节切分**（基于 LLM 与结构规则）。
2. 提取 **主要角色**，为每个角色生成 **形象参考图** 与 **一致性人设**。
3. 按章节拆解 **场景 → 分镜**，生成 **场景图 / 镜头图**。
4. 生成 **剧情旁白配音**（TTS，支持多角色音色）。
5. 生成 **配乐 / 音效**（BGM + 场景音效）。
6. 合成 **带字幕的短视频**（章节级成片，可拼接成全本）。
7. 提供 **Web 工作台**：项目管理、章节进度、素材预览、参数调整、导出下载。

## 2. 非功能性目标

- **可演进**：每一步生成都可独立重跑、可替换模型。
- **可降级**：外部 API 失败时回退到本地模型或本地管线，不阻塞作者。
- **可观测**：每条任务全链路 trace、成本、耗时可见。
- **可 CLI**：核心 pipeline 必须有 CLI 入口，方便批量与离线。
- **前后端分离**：前端、后端、AI 引擎为 **三个独立项目**，独立仓库/目录、独立部署。

## 3. 项目结构

```
novel2av/
├─ backend/        # Go 平台底座 (API + 编排 + CLI)
├─ ai-engine/      # Python AI 引擎 (Celery worker，章节/角色/分镜/配音/合成)
├─ frontend/       # React + TS SPA
├─ infra/          # docker-compose / 脚本 / 部署
└─ docs/           # 设计文档（本目录）
```

后端与 AI 引擎通过 **Redis 队列 + gRPC（同步控制面）** 解耦：

- 长任务 → Redis 队列（ai-engine worker 消费）
- 同步控制 / 健康检查 / 模型元数据 → gRPC（可选）
- 视频合成等重型任务由 ai-engine 内的 ffmpeg 进程组完成

## 4. 文档集索引

| 文档 | 内容 |
|---|---|
| `00-overview.md` | 需求、目标、术语、文档索引（本文件） |
| `01-tech-selection.md` | 技术选型：语言/框架/模型/中间件 |
| `02-architecture.md` | 系统架构：分层、模块、部署拓扑、数据流 |
| `03-backend-design.md` | 后端（Go）概要设计：CLI + REST/WebSocket |
| `04-frontend-design.md` | 前端概要设计：路由、状态、UI 组件 |
| `05-pipeline-design.md` | AI 流水线设计：章节/角色/分镜/配音/合成 |
| `06-data-model.md` | 数据模型、存储、API 契约 |
| `07-roadmap.md` | 实施路线、里程碑、风险 |

## 5. 关键术语

- **Project（项目）**：一部小说的总任务容器。
- **Chapter（章节）**：由 LLM 切分得到的章节段落。
- **Character（角色）**：主要人物实体，含外貌/性格/音色设定。
- **Scene（场景）**：章节内的时空单位，对应一组镜头。
- **Shot（分镜）**：最小叙事单元，对应 1 张图 + 1 段配音 + 1 段 BGM/音效。
- **Job（任务）**：pipeline 中的可调度原子单元。

## 6. 默认技术栈快照（详见 `01-tech-selection.md`）

- 后端底座：**Go 1.22+** + chi / gin + pgx + asynq (Redis) + Cobra CLI
- AI 引擎：**Python 3.11** + Celery + Redis + FastAPI (sidecar) + ffmpeg
- 前端：Vite + React 18 + TypeScript + Zustand + React Query + Tailwind
- LLM：豆包（Ark）/ DeepSeek / OpenAI 兼容，统一 LLM Gateway（在 ai-engine 内）
- 视觉：豆包图像（Seedream）/ SDXL-ComfyUI 本地
- 语音：豆包 TTS / Edge-TTS 本地回退
- 视频：FFmpeg 合成（图片 + 音频 + 字幕）
