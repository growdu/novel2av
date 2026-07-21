# 04 · 前端概要设计

前端是 **novel2av-frontend**（独立项目），仅通过后端 OpenAPI 契约通信。

## 1. 设计原则

- **类型即契约**：所有 API 类型由 `openapi-typescript` 生成，禁止手写 interface。
- **数据归属服务端**：组件不存业务数据缓存，统一由 TanStack Query 管理。
- **进度归属事件流**：长任务进度走 WebSocket，不轮询。
- **可中断、可恢复**：用户离开页面再回来，任务继续运行，UI 自动回连。

## 2. 路由

| 路径 | 组件 | 说明 |
|---|---|---|
| `/` | `ProjectsPage` | 项目列表 + 新建 |
| `/projects/:id` | `ProjectDetailPage` | 项目总览 + pipeline 进度 |
| `/projects/:id/chapters` | `ChapterListPage` | 章节列表 + 调整 |
| `/projects/:id/chapters/:n` | `ChapterEditorPage` | 单章编辑 + 分镜预览 |
| `/projects/:id/characters` | `CharacterGalleryPage` | 角色形象墙 |
| `/projects/:id/characters/:cid` | `CharacterDetailPage` | 单角色详情 + 重新生成 |
| `/projects/:id/shots` | `ShotListPage` | 全部分镜 |
| `/projects/:id/preview/:chapter` | `VideoPreviewPage` | 成片播放 |
| `/settings` | `SettingsPage` | API Key / 提供方切换 |
| `/login` | `LoginPage` | 登录 |

加载策略：路由级 `lazy()`；首屏仅 ProjectsPage + Layout。

## 3. 全局布局

```
┌──────────────────────────────────────────────────┐
│ TopBar: 项目切换 / 全局进度 / 用户菜单           │
├────────────┬─────────────────────────────────────┤
│            │                                     │
│ SideNav    │  <Outlet />                         │
│ - 项目     │                                     │
│ - 章节     │                                     │
│ - 角色     │                                     │
│ - 分镜     │                                     │
│ - 预览     │                                     │
│            │                                     │
└────────────┴─────────────────────────────────────┘
```

- SideNav 项按项目内子资源懒加载。
- 全局进度（TopBar 右侧）：从 WS 聚合 `job.progress`。

## 4. 状态管理

### 4.1 服务端态（TanStack Query）

```ts
// 仅示例概念
useProjects()                    // list
useProject(id)                   // detail（含进度聚合）
useChapters(projectId)
useCharacters(projectId)
useShots(projectId, { chapterId? })

useRunPipeline(projectId)        // mutation
useRegenCharacter(characterId)
useComposeChapter(chapterId)
```

- 写操作成功后 `invalidateQueries` 对应列表。
- 文件上传用 `useMutation` + `fetch` 流式直传后端（不直传 MinIO）。
- 失败按 `AppError.code` 弹 Toast / 弹 Modal。

### 4.2 客户端态（Zustand）

```ts
useUiStore: { theme, sidebarOpen, toasts }
usePlayerStore: { chapterId, playing, currentTime }
useWizardStore: { create-step, draft }
```

避免全局业务态：业务数据归 Query。

### 4.3 WebSocket 客户端

```ts
// lib/ws/projectSocket.ts
- 单连接/project，进入页面建立，离开关闭（offload to Beacon 可选）
- 自动重连（指数退避）
- onmessage → 按 type dispatch 到 Query 的 setQueryData
```

订阅 payload 通过 reducer 写入 Query 缓存，避免重复拉取。

## 5. 关键页面

### 5.1 新建项目

- 多步骤向导：上传 → 标题/作者/章节范围 → 选择 provider → 触发。
- 步骤状态进 `wizardStore`，可保存草稿到 `localStorage`。
- 上传时显示实时解析日志（解析本身在服务端，这里只展示返回的预处理结果）。

### 5.2 项目详情 / 流水线视图

```
┌──────────────────────────────────────────────────┐
│ 项目: 《XXX》   状态: 运行中 [暂停] [取消]        │
├──────────────────────────────────────────────────┤
│ Pipeline:                                         │
│ ① 章节切分     ✅ 12 章                           │
│ ② 角色提取     ✅ 5 人                            │
│ ③ 场景拆分     🔄 7/12 章                          │
│ ④ 分镜生成     ⏳ 排队中                          │
│ ⑤ 合成成片     —                                  │
├──────────────────────────────────────────────────┤
│ 日志流:                                          │
│  10:21 doubao chat ok, 8.4s                       │
│  10:22 seedream image ok, 12.3s                   │
└──────────────────────────────────────────────────┘
```

- 每个阶段可点开看中间产物（章节列表 / 角色 / 分镜）。
- 阶段失败时显示错误码 + 「从此步重跑」按钮。

### 5.3 章节编辑器

- 左：原文分章阅读（带大纲/锚点）。
- 中：分镜卡片流（缩略图 + 文本 + 配音波形）。
- 右：分镜属性面板（提示词、风格、时长、重新生成）。
- 顶部：成片预览 + 「合成」按钮。

### 5.4 角色画廊

- 网格布局，每角色一张主图 + 多个变体。
- 点开：可调提示词、再生成、调音色、下载。

## 6. 视觉与交互

- 主题：浅/深色，默认浅色，跟随系统。
- 关键交互反馈：
  - 生成中：骨架屏 + 进度条（细分到阶段与 step）。
  - 失败：错误条带 + 一键复制错误码。
  - 完成：成片缩略图带 🎬 角标，自动滚到视口。

## 7. 错误与空状态

- 404 / 500：统一空状态组件 + 重试。
- 网络断开：全局顶栏条带提示，Query 自动重试。
- 外部 AI 限流（429）：Toast 提示，并自动降级到下一档 provider。

## 8. 性能预算

- 首屏 JS < 250KB gzip；图片用 `<img loading="lazy">`。
- 长列表虚拟化（`@tanstack/react-virtual`）。
- 大文件预览（章节正文 > 1MB）走分页 + 虚拟滚动。

## 9. 可访问性 (a11y)

- 全键盘可达；表单 label 关联。
- 颜色对比 WCAG AA；进度条加 `role="progressbar"` + `aria-valuenow`。

## 10. 构建与产物

- `pnpm build` → `dist/`（Vite 默认）。
- Nginx 配置：`/` 与 `/assets/*` 走 SPA fallback；`/api` 与 `/ws` 反代后端。
- CI：lint + typecheck + test + Playwright 冒烟。

## 11. 与后端解耦的工程约束

- 不允许在前端硬编码后端 URL：用 `import.meta.env.VITE_API_BASE`。
- 不允许出现“手写”的接口类型：所有 API 类型必须 `pnpm gen:api` 重新生成。
- 新增字段后端先提交 → 跑 `gen:api` → 前端再使用。
