FROM cloudpublic/base/devbase:2404

USER root

RUN apt-get update && \
        apt-get install -y apache2-utils rsync openssh-server plocate libpam-google-authenticator \
        autoconf patch build-essential cmake libssl-dev libyaml-dev libreadline6-dev zlib1g-dev libgmp-dev libncurses5-dev libffi-dev libgdbm6 libgdbm-dev libdb-dev uuid-dev libbz2-dev libsqlite3-dev liblzma-dev && \
        rm -rf /var/lib/apt/lists/*

# install docker by APT because it is need to run docker with sudo
RUN apt-get update && \
    apt-get install -y ca-certificates curl && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && \
    chmod a+r /etc/apt/keyrings/docker.asc && \
    echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin && \
    rm -rf /var/lib/apt/lists/*

# It's to  hard to handle the dependency of ruby in nix, so install it by apt
# RUN gem install ruby-lsp rubocop rufo psych solargraph bundler

# RUN curl -LO https://github.com/neovim/neovim/releases/download/v0.10.4/nvim-linux-x86_64.tar.gz && \
#         rm -rf /opt/nvim && \
#         tar -C /opt -xzf nvim-linux-x86_64.tar.gz && \
#         mv /opt/nvim-linux-x86_64 /opt/nvim && \
#         rm nvim-linux-x86_64.tar.gz

RUN mkdir -p /run/sshd && \
        echo 'PasswordAuthentication no' >> /etc/ssh/sshd_config && \
        echo 'PermitEmptyPasswords no' >> /etc/ssh/sshd_config && \
        echo 'PubkeyAcceptedKeyTypes +ssh-rsa' >> /etc/ssh/sshd_config && \
        sed -i 's|HashKnownHosts yes|HashKnownHosts no|g' /etc/ssh/ssh_config

# Google Authenticator for password login
RUN sed -i '/@include common-auth/a auth required pam_google_authenticator.so nullok\nauth required pam_permit.so' /etc/pam.d/sshd && \
        sed -i '/KbdInteractiveAuthentication no/d' /etc/ssh/sshd_config && \
        echo 'ChallengeResponseAuthentication yes' >> /etc/ssh/sshd_config && \
        echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config && \
        echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && \
        echo 'KbdInteractiveAuthentication yes' >> /etc/ssh/sshd_config


USER box

RUN curl -fsSL https://code-server.dev/install.sh | sh -s -- --method=standalone --prefix=/tmp/code-server --version 4.96.4

COPY --chown=box default.editor.nix /home/box/default.editor.nix
RUN NIXPKGS_ALLOW_UNFREE=1 /home/box/.nix-profile/bin/nix-env -if /home/box/default.editor.nix

COPY --chown=box default.nix /home/box/default.nix
RUN NIXPKGS_ALLOW_UNFREE=1 /home/box/.nix-profile/bin/nix-shell /home/box/default.nix --run "echo done"

USER root
