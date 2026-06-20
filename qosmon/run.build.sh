#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

source run.env.sh

sudo docker build -t "${QOSMON_IMAGE}" .
