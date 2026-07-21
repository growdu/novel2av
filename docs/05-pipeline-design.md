# 05 · AI 流水线设计

Pipeline 由 **backend (Go)** 编排，**ai-engine (Python)** 执行。本文档定义任务类型、契约与失败处理。

## 1. 总览

```
source.txt
  │
  ▼  ai:split_chapters        (LLM + 规则)            ai-engine
chapters[]
  │
  ▼  ai:extract_characters    (LLM)                   ai-engine
characters[]  ──► ai:character_image               ai-engine
  │
  ▼  ai:scene_breakdown       (LLM)  per chapter     ai-engine
scenes[] / shots[]
  │
  ▼  ai:generate_shot         (并行: image + tts + bgm) ai-engine
image / audio / subtitle
  │
  ▼  ai:compose_chapter       (ffmpeg)                ai-engine
chapter.mp4
  │
  ▼  ai:compose_full          (ffmpeg concat)         ai-engine
project.mp4
```

## 2. 任务契约（payload / result）

所有任务 payload 走 JSON，存在 `Job.meta`；结果回写由 ai-engine 调用 backend 的回调接口（也可写 Redis hash，由 backend 周期同步）。

### 2.1 `ai:split_chapters`

```jsonc
// payload
{ "project_id": "...", "source_key": "novels/.../source.txt" }
// result（写到 backend chapters 表）
{ "chapters": [ {"index":1,"title":"...","content_key":"projects/.../chapters/1.json","word_count":4200} ] }
```

### 2.2 `ai:extract_characters`

```jsonc
// payload
{ "project_id": "...", "chapter_keys": ["projects/.../chapters/1.json", ...] }
// result
{ "characters": [ {"name":"林远","aliases":["阿远"],"role":"protagonist",
                   "appearance":"...","personality":"...","voice_profile":{...}} ] }
```

### 2.3 `ai:character_image`

```jsonc
// payload
{ "project_id":"...", "character_id":"...", "variants": 4 }
// result
{ "ref_image_key":"characters/.../ref.png", "variants":[".../v1.png", "..."] }
```

### 2.4 `ai:scene_breakdown`

```jsonc
// payload
{ "project_id":"...", "chapter_id":"...", "character_refs":["characters/.../ref.png"] }
// result
{ "shots": [ {"scene_idx":1,"shot_idx":1,"type":"wide","description":"...",
              "narration":"...","duration_hint":4.0,"mood":"压抑"} ] }
```

### 2.5 `ai:generate_shot`

```jsonc
// payload
{ "project_id":"...", "chapter_id":"...", "shot_id":"...",
  "description":"...","narration":"...","mood":"...",
  "character_refs":["..."], "style":"cinematic", "aspect":"9:16" }
// result
{ "image_key":"shots/.../image.png", "tts_key":"shots/.../narration.wav",
  "bgm_key":"shots/.../bgm.wav", "subtitle_key":"shots/.../srt",
  "duration_sec": 4.2 }
```

### 2.6 `ai:compose_chapter`

```jsonc
// payload
{ "project_id":"...", "chapter_id":"...", "aspect":"9:16" }
// result
{ "video_key":"videos/.../chapter_1.mp4", "duration_sec": 184.5 }
```

### 2.7 `ai:compose_full`

```jsonc
// payload
{ "project_id":"..." }
// result
{ "video_key":"videos/.../full.mp4", "duration_sec": 4321.0 }
```

## 3. 状态机

```
Project:  CREATED → SPLITTING → SPLIT → EXTRACTING → READY → RUNNING → DONE
                                       ↘ FAILED ↙
Chapter:  PENDING → SPLIT → BREAKING_DOWN → GENERATING → COMPOSING → READY → FAILED
Shot:     PENDING → IMG_OK / TTS_OK / BGM_OK → ALL_OK → COMPOSED
```

backend (Go) 持有状态机；ai-engine 通过回调 `POST /api/v1/internal/jobs/{id}:complete`（或 Redis 事件）触发状态迁移。

## 4. 缓存与可重入

- ai-engine 内 LLM 结果缓存：以 `(prompt_hash, provider, model)` 为 key，TTL 30 天。
- 同一小说同一切分、同一角色描述可复用。
- 缓存命中直接复用产物，跳过对应任务。

## 5. 失败与降级（在 ai-engine 内执行）

| 失败点 | 降级方案 |
|---|---|
| 豆包文本 LLM 不可用 | DeepSeek → Ollama 本地 |
| Seedream 不可用 | SDXL+ComfyUI 本地 |
| 豆包 TTS 不可用 | Edge-TTS / ChatTTS 本地 |
| FFmpeg 失败 | 重试 → 单镜头失败但继续 |

backend 仅「看到」最终 `success/failed`，不感知降级链路。

## 6. 成本与限额

- ai-engine 估算每章 token 数、图像张数、TTS 字符数，写入 `Job.meta.cost_estimate`。
- backend 聚合到项目 `stats.cost_actual`，前端展示；超额拒绝启动并提示。

## 7. 可观测字段（ai-engine span）

- `project_id / chapter_id / shot_id / job_id`
- `provider, model, prompt_hash`
- `input_tokens / output_tokens / images / audio_seconds`
- `cost_usd / duration_ms`
- `result_keys`

## 8. 提示词管理

放在 `ai-engine/app/prompts/`：

```
├─ split_chapters.yaml
├─ extract_characters.yaml
├─ scene_breakdown.yaml
├─ character_image.yaml
├─ shot_image.yaml
└─ tts_voice_profile.yaml
```

每个 YAML 包含 `system / user_template / schema / examples`，版本化（git）。

## 9. 升级路径

- v0.x：图片 + 配音 + FFmpeg 合成。
- v1.x：LoRA 角色库 + 可选视频生成 API。
- v2.x：双 narrator、字幕双语、ASR 校稿。
