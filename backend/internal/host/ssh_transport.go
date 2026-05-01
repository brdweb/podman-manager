package host

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/brdweb/podman-manager/internal/config"
	"github.com/brdweb/podman-manager/internal/podman"
)

type SSHTransport struct {
	name   string
	client *podman.Client
	pool   *podman.SSHPool
	logger *slog.Logger
}

func NewSSHTransport(name string, client *podman.Client, pool *podman.SSHPool, logger *slog.Logger) *SSHTransport {
	if logger == nil {
		logger = slog.Default()
	}
	return &SSHTransport{name: name, client: client, pool: pool, logger: logger}
}

func (t *SSHTransport) Name() string { return t.name }

func (t *SSHTransport) podmanCmd(host config.HostConfig) string {
	if host.IsRootful() {
		return "sudo podman"
	}
	return "podman"
}

func (t *SSHTransport) HostConfig() (config.HostConfig, bool) {
	return t.pool.HostConfig(t.name)
}

func (t *SSHTransport) ListContainers(ctx context.Context) ([]Container, error) {
	return t.client.ListContainers(ctx, t.name)
}

func (t *SSHTransport) InspectContainer(ctx context.Context, id string) (*ContainerDetail, error) {
	return t.client.InspectContainer(ctx, t.name, id)
}

func (t *SSHTransport) StartContainer(ctx context.Context, id string) (*ActionResult, error) {
	return t.client.StartContainer(ctx, t.name, id)
}

func (t *SSHTransport) StopContainer(ctx context.Context, id string) (*ActionResult, error) {
	return t.client.StopContainer(ctx, t.name, id)
}

func (t *SSHTransport) RestartContainer(ctx context.Context, id string) (*ActionResult, error) {
	return t.client.RestartContainer(ctx, t.name, id)
}

func (t *SSHTransport) RemoveContainer(ctx context.Context, id string, force bool) (*ActionResult, error) {
	return t.client.RemoveContainer(ctx, t.name, id, force)
}

func (t *SSHTransport) CreateContainer(ctx context.Context, req *CreateContainerRequest) (*ActionResult, error) {
	return nil, fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) CheckForUpdate(ctx context.Context, id string) (*UpdateCheckResult, error) {
	return t.client.CheckForUpdate(ctx, t.name, id)
}

func (t *SSHTransport) UpdateContainer(ctx context.Context, id string) (*UpdateResult, error) {
	return t.client.UpdateContainer(ctx, t.name, id)
}

func (t *SSHTransport) ContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	return t.client.ContainerLogs(ctx, t.name, id, tail)
}

func (t *SSHTransport) StreamLogs(ctx context.Context, id string, tail int, output chan<- string) error {
	return t.client.StreamLogs(ctx, t.name, id, tail, output)
}

