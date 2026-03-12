PLUGIN_NAME := podman-manager
VERSION ?= $(shell date +%Y.%m.%d)
ARCH := x86_64
PKG := $(PLUGIN_NAME)-$(VERSION)-$(ARCH)-1
DIST := dist
SRC := src
GITHUB_REPO ?= brdweb/podman-manager

.PHONY: all clean package binary plg release

all: release

binary:
	$(MAKE) -C ../backend build-unraid
	mkdir -p $(SRC)/usr/local/bin
	cp ../backend/bin/podman-manager-linux-amd64 $(SRC)/usr/local/bin/podman-manager
	chmod 755 $(SRC)/usr/local/bin/podman-manager

package: binary
	mkdir -p $(DIST)
	@if makepkg --help 2>&1 | grep -q 'compatible for use with pacman'; then \
		echo "Using tar.xz fallback packaging (non-Slackware makepkg detected)"; \
		tar -C $(SRC) -cJf $(DIST)/$(PKG).txz .; \
	else \
		cd $(SRC) && makepkg -l y -c y ../$(DIST)/$(PKG).txz; \
	fi

plg: package
	VERSION=$(VERSION) GITHUB_REPO=$(GITHUB_REPO) TXZ_PATH=$(DIST)/$(PKG).txz OUTPUT_PATH=podman-manager.plg ./scripts/generate-plg.sh

release: plg

clean:
	rm -rf $(DIST)
	rm -f $(SRC)/usr/local/bin/podman-manager
	rm -f podman-manager.plg
