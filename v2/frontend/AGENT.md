# Devbox V2 Frontend Agent Notes

## Scope

This directory is the current Devbox frontend in the `labring/devbox`
repository. It owns the DevBox list, create/edit flow, detail page, template
repository UI, release/deploy actions, IDE launchers, and frontend-side API
routes under `app/api`.

## Working Rules

- Keep changes scoped to `v2/frontend` unless the task explicitly crosses into
  the controller, server, runtime, or v1 compatibility surface.
- Do not execute database writes or migrations unless the user explicitly asks.
- For production or test cloud images, build `linux/amd64` by default.
- For 70-cluster work, use `KUBECONFIG=/Users/mlhiter/.kube/70` and namespace
  `devbox-frontend` unless the user gives a different target.
- The image build path in this repository uses `v2/frontend/Dockerfile` with
  build context `v2/frontend`; do not use the old Sealos monorepo provider
  build arguments here.
- The deployment has both `devbox-frontend-init` and `devbox-frontend`
  containers. Inspect both image tags when verifying a rollout.
- Frontend-side template image retagging is HTTPS-only. Do not reintroduce
  `REGISTRY_INSECURE` or `registryInsecure`; configure `REGISTRY_ADDR` as an
  HTTPS-capable registry host.
- Use the Codex in-app Browser for local browser verification.

## Verification

```bash
pnpm ts-lint
git diff --check
```

For local runtime smoke checks, use `pnpm dev` on `localhost:3108`. Use
`pnpm dev:desktop` only when checking the Sealos Desktop wrapper flow that
expects `localhost:3000`.

The package has no `test` script as of 2026-05-21, so generic test autodetection
that runs `npm test` will fail. Do not report that as a product regression; call
out the missing test script separately.

## Important Paths

- `app/[lang]/(platform)/(home)/page.tsx` - DevBox list entry.
- `app/[lang]/(platform)/devbox/create/page.tsx` - create/edit page shell.
- `app/[lang]/(platform)/template/page.tsx` - template repository surface.
- `components/drawers/CustomAccessDrawer.tsx` - custom domain verification UX.
- `components/drawers/CreateTemplateDrawer.tsx` - template creation entry.
- `components/drawers/UpdateTemplateDrawer.tsx` - template update entry.
- `services/backend/registry-retag.ts` - server-side registry manifest/blob
  copy used by template repository create/update flows.
- `services/backend/kubernetes.ts` - Kubernetes client setup.
- `services/db/init.ts` - Prisma client setup.
- `stores/env.ts` - frontend-visible environment defaults.

## Registry Retag Notes

Template repository create/update routes call `retagImage()` in
`services/backend/registry-retag.ts`. The helper accepts registry hosts without
a scheme and normalizes them to HTTPS. Explicit `http://` endpoints are rejected
before any registry request is sent.
