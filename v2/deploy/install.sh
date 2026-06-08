#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_FILE="${TOOLS_FILE:-/root/.sealos/cloud/scripts/tools.sh}"

load_cloud_tools() {
  if [ -f "${TOOLS_FILE}" ]; then
    # shellcheck source=/dev/null
    source "${TOOLS_FILE}"
    if declare -f ensure_global_values_ready_for_component >/dev/null 2>&1; then
      ensure_global_values_ready_for_component
    fi
  fi
}

info_msg() {
  if declare -f info >/dev/null 2>&1; then
    info "$*"
  else
    printf '[INFO] %s\n' "$*"
  fi
}

warn_msg() {
  if declare -f warn >/dev/null 2>&1; then
    warn "$*"
  else
    printf '[WARN] %s\n' "$*" >&2
  fi
}

get_configmap_data() {
  local namespace=$1
  local name=$2
  local key=$3

  if declare -f fetch_configmap_data_key >/dev/null 2>&1; then
    fetch_configmap_data_key "${name}" "${key}" "${namespace}" 1 0 2>/dev/null || true
    return
  fi

  kubectl get configmap "${name}" -n "${namespace}" -o "jsonpath={.data.${key}}" 2>/dev/null || true
}

value_or_default() {
  local value=$1
  local fallback=$2

  if [ -n "${value}" ]; then
    printf '%s' "${value}"
  else
    printf '%s' "${fallback}"
  fi
}

escape_sed_replacement() {
  printf '%s' "$1" | sed 's/[&#]/\\&/g'
}

replace_controller_arg() {
  local name=$1
  local value=$2
  local file=$3

  [ -n "${value}" ] || return 0

  local escaped_value
  escaped_value="$(escape_sed_replacement "${value}")"
  sed -i.bak -E "s#--${name}=[^[:space:]]+#--${name}=${escaped_value}#g" "${file}"
  rm -f "${file}.bak"
}

apply_controller_manifests() {
  local tmpdir
  tmpdir="$(mktemp -d)"
  cp -R "${ROOT_DIR}/components/controller/manifests" "${tmpdir}/manifests"

  local registry_addr registry_user registry_password
  registry_addr="$(value_or_default "${registryAddr:-${REGISTRY_ADDR:-}}" "$(get_configmap_data sealos-system registry-config REGISTRY_ADDR)")"
  registry_addr="$(value_or_default "${registry_addr}" "$(get_configmap_data sealos-system devbox-config registryAddress)")"
  registry_addr="$(value_or_default "${registry_addr}" "sealos.hub:5000")"
  registry_user="$(value_or_default "${registryUser:-${REGISTRY_USER:-}}" "$(get_configmap_data sealos-system registry-config ADMIN_USER)")"
  registry_user="$(value_or_default "${registry_user}" "$(get_configmap_data sealos-system devbox-config registryUsername)")"
  registry_user="$(value_or_default "${registry_user}" "admin")"
  registry_password="$(value_or_default "${registryPassword:-${REGISTRY_PASSWORD:-}}" "$(get_configmap_data sealos-system registry-config ADMIN_PASSWORD)")"
  registry_password="$(value_or_default "${registry_password}" "$(get_configmap_data sealos-system devbox-config registryPassword)")"
  registry_password="$(value_or_default "${registry_password}" "passw0rd")"

  if [ -z "${registry_user}" ] || [ -z "${registry_password}" ]; then
    warn_msg "registry user or password was not detected. Controller will keep its built-in defaults; frontend install may still require registry credentials."
  fi

  while IFS= read -r manifest; do
    replace_controller_arg "registry-addr" "${registry_addr}" "${manifest}"
    replace_controller_arg "registry-user" "${registry_user}" "${manifest}"
    replace_controller_arg "registry-password" "${registry_password}" "${manifest}"
  done < <(find "${tmpdir}/manifests" -type f -name '*.yaml' -print)

  info_msg "Applying DevBox v2 controller manifests"
  kubectl apply -f "${tmpdir}/manifests"
  rm -rf "${tmpdir}"
}

install_component() {
  local name=$1
  shift

  info_msg "Installing ${name}"
  (cd "${ROOT_DIR}/components/${name}" && "$@")
}

load_cloud_tools
apply_controller_manifests
install_component httpgate bash install.sh
install_component sshgate bash install.sh
install_component frontend env \
  registryAddr="${registryAddr:-${REGISTRY_ADDR:-}}" \
  registryUser="${registryUser:-${REGISTRY_USER:-}}" \
  registryPassword="${registryPassword:-${REGISTRY_PASSWORD:-}}" \
  bash install.sh
