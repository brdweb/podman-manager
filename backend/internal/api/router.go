package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brdweb/podman-manager/internal/agent"
	"github.com/brdweb/podman-manager/internal/auth"
	"github.com/brdweb/podman-manager/internal/config"
	"github.com/brdweb/podman-manager/internal/enroll"
	"github.com/brdweb/podman-manager/internal/host"
	"github.com/brdweb/podman-manager/internal/podman"
	"google.golang.org/grpc"
)

type Server struct {
	mu                sync.RWMutex
	configPath        string
	config            *config.Config
	hosts             *host.HostManager
	events            *podman.EventStream
	mux               *http.ServeMux
	authStore         *auth.Store
	logger            *slog.Logger
	version           string
	grpcServer        *agent.Server
	grpcAddress       string
	agentGRPCServer   *grpc.Server
	enrollStore       *enroll.Store
	enrollHandler     *enroll.Handler
	enrollCleanupStop chan struct{}
	authCleanupStop   chan struct{}

	eventsMu      sync.RWMutex
	eventClients  map[*eventClient]struct{}
	eventsEnabled bool
}

type eventClient struct {
	ch chan podman.PodmanEvent
}

func NewServer(configPath string, cfg *config.Config, logger *slog.Logger, version string) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}

	configPath = config.ExpandPath(configPath)
	authStore, err := auth.NewStore(authDBPath(configPath, cfg), logger)
	if err != nil {
		return nil, err
	}
	if err := seedConfigAuthUser(context.Background(), authStore, cfg); err != nil {
		authStore.Close()
		return nil, err
	}

	grpcAddress := ":18735"
	enrollStore, err := enroll.NewPersistentStore(time.Hour, enrollCredentialsPath(configPath, cfg))
	if err != nil {
		authStore.Close()
		return nil, err
	}
	enrollHandler := enroll.NewHandler(enrollStore)
	registry := agent.NewRegistry(logger)
	agentServer := agent.NewServer(registry, logger, enrollStore)
	agentGRPCServer, err := agent.StartGRPCServer(grpcAddress, agentServer, logger)
	if err != nil {
		authStore.Close()
		return nil, err
	}

	hosts, pool, err := buildHostManager(cfg, logger, agentServer)
	if err != nil {
		if agentGRPCServer != nil {
			agentGRPCServer.Stop()
		}
		authStore.Close()
		return nil, err
	}

	s := &Server{
		configPath:        configPath,
		config:            cfg,
		hosts:             hosts,
		authStore:         authStore,
		logger:            logger,
		version:           version,
		grpcServer:        agentServer,
		grpcAddress:       grpcAddress,
		agentGRPCServer:   agentGRPCServer,
		enrollStore:       enrollStore,
		enrollHandler:     enrollHandler,
		enrollCleanupStop: make(chan struct{}),
		authCleanupStop:   make(chan struct{}),
		eventClients:      make(map[*eventClient]struct{}),
	}

	if cfg.EnableEventsStream && pool != nil {
		eventStream := podman.NewEventStream(pool, logger)
		for _, hostName := range pool.HostNames() {
			if err := eventStream.Subscribe(hostName); err != nil {
				logger.Warn("failed to subscribe to podman events stream", "host", hostName, "error", err)
			}
		}
		s.events = eventStream
		s.eventsEnabled = true
		go s.runEventBroadcast(eventStream)
	}

	s.mux = http.NewServeMux()
	s.registerRoutes()
	go s.runEnrollCleanup()
	go s.runAuthCleanup()
	return s, nil
}

func authDBPath(configPath string, cfg *config.Config) string {
	if cfg.Server.AuthDBPath != "" {
		return config.ExpandPath(cfg.Server.AuthDBPath)
	}
	normalizedConfigPath := strings.ReplaceAll(filepath.Clean(configPath), "\\", "/")
	if configPath == "" || normalizedConfigPath == "/etc/podman-manager/config.yaml" {
		return "/var/lib/podman-manager/auth.db"
	}
	if configPath != "" {
		return filepath.Join(filepath.Dir(configPath), "auth.db")
	}
	return "/var/lib/podman-manager/auth.db"
}

func enrollCredentialsPath(configPath string, cfg *config.Config) string {
	return filepath.Join(filepath.Dir(authDBPath(configPath, cfg)), "agent-credentials.json")
}

func seedConfigAuthUser(ctx context.Context, store *auth.Store, cfg *config.Config) error {
	if !cfg.Auth.Enabled {
		return nil
	}
	return store.EnsureUserWithPasswordHash(ctx, cfg.Auth.Username, cfg.Auth.PasswordHash, auth.RoleAdmin)
}

