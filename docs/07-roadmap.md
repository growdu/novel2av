# 07 · 实施路线与里程碑

> 两个独立项目：**novel2av-backend**、**novel2av-frontend**。本路线图默认每个里程碑结束时都能 `docker compose up` 起一个端到端可用的版本。

## 1. 里程碑总览

| M | 名称 | 交付物 | 完成判据 |
|---|---|---|---|
| **M0** | 骨架 | 单仓库目录、docker-compose、CI 空跑 | `make up` 可起后端+前端，路由通 |
| **M1** | 上传 + 项目管理 | 上传 / 列表 / 详情 / 删除 | curl 跑通；前端能创建并看到项目 |
| **M2** | 章节切分 | LLM 切分 + 手工校正 | 一本示例小说切出合理章节，前端可编辑 |
| **M3** | 角色提取 + 形象图 | LLM 角色 + 角色图生成 | 角色画廊可用；可重新生成 |
| **M4** | 分镜 + 镜头图 + 配音 | scene_breakdown + image + tts | 章节页能看到分镜卡片，能播放配音 |
| **M5** | BGM + 字幕 + 合成 | bgm + srt + ffmpeg | 单章 mp4 可下载、可在浏览器播放 |
| **M6** | 全本合并 + 预览 | concat + 章节扉页 | 整本 mp4 可导出 |
| **M7** | 稳定性与降级 | 多 provider 切换 + 缓存 + 重试 | 主路径失败自动降级不中断 |
| **M8** | 可观测与限流 | OTel + Prometheus + 限流 + 配额 | Grafana 仪表盘上线 |
| **M9** | 生产化 | Helm/K8s + 备份 + 安全审计 | staging 环境跑通 5 本不同风格小说 |

## 2. M0 — 骨架（1 周）

后端
- `pyproject.toml`、依赖、ruff/mypy 配置。
- FastAPI 启动 `/healthz /readyz`。
- Alembic 初始化。
- CLI `novel2av --help` 可用。

前端
- Vite + React + TS + Tailwind + shadcn/ui。
- Layout + 路由占位。
- `gen:api` 脚本（占位 OpenAPI）。

Infra
- `docker-compose`：api / worker / beat / postgres / redis / minio / web。
- GitHub Actions：lint + test。

## 3. M1 — 项目管理（1 周）

- 上传（流式写入 MinIO，DB 记 meta）。
- `GET /projects` 分页、搜索。
- 删除级联清理。
- 前端 `ProjectsPage` 完整 UI。

## 4. M2 — 章节切分（1.5 周）

- LLM Gateway 抽象 + Doubao 适配。
- `split_chapters` 任务 + 进度。
- 章节编辑器（合并/拆分/改名）。
- 单测：golden 小样本。

## 5. M3 — 角色与形象图（2 周）

- `extract_characters`：JSON schema 强校验。
- 角色图生成（Seedream → 回退 SDXL）。
- 角色画廊、详情、重新生成。

## 6. M4 — 分镜 + 图 + 配音（2.5 周）

- `scene_breakdown` + 规范化（时长、拆分）。
- `gen_image` 批量（限流、并发控制、变体）。
- `gen_tts`（Doubao → 回退 Edge-TTS）。
- 章节页分镜卡片流 + 配音播放。

## 7. M5 — BGM + 字幕 + 合成（2 周）

- `gen_bgm`（MusicGen 本地 / Suno API）。
- ASS 字幕生成。
- FFmpeg 章节合成器 + 配置化滤镜。

## 8. M6 — 全本合并（1 周）

- 全本 concat + 章节扉页 + 整本预览页。

## 9. M7 — 稳定性与降级（1.5 周）

- Provider Router + 健康检查 + 自动降级。
- LLM 结果缓存 + 内容缓存（`(prompt_hash, provider, model)`）。
- Celery 指数退避 + 任务重跑 UI。

## 10. M8 — 可观测与限流（1 周）

- OTel trace；Prometheus exporter。
- `/metrics` 与业务指标。
- slowapi + 用户配额。

## 11. M9 — 生产化（持续）

- Helm chart；K8s 部署。
- 备份（Postgres + MinIO）；日志/告警。
- 安全审计：API Key 加密、签名 URL、CSP。

