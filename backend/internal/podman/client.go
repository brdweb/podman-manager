package podman

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/brdweb/podman-manager/internal/config"
)

type Client struct {
	pool   *SSHPool
	logger *slog.Logger
	cache  *Cache
}

func NewClient(pool *SSHPool, logger *slog.Logger, cacheTTL time.Duration) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{pool: pool, logger: logger, cache: NewCache(cacheTTL)}
}

func (c *Client) podmanCmd(host config.HostConfig) string {
	if host.IsRootful() {
		return "sudo podman"
	}
	return "podman"
}

func (c *Client) ListContainers(ctx context.Context, hostName string) ([]Container, error) {
	if c.cache == nil {
		return c.fetchContainers(ctx, hostName)
	}

	return c.cache.GetContainers(ctx, hostName, c.fetchContainers)
}

func (c *Client) fetchContainers(ctx context.Context, hostName string) ([]Container, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf("%s ps -a --format json", c.podmanCmd(hostCfg))

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("podman command execution failed", "host", hostName, "command", cmd, "error", err)
		return nil, fmt.Errorf("listing containers on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("podman command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return nil, fmt.Errorf("podman ps failed on %s: %s", hostName, result.Stderr)
	}

	stdout := strings.TrimSpace(result.Stdout)
	if stdout == "" || stdout == "[]" {
		return []Container{}, nil
	}

	var raw []podmanPSEntry
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("parsing container list from %s: %w", hostName, err)
	}

	containers := make([]Container, 0, len(raw))
	for _, r := range raw {
		containers = append(containers, r.toContainer(hostName))
	}

	inactive := c.listInactiveQuadletUnits(ctx, hostName, hostCfg, containers)
	containers = append(containers, inactive...)

	stats, err := c.ListContainerStats(ctx, hostName)
	if err == nil {
		for i := range containers {
			if stat, ok := stats[containers[i].ID]; ok {
				containers[i].Stats = stat
				continue
			}
			if stat, ok := stats[containers[i].Name]; ok {
				containers[i].Stats = stat
			}
		}
	}

	return containers, nil
}

func (c *Client) ListContainerStats(ctx context.Context, hostName string) (map[string]*ContainerStats, error) {
	if c.cache == nil {
		return c.fetchContainerStats(ctx, hostName)
	}

	return c.cache.GetStats(ctx, hostName, c.fetchContainerStats)
}

