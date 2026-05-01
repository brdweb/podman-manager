package connect

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/brdweb/podman-manager/agent/internal/podman"
)

// QuadletContainer represents a discovered Quadlet container definition.
type QuadletContainer struct {
	Name          string
	Unit          string
	Image         string
	ContainerName string
	State         string // "active", "inactive", "failed"
	FilePath      string
}

// DiscoverQuadletContainers scans the Quadlet directory for .container files
// and checks their state via the Podman socket.
func DiscoverQuadletContainers(ctx context.Context, rootful bool) ([]QuadletContainer, error) {
	client, err := podman.NewClient("", 0)
	if err != nil {
		return nil, err
	}
	return discoverQuadletContainers(ctx, rootful, client)
}

func discoverQuadletContainers(ctx context.Context, rootful bool, client *podman.Client) ([]QuadletContainer, error) {
	quadletDir := "/etc/containers/systemd"
	if !rootful {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		quadletDir = filepath.Join(home, ".config", "containers", "systemd")
	}

	files, err := filepath.Glob(filepath.Join(quadletDir, "*.container"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	containers := make([]QuadletContainer, 0, len(files))
	for _, path := range files {
		container, err := parseQuadletContainer(path)
		if err != nil {
			return nil, err
		}
		running, err := client.IsContainerRunning(ctx, container.ContainerName)
		if err != nil {
			return nil, err
		}
		if running {
			container.State = "active"
		} else {
			container.State = "inactive"
		}
		containers = append(containers, container)
	}

	return containers, nil
}

func parseQuadletContainer(path string) (QuadletContainer, error) {
	file, err := os.Open(path)
	if err != nil {
		return QuadletContainer{}, err
	}
	defer file.Close()

	name := strings.TrimSuffix(filepath.Base(path), ".container")
	container := QuadletContainer{
		Name:          name,
		Unit:          name + ".service",
		ContainerName: name,
		State:         "inactive",
		FilePath:      path,
	}

	inContainerSection := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inContainerSection = strings.EqualFold(strings.Trim(line, "[]"), "Container")
			continue
		}
		if !inContainerSection {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "image":
			container.Image = strings.TrimSpace(value)
		case "containername":
			if value := strings.TrimSpace(value); value != "" {
				container.ContainerName = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return QuadletContainer{}, err
	}

	return container, nil
}
