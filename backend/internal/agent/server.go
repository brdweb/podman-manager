package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	agentpb "github.com/brdweb/podman-manager/agent/proto"
	"github.com/brdweb/podman-manager/internal/enroll"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the AgentService gRPC service.
type Server struct {
	agentpb.UnimplementedAgentServiceServer
	registry    *Registry
	logger      *slog.Logger
	enrollStore *enroll.Store

	credentials   map[string]string
	credentialsMu sync.RWMutex

	// Pending commands per agent
	commands   map[string]chan *AgentMessage // agentID -> channel
	commandsMu sync.RWMutex

	streamCallbacks   map[string]chan *ManagerMessage
	streamCallbacksMu sync.RWMutex
	sendMu            sync.Mutex
}

func NewServer(registry *Registry, logger *slog.Logger, enrollStore *enroll.Store) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if registry == nil {
		registry = NewRegistry(logger)
	}
	if enrollStore == nil {
		enrollStore = enroll.NewStore(time.Hour)
	}
	return &Server{
		registry:        registry,
		logger:          logger,
		enrollStore:     enrollStore,
		credentials:     make(map[string]string),
		commands:        make(map[string]chan *AgentMessage),
		streamCallbacks: make(map[string]chan *ManagerMessage),
	}
}

// Connect handles the bidirectional stream from an agent.
func (s *Server) Connect(stream agentpb.AgentService_ConnectServer) error {
	agentID, credential, hostname := credentialsFromMetadata(stream.Context())
	if agentID == "" || credential == "" || !s.validCredential(agentID, credential) {
		return status.Error(codes.Unauthenticated, "invalid agent credentials")
	}

	now := time.Now()
	agent := &AgentInfo{ID: agentID, Hostname: hostname, ConnectedAt: now, LastHeartbeat: now, Stream: stream}
	s.registry.Register(agent)
	defer s.registry.Unregister(agentID)

	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(stream.Context().Err(), context.Canceled) {
				return nil
			}
			return err
		}
		if msg == nil {
			continue
		}
		if heartbeat := msg.GetHeartbeat(); heartbeat != nil {
			s.registry.UpdateHeartbeat(agentID)
			s.sendMu.Lock()
			err := stream.Send(&ManagerMessage{Type: &agentpb.ManagerMessage_HeartbeatAck{HeartbeatAck: &agentpb.HeartbeatAck{RequestId: heartbeat.RequestId, Timestamp: timestamppb.Now()}}})
			s.sendMu.Unlock()
			if err != nil {
				return err
			}
			continue
		}
		s.routeResponse(agentID, msg)
	}
}

// Enroll handles agent enrollment (unary RPC).
func (s *Server) Enroll(ctx context.Context, req *agentpb.EnrollRequest) (*agentpb.EnrollResponse, error) {
	if req == nil || strings.TrimSpace(req.Token) == "" {
		return nil, status.Error(codes.InvalidArgument, "enrollment token is required")
	}
	token := strings.TrimSpace(req.Token)
	if _, err := s.enrollStore.ValidateToken(token); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid enrollment token: %v", err)
	}

	agentID, err := randomID("agent")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generating agent id: %v", err)
	}
	credential, err := randomHex(64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generating credential: %v", err)
	}
	if err := s.enrollStore.ConsumeToken(token, agentID); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid enrollment token: %v", err)
	}

	now := time.Now()
	if err := s.enrollStore.RegisterCredential(&enroll.AgentCredential{
		AgentID:    agentID,
		Hostname:   strings.TrimSpace(req.HostId),
		Credential: credential,
		CreatedAt:  now,
		LastSeen:   now,
		Active:     true,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "storing agent credential: %v", err)
	}

	s.credentialsMu.Lock()
	s.credentials[agentID] = credential
	s.credentialsMu.Unlock()

	s.logger.Info("agent enrolled", "agent_id", agentID, "host_id", req.HostId, "agent_version", req.AgentVersion)
	return &agentpb.EnrollResponse{Success: true, AgentId: agentID, Credential: credential}, nil
}