func (t *SSHTransport) StreamEvents(ctx context.Context, output chan<- PodmanEvent) error {
	stream := podman.NewEventStream(t.pool, t.logger)
	if err := stream.Subscribe(t.name); err != nil {
		return err
	}
	defer stream.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-stream.Events():
			if !ok {
				return nil
			}
			select {
			case output <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (t *SSHTransport) ListImages(ctx context.Context) ([]Image, error) {
	return t.client.ListImages(ctx, t.name)
}

func (t *SSHTransport) PullImage(ctx context.Context, imageRef string) error {
	return t.client.PullImage(ctx, t.name, imageRef)
}

func (t *SSHTransport) RemoveImage(ctx context.Context, imageID string, force bool) error {
	return t.client.RemoveImage(ctx, t.name, imageID, force)
}

func (t *SSHTransport) PruneImages(ctx context.Context) (int, error) {
	return t.client.PruneImages(ctx, t.name)
}

func (t *SSHTransport) ListVolumes(ctx context.Context) ([]Volume, error) {
	return nil, fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error) {
	return nil, fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) RemoveVolume(ctx context.Context, name string, force bool) error {
	return fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) PruneVolumes(ctx context.Context) (int, error) {
	return 0, fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) ListNetworks(ctx context.Context) ([]Network, error) {
	conn, err := t.pool.GetConnection(t.name)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := t.pool.HostConfig(t.name)
	podmanCmd := t.podmanCmd(hostCfg)
	cmd := fmt.Sprintf("sh -lc %s", shellQuote(fmt.Sprintf("names=$(%s network ls --format '{{.Name}}'); if [ -z \"$names\" ]; then printf '[]'; else %s network inspect $names --format json; fi", podmanCmd, podmanCmd)))

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		t.logger.Error("network list command execution failed", "host", t.name, "command", cmd, "error", err)
		return nil, fmt.Errorf("listing networks on %s: %w", t.name, err)
	}
	if result.ExitCode != 0 {
		stderr := strings.TrimSpace(result.Stderr)
		t.logger.Error("network list command returned non-zero exit", "host", t.name, "command", cmd, "exit_code", result.ExitCode, "stderr", stderr)
		return nil, fmt.Errorf("podman network list failed on %s: %s", t.name, stderr)
	}

	stdout := strings.TrimSpace(result.Stdout)
	if stdout == "" || stdout == "[]" {
		return []Network{}, nil
	}

	var raw []podmanNetworkInspect
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("parsing network list from %s: %w", t.name, err)
	}

	networks := make([]Network, 0, len(raw))
	for _, entry := range raw {
		networks = append(networks, entry.toNetwork())
	}
	return networks, nil
}

func (t *SSHTransport) CreateNetwork(ctx context.Context, req *CreateNetworkRequest) (*Network, error) {
	if req == nil {
		return nil, fmt.Errorf("network create request is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("network name is required")
	}

	conn, err := t.pool.GetConnection(t.name)
	if err != nil {
		return nil, err
	}

	hostCfg, _ := t.pool.HostConfig(t.name)
	args := []string{t.podmanCmd(hostCfg), "network", "create"}
	if driver := strings.TrimSpace(req.Driver); driver != "" {
		args = append(args, "--driver", shellQuote(driver))
	}
	for _, subnet := range req.Subnets {
		if subnet = strings.TrimSpace(subnet); subnet != "" {
			args = append(args, "--subnet", shellQuote(subnet))
		}
	}
	if gateway := strings.TrimSpace(req.Gateway); gateway != "" {
		args = append(args, "--gateway", shellQuote(gateway))
	}
	if req.IPv6 {
		args = append(args, "--ipv6")
	}
	if req.Internal {
		args = append(args, "--internal")
	}
	for _, key := range sortedKeys(req.Labels) {
		args = append(args, "--label", shellQuote(fmt.Sprintf("%s=%s", key, req.Labels[key])))
	}
	for _, key := range sortedKeys(req.Options) {
		args = append(args, "--opt", shellQuote(fmt.Sprintf("%s=%s", key, req.Options[key])))
	}
	args = append(args, "--", shellQuote(name))
	cmd := strings.Join(args, " ")

	result, err := conn.Run(ctx, cmd)
	if err != nil {
		t.logger.Error("network create command execution failed", "host", t.name, "network", name, "command", cmd, "error", err)
		return nil, fmt.Errorf("creating network %s on %s: %w", name, t.name, err)
	}
	if result.ExitCode != 0 {
		stderr := strings.TrimSpace(result.Stderr)
		t.logger.Error("network create command returned non-zero exit", "host", t.name, "network", name, "command", cmd, "exit_code", result.ExitCode, "stderr", stderr)
		return nil, fmt.Errorf("podman network create failed on %s: %s", t.name, stderr)
	}

	return &Network{Name: name, Driver: req.Driver, Subnets: req.Subnets, Gateway: req.Gateway, IPv6: req.IPv6, Internal: req.Internal, DNSEnabled: req.DNSEnabled, Labels: req.Labels, Options: req.Options}, nil
}

func (t *SSHTransport) RemoveNetwork(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("network name is required")
	}

	conn, err := t.pool.GetConnection(t.name)
	if err != nil {
		return err
	}

	hostCfg, _ := t.pool.HostConfig(t.name)
	cmd := fmt.Sprintf("%s network rm -- %s", t.podmanCmd(hostCfg), shellQuote(name))
	result, err := conn.Run(ctx, cmd)
	if err != nil {
		t.logger.Error("network remove command execution failed", "host", t.name, "network", name, "command", cmd, "error", err)
		return fmt.Errorf("removing network %s on %s: %w", name, t.name, err)
	}
	if result.ExitCode != 0 {
		stderr := strings.TrimSpace(result.Stderr)
		t.logger.Error("network remove command returned non-zero exit", "host", t.name, "network", name, "command", cmd, "exit_code", result.ExitCode, "stderr", stderr)
		return fmt.Errorf("podman network rm failed on %s: %s", t.name, stderr)
	}
	return nil
}

