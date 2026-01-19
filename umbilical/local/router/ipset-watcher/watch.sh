#!/usr/bin/env bash
set -eo pipefail

declare -A sets
sets[priority]="./gen_ips.priority.sh > /dev/null 2>&1 && cat /code/ips.priority.txt"

echo "[$(date +"%Y-%m-%d %H:%M:%S")] watch run ..."

for name in "${!sets[@]}"; do
    cmd=${sets[${name}]}
    echo "watch set ${name}"

    if ! ipset list -n | grep "^${name}$"; then
        echo "set ${name} not exists, create it ..."
        ipset create "${name}" hash:net
    fi

    fin=$(mktemp "/tmp/ipset-watcher.set.${name}.in.XXXXXX")
    fout=$(mktemp "/tmp/ipset-watcher.set.${name}.out.XXXXXX")
    echo "file in: ${fin}"
    echo "file out: ${fout}"

    (ipset list "${name}" | grep -E -v '^[a-zA-Z]' | sort || true) >"${fin}"
    bash -c "${cmd}" 2>/dev/null | sort | uniq >"${fout}"

    if ! cmp -s "${fin}" "${fout}"; then
        echo "set ${name} entries out of date, update it ..."
        ipset flush "${name}"

        for entry in $(cat "${fout}"); do
            ipset add "${name}" "${entry}"
        done
    fi

    rm -f "${fin}"
    rm -f "${fout}"
done
echo "[$(date +"%Y-%m-%d %H:%M:%S")] watch done"
