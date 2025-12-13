FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV MY_TZ="__{{koishi.deploy.timezone}}__"

COPY --from=docker:dind /usr/local/bin/docker /usr/local/bin/
RUN apt-get update && \
	ln -fs /usr/share/zoneinfo/${MY_TZ} /etc/localtime && \
	apt-get install -y tzdata locales && \
	dpkg-reconfigure --frontend noninteractive tzdata && \
	locale-gen en_US.UTF-8 && \
	apt-get install -y sudo curl wget git net-tools nmap zsh rlwrap htop \
		docker-compose vim ranger tmux jq fd-find dnsutils \
		python3 python-is-python3 pip && \
	rm -rf /var/lib/apt/lists/*


ARG USER=box

RUN useradd --groups sudo --no-create-home --shell /bin/bash ${USER} && \
	echo "${USER} ALL=(ALL) NOPASSWD:ALL" >/etc/sudoers.d/${USER} && \
	chmod 0440 /etc/sudoers.d/${USER}

USER ${USER}
WORKDIR /home/${USER}

ENV TERM=xterm-256color
ENV SHELL=/usr/bin/zsh
ENV LC_CTYPE=zh_CN.UTF-8

USER root

RUN chown -R ${USER}:${USER} /home/${USER} && \
	chmod -R 700 /home/${USER}
