package config

import (
	"strings"
	"testing"
)

func TestLoadBytesAcceptsDocumentedStrictHostKeyChecking(t *testing.T) {
	cfg, err := LoadBytes([]byte(testConfigYAML("strict_host_key_checking")))
	if err != nil {
		t.Fatalf("LoadBytes returned error: %v", err)
	}

	if cfg.SSH.StrictHostKeyChecking != "strict" {
		t.Fatalf("StrictHostKeyChecking = %q, want %q", cfg.SSH.StrictHostKeyChecking, "strict")
	}
}

func TestLoadBytesAcceptsLegacyStrictHostKeyChecking(t *testing.T) {
	cfg, err := LoadBytes([]byte(testConfigYAML("ssh_strict_host_key_checking")))
	if err != nil {
		t.Fatalf("LoadBytes returned error: %v", err)
	}

	if cfg.SSH.StrictHostKeyChecking != "strict" {
		t.Fatalf("StrictHostKeyChecking = %q, want %q", cfg.SSH.StrictHostKeyChecking, "strict")
	}
	if cfg.SSH.LegacyStrictHostKeyChecking != "" {
		t.Fatalf("LegacyStrictHostKeyChecking = %q, want empty", cfg.SSH.LegacyStrictHostKeyChecking)
	}
}

func TestMarshalEmitsDocumentedStrictHostKeyChecking(t *testing.T) {
	cfg, err := LoadBytes([]byte(testConfigYAML("ssh_strict_host_key_checking")))
	if err != nil {
		t.Fatalf("LoadBytes returned error: %v", err)
	}

	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	encoded := string(data)
	if !strings.Contains(encoded, "strict_host_key_checking: strict") {
		t.Fatalf("marshaled config missing documented key:\n%s", encoded)
	}
	if strings.Contains(encoded, "ssh_strict_host_key_checking") {
		t.Fatalf("marshaled config contains legacy key:\n%s", encoded)
	}
}

func TestLoadBytesNormalizesAllowedOrigins(t *testing.T) {
	cfg, err := LoadBytes([]byte(strings.Replace(testConfigYAML("strict_host_key_checking"), `server:
  port: 18734
  bind: "127.0.0.1"`, `server:
  port: 18734
  bind: "127.0.0.1"
  allowed_origins:
    - "https://homelab.example.com/"
    - "http://localhost:80"`, 1)))
	if err != nil {
		t.Fatalf("LoadBytes returned error: %v", err)
	}

	want := []string{"https://homelab.example.com", "http://localhost"}
	if len(cfg.Server.AllowedOrigins) != len(want) {
		t.Fatalf("AllowedOrigins length = %d, want %d", len(cfg.Server.AllowedOrigins), len(want))
	}
	for i := range want {
		if cfg.Server.AllowedOrigins[i] != want[i] {
			t.Fatalf("AllowedOrigins[%d] = %q, want %q", i, cfg.Server.AllowedOrigins[i], want[i])
		}
	}
}

func TestLoadBytesRejectsAllowedOriginWithPath(t *testing.T) {
	_, err := LoadBytes([]byte(strings.Replace(testConfigYAML("strict_host_key_checking"), `server:
  port: 18734
  bind: "127.0.0.1"`, `server:
  port: 18734
  bind: "127.0.0.1"
  allowed_origins:
    - "https://homelab.example.com/app"`, 1)))
	if err == nil {
		t.Fatal("LoadBytes returned nil error, want invalid allowed origin")
	}
	if !strings.Contains(err.Error(), "server.allowed_origins[0]") {
		t.Fatalf("error = %q, want allowed origin path context", err)
	}
}

func TestLoadBytesDefaultsDockerComposeRepoPath(t *testing.T) {
	cfg, err := LoadBytes([]byte(testConfigYAML("strict_host_key_checking")))
	if err != nil {
		t.Fatalf("LoadBytes returned error: %v", err)
	}
	if cfg.Docker.ComposeRepoPath != "../homelab-docker" {
		t.Fatalf("Docker.ComposeRepoPath = %q, want ../homelab-docker", cfg.Docker.ComposeRepoPath)
	}
	if cfg.Docker.DockMonURL != "https://dockmon.brdweb.com" {
		t.Fatalf("Docker.DockMonURL = %q, want https://dockmon.brdweb.com", cfg.Docker.DockMonURL)
	}
}

func TestLoadBytesTrimsDockerSettings(t *testing.T) {
	cfg, err := LoadBytes([]byte(testConfigYAML("strict_host_key_checking") + `
docker:
  compose_repo_path: "  /srv/homelab-docker  "
  dockmon_url: " https://dockmon.example.com/ "
`))
	if err != nil {
		t.Fatalf("LoadBytes returned error: %v", err)
	}
	if cfg.Docker.ComposeRepoPath != "/srv/homelab-docker" {
		t.Fatalf("Docker.ComposeRepoPath = %q, want /srv/homelab-docker", cfg.Docker.ComposeRepoPath)
	}
	if cfg.Docker.DockMonURL != "https://dockmon.example.com" {
		t.Fatalf("Docker.DockMonURL = %q, want https://dockmon.example.com", cfg.Docker.DockMonURL)
	}
}

func testConfigYAML(strictHostKeyField string) string {
	return `server:
  port: 18734
  bind: "127.0.0.1"
ssh:
  key_path: "~/.ssh/id_ed25519"
  connect_timeout: "5s"
  keepalive_interval: "30s"
  ` + strictHostKeyField + `: "strict"
hosts:
  - name: "host-alpha"
    address: "10.0.0.101"
    port: 22
    user: "your-user"
    mode: "rootful"
`
}
