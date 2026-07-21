# novel2av-frontend

Vite + React 18 + TypeScript SPA. Only talks to the Go backend at `/api/v1`.

```bash
npm install
npm run dev          # http://localhost:5173 (proxies /api → :8080)
npm run gen:api      # regenerate src/lib/api/schema.ts from backend OpenAPI
npm run build
npm run lint
npm run typecheck
```

Layout follows `src/{app,pages,features,components,lib,stores,styles}`.

See [`../docs/04-frontend-design.md`](../docs/04-frontend-design.md).
