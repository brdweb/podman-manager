package connect

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/brdweb/podman-manager/agent/internal/config"
	"github.com/brdweb/podman-manager/agent/internal/podman"
	agentpb "github.com/brdweb/podman-manager/agent/proto"
)

// Manager manages the connection to the Podman Manager.
type Manager struct {
	mu       sync.Mutex
	conn     *grpc.ClientConn
	stream   agentpb.AgentService_ConnectClient
	cfg      *config.Config
	podman   *podman.Client
	logger   *slog.Logger
	running  bool
	done     chan struct{}
	doneOnce sync.Once

	// Request tracking
	pendingRequests map[string]chan *agentpb.ManagerMessage
	requestMu       sync.RWMutex

	// Active streams started by the manager.
	streamMu     sync.Mutex
	logStreams   map[string]context.CancelFunc
	eventStreams map[string]context.CancelFunc

	reconnectCh chan struct{}
}

// NewManager creates a new manager connection handler.
func NewManager(cfg *config.Config, podmanClient *podman.Client, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	return &Manager{
		cfg:             cfg,
		podman:          podmanClient,
		logger:          logger,
		done:            make(chan struct{}),
		pendingRequests: make(map[string]chan *agentpb.ManagerMessage),
		logStreams:      make(map[string]context.CancelFunc),
		eventStreams:    make(map[string]context.CancelFunc),
		reconnectCh:     make(chan struct{}, 1),
	}
}

func (m *Manager) listContainers(ctx context.Context, all bool) ([]*agentpb.Container, error) {
	containers, err := m.podman.ListContainers(ctx, all)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(containers))
	protoContainers := make([]*agentpb.Container, 0, len(containers))
	for _, container := range containers {
		protoContainer := containerToProto(container)
		if protoContainer.Name != "" {
			seen[protoContainer.Name] = true
		}
		protoContainers = append(protoContainers, protoContainer)
	}

	quadlets, err := discoverQuadletContainers(ctx, os.Geteuid() == 0, m.podman)
	if err != nil {
		m.logger.Debug("quadlet discovery failed", "error", err)
		return protoContainers, nil
	}

	for _, quadlet := range quadlets {
		if quadlet.State == "active" || seen[quadlet.ContainerName] {
			continue
		}
		protoContainers = append(protoContainers, quadletContainerToProto(quadlet))
	}

	return protoContainers, nil
}

func containerToProto(container podman.Container) *agentpb.Container {
	return &agentpb.Container{
		Id:     container.ID,
		Name:   firstContainerName(container.Names),
		Image:  container.Image,
		State:  container.State,
		Status: container.Status,
		Ports:  portMappingsToProto(container.Ports),
		Mounts: mountsToProto(container.Mounts),
		Labels: container.Labels,
	}
}

func quadletContainerToProto(container QuadletContainer) *agentpb.Container {
	name := container.ContainerName
	if name == "" {
		name = container.Name
	}
	return &agentpb.Container{
		Id:          "quadlet-" + container.Name,
		Name:        name,
		Image:       container.Image,
		State:       "exited",
		Status:      "Stopped (Quadlet)",
		Manager:     "quadlet",
		SystemdUnit: container.Unit,
	}
}

func firstContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func portMappingsToProto(ports []podman.PortMapping) []*agentpb.PortMapping {
	converted := make([]*agentpb.PortMapping, 0, len(ports))
	for _, port := range ports {
		converted = append(converted, &agentpb.PortMapping{HostIp: port.HostIP, HostPort: int32(port.HostPort), ContainerPort: int32(port.ContainerPort), Protocol: port.Protocol})
	}
	return converted
}

func mountsToProto(mounts []podman.Mount) []*agentpb.MountInfo {
	converted := make([]*agentpb.MountInfo, 0, len(mounts))
	for _, mount := range mounts {
		converted = append(converted, &agentpb.MountInfo{Type: mount.Type, Source: mount.Source, Destination: mount.Destination, Rw: !mount.ReadOnly})
	}
	return converted
}

