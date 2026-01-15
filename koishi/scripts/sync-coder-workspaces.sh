#!/usr/bin/env bash
set -eo pipefail

fromnode=nur

if [[ "$(hostname)" != "${fromnode}" ]]; then
	echo "This script should be run on ${fromnode}."
	exit 1
fi

dir=/mnt/coder-workspaces

allnodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')
nodes=${allnodes//${fromnode}/}

for node in ${nodes}; do
	echo
	echo "Syncing diretory ${dir} all to ${node} ..."
	echo
	rsync --size-only -rvh \
		--mkpath \
		--links \
		--progress \
		--delete \
		--filter=':- .gitignore' \
		--exclude='node_modules' \
		--exclude='lost+found' \
		--exclude='*.sock' \
		--exclude='public-workspace/thirdpart-repos/' \
		--exclude='public-workspace/coder/' \
		--exclude='public-workspace/cache/' \
		--exclude='public-workspace/cache-global' \
		--exclude='public-workspace/editor/vscode-server/' \
		--exclude='public-workspace/editor/emacs.d/' \
		--exclude='public-workspace/editor/cursor-server/' \
		--exclude='public-workspace/editor/neovim/state/' \
		--exclude='public-workspace/system/bins/bin/android-sdk/' \
		--exclude='public-workspace/system/bins/bin/flutter/' \
		--exclude='public-workspace/system/bins/go/' \
		--exclude='private-workspace/archived-repos/' \
		--exclude='private-workspace/temp/' \
		--exclude='private-workspace/workspace/encrypted-configs/.gnupg/' \
		${dir}/* "${node}:${dir}"
	echo
	echo
done
