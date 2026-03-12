package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/brdweb/podman-manager/internal/podman"
)

type Server struct {
	client *podman.Client
	mux    *http.ServeMux
}

func NewServer(client *podman.Client) *Server {
	s := &Server{client: client}
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return withLogging(withCORS(s.mux))
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/hosts", s.handleListHosts)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers", s.handleListContainers)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}", s.handleInspectContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/start", s.handleStartContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/stop", s.handleStopContainer)
	s.mux.HandleFunc("POST /api/hosts/{host}/containers/{id}/restart", s.handleRestartContainer)
	s.mux.HandleFunc("GET /api/hosts/{host}/containers/{id}/logs", s.handleContainerLogs)
	s.mux.HandleFunc("GET /api/overview", s.handleOverview)
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
