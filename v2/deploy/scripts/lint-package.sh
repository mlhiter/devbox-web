#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPONENTS_DIR="${DEPLOY_DIR}/components"

if [ ! -d "${COMPONENTS_DIR}" ]; then
  echo "components directory is missing; run scripts/prepare-package.sh first" >&2
  exit 1
fi

grep -q 'devbox-v2-controller' "${COMPONENTS_DIR}/controller/manifests/deploy.yaml"

helm lint "${COMPONENTS_DIR}/frontend/charts/devbox-v2-frontend"
helm template devbox-v2-frontend "${COMPONENTS_DIR}/frontend/charts/devbox-v2-frontend" --namespace devbox-frontend >/tmp/devbox-v2-frontend-rendered.yaml

helm lint "${COMPONENTS_DIR}/httpgate/charts/httpgate"
helm template httpgate "${COMPONENTS_DIR}/httpgate/charts/httpgate" --namespace devbox-system >/tmp/devbox-v2-httpgate-rendered.yaml

helm lint "${COMPONENTS_DIR}/sshgate/charts/sshgate"
helm template sshgate "${COMPONENTS_DIR}/sshgate/charts/sshgate" --namespace devbox-system >/tmp/devbox-v2-sshgate-rendered.yaml
