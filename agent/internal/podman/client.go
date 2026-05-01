package podman

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const apiBasePath = "/v5.0.0"

// Client communicates with Podman via its Unix socket REST API.
type Client struct {
	httpClient *http.Client
	socketPath string
	timeout    time.Duration
}

// NewClient creates a Podman client. If socketPath is empty, auto-detects:
// 1. $PODMAN_SOCKET environment variable
// 2. /run/podman/podman.sock (rootful)
// 3. $XDG_RUNTIME_DIR/podman/podman.sock (rootless)
func NewClient(socketPath string, timeout time.Duration) (*Client, error) {
	if socketPath == "" {
		detected, err := detectSocketPath()
		if err != nil {
			return nil, err
		}
		socketPath = detected
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &Client{
		httpClient: &http.Client{Transport: transport, Timeout: timeout},
		socketPath: socketPath,
		timeout:    timeout,
	}, nil
}

// detectSocketPath auto-detects the Podman socket path.
func detectSocketPath() (string, error) {
	candidates := make([]string, 0, 3)
	if socketPath := os.Getenv("PODMAN_SOCKET"); socketPath != "" {
		candidates = append(candidates, socketPath)
	}
	candidates = append(candidates, "/run/podman/podman.sock")
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates, filepath.Join(runtimeDir, "podman", "podman.sock"))
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("podman socket not found; checked %s", strings.Join(candidates, ", "))
}