func buildHostManager(cfg *config.Config, logger *slog.Logger, agentServer *agent.Server) (*host.HostManager, *podman.SSHPool, error) {
	sshCfg := sshOnlyConfig(cfg)
	var pool *podman.SSHPool
	var client *podman.Client
	if len(sshCfg.Hosts) > 0 {
		var err error
		pool, err = podman.NewSSHPool(sshCfg, logger)
		if err != nil {
			return nil, nil, err
		}
		client = podman.NewClient(pool, logger, cfg.CacheTTL)
	}

	hosts := host.NewHostManager()
	for _, hostCfg := range cfg.Hosts {
		transport := strings.ToLower(strings.TrimSpace(hostCfg.Transport))
		if transport == "" {
			transport = "ssh"
		}
		switch transport {
		case "ssh":
			if client == nil || pool == nil {
				return nil, nil, fmt.Errorf("ssh transport is not initialized for host %q", hostCfg.Name)
			}
			hosts.Register(hostCfg.Name, host.NewSSHTransport(hostCfg.Name, client, pool, logger))
		case "agent":
			if agentServer == nil {
				if pool != nil {
					pool.Close()
				}
				return nil, nil, fmt.Errorf("agent transport is not initialized for host %q", hostCfg.Name)
			}
			hosts.Register(hostCfg.Name, agent.NewAgentTransport(hostCfg.Name, hostCfg.AgentID, agentServer, cfg.SSH.ConnectTimeout))
		default:
			if pool != nil {
				pool.Close()
			}
			return nil, nil, fmt.Errorf("unsupported transport %q for host %q", transport, hostCfg.Name)
		}
	}

	return hosts, pool, nil
}

func sshOnlyConfig(cfg *config.Config) *config.Config {
	clone := *cfg
	clone.Hosts = nil
	for _, hostCfg := range cfg.Hosts {
		transport := strings.ToLower(strings.TrimSpace(hostCfg.Transport))
		if transport == "" || transport == "ssh" {
			clone.Hosts = append(clone.Hosts, hostCfg)
		}
	}
	return &clone
}

func (s *Server) Handler() http.Handler {
	return withLogging(withCORS(s.mux))
}

func (s *Server) authHandler(fn http.HandlerFunc) http.HandlerFunc {
	return withAuth(s)(http.HandlerFunc(fn)).ServeHTTP
}

func (s *Server) roleHandler(fn http.HandlerFunc, roles ...auth.Role) http.HandlerFunc {
	h := withRBAC(roles...)(http.HandlerFunc(fn))
	h = withAuth(s)(h)
	return h.ServeHTTP
}

func (s *Server) adminHandler(fn http.HandlerFunc) http.HandlerFunc {
	return s.roleHandler(fn, auth.RoleAdmin)
}

func (s *Server) operatorHandler(fn http.HandlerFunc) http.HandlerFunc {
	return s.roleHandler(fn, auth.RoleOperator, auth.RoleAdmin)
}

func (s *Server) viewerHandler(fn http.HandlerFunc) http.HandlerFunc {
	return s.roleHandler(fn, auth.RoleViewer, auth.RoleOperator, auth.RoleAdmin)
}

func (s *Server) AgentServer() *agent.Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.grpcServer
}

