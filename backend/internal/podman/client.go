package podman

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/brdweb/podman-manager/internal/config"
)

type Client struct {
	pool *SSHPool
}

func NewClient(pool *SSHPool) *Client {
	return &Client{pool: pool}
}

func (c *Client) podmanCmd(host config.HostConfig) string {
	if host.IsRootful() {
		return "sudo podman"
	}
	return "podman"
}

func (c *Client) ListContainers(ctx context.Context, hostName string) ([]Container, error) {
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf("%s ps -a --format json", c.podmanCmd(hostCfg))

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("listing containers on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
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
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := c.pool.HostConfig(hostName)
	cmd := fmt.Sprintf(`%s stats --all --no-stream --format "{{json .}}"`, c.podmanCmd(hostCfg))

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("getting container stats on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
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
	conn, err := c.pool.GetConnection(hostName)
	if err != nil {
		return nil, err
	}

	cmd := `sh -lc 'hostname=$(hostname 2>/dev/null || true); ` +
		`os=$(awk -F= '\''/^PRETTY_NAME=/{gsub(/"/,"",$2); print $2}'\'' /etc/os-release 2>/dev/null || true); ` +
		`kernel=$(uname -sr 2>/dev/null || true); ` +
		`uptime=$(cut -d. -f1 /proc/uptime 2>/dev/null || echo 0); ` +
		`mem_total=$(awk '\''/MemTotal/ {print $2}'\'' /proc/meminfo 2>/dev/null || echo 0); ` +
		`mem_avail=$(awk '\''/MemAvailable/ {print $2}'\'' /proc/meminfo 2>/dev/null || echo 0); ` +
		`load=$(cut -d\" \" -f1-3 /proc/loadavg 2>/dev/null || echo \"0 0 0\"); ` +
		`cores=$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null || echo 0); ` +
		`disk=$(df -B1 / 2>/dev/null | awk '\''NR==2 {print $2\"|\"$3\"|\"$4}'\'' || true); ` +
		`printf \"hostname=%s\nos=%s\nkernel=%s\nuptime=%s\nmem_total_kb=%s\nmem_avail_kb=%s\nload=%s\ncpu_cores=%s\ndisk=%s\n\" \"$hostname\" \"$os\" \"$kernel\" \"$uptime\" \"$mem_total\" \"$mem_avail\" \"$load\" \"$cores\" \"$disk\"'`

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("getting system info on %s: %w", hostName, err)
	}
	if result.ExitCode != 0 {
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
		return nil, fmt.Errorf("inspecting container %s on %s: %w", safeID, hostName, err)
	}
	if result.ExitCode != 0 {
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
		return &ActionResult{Success: false, Error: err.Error()}, nil
	}

	if result.ExitCode != 0 {
		return &ActionResult{
			Success: false,
			Error:   strings.TrimSpace(result.Stderr),
		}, nil
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
		return ""
	}

	unit := strings.TrimSpace(result.Stdout)
	if unit == "" || unit == "<no value>" {
		return ""
	}
	return unit
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
		return "", fmt.Errorf("getting logs for %s on %s: %w", safeID, hostName, err)
	}

	output := result.Stdout
	if output == "" {
		output = result.Stderr
	}
	return output, nil
}

func (c *Client) Overview(ctx context.Context) *OverviewResponse {
	hostNames := c.pool.HostNames()
	resp := &OverviewResponse{
		Hosts: make([]HostStatus, len(hostNames)),
	}

	type result struct {
		index  int
		status HostStatus
	}
	ch := make(chan result, len(hostNames))

	for i, name := range hostNames {
		go func(idx int, hostName string) {
			hs := c.hostStatus(ctx, hostName)
			ch <- result{index: idx, status: hs}
		}(i, name)
	}

	for range hostNames {
		r := <-ch
		resp.Hosts[r.index] = r.status
	}

	return resp
}

func (c *Client) hostStatus(ctx context.Context, hostName string) HostStatus {
	hostCfg, _ := c.pool.HostConfig(hostName)
	hs := HostStatus{
		Name:    hostName,
		Address: hostCfg.Address,
		Mode:    hostCfg.Mode,
	}

	latency, err := c.pool.Ping(hostName)
	if err != nil {
		hs.Status = "offline"
		hs.Error = err.Error()
		return hs
	}
	hs.Latency = latency.Round(time.Millisecond).String()

	containers, err := c.ListContainers(ctx, hostName)
	if err != nil {
		hs.Status = "error"
		hs.Error = err.Error()
		return hs
	}

	hs.Status = "online"
	if info, err := c.HostSystemInfo(ctx, hostName); err == nil {
		hs.System = info
	}
	hs.Containers = containers
	hs.ContainerCount.Total = len(containers)
	for _, ct := range containers {
		if ct.State == "running" {
			hs.ContainerCount.Running++
		} else {
			hs.ContainerCount.Stopped++
		}
	}

	return hs
}

func (c *Client) HostNames() []string {
	return c.pool.HostNames()
}

func (c *Client) Pool() *SSHPool {
	return c.pool
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
		Hostname string            `json:"Hostname"`
		Env      []string          `json:"Env"`
		Labels   map[string]string `json:"Labels"`
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
