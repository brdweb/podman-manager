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
