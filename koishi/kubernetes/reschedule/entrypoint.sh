#!/usr/bin/env bash
set -e

declare -A node_status_map
nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
for node in $nodes; do
    status=$(kubectl get nodes $node -o jsonpath="{.status.conditions[?(@.type=='Ready')].status}")
    node_status_map[$node]=$status
done

for node in "${!node_status_map[@]}"; do
    echo "Node: $node, Status: ${node_status_map[$node]}"
done

apps=$(kubectl get deployments -A -o json | jq -r '.items[] | select((.spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution | length) > 0) | .metadata.namespace + "," + .metadata.name')
reschedule_apps=()
for app in $apps; do
    namespace=$(echo $app | cut -d, -f1)
    deployment=$(echo $app | cut -d, -f2)

    exns=$(kubectl -n $namespace get deployments $deployment -o json | jq -r '.spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution | .[] | .preference.matchExpressions | .[] | .values | join(" ")')
    expected_nodes=" ${exns} "

    pods=$(kubectl -n $namespace get pods -l app=$deployment -o jsonpath="{.items[*].metadata.name}")
    for pod in $pods; do
        node=$(kubectl -n $namespace get pod $pod -o jsonpath="{.spec.nodeName}")
        if [[ ${expected_nodes} != *" $node "* ]]; then
            echo "app ${app} is not on a preferred node expected nodes: _${expected_nodes}_, actual node: ${node}"

            for expected_node in ${exns}; do
                if [ "${node_status_map[$expected_node]}" == "True" ]; then
                    echo "Node ${expected_node} is ready, try rescheduling app ${app}"
                    reschedule_apps+=($app)
                    break
                else
                    echo "Node ${expected_node} is not ready, skipping rescheduling"
                fi
            done
        fi
    done
done

echo "Rescheduling apps: ${reschedule_apps[@]}"
for app in "${reschedule_apps[@]}"; do
    namespace=$(echo $app | cut -d, -f1)
    deployment=$(echo $app | cut -d, -f2)
    echo "Rescheduling app ${app}"
    kubectl -n $namespace rollout restart deployment $deployment
done