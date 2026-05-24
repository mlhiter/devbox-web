#!/usr/bin/env bash
set -euo pipefail

timestamp() {
  date +"%Y-%m-%d %T"
}

info() {
  local flag
  flag="$(timestamp)"
  echo -e "\033[36m INFO [$flag] >> $* \033[0m"
}

warn() {
  local flag
  flag="$(timestamp)"
  echo -e "\033[33m WARN [$flag] >> $* \033[0m"
}

RELEASE_NAME="${RELEASE_NAME:-devbox-v1}"
NAMESPACE="${NAMESPACE:-devbox-system}"
HELM_OPTS="${HELM_OPTS:-}"
DEFAULT_VALUES_FILE="./charts/devbox-v1/devbox-v1-values.yaml"
USER_VALUES_DIR="/root/.sealos/cloud/values/apps/devbox-v1"
USER_VALUES_FILE="${USER_VALUES_DIR}/devbox-v1-values.yaml"
GLOBAL_VALUES_FILE="/root/.sealos/cloud/values/global.yaml"

get_configmap_data() {
  local namespace=$1
  local name=$2
  local key=$3

  kubectl get configmap "${name}" -n "${namespace}" -o "jsonpath={.data.${key}}" 2>/dev/null || true
}

get_desktop_config_value() {
  local key=$1
  local raw=""

  raw="$(kubectl get configmap desktop-frontend-config -n sealos -o "jsonpath={.data.config\\.yaml}" 2>/dev/null || true)"
  [ -n "${raw}" ] || return 0

  printf '%s\n' "${raw}" | awk -v key="${key}" '$1 == key ":" { gsub(/"/, "", $2); print $2; exit }'
}

get_tls_reject_unauthorized() {
  local cert_mode

  cert_mode="$(kubectl get configmap cert-config -n sealos-system -o jsonpath='{.data.CERT_MODE}' 2>/dev/null || true)"
  cert_mode="$(printf '%s' "${cert_mode}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"

  case "${cert_mode}" in
    https|acme)
      printf '0'
      ;;
    *)
      printf '1'
      ;;
  esac
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi

  tr -dc 'a-z0-9' </dev/urandom | head -c64
}

ensure_user_values_file() {
  mkdir -p "${USER_VALUES_DIR}"

  if [ ! -f "${USER_VALUES_FILE}" ]; then
    cp "${DEFAULT_VALUES_FILE}" "${USER_VALUES_FILE}"
    info "Generated default user values at ${USER_VALUES_FILE}"
    return 0
  fi

  info "Using user values from ${USER_VALUES_FILE}"
}

cleanup_legacy_yaml_deploy() {
  if helm status "${RELEASE_NAME}" -n "${NAMESPACE}" >/dev/null 2>&1; then
    info "Helm release ${RELEASE_NAME} already exists in ${NAMESPACE}; skipping legacy YAML cleanup"
    return 0
  fi

  info "Cleaning legacy YAML deployed Devbox v1 resources before Helm install"
  kubectl delete deployment devbox-controller-manager -n devbox-system --ignore-not-found=true
  kubectl delete service devbox-controller-manager-metrics-service -n devbox-system --ignore-not-found=true
  kubectl delete serviceaccount devbox-controller-manager -n devbox-system --ignore-not-found=true
  kubectl delete role devbox-leader-election-role -n devbox-system --ignore-not-found=true
  kubectl delete rolebinding devbox-leader-election-rolebinding devbox-default-user-rolebinding -n devbox-system --ignore-not-found=true

  kubectl delete clusterrole \
    devbox-devbox-admin-role \
    devbox-devbox-editor-role \
    devbox-devbox-viewer-role \
    devbox-devboxrelease-admin-role \
    devbox-devboxrelease-editor-role \
    devbox-devboxrelease-viewer-role \
    devbox-manager-role \
    devbox-metrics-auth-role \
    devbox-metrics-reader \
    --ignore-not-found=true
  kubectl delete clusterrolebinding \
    devbox-manager-rolebinding \
    devbox-metrics-auth-rolebinding \
    --ignore-not-found=true

  local frontend_namespace
  for frontend_namespace in devbox-frontend devbox-system; do
    kubectl delete configmap devbox-frontend-config -n "${frontend_namespace}" --ignore-not-found=true
    kubectl delete deployment devbox-frontend -n "${frontend_namespace}" --ignore-not-found=true
    kubectl delete service devbox-frontend -n "${frontend_namespace}" --ignore-not-found=true
    kubectl delete ingress devbox-frontend devbox-challenge -n "${frontend_namespace}" --ignore-not-found=true
  done

  if kubectl api-resources --api-group=app.sealos.io --no-headers 2>/dev/null | awk '$1 == "apps" { found=1 } END { exit found ? 0 : 1 }'; then
    kubectl delete apps.app.sealos.io devbox -n app-system --ignore-not-found=true
  fi
}

