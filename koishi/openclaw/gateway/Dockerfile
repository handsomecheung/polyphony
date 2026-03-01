FROM node:22-bookworm

RUN npm install -g openclaw clawhub mcporter

ENV NODE_ENV=production
ENV PORT=18789

USER node

EXPOSE 18789

WORKDIR /home/node

CMD ["openclaw", "gateway", "--bind", "lan", "--port", "18789"]
