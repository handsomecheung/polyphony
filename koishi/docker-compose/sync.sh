#!/usr/bin/env bash
set -e

source_dir_repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source_dir_temp=$(mktemp -d -t koishi-docker-compose-XXXXXXXXXX)
source_dir_wip="${source_dir_temp}"

rsync -a "${source_dir_repo}" "${source_dir_wip}"

my-secret render "${source_dir_wip}"

servers="nur nippon miniba"
target_dir="${source_dir_repo}/current/"

echo "wip directory: ${source_dir_wip}"

for server in ${servers}; do
    echo "copying to ${server}"
    rsync -rvh --delete --progress "${source_dir_wip}/docker-compose/${server}/" "${server}":"${target_dir}"
done

rm -rf "${source_dir_wip}"
