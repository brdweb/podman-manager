package host

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/brdweb/podman-manager/internal/config"
)

type HostManager struct {
	mu         sync.RWMutex
	transports map[string]Transport
}

func NewHostManager() *HostManager {
	return &HostManager{transports: make(map[string]Transport)}
}

func (m *HostManager) Register(name string, t Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transports[name] = t
}

func (m *HostManager) Get(name string) (Transport, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.transports[name]
	return t, ok
}

func (m *HostManager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.transports))
	for name := range m.transports {
		names = append(names, name)
	}
	return names
}

func (m *HostManager) Overview(ctx context.Context) *OverviewResponse {
	names := m.Names()
	resp := &OverviewResponse{Hosts: make([]HostStatus, len(names))}

	type result struct {
		index  int
		status HostStatus
	}
	ch := make(chan result, len(names))

	for i, name := range names {
		go func(idx int, hostName string) {
			status := m.hostStatus(ctx, hostName)
			ch <- result{index: idx, status: status}
		}(i, name)
	}

	for range names {
		r := <-ch
		resp.Hosts[r.index] = r.status
	}

	return resp
}

func (m *HostManager) Close() error {
	m.mu.Lock()
	transports := make([]Transport, 0, len(m.transports))
	for name, t := range m.transports {
		delete(m.transports, name)
		transports = append(transports, t)
	}
	m.mu.Unlock()

	var closeErr error
	for _, t := range transports {
		if err := t.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (m *HostManager) hostStatus(ctx context.Context, name string) HostStatus {
	t, ok := m.Get(name)
	if !ok {
		return HostStatus{Name: name, Status: "offline", Error: fmt.Sprintf("unknown host: %s", name)}
	}

	hs := HostStatus{Name: name}
	if provider, ok := t.(interface {
		HostConfig() (config.HostConfig, bool)
	}); ok {
		if cfg, ok := provider.HostConfig(); ok {
			hs.Address = cfg.Address
			hs.Mode = cfg.Mode
		}
	}

	latency, err := t.Ping(ctx)
	if err != nil {
		hs.Status = "offline"
		hs.Error = err.Error()
		return hs
	}
	hs.Latency = latency.Round(time.Millisecond).String()

	containers, err := t.ListContainers(ctx)
	if err != nil {
		hs.Status = "error"
		hs.Error = err.Error()
		return hs
	}

	hs.Status = "online"
	if info, err := t.HostSystemInfo(ctx); err == nil {
		hs.System = info
	} else {
		hs.Error = fmt.Sprintf("system info: %v", err)
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
