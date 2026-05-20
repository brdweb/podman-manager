package api

import (
	"path/filepath"
	"testing"

	"github.com/brdweb/homelab-control/internal/config"
)

func TestContainerDefaultStatePathsUseVarLib(t *testing.T) {
	cfg := &config.Config{}
	configPath := "/etc/homelab-control/config.yaml"

	if got, want := filepath.ToSlash(authDBPath(configPath, cfg)), "/var/lib/homelab-control/auth.db"; got != want {
		t.Fatalf("authDBPath() = %q, want %q", got, want)
	}
	if got, want := filepath.ToSlash(enrollCredentialsPath(configPath, cfg)), "/var/lib/homelab-control/agent-credentials.json"; got != want {
		t.Fatalf("enrollCredentialsPath() = %q, want %q", got, want)
	}
}
