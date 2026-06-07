package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/brdweb/homelab-control/internal/auth"
	"github.com/brdweb/homelab-control/internal/config"
	"github.com/brdweb/homelab-control/internal/host"
)

func TestHandleUpdateConfigRefreshesDockerOriginAndAuth(t *testing.T) {
	root := t.TempDir()
	keyPath := writeTestSSHKey(t, root)
	oldRepo := filepath.Join(root, "old-homelab-docker")
	newRepo := filepath.Join(root, "new-homelab-docker")
	writeAPIComposeFile(t, oldRepo, "yoda-docker", "core", `services:
  traefik:
    image: docker.io/library/traefik:v3.2
`)
	writeAPIComposeFile(t, newRepo, "xwing-docker", "dockmon", `name: dockmon
services:
  dockmon:
    image: darthnorse/dockmon:latest
`)

	configPath := filepath.Join(root, "config.yaml")
	initialYAML := testServerConfigYAML(keyPath, oldRepo, "https://old.example.com", false)
	if err := os.WriteFile(configPath, []byte(initialYAML), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	authStore, err := auth.NewStore(filepath.Join(root, "auth.db"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer authStore.Close()

	server := &Server{
		configPath:   configPath,
		config:       cfg,
		authStore:    authStore,
		hosts:        host.NewHostManager(),
		originPolicy: newOriginPolicy(cfg.Server.AllowedOrigins),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	updateBody, err := json.Marshal(updateConfigRequest{
		YAML: testServerConfigYAML(keyPath, newRepo, "https://new.example.com", false),
		Auth: struct {
			Enabled  bool   `json:"enabled"`
			Username string `json:"username"`
			Password string `json:"password"`
		}{
			Enabled:  true,
			Username: "docker-admin",
			Password: "secret-password",
		},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/admin/config", bytes.NewReader(updateBody))
	server.handleUpdateConfig(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	disallowedReq := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/health", nil)
	disallowedReq.Host = "api.example.com"
	disallowedReq.Header.Set("Origin", "https://old.example.com")
	if _, ok := server.originAllowed(disallowedReq); ok {
		t.Fatal("originAllowed allowed old origin after config update")
	}
	allowedReq := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/health", nil)
	allowedReq.Host = "api.example.com"
	allowedReq.Header.Set("Origin", "https://new.example.com")
	if _, ok := server.originAllowed(allowedReq); !ok {
		t.Fatal("originAllowed denied new origin after config update")
	}

	stackRecorder := httptest.NewRecorder()
	server.handleV1Stacks(stackRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil))
	if stackRecorder.Code != http.StatusOK {
		t.Fatalf("stacks status = %d, want 200; body=%s", stackRecorder.Code, stackRecorder.Body.String())
	}
	var stacksBody struct {
		Count  int `json:"count"`
		Stacks []struct {
			Host string `json:"host"`
			Name string `json:"name"`
		} `json:"stacks"`
	}
	if err := json.NewDecoder(stackRecorder.Body).Decode(&stacksBody); err != nil {
		t.Fatalf("Decode stacks returned error: %v", err)
	}
	if stacksBody.Count != 1 || len(stacksBody.Stacks) != 1 || stacksBody.Stacks[0].Host != "xwing-docker" || stacksBody.Stacks[0].Name != "dockmon" {
		t.Fatalf("stacks response = %#v, want new xwing-docker/dockmon stack", stacksBody)
	}

	loginBody := []byte(`{"username":"docker-admin","password":"secret-password"}`)
	loginRecorder := httptest.NewRecorder()
	server.handleLogin(loginRecorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody)))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginRecorder.Code, loginRecorder.Body.String())
	}
	if cookies := loginRecorder.Result().Cookies(); len(cookies) == 0 || cookies[0].Name != sessionCookieName {
		t.Fatalf("login cookies = %#v, want %s cookie", cookies, sessionCookieName)
	}
}

func testServerConfigYAML(keyPath string, composeRepoPath string, allowedOrigin string, authEnabled bool) string {
	authBlock := "  enabled: false\n  username: \"\"\n  password_hash: \"\"\n"
	if authEnabled {
		authBlock = "  enabled: true\n  username: \"docker-admin\"\n  password_hash: \"$2a$10$abcdefghijklmnopqrstuu5a0vjFvh8WlKTmUeJhTjUyY1JZzGmRa\"\n"
	}
	return `server:
  port: 18734
  bind: "127.0.0.1"
  allowed_origins:
    - "` + allowedOrigin + `"
ssh:
  key_path: "` + filepath.ToSlash(keyPath) + `"
  connect_timeout: "1s"
  keepalive_interval: "30s"
  strict_host_key_checking: "off"
cache_ttl: "1s"
enable_events_stream: false
docker:
  compose_repo_path: "` + filepath.ToSlash(composeRepoPath) + `"
auth:
` + authBlock + `hosts:
  - name: "yoda-docker"
    address: "192.168.40.60"
    port: 22
    user: "brdweb"
    mode: "rootful"
`
}

func writeTestSSHKey(t *testing.T, root string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	encoded, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey returned error: %v", err)
	}
	path := filepath.Join(root, "id_ecdsa")
	data := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encoded})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}