adopt_namespace_for_helm() {
  local namespace=$1

  kubectl create namespace "${namespace}" --dry-run=client -o yaml | kubectl apply -f -
  kubectl label namespace "${namespace}" app.kubernetes.io/managed-by=Helm --overwrite
  kubectl annotate namespace "${namespace}" \
    meta.helm.sh/release-name="${RELEASE_NAME}" \
    meta.helm.sh/release-namespace="${NAMESPACE}" \
    --overwrite
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

cloud_domain="$(value_or_default "${cloudDomain:-${CLOUD_DOMAIN:-}}" "$(get_configmap_data sealos-system sealos-config cloudDomain)")"
cloud_domain="$(value_or_default "${cloud_domain}" "127.0.0.1.nip.io")"

cloud_port="$(value_or_default "${cloudPort:-${CLOUD_PORT:-}}" "$(get_configmap_data sealos-system sealos-config cloudPort)")"
cert_secret_name="$(value_or_default "${certSecretName:-${CERT_SECRET_NAME:-}}" "wildcard-cert")"
tls_reject_unauthorized="$(get_tls_reject_unauthorized)"

registry_addr="$(value_or_default "${registryAddr:-${REGISTRY_ADDR:-}}" "$(get_configmap_data sealos-system devbox-config registryAddress)")"
registry_addr="$(value_or_default "${registry_addr}" "sealos.hub:5000")"
registry_user="$(value_or_default "${registryUser:-${REGISTRY_USER:-}}" "$(get_configmap_data sealos-system devbox-config registryUsername)")"
registry_user="$(value_or_default "${registry_user}" "admin")"
registry_password="$(value_or_default "${registryPassword:-${REGISTRY_PASSWORD:-}}" "$(get_configmap_data sealos-system devbox-config registryPassword)")"
registry_password="$(value_or_default "${registry_password}" "passw0rd")"

database_url="$(value_or_default "${databaseUrl:-${DATABASE_URL:-}}" "")"
if [ -z "${database_url}" ]; then
  global_database_url="$(get_configmap_data sealos-system sealos-config databaseGlobalCockroachdbURI)"
  if [ -n "${global_database_url}" ]; then
    database_url="$(printf '%s' "${global_database_url}" | sed 's/global/devboxdb/')"
  fi
fi
if [ -z "${database_url}" ]; then
  warn "databaseUrl was not detected. Set platform.databaseUrl in ${USER_VALUES_FILE} before relying on the frontend."
fi

region_uid="$(value_or_default "${regionUid:-${REGION_UID:-}}" "$(get_configmap_data sealos-system sealos-config regionUID)")"
region_uid="$(value_or_default "${region_uid}" "$(get_desktop_config_value regionUID)")"
if [ -z "${region_uid}" ]; then
  if command -v uuidgen >/dev/null 2>&1; then
    region_uid="$(uuidgen)"
  else
    region_uid="$(random_secret)"
  fi
  warn "regionUID was not detected; generated ${region_uid}"
fi

jwt_secret="$(value_or_default "${jwtSecret:-${JWT_SECRET:-}}" "$(get_configmap_data sealos-system sealos-config jwtInternal)")"
jwt_secret="$(value_or_default "${jwt_secret}" "$(get_desktop_config_value internal)")"
if [ -z "${jwt_secret}" ]; then
  jwt_secret="$(random_secret)"
  warn "jwtSecret was not detected; generated a new value"
fi

ensure_user_values_file
cleanup_legacy_yaml_deploy
adopt_namespace_for_helm "${NAMESPACE}"

if [ -d "./charts/devbox-v1/crds" ] && compgen -G "./charts/devbox-v1/crds/*.yaml" >/dev/null; then
  info "Applying Devbox v1 CRDs"
  kubectl apply -f ./charts/devbox-v1/crds
else
  warn "No CRDs found in ./charts/devbox-v1/crds; skipping CRD apply"
fi

helm_set_args=(
  --set-string "cloudDomain=${cloud_domain}"
  --set-string "cloudPort=${cloud_port}"
  --set-string "certSecretName=${cert_secret_name}"
  --set-string "registry.addr=${registry_addr}"
  --set-string "registry.user=${registry_user}"
  --set-string "registry.password=${registry_password}"
  --set-string "platform.databaseUrl=${database_url}"
  --set-string "platform.jwtSecret=${jwt_secret}"
  --set-string "platform.regionUid=${region_uid}"
  --set-string "platform.tlsRejectUnauthorized=${tls_reject_unauthorized}"
)

if [ -f "${GLOBAL_VALUES_FILE}" ]; then
  helm_set_args+=(-f "${GLOBAL_VALUES_FILE}")
fi

helm_opts_arr=()
if [ -n "${HELM_OPTS}" ]; then
  # shellcheck disable=SC2206
  helm_opts_arr=(${HELM_OPTS})
fi

info "Installing chart charts/devbox-v1 into namespace ${NAMESPACE}"
helm upgrade -i "${RELEASE_NAME}" -n "${NAMESPACE}" --create-namespace charts/devbox-v1 \
  -f "${DEFAULT_VALUES_FILE}" \
  -f "${USER_VALUES_FILE}" \
  "${helm_set_args[@]}" \
  "${helm_opts_arr[@]}"
