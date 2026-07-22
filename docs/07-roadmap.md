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

## M3 增量 — 角色提取 + 形象图（已落地）

### ai-engine
- `services/character_service.py` — LLM 抽取（OpenAI 兼容 gateway）+ Pydantic 校验 + 去重；`merge_into_manifest` 工具。
- `infra/media/image_provider.py` — 抽象 `generate_image`，默认 Seedream（OpenAI 兼容 images/generations），未配置时回退到带 provider 元数据的占位 PNG。
- `tasks/character_extract.py` — 从 MinIO 读每章 JSON → LLM 抽取 → 上传 `results/<id>/characters.json`。
- `tasks/character_image.py` — 读 character profile → 构造 prompt → 生成 N 张变体 → 上传 `characters/<id>/<cid>/variants/vN.png` + `ref_image.png`（如已存在旧 ref，自动作为 reference_images 传给 provider 保持一致性）。

### backend
- `infra/db/repo/character.go` — UpsertByName / ListByProject / Get / SetRefImage / Patch / Delete。
- `domain/types.go` — `CharacterPatch`、`Chapter.ContentKey` 暴露。
- `infra/db/repo/chapter.go` — List/Get/Patch 都返回 `content_key`。
- `service/character_service.go` — TriggerExtract（用 `Chapter.ContentKey` 作为 ai-engine 的 chapter_keys）+ TriggerRegenImage + IngestExtractResult（upsert + 项目状态置 `READY`）+ IngestCharacterImage（写回 ref_image_key）。
- `service/chapter_service.go` — IngestSplitResult 现在读取每个 chapter JSON 的 `word_count`。
- `service/asset_service.go` — 新增 `URL(key, ttl)` 字符串便捷方法。
- `service/services.go` — 接入 `CharacterService`。
- `httpapi/handlers.go` — `listCharacters / extractCharacters / ingestCharacters / getCharacter / patchCharacter / deleteCharacter / regenCharacterImage / ingestCharacterImage`；列表与详情都自动附加签名 URL。
- `httpapi/router.go` — 新增 `/characters/{id}` GET/PATCH/DELETE 与 `/image:regen` `/image:ingest`；`/projects/{id}/characters:ingest`。
- `cmd/cli/main.go` — `character {extract,ingest,list,show,regen}` 子命令。
- `service/character_service_test.go` — 校验拒绝空名字。

### frontend
- `lib/api/client.ts` — `Character / CharacterRole / CharacterPatch` 类型 + endpoint schema。
- `features/character/api.ts` — `useCharacters / useCharacter / useExtractCharacters / useIngestCharacters / useRegenImage / usePatchCharacter`；角色标签 + 配色。
- `features/character/CharacterGalleryPanel.tsx` — 触发/拉取 + 4s 轮询占位（M4 改 WS）；卡片网格（缩略图 + 名字 + 角色色标 + 一行简介）。
- `pages/CharacterGalleryPage.tsx` — 画廊容器。
- `pages/CharacterDetailPage.tsx` — 形象图大图 + 重新生成按钮 + 名字/外貌/音色的即时保存。
- `app/router.tsx` — `/projects/:id/characters/:cid` 路由。

### 待补（不在 M3）
- WebSocket 进度推送（M4 接）。
- 真正把旧 ref_image 作为 image_reference 喂给 Seedream，需要 ai-engine 端 vendor 接口支持（OpenAI 兼容协议里通常叫 `image` 或 `image_reference`，不同 provider 字段名不一；本骨架先做 capability detection）。
- 角色 LoRA（v1.x）。

## M4 增量 — 分镜 + 镜头图 + 配音（已落地）

### ai-engine
- `services/shot_service.py` — LLM 拆分 scenes/shots；按 narration 长度自动校准 duration；最多 64 镜/章。
- `infra/media/tts_provider.py` — 抽象 `synthesize_speech`（默认 Doubao TTS OpenAI 兼容 endpoint），未配置时返回按 narration 长度生成的静音 WAV；`voice_for(profile)` 从 prompt YAML 解析 voice id。
- `tasks/scene_breakdown.py` — 读 chapter JSON → LLM → 写 `results/<project_id>/chapters/<chapter_id>/breakdown.json`。
- `tasks/generate_shot.py` — 并行 image + tts + bgm；上传到 `shots/<project_id>/<chapter_id>/<shot_id>/{image,tts,bgm,summary}`；image 自动套 `9:16` 默认尺寸。
- `infra/queue/progress.py` — 升级：所有 `report_progress` 调用支持 `project_id` kwarg，并把事件 publish 到 Redis 通道 `events:project:<id>`。
- 所有 5 个 pipeline task 的 progress 调用都补了 `project_id`。

### backend
- `infra/db/repo/shot.go` — Upsert / ListByProject / Get / PatchAssets（image/tts/bgm/subtitle 任一子集）；`scanShot` 解 meta。
- `domain/types.go` — `Shot.Meta` 暴露。
- `service/shot_service.go` — TriggerBreakdown / TriggerGenerateShot / IngestBreakdown / IngestShotAssets / TriggerProjectBreakdown（按章节入队）；自动带上所有 character 的 `ref_image_key` 作为 character_refs。
- `service/services.go` — 接入 ShotService + EventBus。
- `service/asset_service.go` — 早已提供 URL 便捷方法。
- `infra/queue/events.go` + `ws.go` — `EventBus`：基于 `redis/go-redis` Pub/Sub，订阅 `events:project:*` 并 fan-out 到本地订阅者；`ServeWS(w,r,projectID)` 用 gorilla/websocket upgrade。
- `infra/queue/asynq.go` — `NewAsynqClient` 同时返回 `*EventBus`（共享 Redis client）。
- `httpapi/handlers.go` — `listShots / getShot / triggerBreakdown / ingestBreakdown / regenShotImage / regenShotTTS / ingestShotAssets`；列表/详情自动签名 URL；`wsProject` 真实升级。
- `httpapi/router.go` — `/projects/{id}/shots:breakdown` + `:breakdown:ingest`；`/shots/{id}` GET + `/image:regen` + `/tts:regen` + `/assets:ingest`。
- `cmd/api/main.go` + `cmd/cli/main.go` — 接 `EventBus`。
- 单测：`shot_service_test.go` + `shot_test.go`。

### frontend
- `lib/api/client.ts` — Shot + endpoint schema。
- `features/shot/api.ts` — useShots / useShot / useTriggerBreakdown / useIngestBreakdown / useRegenShotImage / useRegenShotTTS / useIngestShotAssets。
- `features/shot/ShotList.tsx` — 按 `chapter#scene_idx` 分组的卡片流：缩略图 / 描述 / 旁白 / 配音播放器 / 重新生图/配音。
- `pages/ShotListPage.tsx` — 容器 + 内嵌 WS pinger，WS 事件触发 list/character/chapter cache 失效（替换轮询占位）。
- `lib/ws/projectSocket.ts` — 类型化为 `WsEvent`；指数退避 reconnect。

### 待补（不在 M4）
- BGM 仍是静音 WAV（M5 接 MusicGen/Suno）。
- 真正按 chapter 维度的进度聚合（前端目前是泛 invalidation）。
- WebSocket 鉴权（v0.1 暂按 IP/project_id）。
