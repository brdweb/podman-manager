package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/brdweb/podman-manager/internal/podman"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	hostNames := s.client.HostNames()
	statuses := make(map[string]string)

	for _, name := range hostNames {
		latency, err := s.client.Pool().Ping(name)
		if err != nil {
			statuses[name] = "offline"
		} else {
			statuses[name] = "online (" + latency.Round(time.Millisecond).String() + ")"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"hosts":  statuses,
	})
}

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	hostNames := s.client.HostNames()
	hosts := make([]map[string]interface{}, 0, len(hostNames))

	for _, name := range hostNames {
		cfg, _ := s.client.Pool().HostConfig(name)
		h := map[string]interface{}{
			"name":    cfg.Name,
			"address": cfg.Address,
			"mode":    cfg.Mode,
		}

		latency, err := s.client.Pool().Ping(name)
		if err != nil {
			h["status"] = "offline"
			h["error"] = err.Error()
		} else {
			h["status"] = "online"
			h["latency"] = latency.Round(time.Millisecond).String()
		}

		hosts = append(hosts, h)
	}

	writeJSON(w, http.StatusOK, hosts)
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	containers, err := s.client.ListContainers(r.Context(), hostName)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

func (s *Server) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	detail, err := s.client.InspectContainer(r.Context(), hostName, containerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, s.client.StartContainer)
}

func (s *Server) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, s.client.StopContainer)
}

func (s *Server) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, s.client.RestartContainer)
}

type containerActionFunc func(ctx context.Context, hostName, containerID string) (*podman.ActionResult, error)

func (s *Server) handleContainerAction(w http.ResponseWriter, r *http.Request, action containerActionFunc) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	result, err := action(r.Context(), hostName, containerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	tail := 100
	if t := r.URL.Query().Get("tail"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil && parsed > 0 {
			tail = parsed
		}
	}

	logs, err := s.client.ContainerLogs(r.Context(), hostName, containerID, tail)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	overview := s.client.Overview(r.Context())
	writeJSON(w, http.StatusOK, overview)
}