func volumesToProto(volumes []podman.Volume) []*agentpb.Volume {
	converted := make([]*agentpb.Volume, 0, len(volumes))
	for _, volume := range volumes {
		converted = append(converted, &agentpb.Volume{Name: volume.Name, Driver: volume.Driver, Mountpoint: volume.Mountpoint, Labels: volume.Labels})
	}
	return converted
}

func networksToProto(networks []podman.Network) []*agentpb.Network {
	converted := make([]*agentpb.Network, 0, len(networks))
	for _, network := range networks {
		converted = append(converted, &agentpb.Network{Name: network.Name, Driver: network.Driver, Subnet: network.Subnet, Gateway: network.Gateway, Ipv6: network.IPv6Enabled, Labels: network.Labels})
	}
	return converted
}

func actionResult(err error, successMessage string) *agentpb.ActionResult {
	if err != nil {
		return &agentpb.ActionResult{Success: false, Error: err.Error()}
	}
	return &agentpb.ActionResult{Success: true, Message: successMessage}
}

func errorAgentMessage(requestID string, err error) *agentpb.AgentMessage {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return &agentpb.AgentMessage{Type: &agentpb.AgentMessage_ErrorResponse{ErrorResponse: &agentpb.ErrorResponse{RequestId: requestID, Code: "agent_error", Message: message}}}
}

// Connect establishes the gRPC connection to the manager and starts the bidirectional stream.
// It blocks until the context is canceled or a fatal error occurs.
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("manager connection already running")
	}
	m.running = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		_ = m.closeConnection()
	}()

	conn, err := m.dial(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()

	stream, err := m.startStream(ctx)
	if err != nil {
		_ = conn.Close()
		return err
	}

	m.mu.Lock()
	m.stream = stream
	m.mu.Unlock()
	m.logger.Info("connected to manager", "address", m.cfg.Manager.Address)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.done:
			return nil
		case <-m.reconnectCh:
			m.reconnect(ctx)
		}
	}
}

// SendRequest sends a request to the manager and waits for the response.
func (m *Manager) SendRequest(ctx context.Context, msg *agentpb.AgentMessage) (*agentpb.ManagerMessage, error) {
	return m.sendRequest(ctx, msg)
}

// Close closes the gRPC connection.
func (m *Manager) Close() error {
	m.doneOnce.Do(func() {
		close(m.done)
	})
	return m.closeConnection()
}

func (m *Manager) closeConnection() error {
	m.cancelAllStreams()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stream != nil {
		if err := m.stream.CloseSend(); err != nil && !errors.Is(err, io.EOF) {
			m.logger.Debug("closing manager stream failed", "error", err)
		}
		m.stream = nil
	}

	var err error
	if m.conn != nil {
		err = m.conn.Close()
		m.conn = nil
	}

	m.failPendingRequests()
	return err
}

// IsConnected returns whether the manager is currently connected.
func (m *Manager) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conn != nil && m.stream != nil
}

func (m *Manager) dial(ctx context.Context) (*grpc.ClientConn, error) {
	var transport credentials.TransportCredentials
	if m.cfg.Manager.TLS {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		if m.cfg.Manager.TLSInsecure {
			tlsConfig.InsecureSkipVerify = true
		}
		transport = credentials.NewTLS(tlsConfig)
	} else {
		transport = insecure.NewCredentials()
	}

	keepaliveParams := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: true,
	}

	conn, err := grpc.DialContext(
		ctx,
		m.cfg.Manager.Address,
		grpc.WithTransportCredentials(transport),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64*1024*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing manager %s: %w", m.cfg.Manager.Address, err)
	}

	return conn, nil
}

func (m *Manager) startStream(ctx context.Context) (agentpb.AgentService_ConnectClient, error) {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("manager connection is not established")
	}

	client := agentpb.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting manager stream: %w", err)
	}

	go m.receiveLoop(ctx, stream)
	go m.heartbeatLoop(ctx, stream)

	return stream, nil
}

