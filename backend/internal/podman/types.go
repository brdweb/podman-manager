package podman

import (
	"fmt"
	"time"
)

type HostStatus struct {
	Name           string         `json:"name"`
	Address        string         `json:"address"`
	Status         string         `json:"status"` // "online", "offline", "error"
	Error          string         `json:"error,omitempty"`
	Latency        string         `json:"latency,omitempty"`
	Containers     []Container    `json:"containers,omitempty"`
	ContainerCount ContainerCount `json:"container_count"`
}

type ContainerCount struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Stopped int `json:"stopped"`
}

type Container struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Image    string            `json:"image"`
	State    string            `json:"state"`
	Status   string            `json:"status"`
	Created  time.Time         `json:"created"`
	Ports    []PortMapping     `json:"ports"`
	Networks []NetworkInfo     `json:"networks"`
	Mounts   []MountInfo       `json:"mounts"`
	Labels   map[string]string `json:"labels,omitempty"`
	Host     string            `json:"host"`
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