## 12. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| 外部 LLM/图像不稳定 | pipeline 中断 | 多 provider + 缓存 + 自动降级 |
| 角色一致性差 | 镜头质量低 | ref_image 强制带入；v1.x 训练 LoRA |
| 长文超出 token 上限 | 切分失败 | 摘要 + 分段 + 并行 |
| FFmpeg 复杂度 | 合成 bug 多 | 单元化滤镜为可复用模板；保留 raw artifact 可重跑 |
| 成本失控 | 月底账单爆 | 配额 + 项目估算；开发默认走本地回退 |
| 视频体量 | 存储/带宽贵 | 分辨率/码率配置；冷归档到 OSS 低频 |

## 13. 团队与协作建议

- 前端 / 后端各 1 主导 + 1 共担（AI 流水线、模型调优）。
- 每周 demo：跑通 1 本新小说 → 生成 1 段 30s 成片。
- 公共契约改动走 RFC 流程（更新 `06-data-model.md`）。

## M1 增量 — 项目管理（已落地）

### 后端
- `infra/db/repo/project.go` — Postgres 仓储（Create / Get / List / Delete / CountByStatus / Touch）
- `internal/service/project_service.go` — 上传到 MinIO（`novels/<id>/source.txt|.md`），20MB 硬上限，仅 .txt/.md
- `internal/httpapi/handlers.go` — `createProject / listProjects / getProject / deleteProject`
- `internal/infra/storage/minio.go` — 新增 `PutObject` 与 `RemovePrefix`
- `cmd/cli/main.go` — `novel2av project {list,show,delete}`

### 前端
- `features/project/api.ts` — `useProjects / useProject / useCreateProject / useDeleteProject`
- `features/project/NewProjectDialog.tsx` — 上传弹窗（书名/作者/文件/比例/风格）
- `pages/ProjectsPage.tsx` — 列表 + 删除确认
- `pages/ProjectDetailPage.tsx` — 项目详情（M1 只读，含步骤占位）

### 待补（不在 M1）
- `migrate up` 命令未实现（先用 `psql -f migrations/0001_init.sql`）
- `countByStatus` 未对外暴露
- 用户体系仍为 stub（`currentUser` 固定一个 UUID）

## M2 增量 — 章节切分（已落地）

### ai-engine
- `services/chapter_service.py` — 规则切分（覆盖 `第N章/回/节/卷/集/部`、`第N篇`、`Chapter N`、`CHAPTER II`、`卷N`）+ LLM 复核；提供 `merge / split_at` 辅助。
- `tasks/chapter_split.py` — 从 MinIO 读 `source_key` → 规则切 → 必要时回退到 LLM → 把每章写为 `projects/<id>/chapters/<n>.json` + 汇总 `results/<id>/split_chapters.json`。

### backend
- `migrations/0002_chapters_relax.sql` — `chapters.content_key` 允许 NULL（创建项目时尚无）。
- `infra/db/repo/chapter.go` — Upsert / List / Get / Patch / DeleteByProject。
- `service/chapter_service.go` — TriggerSplit 入队 `ai:split_chapters`；IngestSplitResult 从 MinIO 拉 manifest → upsert chapters → 标记项目 `SPLIT`。
- `service/http_helpers.go` — `fetchURL` 共享工具。
- `httpapi/handlers.go` — `listChapters / splitChapters / ingestChapters / patchChapter`。
- `httpapi/router.go` — 新增 `POST /api/v1/projects/{id}/chapters:split` 与 `:ingest`；`PATCH /api/v1/chapters/{id}`。
- `cmd/cli/main.go` — `novel2av chapter {split,ingest,list,rename}`；`novel2av migrate up` 用 `schema_migrations` 表顺序应用 `migrations/*.sql`。
- `service/chapter_service_test.go` — 校验拒绝空标题。

### frontend
- `lib/api/client.ts` — 新增 Chapter 与相关端点类型。
- `features/chapter/api.ts` — `useChapters / useSplitChapters / useIngestChapters / usePatchChapter / useMergeChaptersLocally`。
- `features/chapter/ChapterListPanel.tsx` — 触发/拉取按钮 + 列表 + 简易轮询（M4 替换为 WS）。
- `pages/ChapterListPage.tsx` — 章节列表页（容器）。
- `pages/ChapterEditorPage.tsx` — 左列表 + 右编辑（标题即时保存 + 合并工具）。
- `app/router.tsx` — `/projects/:id/chapters` 路由。

### 待补（不在 M2）
- LLM 调用目前使用占位模型名 `doubao-pro-128k`；运维通过 env 注入真实模型。
- WebSocket 进度推送还未实现，前端用 4s 轮询占位。
- 「合并」的语义在 M2 是打标 + 改首章标题；真正把 N 章正文合并为一份并重排 offset，需 M4 的章节→分镜转换时再做。
