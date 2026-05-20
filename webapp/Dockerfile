FROM golang:1.26.2-alpine AS backend-build
ARG VERSION=dev
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/homelab-control ./cmd/homelab-control

FROM node:22-alpine AS frontend-build
WORKDIR /src/webapp
COPY webapp/package.json webapp/package-lock.json ./
RUN npm ci --legacy-peer-deps
COPY webapp/ ./
RUN npm run build

FROM alpine:3.21
RUN apk add --no-cache nginx openssh-client
COPY webapp/nginx.conf /etc/nginx/http.d/default.conf
COPY webapp/start-container.sh /usr/local/bin/start-homelab-control
COPY --from=frontend-build /src/webapp/dist /usr/share/nginx/html
COPY --from=backend-build /out/homelab-control /usr/local/bin/homelab-control
RUN chmod +x /usr/local/bin/start-homelab-control \
    && mkdir -p /etc/homelab-control /var/lib/homelab-control /run/nginx /var/lib/nginx/tmp /tmp/nginx/client-body
ENV HOMELAB_CONTROL_CONFIG=/etc/homelab-control/config.yaml
EXPOSE 80 18735
ENTRYPOINT ["/usr/local/bin/start-homelab-control"]
