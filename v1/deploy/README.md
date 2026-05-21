# Devbox v1 Sealos Packaging

This directory contains the aggregated Sealos cluster-image packaging for Devbox v1.

## Layout

- `Kubefile`: cluster image build recipe
- `install.sh`: install entrypoint executed by the cluster image
- `charts/devbox-v1`: Helm chart for v1 controller, frontend, ingress, App CR, and future components
- `scripts/sync-crds.sh`: copies generated v1 controller CRDs into the chart before packaging

## Local Packaging Flow

1. Build and push `devbox-v1-controller` and `devbox-v1-frontend` runtime images.
2. Update `charts/devbox-v1/values.yaml` image repositories and tags, or override them in the user values file.
3. Run `./scripts/sync-crds.sh`.
4. Run `sealos registry save --registry-dir=registry_<arch> --arch <arch> .` in this directory.
5. Build `Kubefile` to produce the final cluster image.

## User Overrides

During install, `install.sh` ensures this file exists:

- `/root/.sealos/cloud/values/apps/devbox-v1/devbox-v1-values.yaml`

When it does not exist yet, it is initialized from:

- `charts/devbox-v1/devbox-v1-values.yaml`

This user values file keeps frequently changed settings such as resource sizing,
controller matchers, frontend feature flags, `platform.databaseProvider`,
and `frontend.env.enabledIDEs`. Cluster-derived settings such as domain,
registry credentials, database URL, JWT secret, region UID, and TLS verification
are injected by `install.sh`. Frontend registry retagging uses HTTPS and does
not read `frontend.env.registryInsecure`.

Only platform values that can be derived from Sealos ConfigMaps or installer
environment variables are injected automatically by `install.sh`. Optional
frontend deployment settings such as advanced config toggles, CPU/memory slider
marks, storage defaults, NFS storage class, WebIDE port, domain challenge secret,
and custom scripts are left empty by default and are rendered only when set in
the user values file or global values.

## Install Behavior

- Devbox runtime resources are deployed into `devbox-system`.
- The Sealos `App` resource stays in `app-system`.
- Before the first Helm install, `install.sh` removes known legacy YAML deployed Devbox v1 resources. If the Helm release already exists, this cleanup is skipped so normal Helm upgrades keep ownership intact.
