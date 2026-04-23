#!/usr/bin/env bash
set -euo pipefail

# Rotate Devbox SSH credentials and per-Devbox JWT secrets.
#
# What this script does for each Devbox in a fixed manifest:
#   1. Delete Secret <namespace>/<devbox-name>
#   2. Wait until the Secret is recreated with a different public key
#   3. If a Pod with the same name existed at baseline, request Pod deletion
#
# What this script intentionally does NOT do:
#   - It does not wait for the Pod to be recreated
#   - It does not wait for the new Pod to become Ready
#
# Resume behavior:
#   - The first run snapshots a manifest into STATE_DIR/manifest.tsv
#   - Per-target progress is stored under STATE_DIR/targets/
#   - If the script exits midway, rerun it with the same STATE_DIR and it
#     continues from the first unfinished target using the original baseline
#
# Optional env:
#   KUBECTL_BIN            (default: kubectl)
#   KUBECTL_REQUEST_TIMEOUT (default: 20s)
#   STATE_DIR              (default: $PWD/.devbox-credential-rotation-state)
#   SECRET_WAIT_TIMEOUT    (default: 300)
#   SECRET_POLL_INTERVAL   (default: 2)
#
# Usage:
#   ./v2/server/scripts/rotate-devbox-credentials.sh
#   STATE_DIR=/var/tmp/devbox-rotate-20260416 ./v2/server/scripts/rotate-devbox-credentials.sh

: "${KUBECTL_BIN:=kubectl}"
: "${KUBECTL_REQUEST_TIMEOUT:=20s}"
: "${STATE_DIR:=${PWD}/.devbox-credential-rotation-state}"
: "${SECRET_WAIT_TIMEOUT:=300}"
: "${SECRET_POLL_INTERVAL:=2}"

SCRIPT_NAME="$(basename "$0")"
MANIFEST_FILE="${STATE_DIR}/manifest.tsv"
TARGETS_DIR="${STATE_DIR}/targets"
LOCK_DIR="${STATE_DIR}/lock"
CURRENT_TARGET_FILE="${STATE_DIR}/current_target"
LAST_COMPLETED_FILE="${STATE_DIR}/last_completed_target"

log() {
  echo "[${SCRIPT_NAME}] $*"
}

warn() {
  echo "[${SCRIPT_NAME}] WARN: $*" >&2
}

fail() {
  echo "[${SCRIPT_NAME}] ERROR: $*" >&2
  exit 1
}

now_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

kubectl_cmd() {
  "${KUBECTL_BIN}" --request-timeout="${KUBECTL_REQUEST_TIMEOUT}" "$@"
}

write_file_atomic() {
  local path="$1"
  local value="$2"
  local tmp

  tmp="${path}.tmp.$$"
  printf '%s\n' "${value}" >"${tmp}"
  mv "${tmp}" "${path}"
}

write_target_marker() {
  local path="$1"
  local index="$2"
  local namespace="$3"
  local name="$4"
  local tmp

  tmp="${path}.tmp.$$"
  printf '%s\t%s\t%s\n' "${index}" "${namespace}" "${name}" >"${tmp}"
  mv "${tmp}" "${path}"
}

read_file() {
  local path="$1"
  if [[ -f "${path}" ]]; then
    cat "${path}"
  fi
}

cleanup() {
  rm -rf "${LOCK_DIR}"
}

acquire_lock() {
  local lock_pid

  mkdir -p "${STATE_DIR}" "${TARGETS_DIR}"
  if mkdir "${LOCK_DIR}" 2>/dev/null; then
    write_file_atomic "${LOCK_DIR}/pid" "$$"
    return
  fi

  lock_pid="$(read_file "${LOCK_DIR}/pid")"
  if [[ -n "${lock_pid}" ]] && kill -0 "${lock_pid}" 2>/dev/null; then
    fail "another run appears to be active (pid=${lock_pid}); lock=${LOCK_DIR}"
  fi

  warn "found stale lock ${LOCK_DIR}; removing it"
  rm -rf "${LOCK_DIR}"
  mkdir "${LOCK_DIR}"
  write_file_atomic "${LOCK_DIR}/pid" "$$"
}

require_dependencies() {
  command -v "${KUBECTL_BIN}" >/dev/null 2>&1 || fail "missing kubectl binary: ${KUBECTL_BIN}"
}