// SendCommand sends a command to a specific agent and waits for response.
func (s *Server) SendCommand(ctx context.Context, agentID string, cmd *AgentMessage) (*ManagerMessage, error) {
	agent, ok := s.registry.Get(agentID)
	if !ok || agent.Stream == nil {
		return nil, fmt.Errorf("agent %s is not connected", agentID)
	}

	responseCh := make(chan *AgentMessage, 1)
	s.commandsMu.Lock()
	if _, exists := s.commands[agentID]; exists {
		s.commandsMu.Unlock()
		return nil, fmt.Errorf("agent %s already has a pending command", agentID)
	}
	s.commands[agentID] = responseCh
	s.commandsMu.Unlock()
	defer func() {
		s.commandsMu.Lock()
		delete(s.commands, agentID)
		s.commandsMu.Unlock()
	}()

	s.sendMu.Lock()
	err := agent.Stream.Send(commandEnvelope(cmd))
	s.sendMu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-responseCh:
		return managerMessageFromAgentMessage(msg)
	}
}

func (s *Server) SendStreamCommand(ctx context.Context, agentID string, msg *ManagerMessage) error {
	agent, ok := s.registry.Get(agentID)
	if !ok || agent.Stream == nil {
		return fmt.Errorf("agent %s is not connected", agentID)
	}

	done := make(chan error, 1)
	go func() {
		s.sendMu.Lock()
		defer s.sendMu.Unlock()
		done <- agent.Stream.Send(msg)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (s *Server) RegisterStreamCallback(requestID string, ch chan *ManagerMessage) {
	s.streamCallbacksMu.Lock()
	s.streamCallbacks[requestID] = ch
	s.streamCallbacksMu.Unlock()
}

func (s *Server) UnregisterStreamCallback(requestID string) {
	s.streamCallbacksMu.Lock()
	delete(s.streamCallbacks, requestID)
	s.streamCallbacksMu.Unlock()
}

// StartGRPCServer starts the gRPC server on the given address.
func StartGRPCServer(address string, agentServer *Server, logger *slog.Logger) (*grpc.Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	grpcServer := grpc.NewServer()
	agentpb.RegisterAgentServiceServer(grpcServer, agentServer)
	logger.Info("agent gRPC server starting", "address", address)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			logger.Error("agent gRPC server stopped", "error", err)
		}
	}()
	return grpcServer, nil
}

func (s *Server) routeResponse(agentID string, msg *AgentMessage) {
	requestID := requestID(msg)
	if requestID != "" {
		s.streamCallbacksMu.RLock()
		callback := s.streamCallbacks[requestID]
		s.streamCallbacksMu.RUnlock()
		if callback != nil {
			managerMsg, err := managerMessageFromAgentMessage(msg)
			if err != nil {
				s.logger.Warn("failed to convert agent stream message", "agent_id", agentID, "request_id", requestID, "error", err)
				return
			}
			select {
			case callback <- managerMsg:
			default:
				s.logger.Warn("stream callback channel full", "agent_id", agentID, "request_id", requestID)
			}
			return
		}
	}

	s.commandsMu.RLock()
	ch := s.commands[agentID]
	s.commandsMu.RUnlock()
	if ch == nil {
		s.logger.Debug("dropping unmatched agent message", "agent_id", agentID)
		return
	}
	select {
	case ch <- msg:
	default:
		s.logger.Warn("pending agent command channel full", "agent_id", agentID)
	}
}

func (s *Server) validCredential(agentID, credential string) bool {
	if stored, ok := s.enrollStore.GetCredential(agentID); ok {
		return stored.Active && stored.Credential == credential
	}
	if stored, ok := s.enrollStore.GetCredentialByToken(credential); ok {
		return stored.Active && stored.AgentID == agentID
	}

	s.credentialsMu.RLock()
	defer s.credentialsMu.RUnlock()
	stored, ok := s.credentials[agentID]
	return ok && stored == credential
}

func credentialsFromMetadata(ctx context.Context) (agentID, credential, hostname string) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", ""
	}
	agentID = firstMetadata(md, "agent-id")
	credential = firstMetadata(md, "agent-credential", "authorization")
	credential = strings.TrimPrefix(credential, "Bearer ")
	hostname = firstMetadata(md, "agent-hostname", "hostname")
	return agentID, credential, hostname
}

func firstMetadata(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		values := md.Get(key)
		if len(values) > 0 {
			return strings.TrimSpace(values[0])
		}
	}
	return ""
}

