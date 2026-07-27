FROM node:22-bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends chromium fontconfig fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY src ./src

USER node
CMD ["node", "src/index.js"]
