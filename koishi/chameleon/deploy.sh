#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.namespace.yaml
my-k8s-deploy --file=k8s.volumes.yaml

./squid/deploy.sh
./gollum/deploy.sh
./deactivate/deploy.sh
./pikvm-controller/deploy.sh

./glinux/deploy.sh

./gotty/deploy.sh
./mbtty/deploy.sh

./mbvnc/deploy.sh
./novnc/deploy.sh

./homego/deploy.sh