func (c *Client) fetchContainerStats(ctx context.Context, hostName string) (map[string]*ContainerStats, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf(`%s stats --all --no-stream --format "{{json .}}"`, c.podmanCmd(hostCfg))

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("podman command execution failed", "host", hostName, "command", cmd, "error", err)
		return nil, fmt.Errorf("getting container stats on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("podman command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return nil, fmt.Errorf("podman stats failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	stats := make(map[string]*ContainerStats)
	scanner := bufio.NewScanner(strings.NewReader(result.Stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		stat := parseContainerStats(raw)
		if stat == nil {
			continue
		}

		if id := firstString(raw, "ID", "ContainerID", "Container"); id != "" {
			stats[id] = stat
		}
		if name := firstString(raw, "Name"); name != "" {
			stats[name] = stat
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading container stats on %s: %w", hostName, err)
	}

	return stats, nil
}

func (c *Client) HostSystemInfo(ctx context.Context, hostName string) (*HostSystemInfo, error) {
	if c.cache == nil {
		return c.fetchHostSystemInfo(ctx, hostName)
	}

	return c.cache.GetSystemInfo(ctx, hostName, c.fetchHostSystemInfo)
}

func (c *Client) fetchHostSystemInfo(ctx context.Context, hostName string) (*HostSystemInfo, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	cmd := `sh -c '
hostname=$(hostname 2>/dev/null || echo "unknown")
os=$(cat /etc/os-release 2>/dev/null | grep "^PRETTY_NAME=" | cut -d= -f2- | tr -d "\"")
kernel=$(uname -sr 2>/dev/null || echo "unknown")
uptime=$(cat /proc/uptime 2>/dev/null | cut -d. -f1 || echo 0)
mem_total=$(cat /proc/meminfo 2>/dev/null | grep "^MemTotal:" | awk "{print \$2}" || echo 0)
mem_avail=$(cat /proc/meminfo 2>/dev/null | grep "^MemAvailable:" | awk "{print \$2}" || echo 0)
load=$(cat /proc/loadavg 2>/dev/null | awk "{print \$1, \$2, \$3}" || echo "0 0 0")
cores=$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null || echo 0)
disk=$(df -B1 / 2>/dev/null | awk "NR==2 {print \$2\"|\"\$3\"|\"\$4}" || echo "0|0|0")
printf "hostname=%s\nos=%s\nkernel=%s\nuptime=%s\nmem_total_kb=%s\nmem_avail_kb=%s\nload=%s\ncpu_cores=%s\ndisk=%s\n" "$hostname" "$os" "$kernel" "$uptime" "$mem_total" "$mem_avail" "$load" "$cores" "$disk"
'`

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("host system info command execution failed", "host", hostName, "command", cmd, "error", err)
		return nil, fmt.Errorf("getting system info on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("host system info command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return nil, fmt.Errorf("system info command failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	info := &HostSystemInfo{}
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]
		switch key {
		case "hostname":
			info.Hostname = value
		case "os":
			info.OS = value
		case "kernel":
			info.Kernel = value
		case "uptime":
			info.UptimeSeconds = mustParseInt64(value)
		case "mem_total_kb":
			info.MemoryTotalBytes = uint64(mustParseInt64(value) * 1024)
		case "mem_avail_kb":
			available := uint64(mustParseInt64(value) * 1024)
			if info.MemoryTotalBytes >= available {
				info.MemoryUsedBytes = info.MemoryTotalBytes - available
			}
		case "load":
			loadParts := strings.Fields(value)
			if len(loadParts) >= 3 {
				info.Load1 = mustParseFloat(loadParts[0])
				info.Load5 = mustParseFloat(loadParts[1])
				info.Load15 = mustParseFloat(loadParts[2])
			}
		case "cpu_cores":
			info.CPUCores = int(mustParseInt64(value))
		case "disk":
			diskParts := strings.Split(value, "|")
			if len(diskParts) >= 3 {
				info.DiskTotalBytes = uint64(mustParseInt64(diskParts[0]))
				info.DiskUsedBytes = uint64(mustParseInt64(diskParts[1]))
				info.DiskFreeBytes = uint64(mustParseInt64(diskParts[2]))
			}
		}
	}

	return info, nil
}

// listInactiveQuadletUnits discovers Quadlet .container files on the host,
// checks which corresponding systemd services are inactive, and returns
// synthetic Container entries for stopped Quadlet containers.
// Quadlet containers are ephemeral (removed on stop), so podman ps -a
// won't show them when stopped.
func (c *Client) listInactiveQuadletUnits(ctx context.Context, hostName string, hostCfg config.HostConfig, running []Container) []Container {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil
	}

	var quadletDir, systemctlPrefix, envSetup string
	if hostCfg.IsRootful() {
		quadletDir = "/etc/containers/systemd"
		systemctlPrefix = "sudo systemctl"
		envSetup = ""
	} else {
		quadletDir = "$HOME/.config/containers/systemd"
		systemctlPrefix = "systemctl --user"
		envSetup = "export XDG_RUNTIME_DIR=/run/user/$(id -u); "
	}

	cmd := fmt.Sprintf(
		`%sfor f in %s/*.container; do `+
			`[ -f "$f" ] || continue; `+
			`unit=$(basename "$f" .container).service; `+
			`state=$(%s is-active "$unit" 2>/dev/null); `+
			`image=$(grep -i "^Image=" "$f" 2>/dev/null | head -1 | cut -d= -f2-); `+
			`cname=$(grep -i "^ContainerName=" "$f" 2>/dev/null | head -1 | cut -d= -f2-); `+
			`echo "$unit|$state|$image|$cname"; `+
			`done`,
		envSetup, quadletDir, systemctlPrefix,
	)

	result, err := conn.Run(ctx, cmd)
	if err != nil || result.ExitCode != 0 {
		if err != nil {
			c.logger.Error("quadlet discovery command execution failed", "host", hostName, "command", cmd, "error", err)
		} else {
			c.logger.Error("quadlet discovery command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		}
		return nil
	}

	existing := make(map[string]bool)
	for _, ct := range running {
		existing[ct.Name] = true
	}

	var inactive []Container
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		unit := parts[0]
		state := parts[1]
		image := parts[2]
		cname := parts[3]

		if state == "active" || state == "activating" {
			continue
		}

		if cname == "" {
			cname = strings.TrimSuffix(unit, ".service")
		}

		if existing[cname] {
			continue
		}

		inactive = append(inactive, Container{
			ID:          "quadlet-" + strings.TrimSuffix(unit, ".service"),
			Name:        cname,
			Image:       image,
			State:       "exited",
			Status:      "Stopped (Quadlet)",
			Manager:     "quadlet",
			SystemdUnit: unit,
			Host:        hostName,
		})
	}

	return inactive
}

func (c *Client) InspectContainer(ctx context.Context, hostName, containerID string) (*ContainerDetail, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	safeID := sanitizeID(containerID)
	cmd := fmt.Sprintf("%s inspect %s --format json", c.podmanCmd(hostCfg), safeID)

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("podman command execution failed", "host", hostName, "command", cmd, "error", err)
		return nil, fmt.Errorf("inspecting container %s on %s: %w", safeID, hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("podman command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr), "container", safeID)
		return nil, fmt.Errorf("podman inspect failed on %s: %s", hostName, result.Stderr)
	}

	var raw []podmanInspectEntry
	if err := json.Unmarshal([]byte(result.Stdout), &raw); err != nil {
		return nil, fmt.Errorf("parsing inspect from %s: %w", hostName, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("container %s not found on %s", safeID, hostName)
	}

	detail := raw[0].toContainerDetail(hostName)
	stats, err := c.ListContainerStats(ctx, hostName)
	if err == nil {
		if stat, ok := stats[detail.ID]; ok {
			detail.Stats = stat
		} else if stat, ok := stats[detail.Name]; ok {
			detail.Stats = stat
		}
	}
	return &detail, nil
}

func (c *Client) StartContainer(ctx context.Context, hostName, containerID string) (*ActionResult, error) {
	return c.containerAction(ctx, hostName, containerID, "start")
}

func (c *Client) StopContainer(ctx context.Context, hostName, containerID string) (*ActionResult, error) {
	return c.containerAction(ctx, hostName, containerID, "stop")
}

func (c *Client) RestartContainer(ctx context.Context, hostName, containerID string) (*ActionResult, error) {
	return c.containerAction(ctx, hostName, containerID, "restart")
}

func (c *Client) CheckForUpdate(ctx context.Context, hostName, containerID string) (*UpdateCheckResult, error) {
	container, err := c.containerForUpdate(ctx, hostName, containerID)
	if err != nil {
		return nil, err
	}

	result := &UpdateCheckResult{
		ContainerID:   container.ID,
		ContainerName: container.Name,
		Image:         container.Image,
	}

	safeRef := sanitizeImageRef(container.Image)
	if safeRef == "" {
		result.Error = "container image reference is empty or invalid"
		return result, nil
	}

	if !isRegistryImageReference(safeRef) {
		result.UpdateAvailable = false
		return result, nil
	}

	localDigest, err := c.getLocalImageDigest(ctx, hostName, safeRef)
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	result.LocalDigest = localDigest

	remoteDigest, err := c.getRemoteImageDigest(ctx, hostName, safeRef)
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	result.RemoteDigest = remoteDigest

	if localDigest != "" && remoteDigest != "" {
		result.UpdateAvailable = localDigest != remoteDigest
	}

	return result, nil
}

func (c *Client) UpdateContainer(ctx context.Context, hostName, containerID string) (*UpdateResult, error) {
	container, err := c.containerForUpdate(ctx, hostName, containerID)
	if err != nil {
		return nil, err
	}

	imageRef := sanitizeImageRef(container.Image)
	if imageRef == "" {
		return &UpdateResult{Success: false, Error: "container image reference is empty or invalid"}, nil
	}

	oldDigest, _ := c.getLocalImageDigest(ctx, hostName, imageRef)
	oldImage := imageRef
	if oldDigest != "" {
		oldImage = fmt.Sprintf("%s@%s", imageRef, oldDigest)
	}

	result := &UpdateResult{OldImage: oldImage}

	manager := container.Manager
	if manager == "" {
		manager = detectManagerFromLabels(container.Labels)
	}

	switch manager {
	case "quadlet":
		err = c.updateQuadletContainer(ctx, hostName, container, imageRef)
	case "compose":
		err = c.updateComposeContainer(ctx, hostName, container, imageRef)
	default:
		err = c.updateStandaloneContainer(ctx, hostName, container.ID, imageRef)
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, nil
	}

	newDigest, _ := c.getLocalImageDigest(ctx, hostName, imageRef)
	newImage := imageRef
	if newDigest != "" {
		newImage = fmt.Sprintf("%s@%s", imageRef, newDigest)
	}

	if c.cache != nil {
		c.cache.Invalidate(hostName)
	}

	result.Success = true
	result.NewImage = newImage
	result.Message = fmt.Sprintf("Container %s updated successfully", container.Name)
	return result, nil
}

func (c *Client) RemoveContainer(ctx context.Context, hostName, containerID string, force bool) (*ActionResult, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return &ActionResult{Success: false, Error: err.Error()}, nil
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	safeID := sanitizeID(containerID)

	cmd := fmt.Sprintf("%s rm %s", c.podmanCmd(hostCfg), safeID)
	if force {
		cmd = fmt.Sprintf("%s rm -f %s", c.podmanCmd(hostCfg), safeID)
	}

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("container removal command execution failed", "host", hostName, "container", safeID, "command", cmd, "error", err)
		return &ActionResult{Success: false, Error: err.Error()}, nil
	}

	if result.ExitCode != 0 {
		stderr := strings.TrimSpace(result.Stderr)
		if stderr == "" {
			stderr = "failed to remove container"
		}

		c.logger.Error("container removal command returned non-zero exit", "host", hostName, "container", safeID, "command", cmd, "exit_code", result.ExitCode, "stderr", stderr)

		lower := strings.ToLower(stderr)
		switch {
		case strings.Contains(lower, "no such container"), strings.Contains(lower, "not found"):
			return &ActionResult{Success: false, Error: fmt.Sprintf("container %s not found on %s", safeID, hostName)}, nil
		case strings.Contains(lower, "running") && !force:
			return &ActionResult{Success: false, Error: fmt.Sprintf("container %s is running on %s; stop it first or use force=true", safeID, hostName)}, nil
		default:
			return &ActionResult{Success: false, Error: stderr}, nil
		}
	}

	if c.cache != nil {
		c.cache.Invalidate(hostName)
	}

	msg := fmt.Sprintf("Container %s removed successfully", safeID)
	if force {
		msg = fmt.Sprintf("Container %s force removed successfully", safeID)
	}
	return &ActionResult{Success: true, Message: msg}, nil
}

func (c *Client) containerAction(ctx context.Context, hostName, containerID, action string) (*ActionResult, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return &ActionResult{Success: false, Error: err.Error()}, nil
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	safeID := sanitizeID(containerID)

	var cmd string
	if strings.HasPrefix(safeID, "quadlet-") {
		unit := strings.TrimPrefix(safeID, "quadlet-") + ".service"
		cmd = c.systemctlCmd(hostCfg, action, unit)
	} else {
		unit := c.getSystemdUnit(ctx, hostName, hostCfg, safeID)
		if unit != "" {
			cmd = c.systemctlCmd(hostCfg, action, unit)
		} else {
			cmd = fmt.Sprintf("%s %s %s", c.podmanCmd(hostCfg), action, safeID)
		}
	}

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("container action command execution failed", "host", hostName, "container", safeID, "action", action, "command", cmd, "error", err)
		return &ActionResult{Success: false, Error: err.Error()}, nil
	}

	if result.ExitCode != 0 {
		c.logger.Error("container action command returned non-zero exit", "host", hostName, "container", safeID, "action", action, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return &ActionResult{
			Success: false,
			Error:   strings.TrimSpace(result.Stderr),
		}, nil
	}

	if c.cache != nil {
		c.cache.Invalidate(hostName)
	}

	return &ActionResult{
		Success: true,
		Message: fmt.Sprintf("Container %s %sed successfully", safeID, action),
	}, nil
}

func (c *Client) getSystemdUnit(ctx context.Context, hostName string, hostCfg config.HostConfig, containerID string) string {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return ""
	}

	cmd := fmt.Sprintf(`%s inspect --format '{{index .Config.Labels "PODMAN_SYSTEMD_UNIT"}}' %s`, c.podmanCmd(hostCfg), containerID)
	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("systemd unit lookup command execution failed", "host", hostName, "container", containerID, "command", cmd, "error", err)
		return ""
	}

	unit := strings.TrimSpace(result.Stdout)
	if unit == "" || unit == "<no value>" {
		return ""
	}
	return unit
}

func (c *Client) containerForUpdate(ctx context.Context, hostName, containerID string) (*ContainerDetail, error) {
	safeID := sanitizeID(containerID)
	if safeID == "" {
		return nil, fmt.Errorf("invalid container id")
	}

	if strings.HasPrefix(safeID, "quadlet-") {
		containers, err := c.ListContainers(ctx, hostName)
		if err != nil {
			return nil, err
		}
		for _, ct := range containers {
			if ct.ID == safeID {
				return &ContainerDetail{Container: ct}, nil
			}
		}
		return nil, fmt.Errorf("container %s not found on %s", safeID, hostName)
	}

	detail, err := c.InspectContainer(ctx, hostName, safeID)
	if err != nil {
		return nil, err
	}

	if detail.Manager == "" {
		detail.Manager = detectManagerFromLabels(detail.Labels)
	}

	return detail, nil
}

func (c *Client) getLocalImageDigest(ctx context.Context, hostName, imageRef string) (string, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return "", err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf("%s image inspect %s --format '{{if .Digest}}{{.Digest}}{{else}}{{index .RepoDigests 0}}{{end}}'", c.podmanCmd(hostCfg), shellQuote(imageRef))
	result, err := conn.Run(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("inspecting local image digest on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("podman image inspect failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	return normalizeDigest(strings.TrimSpace(result.Stdout)), nil
}

func (c *Client) getRemoteImageDigest(ctx context.Context, hostName, imageRef string) (string, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return "", err
	}

	remoteRef := "docker://" + imageRef
	skopeoCmd := fmt.Sprintf("skopeo inspect --format '{{.Digest}}' %s", shellQuote(remoteRef))
	skopeoResult, err := conn.Run(ctx, skopeoCmd)
	if err == nil && skopeoResult.ExitCode == 0 {
		digest := normalizeDigest(strings.TrimSpace(skopeoResult.Stdout))
		if digest != "" {
			return digest, nil
		}
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	manifestCmd := fmt.Sprintf("%s manifest inspect %s", c.podmanCmd(hostCfg), shellQuote(remoteRef))
	manifestResult, err := conn.Run(ctx, manifestCmd)
	if err != nil {
		return "", fmt.Errorf("inspecting remote image digest on %s: %w", hostName, err)
	}
	if manifestResult.ExitCode != 0 {
		stderr := strings.TrimSpace(manifestResult.Stderr)
		if stderr == "" && skopeoResult.Stderr != "" {
			stderr = strings.TrimSpace(skopeoResult.Stderr)
		}
		if stderr == "" {
			stderr = "unable to inspect remote image"
		}
		return "", fmt.Errorf("remote image inspect failed on %s: %s", hostName, stderr)
	}

	var manifest struct {
		Digest string `json:"Digest"`
	}
	if err := json.Unmarshal([]byte(manifestResult.Stdout), &manifest); err != nil {
		return "", fmt.Errorf("parsing remote image digest on %s: %w", hostName, err)
	}

	digest := normalizeDigest(strings.TrimSpace(manifest.Digest))
	if digest == "" {
		return "", fmt.Errorf("remote image digest not found")
	}

	return digest, nil
}

func (c *Client) updateQuadletContainer(ctx context.Context, hostName string, container *ContainerDetail, imageRef string) error {
	hostCfg, _ := c.pool.HostConfig(hostName)
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return err
	}

	if err := c.PullImage(ctx, hostName, imageRef); err != nil {
		return err
	}

	unit := strings.TrimSpace(container.SystemdUnit)
	if unit == "" {
		if raw, ok := container.Labels["PODMAN_SYSTEMD_UNIT"]; ok {
			unit = strings.TrimSpace(raw)
		}
	}
	if unit == "" {
		if strings.HasPrefix(container.ID, "quadlet-") {
			unit = strings.TrimPrefix(container.ID, "quadlet-") + ".service"
		} else {
			unit = c.getSystemdUnit(ctx, hostName, hostCfg, sanitizeID(container.ID))
		}
	}
	if unit == "" {
		return fmt.Errorf("unable to determine Quadlet systemd unit")
	}

	restartCmd := c.systemctlCmd(hostCfg, "restart", shellQuote(unit))
	restartResult, err := conn.Run(ctx, restartCmd)
	if err != nil {
		return fmt.Errorf("restarting Quadlet unit %s on %s: %w", unit, hostName, err)
	}
	if restartResult.ExitCode != 0 {
		return fmt.Errorf("systemctl restart failed on %s: %s", hostName, strings.TrimSpace(restartResult.Stderr))
	}

	return nil
}

func (c *Client) updateComposeContainer(ctx context.Context, hostName string, container *ContainerDetail, imageRef string) error {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return err
	}

	composeDir := strings.TrimSpace(firstNonEmpty(
		container.Labels["com.docker.compose.project.working_dir"],
		container.Labels["io.podman.compose.project.working_dir"],
	))

	if composeDir == "" {
		return c.updateStandaloneContainer(ctx, hostName, container.ID, imageRef)
	}

	composeCmd := fmt.Sprintf("sh -lc \"cd %s && podman-compose pull && podman-compose up -d\"", shellQuote(composeDir))
	result, err := conn.Run(ctx, composeCmd)
	if err != nil {
		return fmt.Errorf("running podman-compose update on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("podman-compose update failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	return nil
}

func (c *Client) updateStandaloneContainer(ctx context.Context, hostName, containerID, imageRef string) error {
	entry, err := c.inspectContainerEntry(ctx, hostName, containerID)
	if err != nil {
		return err
	}

	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)

	if err := c.PullImage(ctx, hostName, imageRef); err != nil {
		return err
	}

	if entry.State.Running {
		stopCmd := fmt.Sprintf("%s stop %s", c.podmanCmd(hostCfg), shellQuote(entry.ID))
		stopResult, err := conn.Run(ctx, stopCmd)
		if err != nil {
			return fmt.Errorf("stopping container %s on %s: %w", entry.ID, hostName, err)
		}
		if stopResult.ExitCode != 0 {
			return fmt.Errorf("stopping container %s failed on %s: %s", entry.ID, hostName, strings.TrimSpace(stopResult.Stderr))
		}
	}

	rmCmd := fmt.Sprintf("%s rm %s", c.podmanCmd(hostCfg), shellQuote(entry.ID))
	rmResult, err := conn.Run(ctx, rmCmd)
	if err != nil {
		return fmt.Errorf("removing container %s on %s: %w", entry.ID, hostName, err)
	}
	if rmResult.ExitCode != 0 {
		return fmt.Errorf("removing container %s failed on %s: %s", entry.ID, hostName, strings.TrimSpace(rmResult.Stderr))
	}

	runCmd, networks := buildStandaloneRunCommand(c.podmanCmd(hostCfg), &entry, imageRef)
	runResult, err := conn.Run(ctx, runCmd)
	if err != nil {
		return fmt.Errorf("recreating container %s on %s: %w", entry.ID, hostName, err)
	}
	if runResult.ExitCode != 0 {
		return fmt.Errorf("recreating container %s failed on %s: %s", entry.ID, hostName, strings.TrimSpace(runResult.Stderr))
	}

	containerName := strings.TrimPrefix(entry.Name, "/")
	for _, network := range networks {
		connectCmd := fmt.Sprintf("%s network connect %s %s", c.podmanCmd(hostCfg), shellQuote(network), shellQuote(containerName))
		connectResult, err := conn.Run(ctx, connectCmd)
		if err != nil {
			return fmt.Errorf("connecting container %s to network %s on %s: %w", containerName, network, hostName, err)
		}
		if connectResult.ExitCode != 0 {
			return fmt.Errorf("network connect failed on %s: %s", hostName, strings.TrimSpace(connectResult.Stderr))
		}
	}

	return nil
}

func (c *Client) inspectContainerEntry(ctx context.Context, hostName, containerID string) (podmanInspectEntry, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return podmanInspectEntry{}, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	safeID := sanitizeID(containerID)
	if safeID == "" {
		return podmanInspectEntry{}, fmt.Errorf("invalid container id")
	}

	cmd := fmt.Sprintf("%s inspect %s --format json", c.podmanCmd(hostCfg), safeID)
	result, err := conn.Run(ctx, cmd)
	if err != nil {
		return podmanInspectEntry{}, fmt.Errorf("inspecting container %s on %s: %w", safeID, hostName, err)
	}
	if result.ExitCode != 0 {
		return podmanInspectEntry{}, fmt.Errorf("podman inspect failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	var raw []podmanInspectEntry
	if err := json.Unmarshal([]byte(result.Stdout), &raw); err != nil {
		return podmanInspectEntry{}, fmt.Errorf("parsing inspect from %s: %w", hostName, err)
	}
	if len(raw) == 0 {
		return podmanInspectEntry{}, fmt.Errorf("container %s not found on %s", safeID, hostName)
	}

	return raw[0], nil
}

func buildStandaloneRunCommand(podmanBin string, entry *podmanInspectEntry, imageRef string) (string, []string) {
	args := []string{podmanBin, "run", "-d", "--name", shellQuote(strings.TrimPrefix(entry.Name, "/"))}

	if entry.Config.Hostname != "" {
		args = append(args, "--hostname", shellQuote(entry.Config.Hostname))
	}

	if rp := strings.TrimSpace(entry.HostConfig.RestartPolicy.Name); rp != "" {
		args = append(args, "--restart", shellQuote(rp))
	}

	labels := sortedMapKeys(entry.Config.Labels)
	for _, key := range labels {
		args = append(args, "--label", shellQuote(fmt.Sprintf("%s=%s", key, entry.Config.Labels[key])))
	}

	envVars := append([]string(nil), entry.Config.Env...)
	sort.Strings(envVars)
	for _, env := range envVars {
		if strings.TrimSpace(env) == "" {
			continue
		}
		args = append(args, "--env", shellQuote(env))
	}

	mounts := append([]struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	}{}, entry.Mounts...)
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].Destination < mounts[j].Destination
	})
	for _, mount := range mounts {
		if mount.Source == "" || mount.Destination == "" {
			continue
		}
		volume := fmt.Sprintf("%s:%s", mount.Source, mount.Destination)
		if !mount.RW {
			volume += ":ro"
		}
		args = append(args, "-v", shellQuote(volume))
	}

	ports := make([]string, 0, len(entry.NetworkSettings.Ports))
	for portSpec, bindings := range entry.NetworkSettings.Ports {
		for _, binding := range bindings {
			if strings.TrimSpace(binding.HostPort) == "" {
				continue
			}
			if binding.HostIP != "" {
				ports = append(ports, fmt.Sprintf("%s:%s:%s", binding.HostIP, binding.HostPort, portSpec))
			} else {
				ports = append(ports, fmt.Sprintf("%s:%s", binding.HostPort, portSpec))
			}
		}
	}
	sort.Strings(ports)
	for _, port := range ports {
		args = append(args, "-p", shellQuote(port))
	}

	extraNetworks := []string{}
	networkMode := strings.TrimSpace(entry.HostConfig.NetworkMode)
	if networkMode != "" && networkMode != "default" {
		args = append(args, "--network", shellQuote(networkMode))
	} else {
		networks := sortedMapKeys(entry.NetworkSettings.Networks)
		if len(networks) > 0 {
			args = append(args, "--network", shellQuote(networks[0]))
			if len(networks) > 1 {
				extraNetworks = append(extraNetworks, networks[1:]...)
			}
		}
	}

	if entrypoint := parseStringSliceOrScalar(entry.Config.Entrypoint); len(entrypoint) > 0 {
		args = append(args, "--entrypoint", shellQuote(strings.Join(entrypoint, " ")))
	}

	args = append(args, shellQuote(imageRef))
	for _, cmdArg := range entry.Config.Cmd {
		args = append(args, shellQuote(cmdArg))
	}

	return strings.Join(args, " "), extraNetworks
}

func sortedMapKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func detectManagerFromLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "standalone"
	}
	if unit := strings.TrimSpace(labels["PODMAN_SYSTEMD_UNIT"]); unit != "" {
		return "quadlet"
	}
	if _, ok := labels["com.docker.compose.project"]; ok {
		return "compose"
	}
	return "standalone"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeDigest(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "<no value>" || value == "null" {
		return ""
	}
	if idx := strings.LastIndex(value, "@"); idx >= 0 && idx < len(value)-1 {
		value = value[idx+1:]
	}
	return value
}

func isRegistryImageReference(imageRef string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(imageRef))
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "sha256:") || strings.HasPrefix(trimmed, "containers-storage:") || strings.HasPrefix(trimmed, "localhost/") {
		return false
	}
	return true
}