capture_manifest() {
  local tmp

  if [[ -f "${MANIFEST_FILE}" ]]; then
    log "reusing manifest ${MANIFEST_FILE}"
    return
  fi

  tmp="${MANIFEST_FILE}.tmp.$$"
  kubectl_cmd get devboxes.devbox.sealos.io -A \
    -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,STATE:.spec.state \
    --no-headers \
    | awk 'NF == 3 { print $1 "\t" $2 "\t" $3 }' \
    | LC_ALL=C sort -k1,1 -k2,2 >"${tmp}"

  [[ -s "${tmp}" ]] || fail "no devboxes were found"

  mv "${tmp}" "${MANIFEST_FILE}"
  log "captured manifest with $(wc -l <"${MANIFEST_FILE}" | tr -d ' ') devboxes"
}

devbox_exists() {
  local namespace="$1"
  local name="$2"

  kubectl_cmd get devboxes.devbox.sealos.io -n "${namespace}" "${name}" >/dev/null 2>&1
}

read_kubectl_value_or_empty() {
  local attempt output
  output=""

  for attempt in 1 2 3; do
    if output="$("${KUBECTL_BIN}" --request-timeout="${KUBECTL_REQUEST_TIMEOUT}" "$@" 2>/dev/null)"; then
      printf '%s' "${output}"
      return 0
    fi
    sleep 1
  done

  return 1
}

get_secret_public_key() {
  local namespace="$1"
  local name="$2"
  local value

  if value="$(read_kubectl_value_or_empty get secret -n "${namespace}" "${name}" -o jsonpath='{.data.SEALOS_DEVBOX_PUBLIC_KEY}')"; then
    printf '%s' "${value}"
  fi
}

get_pod_uid() {
  local namespace="$1"
  local name="$2"
  local value

  if value="$(read_kubectl_value_or_empty get pod -n "${namespace}" "${name}" -o jsonpath='{.metadata.uid}')"; then
    printf '%s' "${value}"
  fi
}

secret_is_rotated() {
  local current_pub="$1"
  local old_pub="$2"

  [[ -n "${current_pub}" ]] || return 1

  if [[ -z "${old_pub}" ]]; then
    return 0
  fi

  [[ "${current_pub}" != "${old_pub}" ]]
}

wait_for_rotated_secret() {
  local namespace="$1"
  local name="$2"
  local old_pub="$3"
  local deadline now current_pub

  deadline=$(( $(date +%s) + SECRET_WAIT_TIMEOUT ))

  while true; do
    current_pub="$(get_secret_public_key "${namespace}" "${name}" || true)"
    if secret_is_rotated "${current_pub}" "${old_pub}"; then
      printf '%s\n' "${current_pub}"
      return 0
    fi

    now="$(date +%s)"
    if (( now >= deadline )); then
      return 1
    fi

    sleep "${SECRET_POLL_INTERVAL}"
  done
}

terminal_phase() {
  local phase="$1"
  [[ "${phase}" == "done" || "${phase}" == "missing" ]]
}

ensure_baseline() {
  local namespace="$1"
  local name="$2"
  local desired_state="$3"
  local target_dir="$4"
  local phase old_pub old_uid

  phase="$(read_file "${target_dir}/phase")"
  if [[ -n "${phase}" ]]; then
    return
  fi

  if ! devbox_exists "${namespace}" "${name}"; then
    write_file_atomic "${target_dir}/phase" "missing"
    write_file_atomic "${target_dir}/result" "devbox_missing"
    write_file_atomic "${target_dir}/completed_at" "$(now_utc)"
    return
  fi

  old_pub="$(get_secret_public_key "${namespace}" "${name}" || true)"
  old_uid="$(get_pod_uid "${namespace}" "${name}" || true)"

  write_file_atomic "${target_dir}/namespace" "${namespace}"
  write_file_atomic "${target_dir}/name" "${name}"
  write_file_atomic "${target_dir}/desired_state" "${desired_state}"
  write_file_atomic "${target_dir}/old_pub" "${old_pub}"
  write_file_atomic "${target_dir}/old_uid" "${old_uid}"
  write_file_atomic "${target_dir}/started_at" "$(now_utc)"
  write_file_atomic "${target_dir}/phase" "baseline_captured"
}

