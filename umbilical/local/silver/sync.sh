#!/usr/bin/env bash
set -e

server=silver
target_dir=/home/box

source_dir_repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../../" &>/dev/null && pwd)
source_dir_temp=$(mktemp -d -t umbilical-sync-silver-XXXXXXXXXX)
source_dir_wip="${source_dir_temp}/compose"

mkdir -p "${source_dir_wip}/umbilical/local"
rsync -a "${source_dir_repo}/umbilical/local/silver" "${source_dir_wip}/umbilical/local"

my-secret render "${source_dir_wip}"

ssh "${server}" mkdir -p "${target_dir}"
rsync -rvh --delete --progress "${source_dir_wip}" "${server}":"${target_dir}"

rm -rf "${source_dir_wip}"
