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
