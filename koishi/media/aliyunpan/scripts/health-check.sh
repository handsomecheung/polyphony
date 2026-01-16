#!/usr/bin/env sh
set -e

DIR=$1

if [ ! -d "${DIR}" ]; then
    echo "dir ${DIR} not exits"
    exit 1
fi


if [ -z "$(ls -A ${DIR})" ]; then
    echo "dir ${DIR} is empty"
    exit 2
fi

exit 0