func randomID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf), nil
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func commandEnvelope(cmd *AgentMessage) *ManagerMessage {
	errResp := &agentpb.ErrorResponse{RequestId: requestID(cmd)}
	switch m := cmd.Type.(type) {
	case *agentpb.AgentMessage_ContainerListRequest:
		errResp.Code = "container_list"
		errResp.Details = fmt.Sprintf("%t", m.ContainerListRequest.All)
	case *agentpb.AgentMessage_VolumeListRequest:
		errResp.Code = "volume_list"
	case *agentpb.AgentMessage_VolumeCreateRequest:
		errResp.Code = "volume_create"
		errResp.Message = m.VolumeCreateRequest.Name
		errResp.Details = mustCommandJSON(m.VolumeCreateRequest)
	case *agentpb.AgentMessage_VolumeRemoveRequest:
		errResp.Code = "volume_remove"
		errResp.Message = m.VolumeRemoveRequest.Name
		errResp.Details = fmt.Sprintf("%t", m.VolumeRemoveRequest.Force)
	case *agentpb.AgentMessage_VolumePruneRequest:
		errResp.Code = "volume_prune"
	case *agentpb.AgentMessage_NetworkListRequest:
		errResp.Code = "network_list"
	case *agentpb.AgentMessage_NetworkCreateRequest:
		errResp.Code = "network_create"
		errResp.Message = m.NetworkCreateRequest.Name
		errResp.Details = mustCommandJSON(m.NetworkCreateRequest)
	case *agentpb.AgentMessage_NetworkRemoveRequest:
		errResp.Code = "network_remove"
		errResp.Message = m.NetworkRemoveRequest.Name
	case *agentpb.AgentMessage_NetworkPruneRequest:
		errResp.Code = "network_prune"
	case *agentpb.AgentMessage_NetworkConnectRequest:
		errResp.Code = "network_connect"
		errResp.Message = m.NetworkConnectRequest.NetworkName
		errResp.Details = m.NetworkConnectRequest.ContainerId
	case *agentpb.AgentMessage_NetworkDisconnectRequest:
		errResp.Code = "network_disconnect"
		errResp.Message = m.NetworkDisconnectRequest.NetworkName
		errResp.Details = m.NetworkDisconnectRequest.ContainerId
	default:
		errResp.Code = "command"
		errResp.Message = requestID(cmd)
	}
	return &ManagerMessage{Type: &agentpb.ManagerMessage_ErrorResponse{ErrorResponse: errResp}}
}

func mustCommandJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func managerMessageFromAgentMessage(msg *AgentMessage) (*ManagerMessage, error) {
	if msg == nil || msg.Type == nil {
		return nil, fmt.Errorf("empty agent message")
	}
	switch m := msg.Type.(type) {
	case *agentpb.AgentMessage_ContainerListResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_ContainerListResponse{ContainerListResponse: m.ContainerListResponse}}, nil
	case *agentpb.AgentMessage_VolumeListResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_VolumeListResponse{VolumeListResponse: m.VolumeListResponse}}, nil
	case *agentpb.AgentMessage_VolumeActionResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_VolumeActionResponse{VolumeActionResponse: m.VolumeActionResponse}}, nil
	case *agentpb.AgentMessage_VolumePruneResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_VolumePruneResponse{VolumePruneResponse: m.VolumePruneResponse}}, nil
	case *agentpb.AgentMessage_NetworkListResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_NetworkListResponse{NetworkListResponse: m.NetworkListResponse}}, nil
	case *agentpb.AgentMessage_NetworkActionResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_NetworkActionResponse{NetworkActionResponse: m.NetworkActionResponse}}, nil
	case *agentpb.AgentMessage_NetworkPruneResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_NetworkPruneResponse{NetworkPruneResponse: m.NetworkPruneResponse}}, nil
	case *agentpb.AgentMessage_ErrorResponse:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_ErrorResponse{ErrorResponse: m.ErrorResponse}}, nil
	case *agentpb.AgentMessage_ContainerLogsChunk:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_ContainerLogsChunk{ContainerLogsChunk: m.ContainerLogsChunk}}, nil
	case *agentpb.AgentMessage_ContainerLogsError:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_ContainerLogsError{ContainerLogsError: m.ContainerLogsError}}, nil
	case *agentpb.AgentMessage_EventStreamData:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_EventStreamData{EventStreamData: m.EventStreamData}}, nil
	case *agentpb.AgentMessage_EventStreamError:
		return &ManagerMessage{Type: &agentpb.ManagerMessage_EventStreamError{EventStreamError: m.EventStreamError}}, nil
	}
	return &ManagerMessage{Type: &agentpb.ManagerMessage_ErrorResponse{ErrorResponse: &agentpb.ErrorResponse{Code: "unsupported_response", Message: requestID(msg)}}}, nil
}