func (c *Client) ListContainers(ctx context.Context, all bool) ([]Container, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/containers/json?all="+boolParam(all), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var containers []Container
	if err := decodeJSON(resp, &containers); err != nil {
		return nil, err
	}
	return containers, nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*ContainerDetail, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/containers/"+pathSegment(id)+"/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var detail ContainerDetail
	if err := decodeJSON(resp, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// IsContainerRunning checks if a container with the given name or ID is running.
func (c *Client) IsContainerRunning(ctx context.Context, nameOrID string) (bool, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/containers/"+pathSegment(nameOrID)+"/json", nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		return false, nil
	}

	var detail ContainerDetail
	if err := decodeJSON(resp, &detail); err != nil {
		return false, err
	}
	return detail.State.Running || strings.EqualFold(detail.State.Status, "running") || strings.EqualFold(detail.Container.State, "running"), nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.doNoContent(ctx, http.MethodPost, apiBasePath+"/libpod/containers/"+pathSegment(id)+"/start", nil)
}

func (c *Client) StopContainer(ctx context.Context, id string, timeout int) error {
	return c.doNoContent(ctx, http.MethodPost, apiBasePath+"/libpod/containers/"+pathSegment(id)+"/stop?t="+intParam(timeout), nil)
}

func (c *Client) RestartContainer(ctx context.Context, id string, timeout int) error {
	return c.doNoContent(ctx, http.MethodPost, apiBasePath+"/libpod/containers/"+pathSegment(id)+"/restart?t="+intParam(timeout), nil)
}

func (c *Client) RemoveContainer(ctx context.Context, id string, force, volumes bool) error {
	query := url.Values{}
	query.Set("force", boolParam(force))
	query.Set("volumes", boolParam(volumes))
	return c.doNoContent(ctx, http.MethodDelete, apiBasePath+"/libpod/containers/"+pathSegment(id)+"?"+query.Encode(), nil)
}

func (c *Client) CreateContainer(ctx context.Context, spec *CreateSpec) (*CreateResult, error) {
	body, err := jsonBody(spec)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, http.MethodPost, apiBasePath+"/libpod/containers/create", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CreateResult
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ContainerLogs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	resp, err := c.do(ctx, http.MethodGet, containerLogsPath(id, opts, false), nil)
	if err != nil {
		return nil, err
	}
	if err := decodeJSON(resp, nil); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) StreamContainerLogs(ctx context.Context, id string, opts LogOptions, output chan<- string) error {
	resp, err := c.doStream(ctx, http.MethodGet, containerLogsPath(id, opts, true), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := decodeJSON(resp, nil); err != nil {
		return err
	}

	return streamLines(ctx, resp.Body, output)
}

func (c *Client) ListImages(ctx context.Context, all bool) ([]Image, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/images/json?all="+boolParam(all), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var images []Image
	if err := decodeJSON(resp, &images); err != nil {
		return nil, err
	}
	return images, nil
}

func (c *Client) PullImage(ctx context.Context, reference string) error {
	query := url.Values{}
	query.Set("reference", reference)
	return c.doNoContent(ctx, http.MethodPost, apiBasePath+"/libpod/images/pull?"+query.Encode(), nil)
}

func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	return c.doNoContent(ctx, http.MethodDelete, apiBasePath+"/libpod/images/"+pathSegment(id)+"?force="+boolParam(force), nil)
}

func (c *Client) PruneImages(ctx context.Context, all bool) (*PruneResult, error) {
	resp, err := c.do(ctx, http.MethodPost, apiBasePath+"/libpod/images/prune?all="+boolParam(all), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodePruneResult(resp)
}

func (c *Client) InspectImage(ctx context.Context, name string) (*ImageDetail, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/images/"+pathSegment(name)+"/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var detail ImageDetail
	if err := decodeJSON(resp, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/volumes/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result volumeListResult
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return result.Volumes, nil
}

func (c *Client) CreateVolume(ctx context.Context, spec *VolumeCreateSpec) (*Volume, error) {
	body, err := jsonBody(spec)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, http.MethodPost, apiBasePath+"/libpod/volumes/create", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var volume Volume
	if err := decodeJSON(resp, &volume); err != nil {
		return nil, err
	}
	return &volume, nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	return c.doNoContent(ctx, http.MethodDelete, apiBasePath+"/libpod/volumes/"+pathSegment(name)+"?force="+boolParam(force), nil)
}

func (c *Client) PruneVolumes(ctx context.Context) (*PruneResult, error) {
	resp, err := c.do(ctx, http.MethodPost, apiBasePath+"/libpod/volumes/prune", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodePruneResult(resp)
}

func (c *Client) InspectVolume(ctx context.Context, name string) (*Volume, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/volumes/"+pathSegment(name)+"/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var volume Volume
	if err := decodeJSON(resp, &volume); err != nil {
		return nil, err
	}
	return &volume, nil
}

func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/networks/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var networks []Network
	if err := decodeJSON(resp, &networks); err != nil {
		return nil, err
	}
	return networks, nil
}

func (c *Client) CreateNetwork(ctx context.Context, spec *NetworkCreateSpec) (*NetworkCreateResult, error) {
	body, err := jsonBody(spec)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, http.MethodPost, apiBasePath+"/libpod/networks/create", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result NetworkCreateResult
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) RemoveNetwork(ctx context.Context, name string) error {
	return c.doNoContent(ctx, http.MethodDelete, apiBasePath+"/libpod/networks/"+pathSegment(name), nil)
}

func (c *Client) PruneNetworks(ctx context.Context) (*PruneResult, error) {
	resp, err := c.do(ctx, http.MethodPost, apiBasePath+"/libpod/networks/prune", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodePruneResult(resp)
}

func (c *Client) InspectNetwork(ctx context.Context, name string) (*Network, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/networks/"+pathSegment(name)+"/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var network Network
	if err := decodeJSON(resp, &network); err != nil {
		return nil, err
	}
	return &network, nil
}

func (c *Client) ConnectNetwork(ctx context.Context, networkName, containerID string) error {
	body, err := jsonBody(networkConnectRequest{Container: containerID})
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, http.MethodPost, apiBasePath+"/libpod/networks/"+pathSegment(networkName)+"/connect", body)
}

func (c *Client) DisconnectNetwork(ctx context.Context, networkName, containerID string, force bool) error {
	body, err := jsonBody(networkDisconnectRequest{Container: containerID, Force: force})
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, http.MethodPost, apiBasePath+"/libpod/networks/"+pathSegment(networkName)+"/disconnect", body)
}

func (c *Client) SystemInfo(ctx context.Context) (*SystemInfo, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/info", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info SystemInfo
	if err := decodeJSON(resp, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) Version(ctx context.Context) (*Version, error) {
	resp, err := c.do(ctx, http.MethodGet, apiBasePath+"/libpod/version", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var version Version
	if err := decodeJSON(resp, &version); err != nil {
		return nil, err
	}
	return &version, nil
}

func (c *Client) Events(ctx context.Context, since time.Time, output chan<- string) error {
	query := url.Values{}
	query.Set("stream", "true")
	if !since.IsZero() {
		query.Set("since", since.Format(time.RFC3339))
	}

	resp, err := c.doStream(ctx, http.MethodGet, apiBasePath+"/events?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := decodeJSON(resp, nil); err != nil {
		return err
	}

	return streamLines(ctx, resp.Body, output)
}

// do makes an HTTP request over the configured Unix socket.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("podman client is not initialized")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	req, err := http.NewRequestWithContext(ctx, method, "http://podman"+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podman API %s %s: %w", method, path, err)
	}
	return resp, nil
}

func (c *Client) doStream(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("podman client is not initialized")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	req, err := http.NewRequestWithContext(ctx, method, "http://podman"+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	streamClient := *c.httpClient
	streamClient.Timeout = 0
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podman API %s %s: %w", method, path, err)
	}
	return resp, nil
}

// decodeJSON decodes a JSON response and handles error status codes.
func decodeJSON(resp *http.Response, v interface{}) error {
	if resp == nil {
		return fmt.Errorf("podman API returned nil response")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer io.Copy(io.Discard, resp.Body)
		var apiErr podmanError
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("podman API returned %s and response body could not be read: %w", resp.Status, err)
		}
		if len(data) > 0 && json.Unmarshal(data, &apiErr) == nil {
			if apiErr.Message != "" {
				return fmt.Errorf("podman API returned %s: %s", resp.Status, apiErr.Message)
			}
			if apiErr.Cause != "" {
				return fmt.Errorf("podman API returned %s: %s", resp.Status, apiErr.Cause)
			}
		}
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("podman API returned %s: %s", resp.Status, message)
	}

	if v == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding podman API response: %w", err)
	}
	return nil
}

