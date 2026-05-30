#!/usr/bin/env bash
set -euo pipefail

load_cloud_tools_or_exit() {
  local tools_file="/root/.sealos/cloud/scripts/tools.sh"
  local required_functions=(
    ensure_global_values_ready_for_component
    global_http_disable_https
    global_http_external_url
    read_account_service_name
    info
    warn
    error
    fetch_configmap_data_key
    read_cert_tls_reject_unauthorized
    read_jwt_internal
    read_yaml_file_path
  )
  local missing_functions=()
  local function_name

  if [ ! -f "$tools_file" ]; then
    cat >&2 <<'EOF'
错误：未找到 /root/.sealos/cloud/scripts/tools.sh，当前组件镜像无法继续执行。

请先回到当前安装包目录，执行对应命令同步 values + tools：
  Pro 安装包：./sealos-pro.sh sync-config
  OSS 安装包：./sealos-oss.sh sync-config
EOF
    exit 1
  fi

  # shellcheck source=/dev/null
  source "$tools_file"
  for function_name in "${required_functions[@]}"; do
    if ! declare -f "$function_name" >/dev/null 2>&1; then
      missing_functions+=("$function_name")
    fi
  done

  if [ "${#missing_functions[@]}" -gt 0 ]; then
    cat >&2 <<EOF
错误：/root/.sealos/cloud/scripts/tools.sh 版本过旧，缺少配置检测函数，当前组件镜像无法继续执行。

缺少函数：${missing_functions[*]}

请先回到当前安装包目录，执行对应命令同步 values + tools：
  Pro 安装包：./sealos-pro.sh sync-config
  OSS 安装包：./sealos-oss.sh sync-config
EOF
    exit 1
  fi

  ensure_global_values_ready_for_component
}

RELEASE_NAME="${RELEASE_NAME:-devbox-v1}"
NAMESPACE="${NAMESPACE:-devbox-system}"
HELM_OPTS="${HELM_OPTS:-}"
DEFAULT_VALUES_FILE="./charts/devbox-v1/devbox-v1-values.yaml"
USER_VALUES_DIR="/root/.sealos/cloud/values/apps/devbox"
USER_VALUES_FILE="${USER_VALUES_DIR}/devbox-v1-values.yaml"
GLOBAL_VALUES_FILE="/root/.sealos/cloud/values/global.yaml"
TOOLS_FILE="${TOOLS_FILE:-/root/.sealos/cloud/scripts/tools.sh}"

load_cloud_tools_or_exit
if [ -f "${TOOLS_FILE}" ]; then
  # shellcheck source=/dev/null
  source "${TOOLS_FILE}"
else
  echo "tools.sh not found at ${TOOLS_FILE}. Cannot proceed." >&2
  exit 1
fi


get_configmap_data() {
  local namespace=$1
  local name=$2
  local key=$3

  fetch_configmap_data_key "${name}" "${key}" "${namespace}" 1 0 2>/dev/null || true
}

get_desktop_config_value() {
  local key=$1
  local raw=""

  raw="$(kubectl get configmap desktop-frontend-config -n sealos -o "jsonpath={.data.config\\.yaml}" 2>/dev/null || true)"
  [ -n "${raw}" ] || return 0

  printf '%s\n' "${raw}" | awk -v key="${key}" '$1 == key ":" { gsub(/"/, "", $2); print $2; exit }'
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
  for frontend_namespace in devbox-system; do
    kubectl delete configmap devbox-frontend-config -n "${frontend_namespace}" --ignore-not-found=true
    kubectl delete deployment devbox-frontend -n "${frontend_namespace}" --ignore-not-found=true
    kubectl delete service devbox-frontend -n "${frontend_namespace}" --ignore-not-found=true
    kubectl delete ingress devbox-frontend devbox-challenge -n "${frontend_namespace}" --ignore-not-found=true
  done

  if kubectl api-resources --api-group=app.sealos.io --no-headers 2>/dev/null | awk '$1 == "apps" { found=1 } END { exit found ? 0 : 1 }'; then
    kubectl delete apps.app.sealos.io devbox -n app-system --ignore-not-found=true
  fi
}

cleanup_legacy_frontend_namespace() {
  local frontend_namespace=devbox-frontend

  if ! kubectl get namespace "${frontend_namespace}" >/dev/null 2>&1; then
    return 0
  fi

  info "Cleaning legacy frontend resources from namespace ${frontend_namespace}"
  kubectl delete configmap devbox-frontend-config -n "${frontend_namespace}" --ignore-not-found=true
  kubectl delete deployment devbox-frontend -n "${frontend_namespace}" --ignore-not-found=true
  kubectl delete service devbox-frontend -n "${frontend_namespace}" --ignore-not-found=true
  kubectl delete ingress devbox-frontend devbox-challenge -n "${frontend_namespace}" --ignore-not-found=true

  info "Deleting legacy namespace ${frontend_namespace}"
  kubectl delete namespace "${frontend_namespace}" --ignore-not-found=true
}

