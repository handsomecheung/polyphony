#!/usr/bin/env bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

source run.env.sh

PUBLIC_IMAGE_NAME=${PUBLIC_IMAGE_NAME:-"${QOSMON_PUBLIC_IMAGE}"}
my-k8s-build-image "${PUBLIC_IMAGE_NAME}"