func (c *Client) doNoContent(ctx context.Context, method, path string, body io.Reader) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeJSON(resp, nil)
}

func jsonBody(v interface{}) (io.Reader, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(string(data)), nil
}

func pathSegment(value string) string {
	return url.PathEscape(value)
}

func boolParam(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func intParam(value int) string {
	return fmt.Sprintf("%d", value)
}

func containerLogsPath(id string, opts LogOptions, follow bool) string {
	query := url.Values{}
	query.Set("follow", boolParam(follow))
	query.Set("stdout", boolParam(defaultTrue(opts.Stdout)))
	query.Set("stderr", boolParam(defaultTrue(opts.Stderr)))
	if opts.Tail >= 0 {
		query.Set("tail", intParam(opts.Tail))
	}
	if !opts.Since.IsZero() {
		query.Set("since", opts.Since.Format(time.RFC3339))
	}
	if !opts.Until.IsZero() {
		query.Set("until", opts.Until.Format(time.RFC3339))
	}
	return apiBasePath + "/libpod/containers/" + pathSegment(id) + "/logs?" + query.Encode()
}

func defaultTrue(value bool) bool {
	if value {
		return true
	}
	return true
}

func streamLines(ctx context.Context, reader io.Reader, output chan<- string) error {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case output <- line:
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading podman stream: %w", err)
	}
	return nil
}

func decodePruneResult(resp *http.Response) (*PruneResult, error) {
	var raw interface{}
	if err := decodeJSON(resp, &raw); err != nil {
		return nil, err
	}

	result := &PruneResult{}
	collectPruneResult(raw, result)
	return result, nil
}

