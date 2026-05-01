package host

import (
	"time"

	"github.com/brdweb/podman-manager/internal/podman"
)

type Container = podman.Container
type ContainerDetail = podman.ContainerDetail
type Image = podman.Image
type ActionResult = podman.ActionResult
type UpdateCheckResult = podman.UpdateCheckResult
type UpdateResult = podman.UpdateResult
type HostSystemInfo = podman.HostSystemInfo
type ContainerStats = podman.ContainerStats
type PortMapping = podman.PortMapping
type NetworkInfo = podman.NetworkInfo
type MountInfo = podman.MountInfo
type PodmanEvent = podman.PodmanEvent
type HostStatus = podman.HostStatus
type ContainerCount = podman.ContainerCount
type OverviewResponse = podman.OverviewResponse

type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"`
	Mountpoint string            `json:"mountpoint,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
	Scope      string            `json:"scope,omitempty"`
}

type CreateVolumeRequest struct {
	Name    string            `json:"name"`
	Driver  string            `json:"driver,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

type Network struct {
	Name       string            `json:"name"`
	ID         string            `json:"id,omitempty"`
	Driver     string            `json:"driver,omitempty"`
	Interface  string            `json:"interface,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
	Subnets    []string          `json:"subnets,omitempty"`
	Gateway    string            `json:"gateway,omitempty"`
	IPv6       bool              `json:"ipv6,omitempty"`
	Internal   bool              `json:"internal,omitempty"`
	DNSEnabled bool              `json:"dns_enabled,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

type CreateNetworkRequest struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"`
	Subnets    []string          `json:"subnets,omitempty"`
	Gateway    string            `json:"gateway,omitempty"`
	IPv6       bool              `json:"ipv6,omitempty"`
	Internal   bool              `json:"internal,omitempty"`
	DNSEnabled bool              `json:"dns_enabled,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

type CreateContainerRequest struct {
	Name    string            `json:"name,omitempty"`
	Image   string            `json:"image"`
	Command []string          `json:"command,omitempty"`
	Env     []string          `json:"env,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Ports   []PortMapping     `json:"ports,omitempty"`
	Mounts  []MountInfo       `json:"mounts,omitempty"`
	Network string            `json:"network,omitempty"`
}
