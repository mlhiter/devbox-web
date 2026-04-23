# OpenAPI Docs

Repo-level static OpenAPI artifacts used for frontend integration, API import, and mock generation.

Current files:

- `openapi-v2alpha.json`
- `openapi-v2alpha.yaml`
- `openapi-v2-server.json`
- `openapi-v2-server.yaml`

Source of truth:

- `v2/frontend/app/api/v2alpha/openapi/route.ts`
- `v2/server/scripts/export-openapi.go`

Regenerate:

```bash
cd /Users/yy/archary/sealos-devbox/v2/frontend
corepack pnpm export-openapi

cd /Users/yy/archary/sealos-devbox/v2/server
go run ./scripts/export-openapi.go
```

Notes:

- `openapi-v2alpha.*` describes the Next.js API routes exposed by `v2/frontend`
- `openapi-v2-server.*` describes the standalone REST API exposed by `v2/server`
