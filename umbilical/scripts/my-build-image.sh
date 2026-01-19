#!/bin/bash
set -e

NAME=""
PLATFORM="linux/amd64,linux/arm64/v8"
NO_CACHE="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
    --name)
        NAME="$2"
        shift 2
        ;;
    --platform)
        PLATFORM="$2"
        shift 2
        ;;
    --no-cache)
        NO_CACHE="true"
        shift
        ;;
    *)
        echo "Unknown parameter: $1"
        exit 1
        ;;
    esac
done

if [ -z "$NAME" ]; then
    echo "Error: --name is required"
    exit 1
fi

if [[ "${NAME}" == "cloudpublic/"* ]]; then
    host_cloudpublic=$(my-secret getpassword --key=umbilical.deploy.cloudpublic_registry_host 2>/dev/null)

    IMAGE="${NAME/cloudpublic/${host_cloudpublic}}"
else
    echo "Error: --name must start with cloudpublic/"
    exit 1
fi

BUILDER="${NAME##*/}"
BUILDER="umbilical-${BUILDER/:/-}"
DOCKER_COMMAND="sudo DOCKER_CONFIG=/home/box/.docker docker"

echo "Image: ${IMAGE}"
echo "Builder: ${BUILDER}"
echo "Platform: ${PLATFORM}"
echo "No-cache: ${NO_CACHE}"

build_root_dir=/mnt/coder-sharepoint/build-image
build_work_dir=$(mktemp --tmpdir="${build_root_dir}" -d "${BUILDER}-$(date +%Y-%m-%d_%H%M).XXXXXX")
mkdir -p "${build_work_dir}"

echo "build in direcotry ${build_work_dir}"
# rsync is more powerful and flexible than cp and it can handle hidden files without any issues.
rsync -a --exclude='node_modules' . "${build_work_dir}"
cd "${build_work_dir}"

echo "Processing Dockerfile FROM instructions..."
first_line=$(head -n 1 Dockerfile)
if echo "${first_line}" | grep -qE '^FROM[[:space:]]+cloudpublic/'; then
    sed -i "1 s|FROM[[:space:]]\+cloudpublic/|FROM ${host_cloudpublic}/|" Dockerfile
    echo "Replaced cloudpublic/ in Dockerfile FROM instruction"
fi

if [ "${NO_CACHE}" = "true" ]; then
    ${DOCKER_COMMAND} buildx rm "${BUILDER}" || true
fi

${DOCKER_COMMAND} buildx create --name "${BUILDER}" || true
${DOCKER_COMMAND} buildx use "${BUILDER}"
${DOCKER_COMMAND} buildx build --platform "${PLATFORM}" -t "${IMAGE}" --push .

rm -rf "${build_work_dir}"
