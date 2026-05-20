package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brdweb/podman-manager/internal/auth"
	"github.com/brdweb/podman-manager/internal/control"
	"gopkg.in/yaml.v3"
)

type v1OverviewResponse struct {
	Product      string         `json:"product"`
	Version      string         `json:"version"`
	Hosts        any            `json:"hosts"`
	Links        int            `json:"links"`
	Capabilities []string       `json:"capabilities"`
	Integrations map[string]any `json:"integrations"`
}

func (s *Server) handleV1Overview(w http.ResponseWriter, r *http.Request) {
	hosts := s.hostsSnapshot().Overview(r.Context())
	linkCount, err := s.controlStore.CountLinks(r.Context())
	if err != nil {
		s.logger.Warn("failed to count managed links", "error", err)
	}
	cfg := s.configSnapshot()

	writeJSON(w, http.StatusOK, v1OverviewResponse{
		Product: "Homelab Control",
		Version: s.version,
		Hosts:   hosts,
		Links:   linkCount,
		Capabilities: []string{
			"docker-agent-ready",
			"git-compose-control-plane",
			"managed-launchpad-links",
			"homepage-import",
			"openclaw-proxy",
		},
		Integrations: map[string]any{
			"openclaw_configured": strings.TrimSpace(cfg.Integrations.OpenClaw.URL) != "",
			"state_driver":        cfg.State.Driver,
		},
	})
}

func (s *Server) handleV1Inventory(w http.ResponseWriter, r *http.Request) {
	cfg := s.configSnapshot()
	links, err := s.controlStore.ListLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hosts":        cfg.Hosts,
		"links":        links,
		"integrations": cfg.Integrations,
	})
}

func (s *Server) handleV1Service(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	link, err := s.controlStore.GetLink(r.Context(), id)
	if errors.Is(err, control.ErrNotFound) {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           link.ID,
		"name":         link.Title,
		"url":          link.URL,
		"description":  link.Description,
		"category":     link.CategoryName,
		"status":       link.Status,
		"source":       link.Source,
		"managed_link": link,
	})
}

func (s *Server) handleV1Stacks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"stacks":  []any{},
		"message": "Docker Compose stack inventory is ready for agent-backed implementation.",
	})
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.controlStore.ListLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var req control.LinkMutation
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid link payload")
		return
	}
	link, err := s.controlStore.CreateLink(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.controlStore.LogAudit(r.Context(), control.AuditEvent{
		Actor:      actorName(r),
		Action:     "link.create",
		Resource:   "managed_link",
		ResourceID: link.ID,
	})
	writeJSON(w, http.StatusCreated, link)
}

func (s *Server) handleUpdateLink(w http.ResponseWriter, r *http.Request) {
	var req control.LinkMutation
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid link payload")
		return
	}
	link, err := s.controlStore.UpdateLink(r.Context(), r.PathValue("id"), req)
	if errors.Is(err, control.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.controlStore.LogAudit(r.Context(), control.AuditEvent{
		Actor:      actorName(r),
		Action:     "link.update",
		Resource:   "managed_link",
		ResourceID: link.ID,
	})
	writeJSON(w, http.StatusOK, link)
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.controlStore.DeleteLink(r.Context(), id); errors.Is(err, control.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.controlStore.LogAudit(r.Context(), control.AuditEvent{
		Actor:      actorName(r),
		Action:     "link.delete",
		Resource:   "managed_link",
		ResourceID: id,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleImportHomepageLinks(w http.ResponseWriter, r *http.Request) {
	cfg := s.configSnapshot()
	req := control.ImportRequest{
		BookmarksPath: s.resolveConfigPath(cfg.HomepageImport.BookmarksPath),
		ServicesPath:  s.resolveConfigPath(cfg.HomepageImport.ServicesPath),
	}
	if r.Body != nil {
		var body control.ImportRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if strings.TrimSpace(body.BookmarksPath) != "" {
				req.BookmarksPath = s.resolveConfigPath(body.BookmarksPath)
			}
			if strings.TrimSpace(body.ServicesPath) != "" {
				req.ServicesPath = s.resolveConfigPath(body.ServicesPath)
			}
		}
	}
	result, err := s.controlStore.ImportHomepage(r.Context(), req)
	if err != nil && result.Imported == 0 {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.controlStore.LogAudit(r.Context(), control.AuditEvent{
		Actor:    actorName(r),
		Action:   "links.import_homepage",
		Resource: "managed_link",
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleExportLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.controlStore.ListLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "yaml" || format == "yml" {
		w.Header().Set("Content-Type", "application/x-yaml")
		if err := yaml.NewEncoder(w).Encode(links); err != nil {
			s.logger.Warn("failed to encode links export", "error", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, links)
}

type actionRequest struct {
	Type     string         `json:"type"`
	Target   string         `json:"target"`
	Payload  map[string]any `json:"payload"`
	Reason   string         `json:"reason"`
	Approved bool           `json:"approved"`
}

func (s *Server) handleCreateAction(w http.ResponseWriter, r *http.Request) {
	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid action payload")
		return
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Target = strings.TrimSpace(req.Target)
	if req.Type == "" || req.Target == "" {
		writeError(w, http.StatusBadRequest, "action type and target are required")
		return
	}

	id := fmt.Sprintf("act_%d", time.Now().UnixNano())
	_ = s.controlStore.LogAudit(r.Context(), control.AuditEvent{
		ID:         id,
		Actor:      actorName(r),
		Action:     "action.request",
		Resource:   req.Type,
		ResourceID: req.Target,
	})
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":      id,
		"status":  "accepted",
		"message": "Action was recorded. Agent execution will run once the Docker agent backend is connected.",
	})
}

func (s *Server) handleActionStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	id := r.PathValue("id")
	fmt.Fprintf(w, "event: status\ndata: {\"id\":%q,\"status\":\"recorded\"}\n\n", id)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *Server) handleOpenClawChat(w http.ResponseWriter, r *http.Request) {
	cfg := s.configSnapshot().Integrations.OpenClaw
	if strings.TrimSpace(cfg.URL) == "" {
		writeError(w, http.StatusServiceUnavailable, "OpenClaw integration is not configured")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read chat payload")
		return
	}
	target := strings.TrimRight(cfg.URL, "/") + "/" + strings.TrimLeft(cfg.ChatPath, "/")
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(os.Getenv(cfg.TokenEnv)); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func actorName(r *http.Request) string {
	current, ok := r.Context().Value(sessionKey).(*auth.Session)
	if !ok || current == nil {
		return ""
	}
	return current.Username
}

func (s *Server) resolveConfigPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(filepath.Dir(s.configPath), path))
}