func (m *Manager) receiveLoop(ctx context.Context, stream agentpb.AgentService_ConnectClient) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				m.cancelAllStreams()
				return
			}
			m.logger.Warn("manager stream receive failed", "error", err)
			m.cancelAllStreams()
			m.triggerReconnect()
			return
		}

		if m.handleStreamCommand(ctx, msg) {
			continue
		}

		requestID := managerRequestID(msg)
		if requestID == "" {
			m.logger.Debug("received manager message without request id")
			continue
		}

		m.requestMu.RLock()
		responseCh := m.pendingRequests[requestID]
		m.requestMu.RUnlock()

		if responseCh == nil {
			m.logger.Warn("received response for unknown request", "request_id", requestID)
			continue
		}

		select {
		case responseCh <- msg:
		case <-ctx.Done():
			return
		default:
			m.logger.Warn("response channel is full", "request_id", requestID)
		}
	}
}

func (m *Manager) heartbeatLoop(ctx context.Context, stream agentpb.AgentService_ConnectClient) {
	interval := m.cfg.Heartbeat.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			requestID := generateRequestID()
			msg := &agentpb.AgentMessage{
				Type: &agentpb.AgentMessage_Heartbeat{
					Heartbeat: &agentpb.Heartbeat{
						RequestId: requestID,
						Timestamp: timestamppb.Now(),
					},
				},
			}

			responseCh := make(chan *agentpb.ManagerMessage, 1)
			m.registerPendingRequest(requestID, responseCh)

			m.mu.Lock()
			err := stream.Send(msg)
			m.mu.Unlock()

			if err != nil {
				m.removePendingRequest(requestID)
				if ctx.Err() == nil {
					m.logger.Warn("sending heartbeat failed", "error", err)
					m.triggerReconnect()
				}
				return
			}

			timeout := m.cfg.Heartbeat.Timeout
			if timeout <= 0 {
				timeout = 10 * time.Second
			}

			timer := time.NewTimer(timeout)
			select {
			case response, ok := <-responseCh:
				stopTimer(timer)
				m.removePendingRequest(requestID)
				if !ok || response == nil || response.GetHeartbeatAck() == nil {
					m.logger.Warn("heartbeat response was invalid", "request_id", requestID)
					m.triggerReconnect()
					return
				}
			case <-timer.C:
				m.removePendingRequest(requestID)
				m.logger.Warn("heartbeat acknowledgement timed out", "request_id", requestID, "timeout", timeout)
				m.triggerReconnect()
				return
			case <-ctx.Done():
				stopTimer(timer)
				m.removePendingRequest(requestID)
				return
			}
		}
	}
}

func (m *Manager) reconnect(ctx context.Context) {
	m.logger.Info("reconnecting to manager", "address", m.cfg.Manager.Address)
	_ = m.closeConnection()

	initialBackoff := m.cfg.Reconnect.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = time.Second
	}

	maxBackoff := m.cfg.Reconnect.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}

	multiplier := m.cfg.Reconnect.Multiplier
	if multiplier <= 0 {
		multiplier = 2
	}

	for attempt := 0; ; attempt++ {
		backoff := backoffDuration(initialBackoff, maxBackoff, multiplier, attempt)
		m.logger.Info("waiting before reconnect", "attempt", attempt+1, "backoff", backoff)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}

		conn, err := m.dial(ctx)
		if err != nil {
			m.logger.Warn("manager redial failed", "attempt", attempt+1, "error", err)
			continue
		}

		m.mu.Lock()
		m.conn = conn
		m.mu.Unlock()

		stream, err := m.startStream(ctx)
		if err != nil {
			_ = conn.Close()
			m.logger.Warn("manager stream restart failed", "attempt", attempt+1, "error", err)
			continue
		}

		m.mu.Lock()
		m.stream = stream
		m.mu.Unlock()

		m.logger.Info("reconnected to manager", "address", m.cfg.Manager.Address)
		return
	}
}

func (m *Manager) sendRequest(ctx context.Context, msg *agentpb.AgentMessage) (*agentpb.ManagerMessage, error) {
	requestID := generateRequestID()
	if !setAgentRequestID(msg, requestID) {
		return nil, fmt.Errorf("agent message does not support request_id")
	}

	responseCh := make(chan *agentpb.ManagerMessage, 1)
	m.registerPendingRequest(requestID, responseCh)
	defer m.removePendingRequest(requestID)

	m.mu.Lock()
	stream := m.stream
	if stream == nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("manager is not connected")
	}
	err := stream.Send(msg)
	m.mu.Unlock()
	if err != nil {
		m.triggerReconnect()
		return nil, fmt.Errorf("sending request %s: %w", requestID, err)
	}

	select {
	case response, ok := <-responseCh:
		if !ok || response == nil {
			return nil, fmt.Errorf("connection lost while waiting for request %s", requestID)
		}
		return response, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for request %s response: %w", requestID, ctx.Err())
	}
}

