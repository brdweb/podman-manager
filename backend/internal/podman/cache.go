package podman

import (
	"context"
	"sync"
	"time"
)

const defaultCacheTTL = 3 * time.Second

type Cache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	hosts map[string]*cachedHostData
}

type cachedHostData struct {
	containers   []Container
	containersAt time.Time

	stats   map[string]*ContainerStats
	statsAt time.Time

	systemInfo   *HostSystemInfo
	systemInfoAt time.Time
}

func NewCache(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	return &Cache{
		ttl:   ttl,
		hosts: make(map[string]*cachedHostData),
	}
}

func (c *Cache) GetContainers(ctx context.Context, host string, fetch func(context.Context, string) ([]Container, error)) ([]Container, error) {
	now := time.Now()

	c.mu.RLock()
	entry, ok := c.hosts[host]
	if ok && !entry.containersAt.IsZero() && now.Sub(entry.containersAt) < c.ttl {
		containers := cloneContainers(entry.containers)
		c.mu.RUnlock()
		return containers, nil
	}
	c.mu.RUnlock()

	containers, err := fetch(ctx, host)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	entry = c.getOrCreateHost(host)
	entry.containers = cloneContainers(containers)
	entry.containersAt = time.Now()
	c.mu.Unlock()

	return containers, nil
}

func (c *Cache) GetStats(ctx context.Context, host string, fetch func(context.Context, string) (map[string]*ContainerStats, error)) (map[string]*ContainerStats, error) {
	now := time.Now()

	c.mu.RLock()
	entry, ok := c.hosts[host]
	if ok && !entry.statsAt.IsZero() && now.Sub(entry.statsAt) < c.ttl {
		stats := cloneStatsMap(entry.stats)
		c.mu.RUnlock()
		return stats, nil
	}
	c.mu.RUnlock()

	stats, err := fetch(ctx, host)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	entry = c.getOrCreateHost(host)
	entry.stats = cloneStatsMap(stats)
	entry.statsAt = time.Now()
	c.mu.Unlock()

	return stats, nil
}

func (c *Cache) GetSystemInfo(ctx context.Context, host string, fetch func(context.Context, string) (*HostSystemInfo, error)) (*HostSystemInfo, error) {
	now := time.Now()

	c.mu.RLock()
	entry, ok := c.hosts[host]
	if ok && !entry.systemInfoAt.IsZero() && now.Sub(entry.systemInfoAt) < c.ttl {
		info := cloneSystemInfo(entry.systemInfo)
		c.mu.RUnlock()
		return info, nil
	}
	c.mu.RUnlock()

	info, err := fetch(ctx, host)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	entry = c.getOrCreateHost(host)
	entry.systemInfo = cloneSystemInfo(info)
	entry.systemInfoAt = time.Now()
	c.mu.Unlock()

	return info, nil
}

func (c *Cache) Invalidate(host string) {
	c.mu.Lock()
	delete(c.hosts, host)
	c.mu.Unlock()
}

func (c *Cache) getOrCreateHost(host string) *cachedHostData {
	entry, ok := c.hosts[host]
	if ok {
		return entry
	}

	entry = &cachedHostData{}
	c.hosts[host] = entry
	return entry
}

func cloneContainers(containers []Container) []Container {
	if containers == nil {
		return nil
	}

	clone := make([]Container, len(containers))
	for i := range containers {
		clone[i] = containers[i]
		clone[i].Labels = cloneStringMap(containers[i].Labels)
		clone[i].Ports = append([]PortMapping(nil), containers[i].Ports...)
		clone[i].Networks = append([]NetworkInfo(nil), containers[i].Networks...)
		clone[i].Mounts = append([]MountInfo(nil), containers[i].Mounts...)
		clone[i].Stats = cloneContainerStats(containers[i].Stats)
	}
	return clone
}

func cloneStatsMap(stats map[string]*ContainerStats) map[string]*ContainerStats {
	if stats == nil {
		return nil
	}

	clone := make(map[string]*ContainerStats, len(stats))
	for key, stat := range stats {
		clone[key] = cloneContainerStats(stat)
	}
	return clone
}

func cloneContainerStats(stat *ContainerStats) *ContainerStats {
	if stat == nil {
		return nil
	}

	clone := *stat
	return &clone
}

func cloneSystemInfo(info *HostSystemInfo) *HostSystemInfo {
	if info == nil {
		return nil
	}

	clone := *info
	return &clone
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}

	return clone
}
