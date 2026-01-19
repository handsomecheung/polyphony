#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

job_name=vaultwardendatabackup

my-k8s-deploy --file=app.yaml
kubectl -n default-vaultwarden scale --replicas 0 deployment vaultwarden

while true; do
  succeeded=$(kubectl -n default-vaultwarden get jobs ${job_name} -o json | jq -r '.status.succeeded')
  if [ "${succeeded}" == "1" ]; then
    echo "Job ${job_name} finished"
    break
  fi
  echo "Waiting for job ${job_name} to finish..."
  sleep 5
done

kubectl -n default-vaultwarden logs jobs/${job_name}
echo
kubectl -n default-vaultwarden delete job ${job_name}

kubectl -n default-vaultwarden scale --replicas 1 deployment vaultwarden