func parseStringSliceOrScalar(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil
		}
		return []string{single}
	}

	return nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (c *Client) systemctlCmd(host config.HostConfig, action, unit string) string {
	if host.IsRootful() {
		return fmt.Sprintf("sudo systemctl %s %s", action, unit)
	}
	return fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/$(id -u) systemctl --user %s %s", action, unit)
}

func (c *Client) ContainerLogs(ctx context.Context, hostName, containerID string, tail int) (string, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return "", err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	safeID := sanitizeID(containerID)
	if tail <= 0 {
		tail = 100
	}
	cmd := fmt.Sprintf("%s logs --tail %d %s", c.podmanCmd(hostCfg), tail, safeID)

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("container logs command execution failed", "host", hostName, "container", safeID, "command", cmd, "error", err)
		return "", fmt.Errorf("getting logs for %s on %s: %w", safeID, hostName, err)
	}

	output := result.Stdout
	if output == "" {
		output = result.Stderr
	}
	return output, nil
}

func (c *Client) StreamLogs(ctx context.Context, hostName, containerID string, tail int, output chan<- string) error {
	safeID := sanitizeID(containerID)
	if safeID == "" {
		return fmt.Errorf("invalid container id")
	}
	if tail <= 0 {
		tail = 100
	}

	tailForAttempt := tail
	backoff := time.Second
	maxBackoff := 10 * time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := c.streamLogsSession(ctx, hostName, safeID, tailForAttempt, output)
		if ctx.Err() != nil {
			return nil
		}

		if strings.Contains(strings.ToLower(err.Error()), "unknown host") {
			return err
		}

		c.logger.Warn("container logs stream disconnected, retrying", "host", hostName, "container", safeID, "error", err, "retry_in", backoff.String())

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}

		tailForAttempt = 0
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (c *Client) streamLogsSession(ctx context.Context, hostName, containerID string, tail int, output chan<- string) error {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf("%s logs -f --tail %d %s", c.podmanCmd(hostCfg), tail, containerID)
	session, stdout, stderr, err := openStreamingSession(conn, cmd)
	if err != nil {
		return fmt.Errorf("starting logs stream for %s on %s: %w", containerID, hostName, err)
	}
	defer session.Close()

	go func() {
		<-ctx.Done()
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		select {
		case output <- line:
		case <-ctx.Done():
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading logs stream on %s: %w", hostName, err)
	}

	waitErr := session.Wait()
	if ctx.Err() != nil {
		return nil
	}

	if waitErr != nil {
		if exitErr, ok := waitErr.(*ssh.ExitError); ok {
			errText := strings.TrimSpace(stderr.String())
			if errText == "" {
				errText = strings.TrimSpace(exitErr.Error())
			}
			return fmt.Errorf("logs stream process ended: %s", errText)
		}
		return fmt.Errorf("waiting for logs stream on %s: %w", hostName, waitErr)
	}

	if stderr.Len() > 0 {
		return fmt.Errorf("logs stream ended: %s", strings.TrimSpace(stderr.String()))
	}

	return fmt.Errorf("logs stream ended unexpectedly")
}

func (c *Client) ListImages(ctx context.Context, hostName string) ([]Image, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf("%s images --format json", c.podmanCmd(hostCfg))

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("podman command execution failed", "host", hostName, "command", cmd, "error", err)
		return nil, fmt.Errorf("listing images on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("podman command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return nil, fmt.Errorf("podman images failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	stdout := strings.TrimSpace(result.Stdout)
	if stdout == "" || stdout == "[]" {
		return []Image{}, nil
	}

	var raw []podmanImageEntry
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("parsing image list from %s: %w", hostName, err)
	}

	images := make([]Image, 0, len(raw))
	for _, entry := range raw {
		images = append(images, entry.toImage())
	}

	return images, nil
}

func (c *Client) PullImage(ctx context.Context, hostName, imageRef string) error {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	safeRef := sanitizeImageRef(imageRef)
	if safeRef == "" {
		return fmt.Errorf("invalid image reference")
	}

	cmd := fmt.Sprintf("%s pull %s", c.podmanCmd(hostCfg), safeRef)
	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("podman command execution failed", "host", hostName, "command", cmd, "error", err)
		return fmt.Errorf("pulling image %s on %s: %w", safeRef, hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("podman command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return fmt.Errorf("podman pull failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	if c.cache != nil {
		c.cache.Invalidate(hostName)
	}

	return nil
}

func (c *Client) RemoveImage(ctx context.Context, hostName, imageID string, force bool) error {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	safeID := sanitizeID(imageID)
	if safeID == "" {
		return fmt.Errorf("invalid image id")
	}

	cmd := fmt.Sprintf("%s rmi %s", c.podmanCmd(hostCfg), safeID)
	if force {
		cmd = fmt.Sprintf("%s rmi --force %s", c.podmanCmd(hostCfg), safeID)
	}

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("podman command execution failed", "host", hostName, "command", cmd, "error", err)
		return fmt.Errorf("removing image %s on %s: %w", safeID, hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("podman command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return fmt.Errorf("podman rmi failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	if c.cache != nil {
		c.cache.Invalidate(hostName)
	}

	return nil
}

func (c *Client) PruneImages(ctx context.Context, hostName string) (int, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return 0, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf("%s image prune -f", c.podmanCmd(hostCfg))

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		c.logger.Error("podman command execution failed", "host", hostName, "command", cmd, "error", err)
		return 0, fmt.Errorf("pruning images on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
		c.logger.Error("podman command returned non-zero exit", "host", hostName, "command", cmd, "exit_code", result.ExitCode, "stderr", strings.TrimSpace(result.Stderr))
		return 0, fmt.Errorf("podman image prune failed on %s: %s", hostName, strings.TrimSpace(result.Stderr))
	}

	if c.cache != nil {
		c.cache.Invalidate(hostName)
	}

	return parsePrunedImageCount(result.Stdout), nil
}

func parseContainerStats(raw map[string]any) *ContainerStats {
	stat := &ContainerStats{}

	if cpu := firstString(raw, "CPU", "CPUPerc"); cpu != "" {
		stat.CPUPercent = parsePercent(cpu)
	}

	if memUsage := firstString(raw, "MemUsage", "MemUsageBytes"); memUsage != "" {
		used, limit := parseUsagePair(memUsage)
		stat.MemoryUsageBytes = used
		if limit > 0 {
			stat.MemoryLimitBytes = limit
		}
	}

	if stat.MemoryUsageBytes == 0 {
		stat.MemoryUsageBytes = parseHumanBytes(firstString(raw, "Usage", "MemUsageBytes"))
	}
	if stat.MemoryLimitBytes == 0 {
		stat.MemoryLimitBytes = parseHumanBytes(firstString(raw, "MemLimit"))
	}
	if memPercent := firstString(raw, "MemPerc", "MemoryPercent"); memPercent != "" {
		stat.MemoryPercent = parsePercent(memPercent)
	} else if stat.MemoryUsageBytes > 0 && stat.MemoryLimitBytes > 0 {
		stat.MemoryPercent = (float64(stat.MemoryUsageBytes) / float64(stat.MemoryLimitBytes)) * 100
	}

	stat.PIDs = int(mustParseInt64(firstString(raw, "PIDs", "PIDS")))

	if netIO := firstString(raw, "NetIO", "NetworkIO"); netIO != "" {
		stat.NetworkInputBytes, stat.NetworkOutputBytes = parseUsagePair(netIO)
	}
	if blockIO := firstString(raw, "BlockIO"); blockIO != "" {
		stat.BlockInputBytes, stat.BlockOutputBytes = parseUsagePair(blockIO)
	}

	if stat.CPUPercent == 0 &&
		stat.MemoryUsageBytes == 0 &&
		stat.MemoryLimitBytes == 0 &&
		stat.MemoryPercent == 0 &&
		stat.PIDs == 0 &&
		stat.NetworkInputBytes == 0 &&
		stat.NetworkOutputBytes == 0 &&
		stat.BlockInputBytes == 0 &&
		stat.BlockOutputBytes == 0 {
		return nil
	}

	return stat
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			switch v := value.(type) {
			case string:
				return strings.TrimSpace(v)
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
	}
	return ""
}

func parseUsagePair(value string) (uint64, uint64) {
	parts := strings.Split(value, "/")
	if len(parts) >= 2 {
		return parseHumanBytes(parts[0]), parseHumanBytes(parts[1])
	}
	parts = strings.Split(value, "|")
	if len(parts) >= 2 {
		return parseHumanBytes(parts[0]), parseHumanBytes(parts[1])
	}
	return parseHumanBytes(value), 0
}

func parsePercent(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	return mustParseFloat(value)
}

func parseHumanBytes(value string) uint64 {
	cleaned := strings.ToLower(strings.TrimSpace(value))
	if cleaned == "" || cleaned == "n/a" || cleaned == "--" {
		return 0
	}

	cleaned = strings.ReplaceAll(cleaned, "i", "")
	cleaned = strings.ReplaceAll(cleaned, "b", "")
	cleaned = strings.ReplaceAll(cleaned, "bytes", "")
	cleaned = strings.TrimSpace(cleaned)

	end := 0
	for end < len(cleaned) && (cleaned[end] == '.' || cleaned[end] == '-' || (cleaned[end] >= '0' && cleaned[end] <= '9')) {
		end++
	}
	if end == 0 {
		return 0
	}

	number := mustParseFloat(cleaned[:end])
	unit := strings.TrimSpace(cleaned[end:])
	multiplier := float64(1)
	switch unit {
	case "", "byte":
		multiplier = 1
	case "k", "kb":
		multiplier = 1000
	case "m", "mb":
		multiplier = 1000 * 1000
	case "g", "gb":
		multiplier = 1000 * 1000 * 1000
	case "t", "tb":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "ki":
		multiplier = 1024
	case "mi":
		multiplier = 1024 * 1024
	case "gi":
		multiplier = 1024 * 1024 * 1024
	case "ti":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return uint64(math.Round(number * multiplier))
}

func mustParseInt64(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func mustParseFloat(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return parsed
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// sanitizeID strips anything that isn't alphanumeric, dash, underscore, or dot
// to prevent command injection via container ID/name parameters.
func sanitizeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeImageRef(ref string) string {
	var b strings.Builder
	for _, r := range ref {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '@' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var pruneDeletedLinePattern = regexp.MustCompile(`^(sha256:[a-f0-9]{12,}|deleted:\s+.+|untagged:\s+.+)$`)

func parsePrunedImageCount(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.EqualFold(trimmed, "Deleted Images") || strings.HasPrefix(trimmed, "Total reclaimed space:") {
			continue
		}
		if pruneDeletedLinePattern.MatchString(strings.ToLower(trimmed)) {
			count++
		}
	}
	return count
}

// --- Raw Podman JSON types for parsing CLI output ---

type podmanPSEntry struct {
	ID       string            `json:"Id"`
	Names    json.RawMessage   `json:"Names"`
	Image    string            `json:"Image"`
	State    string            `json:"State"`
	Status   string            `json:"Status"`
	Created  interface{}       `json:"Created"`
	Ports    []podmanPort      `json:"Ports"`
	Mounts   json.RawMessage   `json:"Mounts"`
	Labels   map[string]string `json:"Labels"`
	Networks json.RawMessage   `json:"Networks"`
}

type podmanImageEntry struct {
	ID           string      `json:"Id"`
	Names        []string    `json:"Names"`
	Digest       string      `json:"Digest"`
	CreatedAt    string      `json:"CreatedAt"`
	CreatedSince string      `json:"CreatedSince"`
	Created      interface{} `json:"Created"`
	Size         int64       `json:"Size"`
}

func (p *podmanImageEntry) toImage() Image {
	created := parsePodmanTime(p.Created)
	if created.IsZero() {
		created = parsePodmanTime(p.CreatedAt)
	}

	repository := "<none>"
	tag := "<none>"
	if len(p.Names) > 0 && p.Names[0] != "" {
		parts := strings.SplitN(p.Names[0], ":", 2)
		repository = parts[0]
		if len(parts) > 1 {
			tag = parts[1]
		}
	}

	var sizeStr string
	if p.Size > 0 {
		sizeStr = formatBytes(uint64(p.Size))
	}

	return Image{
		ID:         p.ID,
		Repository: repository,
		Tag:        tag,
		Digest:     p.Digest,
		Created:    created,
		CreatedAgo: p.CreatedSince,
		Size:       sizeStr,
	}
}

type podmanPort struct {
	HostIP        string `json:"host_ip"`
	HostPort      uint16 `json:"host_port"`
	ContainerPort uint16 `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func (p *podmanPSEntry) toContainer(hostName string) Container {
	ct := Container{
		ID:     p.ID,
		Image:  p.Image,
		State:  strings.ToLower(p.State),
		Status: p.Status,
		Labels: p.Labels,
		Host:   hostName,
	}

	ct.Name = parseContainerName(p.Names)
	ct.Created = parseCreatedTime(p.Created)
	ct.Ports = parsePorts(p.Ports)
	ct.Networks = parseNetworkNames(p.Networks)
	ct.Mounts = parseMountStrings(p.Mounts)

	if unit, ok := p.Labels["PODMAN_SYSTEMD_UNIT"]; ok && unit != "" {
		ct.Manager = "quadlet"
		ct.SystemdUnit = unit
	} else if _, ok := p.Labels["com.docker.compose.project"]; ok {
		ct.Manager = "compose"
	} else {
		ct.Manager = "standalone"
	}

	return ct
}

func parseContainerName(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}

	var names []string
	if json.Unmarshal(raw, &names) == nil && len(names) > 0 {
		return names[0]
	}

	var name string
	if json.Unmarshal(raw, &name) == nil {
		return name
	}

	return string(raw)
}

func parseCreatedTime(raw interface{}) time.Time {
	switch v := raw.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return t
		}
		t, err = time.Parse(time.RFC3339Nano, v)
		if err == nil {
			return t
		}
	case float64:
		return time.Unix(int64(v), 0)
	}
	return time.Time{}
}

func parsePodmanTime(raw interface{}) time.Time {
	if t := parseCreatedTime(raw); !t.IsZero() {
		return t
	}

	v, ok := raw.(string)
	if !ok {
		return time.Time{}
	}

	value := strings.TrimSpace(v)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05 -0700 MST",
		time.UnixDate,
		time.RubyDate,
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}

	return time.Time{}
}

func parsePorts(raw []podmanPort) []PortMapping {
	if len(raw) == 0 {
		return nil
	}
	ports := make([]PortMapping, len(raw))
	for i, p := range raw {
		ports[i] = PortMapping{
			HostIP:        p.HostIP,
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		}
	}
	return ports
}

func parseNetworkNames(raw json.RawMessage) []NetworkInfo {
	if raw == nil {
		return nil
	}

	var names []string
	if json.Unmarshal(raw, &names) == nil {
		nets := make([]NetworkInfo, len(names))
		for i, n := range names {
			nets[i] = NetworkInfo{Name: n}
		}
		return nets
	}

	return nil
}

func parseMountStrings(raw json.RawMessage) []MountInfo {
	if raw == nil {
		return nil
	}

	var mounts []string
	if json.Unmarshal(raw, &mounts) == nil {
		infos := make([]MountInfo, len(mounts))
		for i, m := range mounts {
			infos[i] = parseSingleMount(m)
		}
		return infos
	}

	return nil
}

func parseSingleMount(s string) MountInfo {
	parts := strings.SplitN(s, ":", 3)
	info := MountInfo{Type: "bind", RW: true}
	if len(parts) >= 1 {
		info.Source = parts[0]
	}
	if len(parts) >= 2 {
		info.Destination = parts[1]
	}
	if len(parts) >= 3 && strings.Contains(parts[2], "ro") {
		info.RW = false
	}
	return info
}

// --- Inspect parsing ---

type podmanInspectEntry struct {
	ID        string `json:"Id"`
	Name      string `json:"Name"`
	Image     string `json:"Image"`
	ImageName string `json:"ImageName"`
	State     struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Pid        int    `json:"Pid"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
	} `json:"State"`
	Mounts []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
		Networks map[string]struct {
			IPAddress  string `json:"IPAddress"`
			Gateway    string `json:"Gateway"`
			MacAddress string `json:"MacAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
	Config struct {
		Hostname   string            `json:"Hostname"`
		Env        []string          `json:"Env"`
		Labels     map[string]string `json:"Labels"`
		Entrypoint json.RawMessage   `json:"Entrypoint"`
		Cmd        []string          `json:"Cmd"`
	} `json:"Config"`
	HostConfig struct {
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
		NetworkMode string `json:"NetworkMode"`
	} `json:"HostConfig"`
	Created string `json:"Created"`
}

func (r *podmanInspectEntry) toContainerDetail(hostName string) ContainerDetail {
	d := ContainerDetail{
		Container: Container{
			ID:     r.ID,
			Name:   strings.TrimPrefix(r.Name, "/"),
			Image:  r.ImageName,
			State:  strings.ToLower(r.State.Status),
			Status: r.State.Status,
			Labels: r.Config.Labels,
			Host:   hostName,
		},
		Hostname:      r.Config.Hostname,
		Env:           r.Config.Env,
		RestartPolicy: r.HostConfig.RestartPolicy.Name,
		NetworkMode:   r.HostConfig.NetworkMode,
		Pid:           r.State.Pid,
	}

	if r.Image == "" {
		d.Container.Image = r.Image
	}

	t, _ := time.Parse(time.RFC3339Nano, r.Created)
	d.Container.Created = t

	d.StartedAt, _ = time.Parse(time.RFC3339Nano, r.State.StartedAt)
	d.FinishedAt, _ = time.Parse(time.RFC3339Nano, r.State.FinishedAt)

	for _, m := range r.Mounts {
		d.Mounts = append(d.Mounts, MountInfo{
			Type:        m.Type,
			Source:      m.Source,
			Destination: m.Destination,
			RW:          m.RW,
		})
	}

	for name, net := range r.NetworkSettings.Networks {
		d.Networks = append(d.Networks, NetworkInfo{
			Name:    name,
			IP:      net.IPAddress,
			Gateway: net.Gateway,
			MAC:     net.MacAddress,
		})
	}

	for portSpec, bindings := range r.NetworkSettings.Ports {
		parts := strings.SplitN(portSpec, "/", 2)
		containerPort, _ := strconv.ParseUint(parts[0], 10, 16)
		protocol := "tcp"
		if len(parts) > 1 {
			protocol = parts[1]
		}
		for _, b := range bindings {
			hostPort, _ := strconv.ParseUint(b.HostPort, 10, 16)
			d.Ports = append(d.Ports, PortMapping{
				HostIP:        b.HostIP,
				HostPort:      uint16(hostPort),
				ContainerPort: uint16(containerPort),
				Protocol:      protocol,
			})
		}
	}

	return d
}
