#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DEPLOY_DIR}/../.." && pwd)"
COMPONENTS_DIR="${DEPLOY_DIR}/components"

: "${CONTROLLER_IMAGE:?CONTROLLER_IMAGE is required}"
: "${FRONTEND_IMAGE:?FRONTEND_IMAGE is required}"
: "${HTTPGATE_IMAGE:?HTTPGATE_IMAGE is required}"
: "${SSHGATE_IMAGE:?SSHGATE_IMAGE is required}"

require_tagged_image() {
  local image=$1
  local image_name=${image##*/}

  if [[ "${image_name}" != *:* ]]; then
    printf 'image %s must include an explicit tag\n' "${image}" >&2
    exit 1
  fi
}

split_image() {
  local image=$1
  local repo_var=$2
  local tag_var=$3

  require_tagged_image "${image}"

  export "${repo_var}=${image%:*}"
  export "${tag_var}=${image##*:}"
}

replace_line() {
  local file=$1
  local pattern=$2

  sed -i.bak -E "${pattern}" "${file}"
  rm -f "${file}.bak"
}

require_tagged_image "${CONTROLLER_IMAGE}"
require_tagged_image "${FRONTEND_IMAGE}"
require_tagged_image "${HTTPGATE_IMAGE}"
require_tagged_image "${SSHGATE_IMAGE}"

rm -rf "${COMPONENTS_DIR}"
mkdir -p \
  "${COMPONENTS_DIR}/controller" \
  "${COMPONENTS_DIR}/frontend" \
  "${COMPONENTS_DIR}/httpgate" \
  "${COMPONENTS_DIR}/sshgate"

cp -R "${REPO_ROOT}/v2/controller/deploy/manifests" "${COMPONENTS_DIR}/controller/manifests"

cp -R "${REPO_ROOT}/v2/frontend/deploy/charts" "${COMPONENTS_DIR}/frontend/charts"
cp "${REPO_ROOT}/v2/frontend/deploy/install.sh" "${COMPONENTS_DIR}/frontend/install.sh"
cp "${REPO_ROOT}/v2/frontend/deploy/devbox-v2-frontend-values.yaml" "${COMPONENTS_DIR}/frontend/devbox-v2-frontend-values.yaml"

cp -R "${REPO_ROOT}/v2/httpgate/deploy/charts" "${COMPONENTS_DIR}/httpgate/charts"
cp "${REPO_ROOT}/v2/httpgate/deploy/install.sh" "${COMPONENTS_DIR}/httpgate/install.sh"

cp -R "${REPO_ROOT}/v2/sshgate/deploy/charts" "${COMPONENTS_DIR}/sshgate/charts"
cp "${REPO_ROOT}/v2/sshgate/deploy/install.sh" "${COMPONENTS_DIR}/sshgate/install.sh"

replace_line "${COMPONENTS_DIR}/controller/manifests/deploy.yaml" \
  "s#ghcr.io/[^[:space:]\"]+/devbox-v2-controller:[^[:space:]\"]+#${CONTROLLER_IMAGE}#g"

split_image "${FRONTEND_IMAGE}" FRONTEND_REPOSITORY FRONTEND_TAG
replace_line "${COMPONENTS_DIR}/frontend/charts/devbox-v2-frontend/values.yaml" \
  "s#^([[:space:]]*repository:[[:space:]]*).*#\\1${FRONTEND_REPOSITORY}#"
replace_line "${COMPONENTS_DIR}/frontend/charts/devbox-v2-frontend/values.yaml" \
  "s#^([[:space:]]*tag:[[:space:]]*).*#\\1${FRONTEND_TAG}#"

split_image "${HTTPGATE_IMAGE}" HTTPGATE_REPOSITORY HTTPGATE_TAG
replace_line "${COMPONENTS_DIR}/httpgate/charts/httpgate/values.yaml" \
  "s#^([[:space:]]*repository:[[:space:]]*).*#\\1${HTTPGATE_REPOSITORY}#"
replace_line "${COMPONENTS_DIR}/httpgate/charts/httpgate/values.yaml" \
  "s#^([[:space:]]*tag:[[:space:]]*).*#\\1${HTTPGATE_TAG}#"

replace_line "${COMPONENTS_DIR}/sshgate/charts/sshgate/values.yaml" \
  "s#^image: .*#image: ${SSHGATE_IMAGE}#"
