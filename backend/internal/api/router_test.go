package api

import (
	"path/filepath"
	"testing"

	"github.com/brdweb/podman-manager/internal/config"
)

func TestContainerDefaultStatePathsUseVarLib(t *testing.T) {
	cfg := &config.Config{}
	configPath := "/etc/podman-manager/config.yaml"

	if got, want := filepath.ToSlash(authDBPath(configPath, cfg)), "/var/lib/podman-manager/auth.db"; got != want {
		t.Fatalf("authDBPath() = %q, want %q", got, want)
	}
	if got, want := filepath.ToSlash(enrollCredentialsPath(configPath, cfg)), "/var/lib/podman-manager/agent-credentials.json"; got != want {
		t.Fatalf("enrollCredentialsPath() = %q, want %q", got, want)
	}
}
