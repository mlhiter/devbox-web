# Runbook

## Frontend

Install and run v2 frontend:

```bash
cd v2/frontend
pnpm install
pnpm dev
```

Verify v2 frontend:

```bash
pnpm -C v2/frontend ts-lint
pnpm -C v2/frontend build
git diff --check
```

If `pnpm -C v2/frontend ts-lint` reports missing `.next/types` files after `.next` cleanup or concurrent build activity, run:

```bash
pnpm -C v2/frontend build
pnpm -C v2/frontend ts-lint
```

## Local Codex Actions

The repository includes Codex run actions:

- `运行 v1 前端`
- `运行 v2 前端`

They call `.codex/environments/devbox-frontend-70.sh`, which sets up local port-forwards and starts the selected frontend.

## Environment Notes

Common v2 frontend settings include:

- `SEALOS_DOMAIN`
- `SSH_DOMAIN`
- `ACCOUNT_URL`
- `METRICS_URL`
- `DATABASE_URL`
- `REGISTRY_ADDR`, `REGISTRY_USER`, `REGISTRY_PASSWORD`
- `GPU_ENABLE`
- `GPU_SCHEDULER_MODE`, with `native` and `hami` supported
- `ENABLED_IDES`
- `ENABLE_ADVANCED_CONFIG`
- `STORAGE_LIMIT`

Registry retagging requires HTTPS registry access from the frontend route handlers.

## Controller and Services

Controller checks:

```bash
cd v2/controller
make test
make build
```

Server checks:

```bash
cd v2/server
make test
make build
```

Gateway checks:

```bash
cd v2/httpgate
cargo test --locked
cargo build --locked --release

cd ../sshgate
go test ./...
make build
```

## Troubleshooting

- GPU inventory missing: check `GPU_ENABLE`, `GPU_SCHEDULER_MODE`, and the `node-system/node-gpu-info` ConfigMap.
- Quota errors look like permissions errors: inspect `services/backend/response.ts` mapping and the raw Kubernetes status message.
- ConfigMap edits do not appear in the workspace: restart the DevBox after update.
- IDE options missing: check `ENABLED_IDES` and `components/IDEButton.tsx` grouping/filtering.
