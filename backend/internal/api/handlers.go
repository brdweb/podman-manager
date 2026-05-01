package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/brdweb/podman-manager/internal/host"
	xwebsocket "golang.org/x/net/websocket"
)

var logsStreamUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return sameOrigin(r)
	},
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return normalizeHost(originURL.Host) == normalizeHost(r.Host)
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, p, err := net.SplitHostPort(host); err == nil {
		if p == "80" || p == "443" {
			return strings.ToLower(h)
		}
	}
	return host
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	hosts := s.hostsSnapshot()
	hostNames := hosts.Names()
	statuses := make(map[string]string)

	for _, name := range hostNames {
		transport, ok := hosts.Get(name)
		if !ok {
			statuses[name] = "offline"
			continue
		}

		latency, err := transport.Ping(r.Context())
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
	hosts := s.hostsSnapshot()
	cfg := s.configSnapshot()
	resp := make([]map[string]interface{}, 0, len(cfg.Hosts))

	for _, hostCfg := range cfg.Hosts {
		h := map[string]interface{}{
			"name":    hostCfg.Name,
			"address": hostCfg.Address,
			"mode":    hostCfg.Mode,
		}

		transport, ok := hosts.Get(hostCfg.Name)
		if !ok {
			h["status"] = "offline"
			h["error"] = "transport not registered"
		} else if latency, err := transport.Ping(r.Context()); err != nil {
			h["status"] = "offline"
			h["error"] = err.Error()
		} else {
			h["status"] = "online"
			h["latency"] = latency.Round(time.Millisecond).String()
		}

		resp = append(resp, h)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	containers, err := transport.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

func (s *Server) handleCreateContainer(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	var req host.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid container create payload")
		return
	}

	req.Image = strings.TrimSpace(req.Image)
	if req.Image == "" {
		writeError(w, http.StatusBadRequest, "image is required")
		return
	}
	req.Name = strings.TrimSpace(req.Name)

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusInternalServerError, "unknown host: "+hostName)
		return
	}

	result, err := transport.CreateContainer(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	detail, err := transport.InspectContainer(r.Context(), containerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, func(ctx context.Context, t host.Transport, containerID string) (*host.ActionResult, error) {
		return t.StartContainer(ctx, containerID)
	})
}

func (s *Server) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, func(ctx context.Context, t host.Transport, containerID string) (*host.ActionResult, error) {
		return t.StopContainer(ctx, containerID)
	})
}

func (s *Server) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerAction(w, r, func(ctx context.Context, t host.Transport, containerID string) (*host.ActionResult, error) {
		return t.RestartContainer(ctx, containerID)
	})
}

func (s *Server) handleCheckContainerUpdate(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	result, err := transport.CheckForUpdate(r.Context(), containerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	result, err := transport.UpdateContainer(r.Context(), containerID)
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

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	result, err := transport.RemoveContainer(r.Context(), containerID, force)
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

type containerActionFunc func(ctx context.Context, transport host.Transport, containerID string) (*host.ActionResult, error)

func (s *Server) handleContainerAction(w http.ResponseWriter, r *http.Request, action containerActionFunc) {
	hostName := r.PathValue("host")
	containerID := r.PathValue("id")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusInternalServerError, "unknown host: "+hostName)
		return
	}

	result, err := action(r.Context(), transport, containerID)
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

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	logs, err := transport.ContainerLogs(r.Context(), containerID, tail)
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

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		_ = ws.WriteJSON(map[string]string{"error": "unknown host: " + hostName})
		return
	}

	go func() {
		errCh <- transport.StreamLogs(ctx, containerID, tail, logsCh)
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
	overview := s.hostsSnapshot().Overview(r.Context())
	writeJSON(w, http.StatusOK, overview)
}

type pullImageRequest struct {
	Image     string `json:"image"`
	ImageRef  string `json:"image_ref"`
	Reference string `json:"reference"`
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	images, err := transport.ListImages(r.Context())
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

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	if err := transport.PullImage(r.Context(), imageRef); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, host.ActionResult{
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

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	if err := transport.RemoveImage(r.Context(), imageID, force); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, host.ActionResult{
		Success: true,
		Message: "Image removed successfully",
	})
}

func (s *Server) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	pruned, err := transport.PruneImages(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"pruned_images": pruned,
	})
}

func (s *Server) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	var req host.CreateVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid volume create payload")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "volume name is required")
		return
	}
	req.Driver = strings.TrimSpace(req.Driver)

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusInternalServerError, "unknown host: "+hostName)
		return
	}

	volume, err := transport.CreateVolume(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, volume)
}

func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	networks, err := transport.ListNetworks(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, networks)
}

func (s *Server) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")

	var req host.CreateNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid network create payload")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "network name is required")
		return
	}
	req.Driver = strings.TrimSpace(req.Driver)
	req.Gateway = strings.TrimSpace(req.Gateway)

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusInternalServerError, "unknown host: "+hostName)
		return
	}

	network, err := transport.CreateNetwork(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, network)
}

func (s *Server) handleRemoveNetwork(w http.ResponseWriter, r *http.Request) {
	hostName := r.PathValue("host")
	networkName := strings.TrimSpace(r.PathValue("name"))
	if networkName == "" {
		writeError(w, http.StatusBadRequest, "network name is required")
		return
	}

	transport, ok := s.hostsSnapshot().Get(hostName)
	if !ok {
		writeError(w, http.StatusBadGateway, "unknown host: "+hostName)
		return
	}

	if err := transport.RemoveNetwork(r.Context(), networkName); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, host.ActionResult{
		Success: true,
		Message: "Network removed successfully",
	})
}

func (s *Server) handleAllContainers(w http.ResponseWriter, r *http.Request) {
	overview := s.hostsSnapshot().Overview(r.Context())
	all := make([]host.Container, 0)
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
