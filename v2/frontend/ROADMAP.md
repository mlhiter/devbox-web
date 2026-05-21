# Devbox V2 Frontend Roadmap

## Near Term

- Keep v2 frontend behavior aligned with the current DevBox CRD and template
  repository flows.
- Preserve HTTPS-only registry retag behavior in frontend server routes.
- Improve template create/update failure messages around registry credentials,
  image references, and unreachable registries.

## Medium Term

- Add focused regression coverage for registry retag URL normalization and HTTP
  rejection.
- Document 70-cluster rollout commands once the v2 frontend deploy path is used
  as the primary operational path.
- Consolidate duplicated v1/v2 frontend notes where behavior is intentionally
  shared.

## Deferred

- Controller registry transport changes.
- Runtime snapshotter changes.
- Full redesign of the DevBox app shell.
