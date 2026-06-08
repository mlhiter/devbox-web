# Roadmap

## Current Focus

- Keep v2 as the current development line.
- Sync v1 frontend features and bug fixes into v2 only when they remain valid for v2 contracts.
- Preserve v2 backend, controller, gateway, CRD, storage, and deployment semantics during frontend sync work.

## Near Term

- Finish frontend parity items that improve create/edit/detail workflows.
- Keep GPU, quota, IDE, ConfigMap, runtime asset, and import flows covered by focused verification.
- Maintain one logical commit per synced behavior so regressions can be traced and reverted cleanly.

## Verification Priorities

- `pnpm -C v2/frontend ts-lint`
- `pnpm -C v2/frontend build`
- `git diff --check`
- Targeted browser checks when UI layout or interaction changes.

## Deferred Unless Explicitly Requested

- Backend/controller migrations.
- CRD API version rewrites.
- v1/v2 deploy YAML convergence.
- Shared memory or owner-reference behavior changes that require backend ownership decisions.
- `lastTerminatedReason` UI/event parity, which is intentionally not part of the current sync batch.
