# Devbox V2 Frontend Runbook

## Local Setup

Start from the v2 frontend directory, then create a local env file:

```bash
cd /Users/mlhiter/labring/devbox/v2/frontend
cp .env.template .env.local
```

Set at least:

```bash
NEXT_PUBLIC_MOCK_USER='<kubeconfig JSON string>'
SEALOS_DOMAIN='192.168.10.70.nip.io'
INGRESS_DOMAIN='192.168.10.70.nip.io'
REGISTRY_ADDR='hub.192.168.10.70.nip.io'
REGISTRY_USER='<registry user>'
REGISTRY_PASSWORD='<registry password>'
JWT_SECRET='<desktop jwt secret>'
REGION_UID='<region uid>'
DATABASE_URL='<template database url>'
METRICS_URL='http://vmselect-vm-stack-victoria-metrics-k8s-stack.vm.svc.cluster.local:8481/select/0/prometheus'
STORAGE_LIMIT='20Gi'
APP_LAUNCHPAD_URL='http://applaunchpad-frontend.applaunchpad-frontend.svc.cluster.local:3000/api/v1alpha'
ENABLE_ADVANCED_CONFIG='true'
```

Do not commit real `.env.*.local` secrets.

`REGISTRY_ADDR` is used over HTTPS for backend retag requests. It may be set as
`hub.example.com`; the frontend server adds `https://` when no scheme is
provided. Explicit `http://` registry endpoints are rejected.

## Commands

```bash
pnpm dev
pnpm build
pnpm ts-lint
pnpm gen-client
```

There is no `test` script in `package.json` as of 2026-05-21. Use
`pnpm ts-lint` for focused type verification unless a task provides another test
command.

## Helm Deployment

The v2 frontend deployment package lives under `v2/frontend/deploy` and now uses
the same chart-based shape as the other v2 deployable components:

```bash
cd /Users/mlhiter/labring/devbox/v2/frontend/deploy
make lint
helm template devbox-v2-frontend charts/devbox-v2-frontend --namespace devbox-frontend
```

The cluster image entrypoint runs `install.sh`, which creates or reuses:

- chart defaults from `devbox-v2-frontend-values.yaml`
- user overrides from `/root/.sealos/cloud/values/apps/devbox/devbox-v2-frontend-values.yaml`
- global overrides from `/root/.sealos/cloud/values/global.yaml`

Runtime credentials are rendered into the `devbox-frontend-runtime` Secret and
loaded by the Deployment through `envFrom`. This Secret carries
`REGISTRY_USER`, `REGISTRY_PASSWORD`, and `DEVBOX_DOMAIN_CHALLENGE_SECRET`.
`install.sh` fails if registry user/password cannot be read from component
values, environment variables, or `sealos-system/registry-config`; the chart
does not provide default registry credentials.

The chart keeps both the v2 frontend env contract and the existing compatibility
keys used by older deployments. Key values to check after rendering are:

- `METRICS_URL`
- `STORAGE_LIMIT`
- `APP_LAUNCHPAD_URL`
- `ENABLE_ADVANCED_CONFIG`
- `ENABLE_ADVANCED_ENV_AND_CONFIGMAP`
- `GPU_SCHEDULER_MODE`
- `STORAGE_DEFAULT`

## Registry Retag Smoke Check

Template repository create/update depends on:

- `REGISTRY_ADDR`
- `REGISTRY_USER`
- `REGISTRY_PASSWORD`
- a reachable HTTPS registry API

If retagging fails immediately with `Registry HTTP endpoints are not supported`,
remove the `http://` scheme from `REGISTRY_ADDR` or switch it to an HTTPS
registry endpoint.

## Cluster 70 Notes

Use the named kubeconfig and namespace when checking the live frontend:

```bash
export KUBECONFIG=/Users/mlhiter/.kube/70
kubectl -n devbox-frontend get deploy devbox-frontend
```

Verify both init and main images after a rollout:

```bash
kubectl -n devbox-frontend rollout status deployment/devbox-frontend --timeout=10m
kubectl -n devbox-frontend get deploy devbox-frontend \
  -o jsonpath='{.spec.template.spec.initContainers[*].image}{"\n"}{.spec.template.spec.containers[*].image}{"\n"}'
curl -k -I https://devbox.192.168.10.70.nip.io
```

Compare the live Deployment with the chart contract:

```bash
kubectl -n devbox-frontend get deploy devbox-frontend -o json | jq -r '
  .spec.template.spec.containers[]
  | select(.name=="devbox-frontend")
  | "envFrom=" + ((.envFrom // []) | map(.secretRef.name) | join(",")),
    (.env[]
      | select(.name|test("METRICS_URL|STORAGE_LIMIT|APP_LAUNCHPAD_URL|ENABLE_ADVANCED_CONFIG|GPU_SCHEDULER_MODE|REGISTRY_ADDR"))
      | .name + "=" + .value)
'
```

Cluster 70 currently keeps runtime secrets in `devbox-frontend-runtime`. The
Deployment should load it through `envFrom`, with at least
`REGISTRY_USER`, `REGISTRY_PASSWORD`, and `DEVBOX_DOMAIN_CHALLENGE_SECRET`.

The advanced env and ConfigMap editor is gated by `ENABLE_ADVANCED_CONFIG`.
When the UI does not show those sections, first confirm the live Deployment has
`ENABLE_ADVANCED_CONFIG=true`; the public `/api/getEnv` route requires a valid
session, so unauthenticated curl returning `403` does not prove the env is
missing.
