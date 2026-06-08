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
- `METRICS_URL`; the metrics SDK expects a Prometheus-compatible query endpoint, for example the VictoriaMetrics select service at `http://vmselect-vm-stack-victoria-metrics-k8s-stack.vm.svc.cluster.local:8481/select/0/prometheus`
- `DATABASE_URL`
- `REGISTRY_ADDR`, `REGISTRY_USER`, `REGISTRY_PASSWORD`
- `GPU_ENABLE`
- `GPU_SCHEDULER_MODE`, with `native` and `hami` supported
- `DEVBOX_RUNTIME_CLASS_NAME`, the RuntimeClass written by v2 frontend create flows; default `devbox-runtime`, with `devbox-stargz-runtime` available when the cluster has the matching runtime handler and snapshotter
- `ENABLED_IDES`
- `ENABLE_ADVANCED_CONFIG`; set it to `true` to show advanced env and ConfigMap editing in the frontend
- `STORAGE_LIMIT`
- `APP_LAUNCHPAD_URL`
- `DEVBOX_DOMAIN_CHALLENGE_SECRET`

Registry retagging requires HTTPS registry access from the frontend route handlers.

For cluster-local deployments, keep credentials such as `REGISTRY_USER`,
`REGISTRY_PASSWORD`, and `DEVBOX_DOMAIN_CHALLENGE_SECRET` in a Secret and mount
them through `envFrom` rather than storing them directly in the Deployment
manifest.

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

The v2 server reads create defaults from its YAML config. `devbox.createDefaults.runtimeClassName` controls the RuntimeClass written by server-side create requests and defaults to `devbox-runtime`.

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
- DevBox pods fail after changing RuntimeClass: confirm the cluster has the matching RuntimeClass, runtime handler, and snapshotter installed; the controller manifest creates the RuntimeClass objects, but node runtime support must exist separately.
- Quota errors look like permissions errors: inspect `services/backend/response.ts` mapping and the raw Kubernetes status message.
- Advanced env or ConfigMap UI missing: check `/api/getEnv` and confirm `enableAdvancedConfig` is `true`, then verify the Deployment has `ENABLE_ADVANCED_CONFIG=true`.
- ConfigMap edits do not appear in the workspace: restart the DevBox after update.
- IDE options missing: check `ENABLED_IDES` and `components/IDEButton.tsx` grouping/filtering.
