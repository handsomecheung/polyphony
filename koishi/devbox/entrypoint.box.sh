#!/usr/bin/env bash
set -e

if [ -f /mnt/coder-workspaces/public-workspace/init.yaml ]; then
    echo "init public-workspace ..."
    ansible-playbook /mnt/coder-workspaces/public-workspace/init.yaml
fi

if [ -f /mnt/coder-workspaces/public-workspace/dev-config/init.sh ]; then
    echo "init dev-config ..."
    bash /mnt/coder-workspaces/public-workspace/dev-config/init.sh
fi

if [ -f /mnt/coder-workspaces/private-workspace/init.yaml ]; then
    echo "init private-workspace ..."
    ansible-playbook /mnt/coder-workspaces/private-workspace/init.yaml
fi

if [ -f /mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/init.sh ]; then
    echo "init polyphony koishi ..."
    bash /mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/init.sh
fi

if [ -f /mnt/coder-workspaces/private-workspace/workspace/init.sh ]; then
    echo "init workspace ..."
    bash /mnt/coder-workspaces/private-workspace/workspace/init.sh
fi

echo "set asdf global versions ..."
cat >/home/box/.tool-versions <<EOF
ruby 3.2.2
python 3.12.4
EOF

echo "install default.editor.nix ..."
NIXPKGS_ALLOW_UNFREE=1 /home/box/.nix-profile/bin/nix-env -if /home/box/default.editor.nix

echo "run code-server"
/tmp/code-server/bin/code-server --user-data-dir /home/box/.config/code-server --install-extension vscodevim.vim --install-extension kahole.magit || true
/tmp/code-server/bin/code-server --user-data-dir /home/box/.config/code-server /mnt/coder-workspaces/private-workspace/repos/local/home-service >/dev/null 2>&1 &
