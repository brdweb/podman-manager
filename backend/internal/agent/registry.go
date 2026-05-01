package agent

import (
	"context"
	"log/slog"
	"sync"
	"time"

	agentpb "github.com/brdweb/podman-manager/agent/proto"
)

type AgentMessage = agentpb.AgentMessage
type ManagerMessage = agentpb.ManagerMessage

// AgentInfo holds information about a connected agent.
type AgentInfo struct {
	ID            string
	Hostname      string
	AgentVersion  string
	PodmanVersion string
	Capabilities  []string
	ConnectedAt   time.Time
	LastHeartbeat time.Time
	Stream        AgentStream // interface for sending messages
}

// AgentStream is the interface for bidirectional gRPC streams.
type AgentStream interface {
	Send(*ManagerMessage) error
	Recv() (*AgentMessage, error)
	Context() context.Context
}

// Registry tracks connected agents.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo // agent ID -> info
	byHost map[string]string     // hostname -> agent ID
	logger *slog.Logger
}

func NewRegistry(logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		agents: make(map[string]*AgentInfo),
		byHost: make(map[string]string),
		logger: logger,
	}
}

func (r *Registry) Register(agent *AgentInfo) {
	if agent == nil || agent.ID == "" {
		return
	}
	if agent.ConnectedAt.IsZero() {
		agent.ConnectedAt = time.Now()
	}
	if agent.LastHeartbeat.IsZero() {
		agent.LastHeartbeat = agent.ConnectedAt
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if previousID, ok := r.byHost[agent.Hostname]; ok && previousID != agent.ID {
		delete(r.agents, previousID)
	}
	r.agents[agent.ID] = agent
	if agent.Hostname != "" {
		r.byHost[agent.Hostname] = agent.ID
	}
	r.logger.Info("agent registered", "agent_id", agent.ID, "hostname", agent.Hostname)
}

func (r *Registry) Unregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[agentID]
	if !ok {
		return
	}
	delete(r.agents, agentID)
	if agent.Hostname != "" && r.byHost[agent.Hostname] == agentID {
		delete(r.byHost, agent.Hostname)
	}
	r.logger.Info("agent unregistered", "agent_id", agentID, "hostname", agent.Hostname)
}

func (r *Registry) Get(agentID string) (*AgentInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[agentID]
	return agent, ok
}

func (r *Registry) GetByHost(hostname string) (*AgentInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agentID, ok := r.byHost[hostname]
	if !ok {
		return nil, false
	}
	agent, ok := r.agents[agentID]
	return agent, ok
}

func (r *Registry) List() []*AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agents := make([]*AgentInfo, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (r *Registry) UpdateHeartbeat(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if agent, ok := r.agents[agentID]; ok {
		agent.LastHeartbeat = time.Now()
	}
}

func (r *Registry) StaleAgents(timeout time.Duration) []*AgentInfo {
	cutoff := time.Now().Add(-timeout)
	r.mu.RLock()
	defer r.mu.RUnlock()
	agents := make([]*AgentInfo, 0)
	for _, agent := range r.agents {
		if agent.LastHeartbeat.Before(cutoff) {
			agents = append(agents, agent)
		}
	}
	return agents
}
