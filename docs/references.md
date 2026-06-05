# References

## Internal Source References

- `README.md`: repository overview, quick start, images, and release workflow.
- `v2/frontend/README.md`: frontend setup and package source notes.
- `v2/frontend/package.json`: frontend scripts and dependency versions.
- `.github/workflows/ci.yml`: CI verification commands.
- `.github/workflows/images.yml`: image build workflow for main/manual dispatch.
- `.github/workflows/release.yml`: tag release workflow.
- `.codex/environments/environment.toml`: local Codex run actions for v1/v2 frontend.

## External Documentation

- Next.js: https://nextjs.org/docs
- next-intl: https://next-intl.dev
- Kubernetes JavaScript client: https://github.com/kubernetes-client/javascript
- React Hook Form: https://react-hook-form.com
- TanStack Query: https://tanstack.com/query
- Zustand: https://zustand-demo.pmnd.rs
- Sealos: https://sealos.io/docs

## Product Comparisons

DevBox is closer to a cloud workspace control panel than a marketing site. Useful interface references are operational product tools with clear forms, tables, status indicators, and predictable lifecycle actions.

## Historical Sync Guidance

For v1-to-v2 sync, treat v1 as a source of product behavior and bug fixes, not a source of v2 architecture. Skip systemic differences unless the user explicitly asks to change those contracts.
