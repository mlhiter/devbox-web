# GitHub Actions Release Flow

This repository publishes CI artifacts and container images from `github.com/sealos-apps/devbox`.

## Workflows

- `CI`
  Runs controller tests, server checks, frontend verification, and gateway/service builds for `v1` and `v2`.
- `Images`
  Builds and pushes the following images to GHCR:
  - `ghcr.io/sealos-apps/devbox-v1-controller`
  - `ghcr.io/sealos-apps/devbox-v1-frontend`
  - `ghcr.io/sealos-apps/devbox-v2-controller`
  - `ghcr.io/sealos-apps/devbox-v2-frontend`
  - `ghcr.io/sealos-apps/devbox-v2-server`
  - `ghcr.io/sealos-apps/devbox-v2-httpgate`
  - `ghcr.io/sealos-apps/devbox-v2-sshgate`
- `Release`
  Triggers on `v*` tags, creates a GitHub Release, and uploads generated controller manifests plus `v1-cri-shim`, `v2-server`, `v2-httpgate`, and `v2-sshgate` release artifacts.

## Trigger Rules

- Pull requests: run `CI`
- Push to `main`: run `CI` and `Images`
- Push tag `v*`: run `Images` and `Release`
- Manual dispatch: run `Images`

## Required GitHub Permissions

The workflows are designed to use the built-in `GITHUB_TOKEN`.

- `contents: read` for CI
- `packages: write` for image publishing
- `contents: write` for GitHub Release creation

No extra registry secret is required when publishing to `ghcr.io` from the same repository owner, as long as GitHub Actions package write access is enabled.
