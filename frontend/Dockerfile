ARG BASE_IMAGE_REGISTRY=
FROM ${BASE_IMAGE_REGISTRY}node:22-alpine@sha256:0a7108bf6c7bf5de370ffb1a3ed6be93d405b43ff159f681a8d18c0e2bc2e402 AS build

WORKDIR /src
ARG VITE_API_BASE_URL=/api/v1
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN npm config set registry https://registry.npmmirror.com \
    && npm install -g pnpm@10.27.0 \
    && pnpm config set registry https://registry.npmmirror.com \
    && pnpm install --frozen-lockfile
COPY frontend ./
RUN pnpm build

FROM ${BASE_IMAGE_REGISTRY}nginx:1.29-alpine@sha256:5616878291a2eed594aee8db4dade5878cf7edcb475e59193904b198d9b830de
COPY --from=build /src/dist /usr/share/nginx/html
COPY infra/nginx/default.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
