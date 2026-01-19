#!/usr/bin/env bash
set -e

server=r3b
target_dir=/root

source_dir_repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../../" &>/dev/null && pwd)
source_dir_temp=$(mktemp -d -t umbilical-sync-router-XXXXXXXXXX)
source_dir_wip="${source_dir_temp}/compose"

mkdir -p "${source_dir_wip}/static/letsencrypt"
rsync -a "${source_dir_repo}/static/letsencrypt" "${source_dir_wip}/static"

mkdir -p "${source_dir_wip}/umbilical/local"
rsync -a "${source_dir_repo}/umbilical/local/router" "${source_dir_wip}/umbilical/local"

my-secret render "${source_dir_wip}"

ssh "${server}" mkdir -p "${target_dir}"
rsync -rvh --delete --progress "${source_dir_wip}" "${server}":"${target_dir}"

rm -rf "${source_dir_wip}"
