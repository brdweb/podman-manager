FROM golang:1.26.2-alpine AS build
WORKDIR /src
COPY agent/go.mod agent/go.sum ./
RUN go mod download
COPY agent/ ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/podman-agent ./cmd/agent

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /out/podman-agent /usr/local/bin/podman-agent
RUN chmod +x /usr/local/bin/podman-agent \
    && addgroup -S podman-agent \
    && adduser -S podman-agent -G podman-agent
USER podman-agent
ENTRYPOINT ["/usr/local/bin/podman-agent"]
