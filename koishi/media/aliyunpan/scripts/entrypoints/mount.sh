#!/usr/bin/env bash
set -eo pipefail

TO_MOUNT=$1
MOUNTED=$2

echo "TO_MOUNT: ${TO_MOUNT}. MOUNTED: ${MOUNTED}"

trycount=5
while true; do
    if [ ! -d "${TO_MOUNT}" ] || [ -z "$(ls -A ${TO_MOUNT})" ]; then
        if [ "${4}" == "--allow-empty" ]; then
            echo "dir ${TO_MOUNT} is empty but maybe there is no files in the dir. skip ..."
            break
        else
            echo "dir ${TO_MOUNT} is not existing or empty. wait for rclone to mount it ..."
            sleep 5
        fi
    else
        echo "dir ${TO_MOUNT} is mounted ..."
        break
    fi

    trycount=$((trycount-1))
    if [ ${trycount} -le 0 ]; then
        echo "timeout. exit."
        exit 1
    fi
done

umount "${MOUNTED}" || true
# if mountpoint -q "${MOUNTED}"; then
#     echo "dir ${MOUNTED} is still be mounted. umount it ..."
#     umount "${MOUNTED}"
# fi

if [ ! -d "${MOUNTED}" ]; then
    echo "dir ${MOUNTED} is not existing. create ..."
    mkdir -p "${MOUNTED}" || true
fi

if [ ! -z "$(ls -A ${MOUNTED})" ]; then
    echo "dir ${MOUNTED} is not empty"

    cd ${MOUNTED}
    if [[ $(find . -type f | head -n 5 | wc -l) -gt 1 ]]; then
        echo "dir ${MOUNTED} contains files, may be mounted. exit"
        exit 2
    else
        echo "dir ${MOUNTED} not contains files. try delete empty directories ..."
        find . -empty -type d -delete
    fi
    cd -
fi

if [ ! -z "$(ls -A ${MOUNTED})" ]; then
    echo "dir ${MOUNTED} is not empty yet. exit."
    exit 3
fi

if [ "${3}" == "--reverse" ]; then
    encfs -c "${ENCFS6_CONFIG}" --extpass="echo ${ENCFS_PWD}" --reverse "${TO_MOUNT}" "${MOUNTED}"
else
    encfs -c "${ENCFS6_CONFIG}" --extpass="echo ${ENCFS_PWD}" -o allow_other,nonempty "${TO_MOUNT}" "${MOUNTED}"
fi
