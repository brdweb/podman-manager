package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

	newPool, err := podman.NewSSHPool(cfg, s.logger)
	if err != nil {
		s.logger.Error("failed to initialize SSH pool for updated config", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := writeConfigAtomically(s.configPath, data); err != nil {
		s.logger.Error("failed to write config file", "path", s.configPath, "error", err)
		newPool.Close()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	newClient := podman.NewClient(newPool, s.logger)

	s.mu.Lock()
	oldPool := s.pool
	s.pool = newPool
	s.client = newClient
	s.config = cfg
	s.sessions = newSessionStore()
	s.mu.Unlock()

	if oldPool != nil {
		oldPool.Close()
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
	tmp, err := os.CreateTemp(getTempDir(), "podman-manager-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp config file: %w", err)
	}

	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp config file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replacing config file: %w", err)
	}

	return nil
}