func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.events != nil {
		s.events.Close()
		s.events = nil
	}
	s.eventsEnabled = false

	if s.hosts != nil {
		_ = s.hosts.Close()
		s.hosts = nil
	}

	if s.agentGRPCServer != nil {
		s.agentGRPCServer.GracefulStop()
		s.agentGRPCServer = nil
	}

	if s.enrollCleanupStop != nil {
		close(s.enrollCleanupStop)
		s.enrollCleanupStop = nil
	}

	if s.authCleanupStop != nil {
		close(s.authCleanupStop)
		s.authCleanupStop = nil
	}

	if s.authStore != nil {
		if err := s.authStore.Close(); err != nil {
			s.logger.Warn("failed to close auth store", "error", err)
		}
		s.authStore = nil
	}

	s.eventsMu.Lock()
	for client := range s.eventClients {
		delete(s.eventClients, client)
		close(client.ch)
	}
	s.eventsMu.Unlock()
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/version", s.handleVersion)
	s.mux.HandleFunc("GET /api/auth/session", s.handleSession)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.authHandler(s.handleLogout))
	s.mux.HandleFunc("GET /api/admin/config", s.adminHandler(s.handleGetConfig))
	s.mux.HandleFunc("PUT /api/admin/config", s.adminHandler(s.handleUpdateConfig))
	s.mux.HandleFunc("GET /api/admin/users", s.adminHandler(s.handleListUsers))
	s.mux.HandleFunc("POST /api/admin/users", s.adminHandler(s.handleCreateUser))
	s.mux.HandleFunc("GET /api/admin/users/{id}", s.adminHandler(s.handleGetUser))
	s.mux.HandleFunc("PUT /api/admin/users/{id}", s.adminHandler(s.handleUpdateUser))
	s.mux.HandleFunc("DELETE /api/admin/users/{id}", s.adminHandler(s.handleDeleteUser))
	s.mux.HandleFunc("POST /api/admin/users/{id}/reset-password", s.adminHandler(s.handleResetPassword))
	s.mux.HandleFunc("POST /api/users/me/password", s.authHandler(s.handleChangePassword))
	s.mux.HandleFunc("POST /api/admin/agents/enroll", s.adminHandler(s.enrollHandler.CreateToken))
	s.mux.HandleFunc("GET /api/admin/agents", s.adminHandler(s.enrollHandler.ListAgents))
	s.mux.HandleFunc("POST /api/admin/agents/{id}/revoke", s.adminHandler(s.enrollHandler.RevokeAgent))
	s.mux.HandleFunc("GET /api/agent/install.sh", s.enrollHandler.InstallScript)
	s.mux.HandleFunc("GET /api/hosts", s.viewerHandler(s.handleListHosts))
	s.mux.HandleFunc("GET /api/hosts/{host}/containers", s.viewerHandler(s.handleListContainers))
	s.mux.HandleFunc("POST /api/hosts/{host}/containers", s.operatorHandler(s.handleCreateContainer))
	s.mux.HandleFunc("POST /api/hosts/{host}/volumes", s.operatorHandler(s.handleCreateVolume))
	s.mux.HandleFunc("GET /api/hosts/{host}/networks", s.viewerHandler(s.handleListNetworks))
	s.mux.HandleFunc("POST /api/hosts/{host}/networks", s.operatorHandler(s.handleCreateNetwork))
	s.mux.HandleFunc("DELETE /api/hosts/{host}/networks/{name}", s.operatorHandler(s.handleRemoveNetwork))
	s.mux.HandleFunc("GET /api/hosts/{host}/images", s.viewerHandler(s.handleListImages))
	s.mux.HandleFunc("POST /api/hosts/{host}/images/pull", s.operatorHandler(s.handlePullImage))
	s.mux.HandleFunc("DELETE /api/hosts/{host}/images/{id}", s.operatorHandler(s.handleRemoveImage))
	s.mux.HandleFunc("POST /api/hosts/{host}/images/prune", s.operatorHandler(s.handlePruneImages))
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}", s.viewerHandler(s.handleInspectContainer))
	s.mux.HandleFunc("DELETE /api/hosts/{host}/containers/{id}", s.operatorHandler(s.handleRemoveContainer))
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/start", s.operatorHandler(s.handleStartContainer))
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/stop", s.operatorHandler(s.handleStopContainer))
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/restart", s.operatorHandler(s.handleRestartContainer))
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/update-check", s.viewerHandler(s.handleCheckContainerUpdate))
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/update", s.operatorHandler(s.handleUpdateContainer))
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/logs", s.operatorHandler(s.handleContainerLogs))
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/logs/stream", s.operatorHandler(s.handleContainerLogsStream))
	s.mux.HandleFunc("GET /api/containers", s.viewerHandler(s.handleAllContainers))
	s.mux.HandleFunc("GET /api/overview", s.viewerHandler(s.handleOverview))
	s.mux.HandleFunc("GET /api/events", s.viewerHandler(s.handleEvents))
}

func (s *Server) hostsSnapshot() *host.HostManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hosts
}

func (s *Server) eventStreamSnapshot() (*podman.EventStream, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events, s.eventsEnabled
}

func (s *Server) runEventBroadcast(stream *podman.EventStream) {
	for evt := range stream.Events() {
		s.eventsMu.RLock()
		for client := range s.eventClients {
			select {
			case client.ch <- evt:
			default:
			}
		}
		s.eventsMu.RUnlock()
	}
}

func (s *Server) runEnrollCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.enrollStore.CleanupExpired()
		case <-s.enrollCleanupStop:
			return
		}
	}
}

func (s *Server) runAuthCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			deleted, err := s.authStore.CleanupExpiredSessions(context.Background())
			if err != nil {
				s.logger.Warn("failed to clean up expired auth sessions", "error", err)
			} else if deleted > 0 {
				s.logger.Debug("cleaned up expired auth sessions", "count", deleted)
			}
		case <-s.authCleanupStop:
			return
		}
	}
}

func (s *Server) registerEventClient() *eventClient {
	client := &eventClient{ch: make(chan podman.PodmanEvent, 64)}
	s.eventsMu.Lock()
	s.eventClients[client] = struct{}{}
	s.eventsMu.Unlock()
	return client
}

func (s *Server) unregisterEventClient(client *eventClient) {
	s.eventsMu.Lock()
	if _, ok := s.eventClients[client]; ok {
		delete(s.eventClients, client)
		close(client.ch)
	}
	s.eventsMu.Unlock()
}

func (s *Server) authConfig() config.AuthConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Auth
}

func (s *Server) configSnapshot() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := *s.config
	clone.Hosts = append([]config.HostConfig(nil), s.config.Hosts...)
	return &clone
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("error encoding response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).Round(time.Millisecond))
	})
}
