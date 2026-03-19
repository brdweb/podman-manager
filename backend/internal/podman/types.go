package podman

import (
	"fmt"
	"time"
)

type HostStatus struct {
	Name           string          `json:"name"`
	Address        string          `json:"address"`
	Mode           string          `json:"mode,omitempty"`
	Status         string          `json:"status"` // "online", "offline", "error"
	Error          string          `json:"error,omitempty"`
	Latency        string          `json:"latency,omitempty"`
	System         *HostSystemInfo `json:"system,omitempty"`
	Containers     []Container     `json:"containers,omitempty"`
	ContainerCount ContainerCount  `json:"container_count"`
}

type ContainerCount struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Stopped int `json:"stopped"`
}

type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Created     time.Time         `json:"created"`
	Ports       []PortMapping     `json:"ports"`
	Networks    []NetworkInfo     `json:"networks"`
	Mounts      []MountInfo       `json:"mounts"`
	Labels      map[string]string `json:"labels,omitempty"`
	Host        string            `json:"host"`
	Manager     string            `json:"manager"`
	SystemdUnit string            `json:"systemd_unit,omitempty"`
	Stats       *ContainerStats   `json:"stats,omitempty"`
}

type PortMapping struct {
	HostIP        string `json:"host_ip"`
	HostPort      uint16 `json:"host_port"`
	ContainerPort uint16 `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func (p PortMapping) HostBinding() string {
	if p.HostIP == "" || p.HostIP == "0.0.0.0" {
		return fmt.Sprintf("%d", p.HostPort)
	}
	return fmt.Sprintf("%s:%d", p.HostIP, p.HostPort)
}

type NetworkInfo struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Gateway string `json:"gateway"`
	MAC     string `json:"mac,omitempty"`
}

type MountInfo struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	RW          bool   `json:"rw"`
}

type ContainerDetail struct {
	Container
	Env           []string  `json:"env,omitempty"`
	Hostname      string    `json:"hostname"`
	RestartPolicy string    `json:"restart_policy"`
	NetworkMode   string    `json:"network_mode"`
	Pid           int       `json:"pid"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
}

type ActionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type OverviewResponse struct {
	Hosts []HostStatus `json:"hosts"`
}

type HostSystemInfo struct {
	Hostname         string  `json:"hostname,omitempty"`
	OS               string  `json:"os,omitempty"`
	Kernel           string  `json:"kernel,omitempty"`
	UptimeSeconds    int64   `json:"uptime_seconds,omitempty"`
	CPUCores         int     `json:"cpu_cores,omitempty"`
	Load1            float64 `json:"load_1,omitempty"`
	Load5            float64 `json:"load_5,omitempty"`
	Load15           float64 `json:"load_15,omitempty"`
	MemoryTotalBytes uint64  `json:"memory_total_bytes,omitempty"`
	MemoryUsedBytes  uint64  `json:"memory_used_bytes,omitempty"`
	DiskTotalBytes   uint64  `json:"disk_total_bytes,omitempty"`
	DiskUsedBytes    uint64  `json:"disk_used_bytes,omitempty"`
	DiskFreeBytes    uint64  `json:"disk_free_bytes,omitempty"`
}

type ContainerStats struct {
	CPUPercent         float64 `json:"cpu_percent,omitempty"`
	MemoryUsageBytes   uint64  `json:"memory_usage_bytes,omitempty"`
	MemoryLimitBytes   uint64  `json:"memory_limit_bytes,omitempty"`
	MemoryPercent      float64 `json:"memory_percent,omitempty"`
	PIDs               int     `json:"pids,omitempty"`
	NetworkInputBytes  uint64  `json:"network_input_bytes,omitempty"`
	NetworkOutputBytes uint64  `json:"network_output_bytes,omitempty"`
	BlockInputBytes    uint64  `json:"block_input_bytes,omitempty"`
	BlockOutputBytes   uint64  `json:"block_output_bytes,omitempty"`
}