func (t *SSHTransport) PruneNetworks(ctx context.Context) (int, error) {
	return 0, fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) ConnectNetwork(ctx context.Context, networkName, containerID string) error {
	return fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) DisconnectNetwork(ctx context.Context, networkName, containerID string) error {
	return fmt.Errorf("not supported in SSH transport")
}

func (t *SSHTransport) HostSystemInfo(ctx context.Context) (*HostSystemInfo, error) {
	return t.client.HostSystemInfo(ctx, t.name)
}

func (t *SSHTransport) Ping(ctx context.Context) (time.Duration, error) {
	return t.pool.Ping(t.name)
}

func (t *SSHTransport) Close() error {
	t.pool.Close()
	return nil
}

type podmanNetworkInspect struct {
	Name       string            `json:"name"`
	ID         string            `json:"id"`
	Driver     string            `json:"driver"`
	Interface  string            `json:"network_interface"`
	Created    time.Time         `json:"created"`
	Subnets    json.RawMessage   `json:"subnets"`
	Subnet     string            `json:"subnet"`
	Gateway    string            `json:"gateway"`
	IPv6       bool              `json:"ipv6_enabled"`
	Internal   bool              `json:"internal"`
	DNSEnabled bool              `json:"dns_enabled"`
	Labels     map[string]string `json:"labels"`
	Options    map[string]string `json:"options"`
	Plugins    []struct {
		Type string `json:"type"`
	} `json:"plugins"`
}

func (p podmanNetworkInspect) toNetwork() Network {
	subnets, gateway := parseNetworkSubnets(p.Subnets)
	if len(subnets) == 0 && strings.TrimSpace(p.Subnet) != "" {
		subnets = []string{strings.TrimSpace(p.Subnet)}
	}
	if gateway == "" {
		gateway = p.Gateway
	}
	driver := p.Driver
	if driver == "" && len(p.Plugins) > 0 {
		driver = p.Plugins[0].Type
	}
	return Network{
		Name:       p.Name,
		ID:         p.ID,
		Driver:     driver,
		Interface:  p.Interface,
		CreatedAt:  p.Created,
		Subnets:    subnets,
		Gateway:    gateway,
		IPv6:       p.IPv6,
		Internal:   p.Internal,
		DNSEnabled: p.DNSEnabled,
		Labels:     p.Labels,
		Options:    p.Options,
	}
}

type podmanNetworkSubnet struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

func parseNetworkSubnets(raw json.RawMessage) ([]string, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, ""
	}

	var stringsOnly []string
	if err := json.Unmarshal(raw, &stringsOnly); err == nil {
		return compactStrings(stringsOnly), ""
	}

	var objects []podmanNetworkSubnet
	if err := json.Unmarshal(raw, &objects); err != nil {
		return nil, ""
	}

	subnets := make([]string, 0, len(objects))
	gateway := ""
	for _, entry := range objects {
		if subnet := strings.TrimSpace(entry.Subnet); subnet != "" {
			subnets = append(subnets, subnet)
		}
		if gateway == "" {
			gateway = strings.TrimSpace(entry.Gateway)
		}
	}
	return subnets, gateway
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			compacted = append(compacted, trimmed)
		}
	}
	return compacted
}

func sortedKeys(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
