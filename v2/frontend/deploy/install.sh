#!/usr/bin/env bash
set -euo pipefail

load_cloud_tools_or_exit() {
  local tools_file="${TOOLS_FILE:-/root/.sealos/cloud/scripts/tools.sh}"
  local required_functions=(
    ensure_global_values_ready_for_component
    global_http_disable_https
    info
    warn
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

RELEASE_NAME="${RELEASE_NAME:-devbox-v2-frontend}"
NAMESPACE="${NAMESPACE:-devbox-frontend}"
HELM_OPTS="${HELM_OPTS:-}"
DEFAULT_VALUES_FILE="./devbox-v2-frontend-values.yaml"
USER_VALUES_DIR="/root/.sealos/cloud/values/apps/devbox"
USER_VALUES_FILE="${USER_VALUES_DIR}/devbox-v2-frontend-values.yaml"
GLOBAL_VALUES_FILE="/root/.sealos/cloud/values/global.yaml"
TOOLS_FILE="${TOOLS_FILE:-/root/.sealos/cloud/scripts/tools.sh}"

load_cloud_tools_or_exit

get_configmap_data() {
  local namespace=$1
  local name=$2
  local key=$3

  fetch_configmap_data_key "${name}" "${key}" "${namespace}" 1 0 2>/dev/null || true
}

get_secret_data() {
  local namespace=$1
  local name=$2
  local key=$3
  local raw=""

  raw="$(kubectl get secret "${name}" -n "${namespace}" -o "jsonpath={.data.${key}}" 2>/dev/null || true)"
  if [ -n "${raw}" ]; then
    printf '%s' "${raw}" | base64 -d 2>/dev/null || true
  fi
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi

  tr -dc 'a-z0-9' </dev/urandom | head -c64
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

ensure_user_values_file() {
  mkdir -p "${USER_VALUES_DIR}"

  if [ ! -f "${USER_VALUES_FILE}" ]; then
    cp "${DEFAULT_VALUES_FILE}" "${USER_VALUES_FILE}"
    info "Generated default user values at ${USER_VALUES_FILE}"
    return 0
  fi

  info "Using user values from ${USER_VALUES_FILE}"
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
registry_password="$(value_or_default "${registryPassword:-${REGISTRY_PASSWORD:-}}" "$(get_configmap_data sealos-system registry-config ADMIN_PASSWORD)")"
registry_password="$(value_or_default "${registry_password}" "$(get_configmap_data sealos-system devbox-config registryPassword)")"
if [ -z "${registry_user}" ] || [ -z "${registry_password}" ]; then
  warn "registry user or password was not detected. Set registry.user and registry.password in ${USER_VALUES_FILE}, export REGISTRY_USER/REGISTRY_PASSWORD, or ensure sealos-system/registry-config contains ADMIN_USER and ADMIN_PASSWORD."
  exit 1
fi

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
  warn "regionUID was not detected; set platform.regionUid in ${USER_VALUES_FILE} or the regionUID key in sealos-system/sealos-config."
  exit 1
fi

jwt_secret="$(value_or_default "${jwtSecret:-${JWT_SECRET:-}}" "$(read_jwt_internal)")"
if [ -z "${jwt_secret}" ]; then
  warn "jwtSecret was not detected; set platform.jwtSecret in ${USER_VALUES_FILE} or ensure it can be read from the cluster configuration."
  exit 1
fi

domain_challenge_secret="$(value_or_default "${devboxDomainChallengeSecret:-${DEVBOX_DOMAIN_CHALLENGE_SECRET:-}}" "")"
domain_challenge_secret="$(value_or_default "${domain_challenge_secret}" "$(get_configmap_data sealos-system devbox-config domainChallengeSecret)")"
domain_challenge_secret="$(value_or_default "${domain_challenge_secret}" "$(get_secret_data "${NAMESPACE}" devbox-frontend-runtime DEVBOX_DOMAIN_CHALLENGE_SECRET)")"
domain_challenge_secret="$(value_or_default "${domain_challenge_secret}" "$(random_secret)")"

metrics_url="$(value_or_default "${metricsUrl:-${METRICS_URL:-}}" "$(get_configmap_data vm vmselect-vm-stack-victoria-metrics-k8s-stack prometheusURL)")"
metrics_url="$(value_or_default "${metrics_url}" "http://vmselect-vm-stack-victoria-metrics-k8s-stack.vm.svc.cluster.local:8481/select/0/prometheus")"

app_launchpad_url="$(value_or_default "${appLaunchpadUrl:-${APP_LAUNCHPAD_URL:-}}" "http://applaunchpad-frontend.applaunchpad-frontend.svc.cluster.local:3000/api/v1alpha")"
storage_limit="$(value_or_default "${storageLimit:-${STORAGE_LIMIT:-}}" "20Gi")"
runtime_class_name="$(value_or_default "${runtimeClassName:-${DEVBOX_RUNTIME_CLASS_NAME:-}}" "devbox-runtime")"
storage_default="$(value_or_default "${storageDefault:-${STORAGE_DEFAULT:-}}" "20")"
enable_advanced_config="$(value_or_default "${enableAdvancedConfig:-${ENABLE_ADVANCED_CONFIG:-}}" "true")"
gpu_scheduler_mode="$(value_or_default "${gpuSchedulerMode:-${GPU_SCHEDULER_MODE:-}}" "native")"
gpu_enable="$(value_or_default "${gpuEnable:-${GPU_ENABLE:-}}" "false")"
billing_currency="$(read_yaml_file_path '.global.billing.currency')"
billing_currency="$(value_or_default "${billing_currency}" "cny")"

ensure_user_values_file

helm_set_args=(
  --set-string "cloudDomain=${cloud_domain}"
  --set-string "cloudPort=${cloud_port}"
  --set-string "httpPort=${http_port}"
  --set-string "disableHttps=${disable_https}"
  --set-string "certSecretName=${cert_secret_name}"
  --set-string "registry.addr=${registry_addr}"
  --set-string "registry.user=${registry_user}"
  --set-string "registry.password=${registry_password}"
  --set-string "platform.databaseUrl=${database_url}"
  --set-string "platform.jwtSecret=${jwt_secret}"
  --set-string "platform.regionUid=${region_uid}"
  --set-string "platform.tlsRejectUnauthorized=${tls_reject_unauthorized}"
  --set-string "frontend.env.currencySymbol=${billing_currency}"
  --set-string "frontend.env.metricsUrl=${metrics_url}"
  --set-string "frontend.env.appLaunchpadUrl=${app_launchpad_url}"
  --set-string "frontend.env.storageLimit=${storage_limit}"
  --set-string "frontend.env.runtimeClassName=${runtime_class_name}"
  --set-string "frontend.env.storageDefault=${storage_default}"
  --set-string "frontend.env.enableAdvancedConfig=${enable_advanced_config}"
  --set-string "frontend.env.gpuSchedulerMode=${gpu_scheduler_mode}"
  --set-string "frontend.env.gpuEnable=${gpu_enable}"
  --set-string "frontend.env.domainChallengeSecret=${domain_challenge_secret}"
)

if [ -f "${GLOBAL_VALUES_FILE}" ]; then
  helm_set_args+=(-f "${GLOBAL_VALUES_FILE}")
fi

helm_opts_arr=()
if [ -n "${HELM_OPTS}" ]; then
  # shellcheck disable=SC2206
  helm_opts_arr=(${HELM_OPTS})
fi

info "Installing chart charts/devbox-v2-frontend into namespace ${NAMESPACE}"
helm upgrade -i "${RELEASE_NAME}" -n "${NAMESPACE}" --create-namespace charts/devbox-v2-frontend \
  -f "${DEFAULT_VALUES_FILE}" \
  -f "${USER_VALUES_FILE}" \
  "${helm_set_args[@]}" \
  "${helm_opts_arr[@]}" \
  --wait
