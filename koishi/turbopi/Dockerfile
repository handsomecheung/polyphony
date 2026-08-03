FROM node:22-alpine

WORKDIR /app

# Copy package files first for caching
COPY package*.json ./

# Install dependencies (production only)
RUN npm ci --only=production

# Copy application source code
COPY . .

# Expose port (default for server.js is 3000)
EXPOSE 3000

ENV PORT=3000
ENV NODE_ENV=production

CMD ["npm", "start"]
