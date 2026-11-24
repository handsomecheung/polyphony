#!/usr/bin/env bash
set -e

if [ -f /mnt/coder-workspaces/public-workspace/init.sh ]; then
    echo "init public-workspace ..."
    bash /mnt/coder-workspaces/public-workspace/init.sh
fi

if [ -f /mnt/coder-workspaces/private-workspace/repos/local/polyphony/devconfigs/init.sh ]; then
    echo "init devconfigs ..."
    bash /mnt/coder-workspaces/private-workspace/repos/local/polyphony/devconfigs/init.sh
fi

if [ -f /mnt/coder-workspaces/private-workspace/init.sh ]; then
    echo "init private-workspace ..."
    bash /mnt/coder-workspaces/private-workspace/init.sh
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

if [[ "${ENABLE_CODER}" != "0" ]]; then
    echo "run code-server"
    /tmp/code-server/bin/code-server --user-data-dir /home/box/.config/code-server --install-extension vscodevim.vim --install-extension kahole.magit || true
    /tmp/code-server/bin/code-server --user-data-dir /home/box/.config/code-server /mnt/coder-workspaces/private-workspace/repos/local/polyphony >/dev/null 2>&1 &
fi

if [ -f /home/box/.nix-profile/etc/profile.d/nix.sh ]; then
    echo "load Nix profile ..."
    . /home/box/.nix-profile/etc/profile.d/nix.sh
fi

echo "start arondo-runner ..."
zsh -ic 'exec nix-shell /home/box/default.nix --run "exec bash /mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/devbox/run.arondo-runner.sh"'
