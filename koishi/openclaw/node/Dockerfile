FROM node:22-bookworm

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ffmpeg \
    gh \
    curl \
    wget \
    build-essential \
    python3 \
    python3-dev \
    poppler-utils \
    imagemagick \
    jq ripgrep \
    tmux \
    && rm -rf /var/lib/apt/lists/*

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/bin/

RUN npm install -g openclaw clawhub mcporter @google/gemini-cli

WORKDIR /home/node
ENV NODE_ENV=production

USER node
