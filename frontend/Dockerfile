FROM node:22-alpine AS build

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

FROM nginx:1.29-alpine
COPY --from=build /src/dist /usr/share/nginx/html
COPY infra/nginx/default.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
