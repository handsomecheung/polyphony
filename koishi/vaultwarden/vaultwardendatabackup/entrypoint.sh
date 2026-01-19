#!/bin/bash
set -e

data_dir=/data/vaultwarden
local_dir=/data/local

pass_dir="$(mktemp -d -t datadirbackup-pass-XXXXX)"
temp_dir="$(mktemp -d -t datadirbackup-temp-XXXXX)"

zip_file="${temp_dir}/datadir.$(date +"%Y%m%d-%H%M%S").zip"
zip -r "${zip_file}" "${data_dir}"

curl "${URL_PASS}" >"${pass_dir}/thepassbase"
passphrase=$(sha256sum "${pass_dir}/thepassbase" | awk '{print $1}')

gpg_file="${zip_file}.gpg"
gpg --batch --passphrase "${passphrase}" -c "${zip_file}"

mv "${gpg_file}" "${local_dir}"
chmod go-rwx "${local_dir}/$(basename "${gpg_file}")"

rm -rf "${pass_dir}" "${temp_dir}"