func collectPruneResult(value interface{}, result *PruneResult) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			collectPruneResult(item, result)
		}
	case map[string]interface{}:
		for key, item := range typed {
			switch strings.ToLower(key) {
			case "deleted", "imagesdeleted", "containersdeleted", "volumesdeleted", "networksdeleted":
				appendDeleted(item, result)
			case "id", "name":
				if text, ok := item.(string); ok && text != "" {
					result.Deleted = append(result.Deleted, text)
				}
			case "spacereclaimed", "space_reclaimed", "sizereclaimed":
				result.SpaceReclaimed += numberAsInt64(item)
			default:
				collectPruneResult(item, result)
			}
		}
	}
}

func appendDeleted(value interface{}, result *PruneResult) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			appendDeleted(item, result)
		}
	case string:
		if typed != "" {
			result.Deleted = append(result.Deleted, typed)
		}
	case map[string]interface{}:
		collectPruneResult(typed, result)
	}
}

func numberAsInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		n, _ := typed.Int64()
		return n
	default:
		return 0
	}
}

type podmanError struct {
	Cause    string `json:"cause"`
	Message  string `json:"message"`
	Response int    `json:"response"`
}

type volumeListResult struct {
	Volumes []Volume `json:"Volumes"`
}

type networkConnectRequest struct {
	Container string `json:"container"`
}

type networkDisconnectRequest struct {
	Container string `json:"container"`
	Force     bool   `json:"force"`
}

type Container struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Created string            `json:"Created"`
	Ports   []PortMapping     `json:"Ports"`
	Labels  map[string]string `json:"Labels"`
	Command []string          `json:"Command,omitempty"`
	Mounts  []Mount           `json:"Mounts,omitempty"`
}

type ContainerDetail struct {
	Container
	Config          ContainerConfig `json:"Config"`
	HostConfig      HostConfig      `json:"HostConfig"`
	State           ContainerState  `json:"State,omitempty"`
	Mounts          []Mount         `json:"Mounts,omitempty"`
	NetworkSettings NetworkSettings `json:"NetworkSettings,omitempty"`
}

type ContainerConfig struct {
	Hostname string            `json:"Hostname,omitempty"`
	Image    string            `json:"Image,omitempty"`
	Env      []string          `json:"Env,omitempty"`
	Cmd      []string          `json:"Cmd,omitempty"`
	Labels   map[string]string `json:"Labels,omitempty"`
}

type HostConfig struct {
	NetworkMode  string               `json:"NetworkMode,omitempty"`
	PortBindings map[string][]Binding `json:"PortBindings,omitempty"`
	Binds        []string             `json:"Binds,omitempty"`
}

type ContainerState struct {
	Status     string `json:"Status,omitempty"`
	Running    bool   `json:"Running,omitempty"`
	Paused     bool   `json:"Paused,omitempty"`
	Pid        int    `json:"Pid,omitempty"`
	StartedAt  string `json:"StartedAt,omitempty"`
	FinishedAt string `json:"FinishedAt,omitempty"`
}

type CreateSpec struct {
	Name          string                    `json:"name,omitempty"`
	Image         string                    `json:"image,omitempty"`
	Command       []string                  `json:"command,omitempty"`
	Env           []string                  `json:"env,omitempty"`
	Labels        map[string]string         `json:"labels,omitempty"`
	Hostname      string                    `json:"hostname,omitempty"`
	PortMappings  []PortMapping             `json:"portmappings,omitempty"`
	Mounts        []Mount                   `json:"mounts,omitempty"`
	Networks      map[string]NetworkOptions `json:"networks,omitempty"`
	RestartPolicy string                    `json:"restart_policy,omitempty"`
}

type CreateResult struct {
	ID       string   `json:"Id"`
	Warnings []string `json:"Warnings"`
}

type Image struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Digest  string   `json:"Digest"`
	Created string   `json:"Created"`
	Size    int64    `json:"Size"`
}

type ImageDetail struct {
	ID           string            `json:"Id"`
	RepoTags     []string          `json:"RepoTags"`
	RepoDigests  []string          `json:"RepoDigests"`
	Digest       string            `json:"Digest"`
	Created      string            `json:"Created"`
	Size         int64             `json:"Size"`
	Labels       map[string]string `json:"Labels"`
	Architecture string            `json:"Architecture,omitempty"`
	OS           string            `json:"Os,omitempty"`
}

