#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

kubectl run test-ipv4 --image=busybox --rm -it -- wget -O- -q https://ifconfig.me
kubectl run test-ipv6 --image=busybox --rm -it -- wget -O- -q -6 https://ifconfig.me

kubectl describe node nur | grep InternalIP

kubectl get ippools.crd.projectcalico.org

kubectl run test-ipv6-a --image=busybox -- sleep 3600
ipv6_b=$(kubectl get pod test-ipv6-b -o jsonpath='{.status.podIPs[1].ip}')

kubectl exec -it test-ipv6-a -- ping6 -c 3 ${ipv6_b}

kubectl describe pod test-ipv6-a
