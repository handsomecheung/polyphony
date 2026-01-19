#!/bin/bash
set -e

if [ "$(hostname)" != "nur" ]; then
    echo "This script can only be run on the 'nur' host." >&2
    exit 1
fi

cd "$(dirname "${BASH_SOURCE[0]}")"

sudo virsh nwfilter-define kvm-deny-lan-subnets.xml

# eidt nwfilter on the fly
# sudo virsh nwfilter-edit deny-lan-subnets
