#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
deploy_dir="$(cd "${script_dir}/.." && pwd)"
repo_root="$(cd "${deploy_dir}/../.." && pwd)"
source_dir="${repo_root}/v1/controller/config/crd/bases"
target_dir="${deploy_dir}/charts/devbox-v1/crds"

mkdir -p "${target_dir}"
rm -f "${target_dir}"/*.yaml
cp "${source_dir}"/*.yaml "${target_dir}/"

echo "Synced Devbox v1 CRDs into ${target_dir}"
