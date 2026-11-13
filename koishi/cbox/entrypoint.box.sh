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

echo "set asdf global versions ..."
cat >/home/box/.tool-versions <<EOF
ruby 3.2.2
python 3.12.4
EOF

if [ -f /home/box/.nix-profile/etc/profile.d/nix.sh ]; then
    echo "load Nix profile ..."
    . /home/box/.nix-profile/etc/profile.d/nix.sh
fi

echo "start arondo-runner ..."
zsh -ic 'exec bash /mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/cbox/run.arondo-runner.sh'
