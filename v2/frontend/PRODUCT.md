# Devbox V2 Frontend Product Context

## Product Purpose

Devbox gives Sealos users a managed cloud development workspace. The frontend
helps users create a DevBox, configure compute, storage, network access, IDE
entry points, release images, deploy releases, and turn validated DevBox
releases into reusable templates.

## Users

- Developers who need browser-managed DevBox workspaces and local IDE access.
- Platform operators who verify region-specific DevBox frontend deployments.
- Template maintainers who publish and update reusable DevBox templates.

## Core Scenarios

- Create or edit a DevBox from a runtime/template.
- Inspect DevBox status, network endpoints, resource use, logs, releases, and
  IDE connection options.
- Release a DevBox image and deploy an app from a release.
- Create or update a template repository from a DevBox release image.
- Validate custom domain ownership before persisting network changes.

## Product Principles

- Prefer explicit operational feedback over silent failures.
- Keep registry, database, Kubernetes, and account integrations server-side.
- Treat template image retagging as an HTTPS registry operation. HTTP registry
  endpoints are not a supported frontend deployment mode.
- Preserve existing workspace-oriented flows rather than turning Devbox into a
  marketing or landing-page surface.

## Out Of Scope

- Controller reconciliation behavior.
- Runtime snapshotter implementation.
- Registry backend implementation.
- Database schema changes unless the user explicitly asks for them.