type Volume struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	Labels     map[string]string `json:"Labels"`
	Options    map[string]string `json:"Options"`
}

type VolumeCreateSpec struct {
	Name    string            `json:"Name"`
	Driver  string            `json:"Driver"`
	Labels  map[string]string `json:"Labels"`
	Options map[string]string `json:"Options"`
}

type Network struct {
	Name        string            `json:"name"`
	Driver      string            `json:"driver"`
	Subnet      string            `json:"subnets"`
	Gateway     string            `json:"network_interface"`
	Labels      map[string]string `json:"labels"`
	IPv6Enabled bool              `json:"ipv6_enabled"`
}

type NetworkCreateSpec struct {
	Name        string            `json:"name"`
	Driver      string            `json:"driver"`
	Subnet      string            `json:"subnet,omitempty"`
	Gateway     string            `json:"gateway,omitempty"`
	IPv6Enabled bool              `json:"ipv6_enabled,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Options     map[string]string `json:"options,omitempty"`
}

type NetworkCreateResult struct {
	Name     string   `json:"name"`
	Warnings []string `json:"warnings"`
}

type SystemInfo struct {
	Host    HostInfo    `json:"host"`
	Store   StoreInfo   `json:"store"`
	Version VersionInfo `json:"version"`
}

type HostInfo struct {
	Arch           string            `json:"arch,omitempty"`
	BuildahVersion string            `json:"buildahVersion,omitempty"`
	CgroupVersion  string            `json:"cgroupVersion,omitempty"`
	ConmonVersion  string            `json:"conmonVersion,omitempty"`
	Distribution   map[string]string `json:"distribution,omitempty"`
	Hostname       string            `json:"hostname,omitempty"`
	Kernel         string            `json:"kernel,omitempty"`
	MemFree        int64             `json:"memFree,omitempty"`
	MemTotal       int64             `json:"memTotal,omitempty"`
	OS             string            `json:"os,omitempty"`
	Uptime         string            `json:"uptime,omitempty"`
}

type StoreInfo struct {
	ConfigFile      string     `json:"configFile,omitempty"`
	ContainerStore  StoreCount `json:"containerStore,omitempty"`
	GraphDriverName string     `json:"graphDriverName,omitempty"`
	GraphRoot       string     `json:"graphRoot,omitempty"`
	ImageStore      StoreCount `json:"imageStore,omitempty"`
	RunRoot         string     `json:"runRoot,omitempty"`
}

type StoreCount struct {
	Number int `json:"number,omitempty"`
}

type VersionInfo struct {
	Version    string `json:"Version"`
	APIVersion string `json:"APIVersion"`
	GoVersion  string `json:"GoVersion"`
	GitCommit  string `json:"GitCommit"`
	BuiltTime  string `json:"BuiltTime"`
	Built      int64  `json:"Built"`
	OsArch     string `json:"OsArch"`
	Os         string `json:"Os"`
}

type Version struct {
	VersionInfo
}

type PruneResult struct {
	Deleted        []string `json:"deleted"`
	SpaceReclaimed int64    `json:"space_reclaimed"`
}

type PortMapping struct {
	HostIP        string `json:"host_ip"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type Binding struct {
	HostIP   string `json:"HostIp,omitempty"`
	HostPort string `json:"HostPort,omitempty"`
}

type Mount struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

type NetworkOptions struct {
	Aliases  []string `json:"aliases,omitempty"`
	StaticIP string   `json:"static_ip,omitempty"`
}

type NetworkSettings struct {
	IPAddress string                     `json:"IPAddress,omitempty"`
	Networks  map[string]NetworkEndpoint `json:"Networks,omitempty"`
}

type NetworkEndpoint struct {
	IPAddress  string `json:"IPAddress,omitempty"`
	Gateway    string `json:"Gateway,omitempty"`
	MacAddress string `json:"MacAddress,omitempty"`
}

type LogOptions struct {
	Follow bool
	Stdout bool
	Stderr bool
	Tail   int
	Since  time.Time
	Until  time.Time
}