// StartLogStream starts streaming logs for a container.
// The agent reads log lines from the Podman socket and sends them as ContainerLogsChunk messages.
func (m *Manager) StartLogStream(ctx context.Context, requestID, containerID string, tail int) error {
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("request_id is required")
	}
	if strings.TrimSpace(containerID) == "" {
		return fmt.Errorf("container_id is required")
	}
	if tail < 0 {
		tail = 0
	}

	streamCtx, cancel := context.WithCancel(ctx)
	m.streamMu.Lock()
	if existing := m.logStreams[requestID]; existing != nil {
		existing()
	}
	m.logStreams[requestID] = cancel
	m.streamMu.Unlock()

	go m.runLogStream(streamCtx, requestID, containerID, tail)
	return nil
}

// StopLogStream stops streaming logs for a container.
func (m *Manager) StopLogStream(requestID string) {
	m.streamMu.Lock()
	cancel := m.logStreams[requestID]
	delete(m.logStreams, requestID)
	m.streamMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// StartEventStream starts streaming Podman events.
func (m *Manager) StartEventStream(ctx context.Context, requestID string) error {
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("request_id is required")
	}

	streamCtx, cancel := context.WithCancel(ctx)
	m.streamMu.Lock()
	if existing := m.eventStreams[requestID]; existing != nil {
		existing()
	}
	m.eventStreams[requestID] = cancel
	m.streamMu.Unlock()

	go m.runEventStream(streamCtx, requestID)
	return nil
}

// StopEventStream stops streaming events.
func (m *Manager) StopEventStream(requestID string) {
	m.streamMu.Lock()
	cancel := m.eventStreams[requestID]
	delete(m.eventStreams, requestID)
	m.streamMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) runLogStream(ctx context.Context, requestID, containerID string, tail int) {
	defer m.StopLogStream(requestID)

	lines := make(chan string, 256)
	errCh := make(chan error, 1)
	go func() {
		errCh <- m.podman.StreamContainerLogs(ctx, containerID, podman.LogOptions{Stdout: true, Stderr: true, Tail: tail}, lines)
		close(lines)
	}()

	entries := make([]*agentpb.LogEntry, 0, 50)
	flush := func() bool {
		if len(entries) == 0 {
			return true
		}
		chunk := &agentpb.AgentMessage{Type: &agentpb.AgentMessage_ContainerLogsChunk{ContainerLogsChunk: &agentpb.ContainerLogsChunk{RequestId: requestID, ContainerId: containerID, Entries: entries}}}
		entries = make([]*agentpb.LogEntry, 0, 50)
		if err := m.sendStreamMessage(chunk); err != nil {
			m.logger.Warn("sending container log chunk failed", "request_id", requestID, "container_id", containerID, "error", err)
			return false
		}
		return true
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case line, ok := <-lines:
			if !ok {
				flush()
				lines = nil
				continue
			}
			entries = append(entries, &agentpb.LogEntry{Timestamp: timestamppb.Now(), Message: line})
			if len(entries) >= 50 && !flush() {
				return
			}
		case err := <-errCh:
			flush()
			if err != nil && ctx.Err() == nil {
				_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_ContainerLogsError{ContainerLogsError: &agentpb.ContainerLogsError{RequestId: requestID, ContainerId: containerID, Error: err.Error()}}})
			}
			return
		}
	}
}

