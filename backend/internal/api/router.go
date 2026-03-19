package api

import (
	"encoding/json"
	"log"
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
	mux        *http.ServeMux
	sessions   *sessionStore
}

func NewServer(configPath string, cfg *config.Config) (*Server, error) {
	pool, err := podman.NewSSHPool(cfg)
	if err != nil {
		return nil, err
	}

	s := &Server{
		configPath: config.ExpandPath(configPath),
		config:     cfg,
		client:     podman.NewClient(pool),
		pool:       pool,
		sessions:   newSessionStore(),
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

	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/auth/session", s.handleSession)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/admin/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/admin/config", s.handleUpdateConfig)
	s.mux.HandleFunc("GET /api/hosts", s.handleListHosts)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers", s.handleListContainers)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}", s.handleInspectContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/start", s.handleStartContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/stop", s.handleStopContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/restart", s.handleRestartContainer)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/logs", s.handleContainerLogs)
	s.mux.HandleFunc("GET /api/containers", s.handleAllContainers)
	s.mux.HandleFunc("GET /api/overview", s.handleOverview)
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
		log.Printf("error encoding response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
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
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
