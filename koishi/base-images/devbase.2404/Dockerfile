FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
ENV MY_TZ="__{{koishi.deploy.timezone}}__"

RUN apt-get update && \
	ln -fs /usr/share/zoneinfo/${MY_TZ} /etc/localtime && \
        apt-get install -y tzdata locales software-properties-common && \
        dpkg-reconfigure --frontend noninteractive tzdata && \
        locale-gen en_US.UTF-8 && \
        add-apt-repository -y --update ppa:ansible/ansible && \
        apt-get install -y sudo curl wget ncat xz-utils net-tools dnsutils iproute2 time \
        zsh vim git git-crypt htop jq ansible ranger && \
        rm -rf /var/lib/apt/lists/*

RUN usermod -l box ubuntu && \
        groupmod -n box ubuntu && \
        usermod -d /home/box -m box

RUN sed -i '/^root/a box     ALL=(ALL:ALL) ALL' /etc/sudoers && \
        sed -i 's/^%sudo.*$/%sudo ALL=(ALL:ALL) ALL/g' /etc/sudoers && \
        chsh --shell /bin/zsh box

RUN mkdir -m 0755 /nix && chown box /nix


USER box

WORKDIR /home/box

RUN curl -L https://nixos.org/nix/install | sh && \
        echo ". /home/box/.nix-profile/etc/profile.d/nix.sh" >> /home/box/.bashrc

COPY --chown=box default.base.nix /home/box/default.base.nix
RUN NIXPKGS_ALLOW_UNFREE=1 /home/box/.nix-profile/bin/nix-env -if /home/box/default.base.nix
