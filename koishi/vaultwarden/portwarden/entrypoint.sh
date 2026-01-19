#!/bin/bash
set -e

bw config server "${URL_VW}"
source /.env
bw login --apikey
session=$(bw unlock --passwordenv BW_PASSWORD --raw)
export BW_SESSION=${session}

pack_dir="$(mktemp -d -t portwarden-pack-XXXXX)"
filename="backup.$(date +"%Y%m%d-%H%M%S").portwarden"
pack_passphrase=$(tr -dc _A-Z-a-z-0-9 </dev/urandom | head -c 12)
portwarden --passphrase "${pack_passphrase}" --filename "${pack_dir}/${filename}" encrypt
portwarden --passphrase "${pack_passphrase}" --filename "${pack_dir}/${filename}" decrypt

text_dir="$(mktemp -d -t portwarden--text-XXXXX)"
curl "${URL_PASS}" >"${text_dir}/thepassbase"
passphrase=$(sha256sum "${text_dir}/thepassbase" | awk '{print $1}')
gpg --batch --passphrase "${passphrase}" -c "${pack_dir}/${filename}.decrypted.zip"
mv "${pack_dir}/${filename}.decrypted.zip.gpg" /data/export/
chmod go-rwx "/data/export/${filename}.decrypted.zip.gpg"

rm -rf "${pack_dir}" "${text_dir}"
