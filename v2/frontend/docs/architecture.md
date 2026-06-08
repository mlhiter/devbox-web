# Devbox V2 Frontend Architecture

## Overview

Devbox v2 frontend is a Next.js application that combines React pages,
frontend-side API routes, Kubernetes clients, Prisma-backed template storage,
and shared Sealos packages.

## Main Layers

- `app/[lang]/(platform)/(home)` renders the DevBox list.
- `app/[lang]/(platform)/devbox/create` renders create and edit flows.
- `app/[lang]/(platform)/devbox/detail/[name]` renders DevBox detail surfaces.
- `app/[lang]/(platform)/template` renders template repository management.
- `components/` contains shared dialogs, drawers, IDE buttons, runtime selectors,
  charts, and template controls.
- `stores/` contains Zustand stores for DevBox, env, runtime, IDE, price, guide,
  and user state.
- `api/` contains browser-side request wrappers.
- `app/api/` contains Next.js route handlers that talk to Kubernetes, Prisma,
  account, monitor, platform, and registry services.
- `services/backend/kubernetes.ts` prepares Kubernetes clients from the current
  session.
- `services/db/init.ts` prepares the Prisma client.
- `services/backend/registry-retag.ts` copies registry manifests and blobs for
  template repository create/update flows.

## Registry Retag Flow

Template repository create/update routes read the DevBox release image and call
`retagImage(original, target)`.

The retag helper:

1. Parses source and target image references.
2. Reads `REGISTRY_USER` and `REGISTRY_PASSWORD`.
3. Builds registry API URLs from the image registry host.
4. Rejects explicit `http://` registry endpoints.
5. Defaults host-only registry values to `https://`.
6. Copies child manifests, blobs, and the final manifest to the target tag.

`REGISTRY_INSECURE` and `registryInsecure` are not part of the v2 frontend
contract.

## API Surface

Legacy provider routes under `app/api` handle UI operations such as:

- DevBox CRUD and lifecycle routes.
- Release and deployment routes.
- Template repository routes.
- Platform, monitor, guide, and OpenAPI routes.

Versioned API routes also exist under `app/api/v1` and `app/api/v2alpha`.

## Deployment Shape

The frontend deploy package under `deploy/` installs the
`deploy/charts/devbox-v2-frontend` Helm chart through `install.sh`. The chart
renders:

- runtime Secret `devbox-frontend-runtime` for `REGISTRY_USER`,
  `REGISTRY_PASSWORD`, and `DEVBOX_DOMAIN_CHALLENGE_SECRET`.
- ConfigMap `devbox-frontend-config`.
- init container `devbox-frontend-init` for migration deployment.
- main container `devbox-frontend` for the Next.js app.
- service `devbox-frontend`.
- ingress `devbox.<cloudDomain>`.
- challenge ingress for `/.well-known/devbox-domain-challenge`.
- App CR `app-system/devbox`.

The Deployment loads the runtime Secret through `envFrom`; non-secret
deployment contract values such as `METRICS_URL`, `STORAGE_LIMIT`,
`APP_LAUNCHPAD_URL`, `ENABLE_ADVANCED_CONFIG`, and `GPU_SCHEDULER_MODE` remain
explicit environment variables so operators can compare live cluster state with
the rendered chart.
