# syntax=docker/dockerfile:1.7

ARG NODE_IMAGE=node:22-alpine
ARG NGINX_IMAGE=nginx:1.27-alpine

FROM ${NODE_IMAGE} AS builder
WORKDIR /src
ARG VITE_BASE_PATH=/lottery/
ENV VITE_BASE_PATH=${VITE_BASE_PATH}
COPY package.json package-lock.json ./
RUN npm ci
COPY admin-web ./admin-web
COPY user-web ./user-web
COPY shared ./shared
COPY tsconfig.json vite.config.ts ./
RUN npm run build

FROM ${NGINX_IMAGE}
ENV PORTAL_API_UPSTREAM=http://portal-api:18081
ENV NGINX_ENVSUBST_FILTER=PORTAL_API_UPSTREAM
COPY deploy/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /src/dist/ /usr/share/nginx/html/
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -T 3 -O /dev/null http://127.0.0.1:8080/health || exit 1