process_target() {
  local index="$1"
  local namespace="$2"
  local name="$3"
  local desired_state="$4"
  local target_id target_dir phase old_pub old_uid current_pub current_uid new_pub

  target_id="${namespace}__${name}"
  target_dir="${TARGETS_DIR}/${target_id}"
  mkdir -p "${target_dir}"
  write_target_marker "${CURRENT_TARGET_FILE}" "${index}" "${namespace}" "${name}"

  ensure_baseline "${namespace}" "${name}" "${desired_state}" "${target_dir}"

  phase="$(read_file "${target_dir}/phase")"
  if terminal_phase "${phase}"; then
    log "[${index}] skipping ${namespace}/${name}; phase=${phase}"
    return
  fi

  old_pub="$(read_file "${target_dir}/old_pub")"
  old_uid="$(read_file "${target_dir}/old_uid")"

  log "[${index}] processing ${namespace}/${name} (state=${desired_state})"

  if [[ "${phase}" != "secret_rotated" ]]; then
    current_pub="$(get_secret_public_key "${namespace}" "${name}" || true)"
    if secret_is_rotated "${current_pub}" "${old_pub}"; then
      log "[${index}] secret is already rotated relative to baseline"
      new_pub="${current_pub}"
    else
      log "[${index}] deleting secret ${namespace}/${name}"
      kubectl_cmd delete secret -n "${namespace}" "${name}" --ignore-not-found=true >/dev/null

      log "[${index}] waiting for secret ${namespace}/${name} to be recreated"
      if ! new_pub="$(wait_for_rotated_secret "${namespace}" "${name}" "${old_pub}")"; then
        fail "timed out waiting for secret recreation for ${namespace}/${name}"
      fi
    fi

    write_file_atomic "${target_dir}/new_pub" "${new_pub}"
    write_file_atomic "${target_dir}/secret_rotated_at" "$(now_utc)"
    write_file_atomic "${target_dir}/phase" "secret_rotated"
  else
    log "[${index}] secret already rotated in a previous attempt"
  fi

  if [[ -n "${old_uid}" ]]; then
    current_uid="$(get_pod_uid "${namespace}" "${name}" || true)"
    if [[ -z "${current_uid}" ]]; then
      log "[${index}] pod ${namespace}/${name} is already absent; no delete needed"
    elif [[ "${current_uid}" != "${old_uid}" ]]; then
      log "[${index}] pod ${namespace}/${name} has already been replaced; no delete needed"
    else
      log "[${index}] requesting pod deletion for ${namespace}/${name} (uid=${old_uid})"
      kubectl_cmd delete pod -n "${namespace}" "${name}" --wait=false --ignore-not-found=true >/dev/null
    fi
  else
    log "[${index}] no baseline pod found for ${namespace}/${name}; skipping pod deletion"
  fi

  write_file_atomic "${target_dir}/result" "completed"
  write_file_atomic "${target_dir}/completed_at" "$(now_utc)"
  write_file_atomic "${target_dir}/phase" "done"
  write_target_marker "${LAST_COMPLETED_FILE}" "${index}" "${namespace}" "${name}"
}

count_targets_with_phase() {
  local wanted_phase="$1"
  local dir phase count

  count=0
  for dir in "${TARGETS_DIR}"/*; do
    [[ -d "${dir}" ]] || continue
    phase="$(read_file "${dir}/phase")"
    if [[ "${phase}" == "${wanted_phase}" ]]; then
      count=$((count + 1))
    fi
  done

  printf '%s' "${count}"
}

summarize() {
  local total done missing

  total="$(wc -l <"${MANIFEST_FILE}" | tr -d ' ')"
  done="$(count_targets_with_phase "done")"
  missing="$(count_targets_with_phase "missing")"

  log "summary: completed=${done}, missing=${missing}, total=${total}, state_dir=${STATE_DIR}"
  log "to start a fresh campaign, remove ${STATE_DIR} or point STATE_DIR at a new directory"
}

main() {
  local index namespace name desired_state

  require_dependencies
  acquire_lock
  trap cleanup EXIT

  capture_manifest
  if [[ -f "${CURRENT_TARGET_FILE}" ]]; then
    log "resume marker: $(read_file "${CURRENT_TARGET_FILE}")"
  fi
  if [[ -f "${LAST_COMPLETED_FILE}" ]]; then
    log "last completed target: $(read_file "${LAST_COMPLETED_FILE}")"
  fi

  index=0
  while IFS=$'\t' read -r namespace name desired_state; do
    index=$((index + 1))
    [[ -n "${namespace}" && -n "${name}" && -n "${desired_state}" ]] || continue
    process_target "${index}" "${namespace}" "${name}" "${desired_state}"
  done <"${MANIFEST_FILE}"

  rm -f "${CURRENT_TARGET_FILE}"
  summarize
}

main "$@"
