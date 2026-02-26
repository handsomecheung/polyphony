FROM node:22-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    unzip \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://bun.sh/install | bash
ENV PATH="/root/.bun/bin:${PATH}"

RUN corepack enable

WORKDIR /app

RUN git clone https://github.com/openclaw/openclaw.git .

RUN pnpm install --frozen-lockfile

RUN pnpm build
ENV OPENCLAW_PREFER_PNPM=1
RUN pnpm ui:build


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
    && rm -rf /var/lib/apt/lists/*

# Install uv (Fast Python package manager)
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/bin/

RUN npm install -g clawhub && \
    npm install -g @google/gemini-cli && \
    npm install -g mcporter

WORKDIR /app

COPY --from=builder /app /app

ENV NODE_ENV=production
ENV PORT=18789

USER node

EXPOSE 18789

CMD ["node", "openclaw.mjs", "gateway", "--bind", "lan", "--port", "18789"]
