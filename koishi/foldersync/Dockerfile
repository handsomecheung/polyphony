FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y tzdata locales ssh && \
    locale-gen en_US.UTF-8

RUN echo 'PasswordAuthentication no' >> /etc/ssh/sshd_config && \
    echo 'PermitEmptyPasswords no' >> /etc/ssh/sshd_config && \
    sed -i 's|HashKnownHosts yes|HashKnownHosts no|g' /etc/ssh/ssh_config

ADD configs/sshd_config /etc/ssh/sshd_config
RUN chown -R root:root /etc/ssh/* && \
    chmod 600 /etc/ssh/*

ENV USER_HH="photo-sync-__{{infra.common-users:f:hh}}__"
ENV USER_CC="photo-sync-__{{infra.common-users:f:cc}}__"

RUN useradd -m -s /bin/bash "${USER_HH}" && \
    passwd -d "${USER_HH}" && \
    useradd -m -s /bin/bash "${USER_CC}" && \
    passwd -d "${USER_CC}" && \
    useradd -m -s /bin/bash photo-sync-001 && \
    passwd -d photo-sync-001

ADD entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]

WORKDIR /root
