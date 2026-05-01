VERSION ?= dev

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o podman-agent ./cmd/agent

run: build
	./podman-agent --config config.yaml

clean:
	rm -f podman-agent

.PHONY: build run clean
