FROM golang:1.26.2-alpine AS backend-build
ARG VERSION=dev
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/podman-manager ./cmd/podman-manager

FROM node:22-alpine AS frontend-build
WORKDIR /src/webapp
COPY webapp/package.json webapp/package-lock.json ./
RUN npm ci --legacy-peer-deps
COPY webapp/ ./
RUN npm run build

FROM alpine:3.21
RUN apk add --no-cache nginx openssh-client
COPY webapp/nginx.conf /etc/nginx/http.d/default.conf
COPY webapp/start-container.sh /usr/local/bin/start-podman-manager
COPY --from=frontend-build /src/webapp/dist /usr/share/nginx/html
COPY --from=backend-build /out/podman-manager /usr/local/bin/podman-manager
RUN chmod +x /usr/local/bin/start-podman-manager \
    && mkdir -p /etc/podman-manager /run/nginx /var/lib/nginx/tmp /tmp/nginx/client-body
ENV PODMAN_MANAGER_CONFIG=/etc/podman-manager/config.yaml
EXPOSE 80
ENTRYPOINT ["/usr/local/bin/start-podman-manager"]
