package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/brdweb/podman-manager/internal/config"
	"github.com/brdweb/podman-manager/internal/podman"
)

type configResponse struct {
	Path string `json:"path"`
	YAML string `json:"yaml"`
	Auth struct {
		Enabled     bool   `json:"enabled"`
		Username    string `json:"username,omitempty"`
		HasPassword bool   `json:"has_password"`
	} `json:"auth"`
}

type updateConfigRequest struct {
	YAML string `json:"yaml"`
	Auth struct {
		Enabled  bool   `json:"enabled"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"auth"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.configSnapshot()
	raw, err := os.ReadFile(s.configPath)
	if err != nil {
		s.logger.Error("failed to read config file", "path", s.configPath, "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("reading config file: %v", err))
		return
	}

	resp := configResponse{
		Path: s.configPath,
		YAML: string(raw),
	}
	resp.Auth.Enabled = cfg.Auth.Enabled
	resp.Auth.Username = cfg.Auth.Username
	resp.Auth.HasPassword = cfg.Auth.PasswordHash != ""

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req updateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode config update payload", "error", err)
		writeError(w, http.StatusBadRequest, "invalid config payload")
		return
	}

	cfg, err := config.LoadBytes([]byte(req.YAML))
	if err != nil {
		s.logger.Error("failed to parse config YAML", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg.Auth.Enabled = req.Auth.Enabled
	if strings.TrimSpace(req.Auth.Username) != "" {
		cfg.Auth.Username = strings.TrimSpace(req.Auth.Username)
	}

	if strings.TrimSpace(req.Auth.Password) != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Auth.Password), bcrypt.DefaultCost)
		if err != nil {
			s.logger.Error("failed to hash auth password", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		cfg.Auth.PasswordHash = string(hashed)
	}

	if !cfg.Auth.Enabled {
		cfg.Auth.Username = ""
		cfg.Auth.PasswordHash = ""
	}

	if err := cfg.Validate(); err != nil {
		s.logger.Error("config validation failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		s.logger.Error("failed to marshal config", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	newHosts, newPool, err := buildHostManager(cfg, s.logger, s.grpcServer)
	if err != nil {
		s.logger.Error("failed to initialize host transports for updated config", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var newEvents *podman.EventStream
	if cfg.EnableEventsStream && newPool != nil {
		newEvents = podman.NewEventStream(newPool, s.logger)
		for _, hostName := range newPool.HostNames() {
			if err := newEvents.Subscribe(hostName); err != nil {
				s.logger.Warn("failed to subscribe to podman events stream", "host", hostName, "error", err)
			}
		}
	}

	if err := writeConfigAtomically(s.configPath, data); err != nil {
		s.logger.Error("failed to write config file", "path", s.configPath, "error", err)
		if newEvents != nil {
			newEvents.Close()
		}
		_ = newHosts.Close()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := seedConfigAuthUser(context.Background(), s.authStore, cfg); err != nil {
		s.logger.Error("failed to synchronize auth user", "error", err)
		if newEvents != nil {
			newEvents.Close()
		}
		_ = newHosts.Close()
		writeError(w, http.StatusInternalServerError, "failed to synchronize auth user")
		return
	}

	s.mu.Lock()
	oldHosts := s.hosts
	oldEvents := s.events
	s.hosts = newHosts
	s.config = cfg
	s.events = newEvents
	s.eventsEnabled = cfg.EnableEventsStream
	s.mu.Unlock()

	if oldHosts != nil {
		_ = oldHosts.Close()
	}

	if oldEvents != nil {
		oldEvents.Close()
	}

	if newEvents != nil {
		go s.runEventBroadcast(newEvents)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Configuration saved and applied",
	})
}

func getTempDir() string {
	if dir := os.Getenv("PODMAN_MANAGER_TEMP_DIR"); dir != "" {
		return dir
	}
	return "/tmp"
}

func writeConfigAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = getTempDir()
	}

	tmp, err := os.CreateTemp(dir, "podman-manager-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp config file: %w", err)
	}

	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp config file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing temp config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp config file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("writing config file: %w", err)
		}
	}

	return nil
}
