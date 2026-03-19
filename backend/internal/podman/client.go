package podman

import (
	"context"
	"encoding/json"
	"fmt"
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

	return containers, nil
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
