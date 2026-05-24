# GitHub Actions Release Flow

This repository publishes CI artifacts and container images from `github.com/sealos-apps/devbox`.

## Workflows

- `CI`
  Runs controller tests, server checks, frontend verification, gateway/service builds, and VS Code extension verification for `v1` and `v2`.
- `Images`
  Builds and pushes the following images to GHCR:
  - `ghcr.io/sealos-apps/devbox-v1-controller`
  - `ghcr.io/sealos-apps/devbox-v1-frontend`
  - `ghcr.io/sealos-apps/devbox-v1-cluster`
  - `ghcr.io/sealos-apps/devbox-v1-cri-shim-patch`
  - `ghcr.io/sealos-apps/devbox-v2-controller`
  - `ghcr.io/sealos-apps/devbox-v2-frontend`
  - `ghcr.io/sealos-apps/devbox-v2-server`
  - `ghcr.io/sealos-apps/devbox-v2-httpgate`
  - `ghcr.io/sealos-apps/devbox-v2-sshgate`
  The workflow keeps v1 and v2 image build jobs separate. `devbox-v1-cluster`
  and v1 patch OSS packaging depend only on the v1 image job, so unrelated v2
  image failures do not block v1 cluster image publication.
  On `main`, it also uploads offline image packages for `devbox-v1-cluster` and `devbox-v1-cri-shim-patch` to OSS.
- `Release`
  Triggers on `v*` tags, creates a GitHub Release, and uploads generated controller manifests plus `v1-cri-shim`, `v2-server`, `v2-httpgate`, and `v2-sshgate` release artifacts.
  The release flow keeps large offline image packages out of GitHub Release assets and uploads them to OSS instead.

## Trigger Rules

- Pull requests: run `CI`
- Push to `main`: run `CI` and `Images`
- Push tag `v*`: run `Release`, including release image builds
- Manual dispatch: run `Images`

## OSS Offline Image Packages

The workflows upload compressed `docker save` packages to OSS for offline distribution:

- Main branch:
  - `ci/main/<short_sha>/devbox-v1-cluster-main-<short_sha>-<arch>.tar`
  - `ci/main/<short_sha>/devbox-v1-cri-shim-patch-main-<short_sha>-<arch>.tar`
- Release tags:
  - `release/<tag>/devbox-v1-cluster-<tag>-<arch>.tar`
  - `release/<tag>/devbox-v1-cri-shim-patch-<tag>-<arch>.tar`

Each package is uploaded with a matching `.md5` file.

## Required GitHub Permissions

The workflows are designed to use the built-in `GITHUB_TOKEN`.

- `contents: read` for CI
- `packages: write` for image publishing
- `contents: write` for GitHub Release creation

No extra registry secret is required when publishing to `ghcr.io` from the same repository owner, as long as GitHub Actions package write access is enabled.

OSS uploads require these repository settings:

- `secrets.OSS_ENDPOINT`
- `secrets.OSS_ACCESS_KEY_ID`
- `secrets.OSS_ACCESS_KEY_SECRET`
- `vars.OSS_BUCKET`
