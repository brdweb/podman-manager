package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/brdweb/homelab-control/internal/config"
)

func TestHandleV1StacksReturnsComposeInventory(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	repoPath := filepath.Join(root, "homelab-docker")
	writeAPIComposeFile(t, repoPath, "xwing-docker", "dockmon", `name: dockmon
services:
  dockmon:
    image: darthnorse/dockmon:latest
    container_name: dockmon
`)

	server := &Server{
		configPath: configPath,
		config: &config.Config{
			Docker: config.DockerConfig{ComposeRepoPath: "homelab-docker"},
		},
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	server.handleV1Stacks(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
		Stacks []struct {
			Host         string `json:"host"`
			Name         string `json:"name"`
			ProjectName  string `json:"project_name"`
			RelativePath string `json:"relative_path"`
			Services     []struct {
				Name          string `json:"name"`
				Image         string `json:"image"`
				ContainerName string `json:"container_name"`
			} `json:"services"`
		} `json:"stacks"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if body.Status != "ok" || body.Count != 1 || len(body.Stacks) != 1 {
		t.Fatalf("response = %#v, want one ok stack", body)
	}
	stack := body.Stacks[0]
	if stack.Host != "xwing-docker" || stack.Name != "dockmon" || stack.ProjectName != "dockmon" {
		t.Fatalf("stack = %#v, want xwing-docker/dockmon", stack)
	}
	if stack.RelativePath != "stacks/xwing-docker/dockmon/compose.yaml" {
		t.Fatalf("RelativePath = %q", stack.RelativePath)
	}
	if len(stack.Services) != 1 || stack.Services[0].Name != "dockmon" || stack.Services[0].ContainerName != "dockmon" {
		t.Fatalf("services = %#v, want dockmon service", stack.Services)
	}
}

func TestHandleV1StacksReportsMissingComposeRepo(t *testing.T) {
	root := t.TempDir()
	server := &Server{
		configPath: filepath.Join(root, "config.yaml"),
		config: &config.Config{
			Docker: config.DockerConfig{ComposeRepoPath: "missing"},
		},
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	server.handleV1Stacks(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if body.Status != "unavailable" || body.Count != 0 {
		t.Fatalf("response = %#v, want unavailable empty inventory", body)
	}
}

func TestHandleV1DockerDiagnosticsReportsComposeSource(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "homelab-docker")
	writeAPIComposeFile(t, repoPath, "xwing-docker", "dockmon", `services:
  dockmon:
    image: darthnorse/dockmon:latest
`)
	writeAPIComposeFile(t, repoPath, "obiwan-docker", "utilities", `services:
  homepage:
    image: ghcr.io/gethomepage/homepage:v1.12.2
`)

	server := &Server{
		configPath: filepath.Join(root, "config.yaml"),
		config: &config.Config{
			Docker: config.DockerConfig{
				ComposeRepoPath: "homelab-docker",
				DockMonURL:      "https://dockmon.example.com",
			},
		},
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/docker", nil)
	server.handleV1DockerDiagnostics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Status           string   `json:"status"`
		Runtime          string   `json:"runtime"`
		DockMonURL       string   `json:"dockmon_url"`
		ComposeAvailable bool     `json:"compose_available"`
		StackCount       int      `json:"stack_count"`
		Hosts            []string `json:"hosts"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if body.Status != "ok" || body.Runtime != "docker" || body.DockMonURL != "https://dockmon.example.com" {
		t.Fatalf("diagnostics = %#v, want ok docker diagnostics", body)
	}
	if !body.ComposeAvailable || body.StackCount != 2 {
		t.Fatalf("compose diagnostics = %#v, want available with 2 stacks", body)
	}
	if len(body.Hosts) != 2 || body.Hosts[0] != "obiwan-docker" || body.Hosts[1] != "xwing-docker" {
		t.Fatalf("hosts = %#v, want sorted docker hosts", body.Hosts)
	}
}

func writeAPIComposeFile(t *testing.T, root string, host string, stack string, content string) {
	t.Helper()
	dir := filepath.Join(root, "stacks", host, stack)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
