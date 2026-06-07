package control

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadComposeStacksReadsHomelabDockerLayout(t *testing.T) {
	root := t.TempDir()
	writeComposeFile(t, root, "xwing-docker", "media", `name: xwing-docker--media
services:
  jellyfin:
    image: docker.io/jellyfin/jellyfin:latest
    container_name: jellyfin
  tdarr-node-xwing:
    image: ghcr.io/haveagitgat/tdarr_node:latest
    profiles:
      - gpu
`)
	writeComposeFile(t, root, "obiwan-docker", "utilities", `services:
  homepage:
    image: ghcr.io/gethomepage/homepage:v1.12.2
`)

	stacks, err := LoadComposeStacks(context.Background(), root)
	if err != nil {
		t.Fatalf("LoadComposeStacks returned error: %v", err)
	}
	if len(stacks) != 2 {
		t.Fatalf("len(stacks) = %d, want 2", len(stacks))
	}

	first := stacks[0]
	if first.Host != "obiwan-docker" || first.Name != "utilities" {
		t.Fatalf("first stack = %s/%s, want obiwan-docker/utilities", first.Host, first.Name)
	}
	if first.RelativePath != "stacks/obiwan-docker/utilities/compose.yaml" {
		t.Fatalf("RelativePath = %q", first.RelativePath)
	}
	if first.ServiceCount != 1 || first.Services[0].Name != "homepage" {
		t.Fatalf("services = %#v, want homepage", first.Services)
	}

	second := stacks[1]
	if second.Host != "xwing-docker" || second.Name != "media" {
		t.Fatalf("second stack = %s/%s, want xwing-docker/media", second.Host, second.Name)
	}
	if second.ProjectName != "xwing-docker--media" {
		t.Fatalf("ProjectName = %q, want xwing-docker--media", second.ProjectName)
	}
	if second.ServiceCount != 2 {
		t.Fatalf("ServiceCount = %d, want 2", second.ServiceCount)
	}
	if second.Services[0].Name != "jellyfin" || second.Services[0].ContainerName != "jellyfin" {
		t.Fatalf("first xwing service = %#v, want jellyfin with container name", second.Services[0])
	}
	if second.Services[1].Name != "tdarr-node-xwing" || len(second.Services[1].Profiles) != 1 || second.Services[1].Profiles[0] != "gpu" {
		t.Fatalf("second xwing service = %#v, want tdarr-node-xwing gpu profile", second.Services[1])
	}
}

func TestLoadComposeStacksMissingRepo(t *testing.T) {
	_, err := LoadComposeStacks(context.Background(), filepath.Join(t.TempDir(), "missing"))
	if !IsComposeRepoMissing(err) {
		t.Fatalf("IsComposeRepoMissing(%v) = false, want true", err)
	}
}

func writeComposeFile(t *testing.T, root string, host string, stack string, content string) {
	t.Helper()
	dir := filepath.Join(root, "stacks", host, stack)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
