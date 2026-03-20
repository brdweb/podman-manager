package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/brdweb/podman-manager/internal/config"
	"github.com/brdweb/podman-manager/internal/podman"
)

type Server struct {
	mu         sync.RWMutex
	configPath string
	config     *config.Config
	client     *podman.Client
	pool       *podman.SSHPool
	events     *podman.EventStream
	mux        *http.ServeMux
	sessions   *sessionStore
	logger     *slog.Logger
	version    string

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

	pool, err := podman.NewSSHPool(cfg, logger)
	if err != nil {
		return nil, err
	}

	s := &Server{
		configPath:   config.ExpandPath(configPath),
		config:       cfg,
		client:       podman.NewClient(pool, logger, cfg.CacheTTL),
		pool:         pool,
		sessions:     newSessionStore(),
		logger:       logger,
		version:      version,
		eventClients: make(map[*eventClient]struct{}),
	}

	if cfg.EnableEventsStream {
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
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return withLogging(withCORS(withAuth(s, s.mux)))
}

func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.events != nil {
		s.events.Close()
		s.events = nil
	}
	s.eventsEnabled = false

	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
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
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/admin/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/admin/config", s.handleUpdateConfig)
	s.mux.HandleFunc("GET /api/hosts", s.handleListHosts)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers", s.handleListContainers)
	s.mux.HandleFunc("GET /api/hosts/{host}/images", s.handleListImages)
	s.mux.HandleFunc("POST /api/hosts/{host}/images/pull", s.handlePullImage)
	s.mux.HandleFunc("DELETE /api/hosts/{host}/images/{id}", s.handleRemoveImage)
	s.mux.HandleFunc("POST /api/hosts/{host}/images/prune", s.handlePruneImages)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}", s.handleInspectContainer)
	s.mux.HandleFunc("DELETE /api/hosts/{host}/containers/{id}", s.handleRemoveContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/start", s.handleStartContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/stop", s.handleStopContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/restart", s.handleRestartContainer)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/update-check", s.handleCheckContainerUpdate)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/update", s.handleUpdateContainer)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/logs", s.handleContainerLogs)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/logs/stream", s.handleContainerLogsStream)
	s.mux.HandleFunc("GET /api/containers", s.handleAllContainers)
	s.mux.HandleFunc("GET /api/overview", s.handleOverview)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
}

func (s *Server) clientSnapshot() *podman.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *Server) poolSnapshot() *podman.SSHPool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pool
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
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