func (m *Manager) runEventStream(ctx context.Context, requestID string) {
	defer m.StopEventStream(requestID)

	events := make(chan string, 256)
	errCh := make(chan error, 1)
	go func() {
		errCh <- m.podman.Events(ctx, time.Time{}, events)
		close(events)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case raw, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			event, err := podmanEventFromJSON(raw)
			if err != nil {
				m.logger.Warn("failed to parse podman event", "request_id", requestID, "error", err)
				continue
			}
			if err := m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_EventStreamData{EventStreamData: &agentpb.EventStreamData{RequestId: requestID, Event: event}}}); err != nil {
				m.logger.Warn("sending podman event failed", "request_id", requestID, "error", err)
				return
			}
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_EventStreamError{EventStreamError: &agentpb.EventStreamError{RequestId: requestID, Error: err.Error()}}})
			}
			return
		}
	}
}

func (m *Manager) sendStreamMessage(msg *agentpb.AgentMessage) error {
	m.mu.Lock()
	stream := m.stream
	if stream == nil {
		m.mu.Unlock()
		return fmt.Errorf("manager is not connected")
	}
	err := stream.Send(msg)
	m.mu.Unlock()
	if err != nil {
		m.triggerReconnect()
	}
	return err
}

func (m *Manager) handleStreamCommand(ctx context.Context, msg *agentpb.ManagerMessage) bool {
	if msg == nil {
		return false
	}
	errResp := msg.GetErrorResponse()
	if errResp == nil {
		return false
	}

	switch errResp.Code {
	case "container_list":
		all, _ := strconv.ParseBool(errResp.Details)
		containers, err := m.listContainers(ctx, all)
		if err != nil {
			_ = m.sendStreamMessage(errorAgentMessage(errResp.RequestId, err))
			return true
		}
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_ContainerListResponse{ContainerListResponse: &agentpb.ContainerListResponse{RequestId: errResp.RequestId, Containers: containers}}})
		return true
	case "volume_list":
		volumes, err := m.podman.ListVolumes(ctx)
		if err != nil {
			_ = m.sendStreamMessage(errorAgentMessage(errResp.RequestId, err))
			return true
		}
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_VolumeListResponse{VolumeListResponse: &agentpb.VolumeListResponse{RequestId: errResp.RequestId, Volumes: volumesToProto(volumes)}}})
		return true
	case "volume_create":
		var req agentpb.VolumeCreateRequest
		if err := json.Unmarshal([]byte(errResp.Details), &req); err != nil {
			_ = m.sendStreamMessage(errorAgentMessage(errResp.RequestId, err))
			return true
		}
		_, err := m.podman.CreateVolume(ctx, &podman.VolumeCreateSpec{Name: req.Name, Driver: req.Driver, Labels: req.Labels, Options: req.Options})
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_VolumeActionResponse{VolumeActionResponse: &agentpb.VolumeActionResponse{RequestId: errResp.RequestId, Name: req.Name, Result: actionResult(err, "volume created")}}})
		return true
	case "volume_remove":
		force, _ := strconv.ParseBool(errResp.Details)
		err := m.podman.RemoveVolume(ctx, errResp.Message, force)
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_VolumeActionResponse{VolumeActionResponse: &agentpb.VolumeActionResponse{RequestId: errResp.RequestId, Name: errResp.Message, Result: actionResult(err, "volume removed")}}})
		return true
	case "volume_prune":
		result, err := m.podman.PruneVolumes(ctx)
		pruned := []string(nil)
		if result != nil {
			pruned = result.Deleted
		}
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_VolumePruneResponse{VolumePruneResponse: &agentpb.VolumePruneResponse{RequestId: errResp.RequestId, VolumesDeleted: pruned, Result: actionResult(err, "volumes pruned")}}})
		return true
	case "network_list":
		networks, err := m.podman.ListNetworks(ctx)
		if err != nil {
			_ = m.sendStreamMessage(errorAgentMessage(errResp.RequestId, err))
			return true
		}
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_NetworkListResponse{NetworkListResponse: &agentpb.NetworkListResponse{RequestId: errResp.RequestId, Networks: networksToProto(networks)}}})
		return true
	case "network_create":
		var req agentpb.NetworkCreateRequest
		if err := json.Unmarshal([]byte(errResp.Details), &req); err != nil {
			_ = m.sendStreamMessage(errorAgentMessage(errResp.RequestId, err))
			return true
		}
		_, err := m.podman.CreateNetwork(ctx, &podman.NetworkCreateSpec{Name: req.Name, Driver: req.Driver, Subnet: req.Subnet, Gateway: req.Gateway, IPv6Enabled: req.Ipv6, Labels: req.Labels, Options: req.Options})
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_NetworkActionResponse{NetworkActionResponse: &agentpb.NetworkActionResponse{RequestId: errResp.RequestId, NetworkName: req.Name, Result: actionResult(err, "network created")}}})
		return true
	case "network_remove":
		err := m.podman.RemoveNetwork(ctx, errResp.Message)
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_NetworkActionResponse{NetworkActionResponse: &agentpb.NetworkActionResponse{RequestId: errResp.RequestId, NetworkName: errResp.Message, Result: actionResult(err, "network removed")}}})
		return true
	case "network_prune":
		result, err := m.podman.PruneNetworks(ctx)
		pruned := []string(nil)
		if result != nil {
			pruned = result.Deleted
		}
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_NetworkPruneResponse{NetworkPruneResponse: &agentpb.NetworkPruneResponse{RequestId: errResp.RequestId, NetworksDeleted: pruned, Result: actionResult(err, "networks pruned")}}})
		return true
	case "network_connect":
		err := m.podman.ConnectNetwork(ctx, errResp.Message, errResp.Details)
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_NetworkActionResponse{NetworkActionResponse: &agentpb.NetworkActionResponse{RequestId: errResp.RequestId, NetworkName: errResp.Message, ContainerId: errResp.Details, Result: actionResult(err, "network connected")}}})
		return true
	case "network_disconnect":
		err := m.podman.DisconnectNetwork(ctx, errResp.Message, errResp.Details, true)
		_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_NetworkActionResponse{NetworkActionResponse: &agentpb.NetworkActionResponse{RequestId: errResp.RequestId, NetworkName: errResp.Message, ContainerId: errResp.Details, Result: actionResult(err, "network disconnected")}}})
		return true
	case "container_logs_start":
		tail, _ := strconv.Atoi(errResp.Details)
		if err := m.StartLogStream(ctx, errResp.RequestId, errResp.Message, tail); err != nil {
			_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_ContainerLogsError{ContainerLogsError: &agentpb.ContainerLogsError{RequestId: errResp.RequestId, ContainerId: errResp.Message, Error: err.Error()}}})
		}
		return true
	case "container_logs_stop":
		m.StopLogStream(errResp.RequestId)
		return true
	case "event_stream_start":
		if err := m.StartEventStream(ctx, errResp.RequestId); err != nil {
			_ = m.sendStreamMessage(&agentpb.AgentMessage{Type: &agentpb.AgentMessage_EventStreamError{EventStreamError: &agentpb.EventStreamError{RequestId: errResp.RequestId, Error: err.Error()}}})
		}
		return true
	case "event_stream_stop":
		m.StopEventStream(errResp.RequestId)
		return true
	default:
		return false
	}
}

