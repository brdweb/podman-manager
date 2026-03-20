package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/brdweb/podman-manager/internal/podman"
	xwebsocket "golang.org/x/net/websocket"
)

var logsStreamUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	client := s.clientSnapshot()
	hostNames := client.HostNames()
	statuses := make(map[string]string)

	for _, name := range hostNames {
		latency, err := client.Pool().Ping(name)
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

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": s.version,
	})
}

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	client := s.clientSnapshot()
	hostNames := client.HostNames()
	hosts := make([]map[string]interface{}, 0, len(hostNames))

	for _, name := range hostNames {
		cfg, _ := client.Pool().HostConfig(name)
		h := map[string]interface{}{
			"name":    cfg.Name,
			"address": cfg.Address,
			"mode":    cfg.Mode,
		}

		latency, err := client.Pool().Ping(name)
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

	containers, err := s.clientSnapshot().ListContainers(r.Context(), hostName)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

func (s *Server) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	detail, err := s.clientSnapshot().InspectContainer(r.Context(), hostName, containerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, s.clientSnapshot().StartContainer)
}

func (s *Server) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, s.clientSnapshot().StopContainer)
}

func (s *Server) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, s.clientSnapshot().RestartContainer)
}

func (s *Server) handleCheckContainerUpdate(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	result, err := s.clientSnapshot().CheckForUpdate(r.Context(), hostName, containerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	result, err := s.clientSnapshot().UpdateContainer(r.Context(), hostName, containerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}

	writeJSON(w, status, result)
}

func (s *Server) handleRemoveContainer(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	force := false
	if forceParam := r.URL.Query().Get("force"); forceParam != "" {
		parsedForce, err := strconv.ParseBool(forceParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid force query parameter; expected true or false")
			return
		}
		force = parsedForce
	}

	result, err := s.clientSnapshot().RemoveContainer(r.Context(), hostName, containerID, force)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
		errMsg := strings.ToLower(result.Error)
		switch {
		case strings.Contains(errMsg, "not found"):
			status = http.StatusNotFound
		case strings.Contains(errMsg, "running"):
			status = http.StatusConflict
		}
	}

	writeJSON(w, status, result)
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

	logs, err := s.clientSnapshot().ContainerLogs(r.Context(), hostName, containerID, tail)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (s *Server) handleContainerLogsStream(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	tail := 100
	if t := strings.TrimSpace(r.URL.Query().Get("tail")); t != "" {
		parsed, err := strconv.Atoi(t)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid tail query parameter; expected non-negative integer")
			return
		}
		tail = parsed
	}

	ws, err := logsStreamUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	logsCh := make(chan string, 256)
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.clientSnapshot().StreamLogs(ctx, hostName, containerID, tail, logsCh)
	}()

	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case line := <-logsCh:
			logEntry := map[string]string{
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				"message":   line,
			}
			if err := ws.WriteJSON(logEntry); err != nil {
				cancel()
				return
			}
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				_ = ws.WriteJSON(map[string]string{"error": err.Error()})
			}
			return
		}
	}
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	overview := s.clientSnapshot().Overview(r.Context())
	writeJSON(w, http.StatusOK, overview)
}

type pullImageRequest struct {
	Image     string `json:"image"`
	ImageRef  string `json:"image_ref"`
	Reference string `json:"reference"`
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	images, err := s.clientSnapshot().ListImages(r.Context(), hostName)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, images)
}

func (s *Server) handlePullImage(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	var req pullImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid image pull payload")
		return
	}

	imageRef := strings.TrimSpace(req.Image)
	if imageRef == "" {
		imageRef = strings.TrimSpace(req.ImageRef)
	}
	if imageRef == "" {
		imageRef = strings.TrimSpace(req.Reference)
	}
	if imageRef == "" {
		writeError(w, http.StatusBadRequest, "image reference is required")
		return
	}

	if err := s.clientSnapshot().PullImage(r.Context(), hostName, imageRef); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, podman.ActionResult{
		Success: true,
		Message: "Image pulled successfully",
	})
}

func (s *Server) handleRemoveImage(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	imageID := r.PathValue("id")

	force := false
	if forceRaw := strings.TrimSpace(r.URL.Query().Get("force")); forceRaw != "" {
		force = forceRaw == "1" || strings.EqualFold(forceRaw, "true") || strings.EqualFold(forceRaw, "yes")
	}

	if err := s.clientSnapshot().RemoveImage(r.Context(), hostName, imageID, force); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, podman.ActionResult{
		Success: true,
		Message: "Image removed successfully",
	})
}

func (s *Server) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	pruned, err := s.clientSnapshot().PruneImages(r.Context(), hostName)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"pruned_images": pruned,
	})
}

func (s *Server) handleAllContainers(w http.ResponseWriter, r *http.Request) {
	overview := s.clientSnapshot().Overview(r.Context())
	all := make([]podman.Container, 0)
	for _, host := range overview.Hosts {
		all = append(all, host.Containers...)
	}
	writeJSON(w, http.StatusOK, all)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	_, enabled := s.eventStreamSnapshot()
	if !enabled {
		writeError(w, http.StatusNotFound, "events stream is disabled")
		return
	}

	requestDone := r.Context().Done()
	xwebsocket.Handler(func(ws *xwebsocket.Conn) {
		defer ws.Close()

		client := s.registerEventClient()
		defer s.unregisterEventClient(client)

		for {
			select {
			case <-requestDone:
				return
			case event, ok := <-client.ch:
				if !ok {
					return
				}
				if err := xwebsocket.JSON.Send(ws, event); err != nil {
					return
				}
			}
		}
	}).ServeHTTP(w, r)
}
