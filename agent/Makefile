VERSION ?= dev

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o homelab-agent ./cmd/agent

run: build
	./homelab-agent --config config.yaml

clean:
	rm -f homelab-agent

.PHONY: build run clean
