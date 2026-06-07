package control

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type ComposeStack struct {
	Host         string           `json:"host"`
	Name         string           `json:"name"`
	ProjectName  string           `json:"project_name,omitempty"`
	RelativePath string           `json:"relative_path"`
	Services     []ComposeService `json:"services"`
	ServiceCount int              `json:"service_count"`
	Source       string           `json:"source"`
}

type ComposeService struct {
	Name          string   `json:"name"`
	Image         string   `json:"image,omitempty"`
	ContainerName string   `json:"container_name,omitempty"`
	Profiles      []string `json:"profiles,omitempty"`
}

type composeFile struct {
	Name     string                    `yaml:"name"`
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image         string   `yaml:"image"`
	ContainerName string   `yaml:"container_name"`
	Profiles      []string `yaml:"profiles"`
}

func LoadComposeStacks(ctx context.Context, repoPath string) ([]ComposeStack, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("compose repository path is required")
	}

	stacksRoot := filepath.Join(repoPath, "stacks")
	if _, err := os.Stat(stacksRoot); err != nil {
		return nil, err
	}

	var stacks []ComposeStack
	err := filepath.WalkDir(stacksRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() || entry.Name() != "compose.yaml" {
			return nil
		}

		stack, err := loadComposeStack(repoPath, path)
		if err != nil {
			return err
		}
		stacks = append(stacks, stack)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(stacks, func(i, j int) bool {
		if stacks[i].Host != stacks[j].Host {
			return stacks[i].Host < stacks[j].Host
		}
		return stacks[i].Name < stacks[j].Name
	})

	return stacks, nil
}

func IsComposeRepoMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

func loadComposeStack(repoPath string, composePath string) (ComposeStack, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return ComposeStack{}, fmt.Errorf("reading compose file %s: %w", composePath, err)
	}

	var parsed composeFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return ComposeStack{}, fmt.Errorf("parsing compose file %s: %w", composePath, err)
	}

	rel, err := filepath.Rel(repoPath, composePath)
	if err != nil {
		return ComposeStack{}, fmt.Errorf("resolving compose file path %s: %w", composePath, err)
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 4 || parts[0] != "stacks" || parts[3] != "compose.yaml" {
		return ComposeStack{}, fmt.Errorf("compose file %s is not under stacks/<host>/<stack>/compose.yaml", composePath)
	}

	services := make([]ComposeService, 0, len(parsed.Services))
	for name, service := range parsed.Services {
		services = append(services, ComposeService{
			Name:          name,
			Image:         strings.TrimSpace(service.Image),
			ContainerName: strings.TrimSpace(service.ContainerName),
			Profiles:      append([]string(nil), service.Profiles...),
		})
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return ComposeStack{
		Host:         parts[1],
		Name:         parts[2],
		ProjectName:  strings.TrimSpace(parsed.Name),
		RelativePath: filepath.ToSlash(rel),
		Services:     services,
		ServiceCount: len(services),
		Source:       "homelab-docker",
	}, nil
}
