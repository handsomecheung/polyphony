#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

my-k8s-deploy --file=k8s.cockpit.yaml
my-k8s-deploy --file=k8s.vm.yaml
my-k8s-deploy --file=k8s.router.yaml
my-k8s-deploy --file=k8s.stable-diffusion.yaml
my-k8s-deploy --file=k8s.shutdown.yaml
my-k8s-deploy --file=k8s.wakeup.yaml
my-k8s-deploy --file=k8s.pikvm.yaml

bash yx/deploy.sh
