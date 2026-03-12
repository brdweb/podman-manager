BINARY_NAME := podman-manager
BUILD_DIR := bin
CMD_DIR := cmd/podman-manager
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build build-unraid clean test lint

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

build-unraid:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)

clean:
	rm -rf $(BUILD_DIR)

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...