ensure_helm_namespace_ownership() {
  local namespace=$1

  if ! kubectl get namespace "${namespace}" >/dev/null 2>&1; then
    return 0
  fi

  info "Ensuring Helm ownership metadata on namespace ${namespace}"
  kubectl label namespace "${namespace}" \
    app.kubernetes.io/managed-by=Helm \
    app.kubernetes.io/name=devbox \
    --overwrite
  kubectl annotate namespace "${namespace}" \
    "meta.helm.sh/release-name=${RELEASE_NAME}" \
    "meta.helm.sh/release-namespace=${NAMESPACE}" \
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

cloud_port="$(value_or_default "${cloudPort:-${CLOUD_PORT:-}}" "$(read_yaml_file_path '.global.http.httpsPort')")"
http_port="$(value_or_default "${httpPort:-${HTTP_PORT:-}}" "$(read_yaml_file_path '.global.http.httpPort')")"
cert_secret_name="$(value_or_default "${certSecretName:-${CERT_SECRET_NAME:-}}" "$(read_yaml_file_path '.global.http.certSecretName')")"
cert_secret_name="$(value_or_default "${cert_secret_name}" "wildcard-cert")"
if global_http_disable_https; then
  disable_https="true"
else
  disable_https="false"
fi
tls_reject_unauthorized="$(read_cert_tls_reject_unauthorized)"

registry_addr="$(value_or_default "${registryAddr:-${REGISTRY_ADDR:-}}" "$(get_configmap_data sealos-system registry-config REGISTRY_ADDR)")"
registry_addr="$(value_or_default "${registry_addr}" "$(get_configmap_data sealos-system devbox-config registryAddress)")"
registry_addr="$(value_or_default "${registry_addr}" "sealos.hub:5000")"
registry_user="$(value_or_default "${registryUser:-${REGISTRY_USER:-}}" "$(get_configmap_data sealos-system registry-config ADMIN_USER)")"
registry_user="$(value_or_default "${registry_user}" "$(get_configmap_data sealos-system devbox-config registryUsername)")"
registry_user="$(value_or_default "${registry_user}" "admin")"
registry_password="$(value_or_default "${registryPassword:-${REGISTRY_PASSWORD:-}}" "$(get_configmap_data sealos-system registry-config ADMIN_PASSWORD)")"
registry_password="$(value_or_default "${registry_password}" "$(get_configmap_data sealos-system devbox-config registryPassword)")"
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
if [ -z "${region_uid}" ]; then
  warn "regionUID was not detected; if you are using features that rely on regionUID, set platform.regionUid in ${USER_VALUES_FILE} or set the regionUID key in the sealos-config ConfigMap."
  exit 1
fi

jwt_secret="$(value_or_default "${jwtSecret:-${JWT_SECRET:-}}" "$(read_jwt_internal)")"
if [ -z "${jwt_secret}" ]; then
  warn "jwtSecret was not detected; generating random JWT secret. This may cause issues if the frontend is redeployed, as existing tokens will become invalid. It is recommended to set a fixed jwtSecret value in ${USER_VALUES_FILE} or ensure it can be read from the cluster configuration."
  exit 1
fi

ensure_user_values_file
cleanup_legacy_yaml_deploy
cleanup_legacy_frontend_namespace
ensure_helm_namespace_ownership "${NAMESPACE}"

if [ -d "./charts/devbox-v1/crds" ] && compgen -G "./charts/devbox-v1/crds/*.yaml" >/dev/null; then
  info "Applying Devbox v1 CRDs"
  kubectl apply -f ./charts/devbox-v1/crds
else
  warn "No CRDs found in ./charts/devbox-v1/crds; skipping CRD apply"
fi

billing_currency="$(read_yaml_file_path '.global.billing.currency')"
billing_currency="$(value_or_default "${billing_currency}" "cny")"

helm_set_args=(
  --set-string "cloudDomain=${cloud_domain}"
  --set-string "cloudPort=${cloud_port}"
  --set-string "httpPort=${http_port}"
  --set-string "disableHttps=${disable_https}"
  --set-string "certSecretName=${cert_secret_name}"
  --set-string "frontend.env.currencySymbol=${billing_currency}"
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
