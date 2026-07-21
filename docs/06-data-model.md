# 06 · 数据模型与接口契约

## 1. ER 概要

```
User 1───n Project 1───n Chapter 1───n Shot
                 │              │
                 │              └── n Asset
                 │
                 ├─n Character 1───n Asset (ref/variants)
                 │
                 └─n Job (DAG)

ProviderConfig (key, type, base_url, api_key, models[], enabled)
UsageMeter (project_id, provider, kind, qty, cost, ts)
```

## 2. 表结构（PostgreSQL 关键列）

```sql
-- projects
id            uuid pk
user_id       uuid fk
title         text
author        text
source_key    text            -- MinIO key
status        text            -- CREATED/SPLITTING/SPLIT/READY/RUNNING/DONE/FAILED
current_step  text
progress      jsonb           -- {step: {current, total}}
config        jsonb           -- {voice_default, style, aspect}
created_at    timestamptz
updated_at    timestamptz

-- chapters
id            uuid pk
project_id    uuid fk
index         int
title         text
content       text
word_count    int
status        text
created_at    timestamptz
UNIQUE(project_id, index)

-- characters
id            uuid pk
project_id    uuid fk
name          text
aliases       text[]
role          text            -- protagonist/antagonist/supporting
appearance    text
personality   text
voice_profile jsonb           -- {provider, voice_id, speed, pitch}
ref_image_key text
meta          jsonb
UNIQUE(project_id, name)

-- shots
id            uuid pk
chapter_id    uuid fk
scene_idx     int
shot_idx      int
type          text            -- wide/medium/closeup
description   text
narration     text
mood          text
duration_sec  real
status        text
image_key     text
tts_key       text
bgm_key       text
subtitle_key  text
meta          jsonb

-- chapter_videos
chapter_id    uuid pk fk
video_key     text
duration_sec  real
status        text
created_at    timestamptz

-- jobs
id            uuid pk
project_id    uuid fk
parent_id     uuid fk null
type          text            -- split/extract/breakdown/shot/compose
status        text            -- queued/running/success/failed/retrying
attempts      int
meta          jsonb           -- {step, current, total, cost_estimate, cost_actual}
error         jsonb
started_at    timestamptz
finished_at   timestamptz

-- assets (统一资产表)
id            uuid pk
project_id    uuid fk
kind          text            -- image/tts/bgm/srt/video
owner_type    text            -- character/shot/chapter
owner_id      uuid
key           text            -- MinIO path
mime          text
bytes         int
meta          jsonb
created_at    timestamptz

-- usage_meters (计费)
id            bigserial pk
project_id    uuid fk
provider      text
kind          text            -- chat/image/tts/video
qty           real
unit          text            -- tokens/images/seconds
cost_usd      numeric(12,6)
ts            timestamptz
```

## 3. 索引

- `projects(user_id, status, updated_at desc)`
- `chapters(project_id, index)`
- `shots(chapter_id, scene_idx, shot_idx)`
- `jobs(project_id, status)` + `jobs(parent_id)`
- `assets(owner_type, owner_id)`

## 4. 关键 API 契约（节选）

> OpenAPI 由 FastAPI 自动生成；下面是请求/响应核心字段示意。

### 4.1 `POST /api/v1/projects`

Request
```json
{ "title": "...", "author": "...", "config": {"aspect":"9:16","style":"cinematic"} }
```
+ multipart `file` (.txt/.md/.epub)

Response 201
```json
{ "id":"...", "status":"CREATED", "word_count": 120000 }
```

### 4.2 `POST /api/v1/projects/{id}/pipeline:run`

Request
```json
{ "steps":["split","characters","shots","compose"],
  "options": { "image_provider":"seedream","tts_provider":"doubao" } }
```

Response 202
```json
{ "job_ids": ["..."], "status":"QUEUED" }
```

### 4.3 `GET /api/v1/projects/{id}`

Response
```json
{
  "id":"...","title":"...","status":"RUNNING","progress":{
    "split":{"current":12,"total":12,"status":"success"},
    "characters":{"current":5,"total":5,"status":"success"},
    "shots":{"current":7,"total":12,"status":"running"}
  },
  "stats": { "chapters":12, "characters":5, "shots":144, "videos":7 }
}
```

### 4.4 `POST /api/v1/characters/{id}/image:regen`

Request
```json
{ "prompt_override":"...", "variants":4 }
```

Response 202
```json
{ "job_id":"...", "status":"QUEUED" }
```

### 4.5 WebSocket 事件（`/api/v1/ws/projects/{id}`）

```jsonc
{ "type":"job.progress", "job_id":"...", "step":"image",
  "current":3, "total":12, "status":"running" }
{ "type":"job.log",      "job_id":"...", "msg":"...", "level":"info" }
{ "type":"chapter.ready","chapter_id":"...", "video_url":"..." }
{ "type":"job.failed",   "job_id":"...", "error":{"code":"upstream_error","msg":"..."} }
```

## 5. Pydantic Schema 规范

- API 入参：`CreateX / UpdateX`（强校验）。
- API 出参：`XRead / XList`（字段裁剪，避免泄漏内部）。
- ORM 模型 ↔ Schema 通过 `model_validate / model_dump` 转换；不允许直接返回 ORM 对象。
- 所有 ID 用 `UUID4`；时间统一 UTC。

## 6. 资产 URL 策略

- 后端不返回 MinIO 直链；返回 `asset_id`。
- 前端 `GET /api/v1/assets/{id}` → 后端生成 **签名 URL**（TTL 1h）并 302。
- 这样切换存储后端、调整权限时前端无感。

## 7. 一致性与迁移

- 一切 DDL 走 Alembic；禁止手工 `CREATE TABLE`。
- 关键写操作放在事务里：`Project` + `Chapter[]` 一次性 commit。
- Celery 任务启动时再次读取 DB 状态，防止“已被用户取消”场景。