func (m *Manager) cancelAllStreams() {
	m.streamMu.Lock()
	logCancels := make([]context.CancelFunc, 0, len(m.logStreams))
	for requestID, cancel := range m.logStreams {
		logCancels = append(logCancels, cancel)
		delete(m.logStreams, requestID)
	}
	eventCancels := make([]context.CancelFunc, 0, len(m.eventStreams))
	for requestID, cancel := range m.eventStreams {
		eventCancels = append(eventCancels, cancel)
		delete(m.eventStreams, requestID)
	}
	m.streamMu.Unlock()

	for _, cancel := range logCancels {
		cancel()
	}
	for _, cancel := range eventCancels {
		cancel()
	}
}

func podmanEventFromJSON(raw string) (*agentpb.PodmanEvent, error) {
	var payload struct {
		Type     string `json:"Type"`
		Action   string `json:"Action"`
		Status   string `json:"status"`
		ID       string `json:"id"`
		Time     int64  `json:"time"`
		TimeNano int64  `json:"timeNano"`
		Actor    struct {
			ID         string            `json:"ID"`
			Attributes map[string]string `json:"Attributes"`
		} `json:"Actor"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	eventType := payload.Action
	if eventType == "" {
		eventType = payload.Status
	}
	if payload.Type != "" && eventType != "" {
		eventType = payload.Type + ":" + eventType
	} else if payload.Type != "" {
		eventType = payload.Type
	}

	actorID := payload.Actor.ID
	if actorID == "" {
		actorID = payload.ID
	}
	actorName := payload.Actor.Attributes["name"]
	if actorName == "" {
		actorName = payload.Actor.Attributes["containerName"]
	}

	timestamp := time.Unix(payload.Time, 0)
	if payload.TimeNano > 0 {
		timestamp = time.Unix(0, payload.TimeNano)
	}
	if timestamp.IsZero() || payload.Time == 0 && payload.TimeNano == 0 {
		timestamp = time.Now()
	}

	return &agentpb.PodmanEvent{EventType: eventType, ActorId: actorID, ActorName: actorName, Timestamp: timestamppb.New(timestamp), Attributes: payload.Actor.Attributes}, nil
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (m *Manager) triggerReconnect() {
	select {
	case m.reconnectCh <- struct{}{}:
	default:
	}
}

func (m *Manager) registerPendingRequest(requestID string, responseCh chan *agentpb.ManagerMessage) {
	m.requestMu.Lock()
	m.pendingRequests[requestID] = responseCh
	m.requestMu.Unlock()
}

func (m *Manager) removePendingRequest(requestID string) {
	m.requestMu.Lock()
	delete(m.pendingRequests, requestID)
	m.requestMu.Unlock()
}

func (m *Manager) failPendingRequests() {
	m.requestMu.Lock()
	for requestID, responseCh := range m.pendingRequests {
		select {
		case responseCh <- nil:
		default:
		}
		delete(m.pendingRequests, requestID)
	}
	m.requestMu.Unlock()
}

func backoffDuration(initial, max time.Duration, multiplier float64, attempt int) time.Duration {
	factor := math.Pow(multiplier, float64(attempt))
	backoff := time.Duration(float64(initial) * factor)
	if backoff > max || backoff <= 0 {
		backoff = max
	}

	jitter := 0.5 + rand.Float64()*0.5
	return time.Duration(float64(backoff) * jitter)
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}

	select {
	case <-timer.C:
	default:
	}
}

func setAgentRequestID(msg *agentpb.AgentMessage, requestID string) bool {
	if msg == nil {
		return false
	}

	switch payload := msg.Type.(type) {
	case *agentpb.AgentMessage_Heartbeat:
		payload.Heartbeat.RequestId = requestID
	case *agentpb.AgentMessage_ContainerListRequest:
		payload.ContainerListRequest.RequestId = requestID
	case *agentpb.AgentMessage_ContainerInspectRequest:
		payload.ContainerInspectRequest.RequestId = requestID
	case *agentpb.AgentMessage_ContainerStartRequest:
		payload.ContainerStartRequest.RequestId = requestID
	case *agentpb.AgentMessage_ContainerStopRequest:
		payload.ContainerStopRequest.RequestId = requestID
	case *agentpb.AgentMessage_ContainerRestartRequest:
		payload.ContainerRestartRequest.RequestId = requestID
	case *agentpb.AgentMessage_ContainerRemoveRequest:
		payload.ContainerRemoveRequest.RequestId = requestID
	case *agentpb.AgentMessage_ContainerCreateRequest:
		payload.ContainerCreateRequest.RequestId = requestID
	case *agentpb.AgentMessage_ContainerLogsRequest:
		payload.ContainerLogsRequest.RequestId = requestID
	case *agentpb.AgentMessage_ImageListRequest:
		payload.ImageListRequest.RequestId = requestID
	case *agentpb.AgentMessage_ImagePullRequest:
		payload.ImagePullRequest.RequestId = requestID
	case *agentpb.AgentMessage_ImageRemoveRequest:
		payload.ImageRemoveRequest.RequestId = requestID
	case *agentpb.AgentMessage_ImagePruneRequest:
		payload.ImagePruneRequest.RequestId = requestID
	case *agentpb.AgentMessage_VolumeListRequest:
		payload.VolumeListRequest.RequestId = requestID
	case *agentpb.AgentMessage_VolumeCreateRequest:
		payload.VolumeCreateRequest.RequestId = requestID
	case *agentpb.AgentMessage_VolumeRemoveRequest:
		payload.VolumeRemoveRequest.RequestId = requestID
	case *agentpb.AgentMessage_VolumePruneRequest:
		payload.VolumePruneRequest.RequestId = requestID
	case *agentpb.AgentMessage_NetworkListRequest:
		payload.NetworkListRequest.RequestId = requestID
	case *agentpb.AgentMessage_NetworkCreateRequest:
		payload.NetworkCreateRequest.RequestId = requestID
	case *agentpb.AgentMessage_NetworkRemoveRequest:
		payload.NetworkRemoveRequest.RequestId = requestID
	case *agentpb.AgentMessage_NetworkPruneRequest:
		payload.NetworkPruneRequest.RequestId = requestID
	case *agentpb.AgentMessage_NetworkConnectRequest:
		payload.NetworkConnectRequest.RequestId = requestID
	case *agentpb.AgentMessage_NetworkDisconnectRequest:
		payload.NetworkDisconnectRequest.RequestId = requestID
	case *agentpb.AgentMessage_HostInfoRequest:
		payload.HostInfoRequest.RequestId = requestID
	case *agentpb.AgentMessage_EventStreamSubscribe:
		payload.EventStreamSubscribe.RequestId = requestID
	case *agentpb.AgentMessage_EventStreamUnsubscribe:
		payload.EventStreamUnsubscribe.RequestId = requestID
	default:
		return false
	}

	return true
}

func managerRequestID(msg *agentpb.ManagerMessage) string {
	if msg == nil {
		return ""
	}

	switch payload := msg.Type.(type) {
	case *agentpb.ManagerMessage_HeartbeatAck:
		return payload.HeartbeatAck.GetRequestId()
	case *agentpb.ManagerMessage_ContainerListResponse:
		return payload.ContainerListResponse.GetRequestId()
	case *agentpb.ManagerMessage_ContainerInspectResponse:
		return payload.ContainerInspectResponse.GetRequestId()
	case *agentpb.ManagerMessage_ContainerActionResponse:
		return payload.ContainerActionResponse.GetRequestId()
	case *agentpb.ManagerMessage_ContainerCreateResponse:
		return payload.ContainerCreateResponse.GetRequestId()
	case *agentpb.ManagerMessage_ContainerLogsChunk:
		return payload.ContainerLogsChunk.GetRequestId()
	case *agentpb.ManagerMessage_ContainerLogsError:
		return payload.ContainerLogsError.GetRequestId()
	case *agentpb.ManagerMessage_ImageListResponse:
		return payload.ImageListResponse.GetRequestId()
	case *agentpb.ManagerMessage_ImageActionResponse:
		return payload.ImageActionResponse.GetRequestId()
	case *agentpb.ManagerMessage_ImagePruneResponse:
		return payload.ImagePruneResponse.GetRequestId()
	case *agentpb.ManagerMessage_VolumeListResponse:
		return payload.VolumeListResponse.GetRequestId()
	case *agentpb.ManagerMessage_VolumeActionResponse:
		return payload.VolumeActionResponse.GetRequestId()
	case *agentpb.ManagerMessage_VolumePruneResponse:
		return payload.VolumePruneResponse.GetRequestId()
	case *agentpb.ManagerMessage_NetworkListResponse:
		return payload.NetworkListResponse.GetRequestId()
	case *agentpb.ManagerMessage_NetworkActionResponse:
		return payload.NetworkActionResponse.GetRequestId()
	case *agentpb.ManagerMessage_NetworkPruneResponse:
		return payload.NetworkPruneResponse.GetRequestId()
	case *agentpb.ManagerMessage_HostInfoResponse:
		return payload.HostInfoResponse.GetRequestId()
	case *agentpb.ManagerMessage_EventStreamData:
		return payload.EventStreamData.GetRequestId()
	case *agentpb.ManagerMessage_EventStreamError:
		return payload.EventStreamError.GetRequestId()
	case *agentpb.ManagerMessage_ErrorResponse:
		return payload.ErrorResponse.GetRequestId()
	default:
		return ""
	}
}