func requestID(msg *AgentMessage) string {
	if msg == nil || msg.Type == nil {
		return ""
	}
	switch m := msg.Type.(type) {
	case *agentpb.AgentMessage_Heartbeat:
		return m.Heartbeat.RequestId
	case *agentpb.AgentMessage_ContainerListRequest:
		return m.ContainerListRequest.RequestId
	case *agentpb.AgentMessage_ContainerInspectRequest:
		return m.ContainerInspectRequest.RequestId
	case *agentpb.AgentMessage_ContainerStartRequest:
		return m.ContainerStartRequest.RequestId
	case *agentpb.AgentMessage_ContainerStopRequest:
		return m.ContainerStopRequest.RequestId
	case *agentpb.AgentMessage_ContainerRestartRequest:
		return m.ContainerRestartRequest.RequestId
	case *agentpb.AgentMessage_ContainerRemoveRequest:
		return m.ContainerRemoveRequest.RequestId
	case *agentpb.AgentMessage_ContainerCreateRequest:
		return m.ContainerCreateRequest.RequestId
	case *agentpb.AgentMessage_ContainerLogsRequest:
		return m.ContainerLogsRequest.RequestId
	case *agentpb.AgentMessage_ImageListRequest:
		return m.ImageListRequest.RequestId
	case *agentpb.AgentMessage_ImagePullRequest:
		return m.ImagePullRequest.RequestId
	case *agentpb.AgentMessage_ImageRemoveRequest:
		return m.ImageRemoveRequest.RequestId
	case *agentpb.AgentMessage_ImagePruneRequest:
		return m.ImagePruneRequest.RequestId
	case *agentpb.AgentMessage_VolumeListRequest:
		return m.VolumeListRequest.RequestId
	case *agentpb.AgentMessage_VolumeCreateRequest:
		return m.VolumeCreateRequest.RequestId
	case *agentpb.AgentMessage_VolumeRemoveRequest:
		return m.VolumeRemoveRequest.RequestId
	case *agentpb.AgentMessage_VolumePruneRequest:
		return m.VolumePruneRequest.RequestId
	case *agentpb.AgentMessage_NetworkListRequest:
		return m.NetworkListRequest.RequestId
	case *agentpb.AgentMessage_NetworkCreateRequest:
		return m.NetworkCreateRequest.RequestId
	case *agentpb.AgentMessage_NetworkRemoveRequest:
		return m.NetworkRemoveRequest.RequestId
	case *agentpb.AgentMessage_NetworkPruneRequest:
		return m.NetworkPruneRequest.RequestId
	case *agentpb.AgentMessage_NetworkConnectRequest:
		return m.NetworkConnectRequest.RequestId
	case *agentpb.AgentMessage_NetworkDisconnectRequest:
		return m.NetworkDisconnectRequest.RequestId
	case *agentpb.AgentMessage_HostInfoRequest:
		return m.HostInfoRequest.RequestId
	case *agentpb.AgentMessage_ContainerListResponse:
		return m.ContainerListResponse.RequestId
	case *agentpb.AgentMessage_VolumeListResponse:
		return m.VolumeListResponse.RequestId
	case *agentpb.AgentMessage_VolumeActionResponse:
		return m.VolumeActionResponse.RequestId
	case *agentpb.AgentMessage_VolumePruneResponse:
		return m.VolumePruneResponse.RequestId
	case *agentpb.AgentMessage_NetworkListResponse:
		return m.NetworkListResponse.RequestId
	case *agentpb.AgentMessage_NetworkActionResponse:
		return m.NetworkActionResponse.RequestId
	case *agentpb.AgentMessage_NetworkPruneResponse:
		return m.NetworkPruneResponse.RequestId
	case *agentpb.AgentMessage_ErrorResponse:
		return m.ErrorResponse.RequestId
	case *agentpb.AgentMessage_ContainerLogsChunk:
		return m.ContainerLogsChunk.RequestId
	case *agentpb.AgentMessage_ContainerLogsError:
		return m.ContainerLogsError.RequestId
	case *agentpb.AgentMessage_EventStreamData:
		return m.EventStreamData.RequestId
	case *agentpb.AgentMessage_EventStreamError:
		return m.EventStreamError.RequestId
	}
	return ""
}
