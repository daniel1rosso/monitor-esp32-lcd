# syntax=docker/dockerfile:1.7

FROM node:20.19.0-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci && npm audit --omit=dev --audit-level=high
COPY . .
RUN npm run build

FROM nginx:1.28.0-alpine AS runtime
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/ || exit 1
