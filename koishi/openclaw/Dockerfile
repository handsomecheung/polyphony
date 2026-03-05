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
    tmux && \
    rm -rf /var/lib/apt/lists/*

RUN apt update && \
    apt install -y fonts-liberation fonts-noto-cjk libatk-bridge2.0-0 libatk1.0-0 libatspi2.0-0 libcups2 libgtk-3-0 libgtk-4-1 libxcomposite1 libxdamage1 xdg-utils && \
    wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && \
    dpkg -i google-chrome-stable_current_amd64.deb && \
    rm google-chrome-stable_current_amd64.deb && \
    rm -rf /var/lib/apt/lists/*

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/bin/

RUN npm install -g openclaw clawhub mcporter @google/gemini-cli

ENV NODE_ENV=production
ENV PORT=18789

USER node

EXPOSE 18789

WORKDIR /home/node

CMD ["openclaw", "gateway", "--bind", "lan", "--port", "18789"]
