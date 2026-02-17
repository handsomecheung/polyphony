#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

# Use yq to inject file contents into the homepage ConfigMap in k8s.yaml
# process each document individually to preserve the --- separator and avoid array syntax
k8s_tmp_file=$(mktemp)
trap "rm -f ${k8s_tmp_file}" EXIT

yq -y --rawfile settings "settings.yaml" \
    --rawfile services "services.yaml" \
    'if (.kind == "ConfigMap" and .metadata.name == "homepage") then
         .data["settings.yaml"] = $settings | .data["services.yaml"] = $services 
       else . end' \
    k8s.app.yaml >"$k8s_tmp_file"

my-k8s-deploy --file="$k8s_tmp_file"
