package agentpb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AgentMessage struct{ Type isAgentMessage_Type }

type isAgentMessage_Type interface{ isAgentMessage_Type() }

type AgentMessage_Heartbeat struct{ Heartbeat *Heartbeat }
type AgentMessage_ContainerListRequest struct{ ContainerListRequest *ContainerListRequest }
type AgentMessage_ContainerInspectRequest struct{ ContainerInspectRequest *ContainerInspectRequest }
type AgentMessage_ContainerStartRequest struct{ ContainerStartRequest *ContainerStartRequest }
type AgentMessage_ContainerStopRequest struct{ ContainerStopRequest *ContainerStopRequest }
type AgentMessage_ContainerRestartRequest struct{ ContainerRestartRequest *ContainerRestartRequest }
type AgentMessage_ContainerRemoveRequest struct{ ContainerRemoveRequest *ContainerRemoveRequest }
type AgentMessage_ContainerCreateRequest struct{ ContainerCreateRequest *ContainerCreateRequest }
type AgentMessage_ContainerLogsRequest struct{ ContainerLogsRequest *ContainerLogsRequest }
type AgentMessage_ContainerListResponse struct{ ContainerListResponse *ContainerListResponse }
type AgentMessage_ImageListRequest struct{ ImageListRequest *ImageListRequest }
type AgentMessage_ImagePullRequest struct{ ImagePullRequest *ImagePullRequest }
type AgentMessage_ImageRemoveRequest struct{ ImageRemoveRequest *ImageRemoveRequest }
type AgentMessage_ImagePruneRequest struct{ ImagePruneRequest *ImagePruneRequest }
type AgentMessage_VolumeListRequest struct{ VolumeListRequest *VolumeListRequest }
type AgentMessage_VolumeCreateRequest struct{ VolumeCreateRequest *VolumeCreateRequest }
type AgentMessage_VolumeRemoveRequest struct{ VolumeRemoveRequest *VolumeRemoveRequest }
type AgentMessage_VolumePruneRequest struct{ VolumePruneRequest *VolumePruneRequest }
type AgentMessage_VolumeListResponse struct{ VolumeListResponse *VolumeListResponse }
type AgentMessage_VolumeActionResponse struct{ VolumeActionResponse *VolumeActionResponse }
type AgentMessage_VolumePruneResponse struct{ VolumePruneResponse *VolumePruneResponse }
type AgentMessage_NetworkListRequest struct{ NetworkListRequest *NetworkListRequest }
type AgentMessage_NetworkCreateRequest struct{ NetworkCreateRequest *NetworkCreateRequest }
type AgentMessage_NetworkRemoveRequest struct{ NetworkRemoveRequest *NetworkRemoveRequest }
type AgentMessage_NetworkPruneRequest struct{ NetworkPruneRequest *NetworkPruneRequest }
type AgentMessage_NetworkConnectRequest struct{ NetworkConnectRequest *NetworkConnectRequest }
type AgentMessage_NetworkDisconnectRequest struct{ NetworkDisconnectRequest *NetworkDisconnectRequest }
type AgentMessage_NetworkListResponse struct{ NetworkListResponse *NetworkListResponse }
type AgentMessage_NetworkActionResponse struct{ NetworkActionResponse *NetworkActionResponse }
type AgentMessage_NetworkPruneResponse struct{ NetworkPruneResponse *NetworkPruneResponse }
type AgentMessage_HostInfoRequest struct{ HostInfoRequest *HostInfoRequest }
type AgentMessage_EventStreamSubscribe struct{ EventStreamSubscribe *EventStreamSubscribe }
type AgentMessage_EventStreamUnsubscribe struct{ EventStreamUnsubscribe *EventStreamUnsubscribe }
type AgentMessage_ContainerLogsChunk struct{ ContainerLogsChunk *ContainerLogsChunk }
type AgentMessage_ContainerLogsError struct{ ContainerLogsError *ContainerLogsError }
type AgentMessage_EventStreamData struct{ EventStreamData *EventStreamData }
type AgentMessage_EventStreamError struct{ EventStreamError *EventStreamError }
type AgentMessage_ErrorResponse struct{ ErrorResponse *ErrorResponse }

func (*AgentMessage_Heartbeat) isAgentMessage_Type()                {}
func (*AgentMessage_ContainerListRequest) isAgentMessage_Type()     {}
func (*AgentMessage_ContainerInspectRequest) isAgentMessage_Type()  {}
func (*AgentMessage_ContainerStartRequest) isAgentMessage_Type()    {}
func (*AgentMessage_ContainerStopRequest) isAgentMessage_Type()     {}
func (*AgentMessage_ContainerRestartRequest) isAgentMessage_Type()  {}
func (*AgentMessage_ContainerRemoveRequest) isAgentMessage_Type()   {}
func (*AgentMessage_ContainerCreateRequest) isAgentMessage_Type()   {}
func (*AgentMessage_ContainerLogsRequest) isAgentMessage_Type()     {}
func (*AgentMessage_ContainerListResponse) isAgentMessage_Type()    {}
func (*AgentMessage_ImageListRequest) isAgentMessage_Type()         {}
func (*AgentMessage_ImagePullRequest) isAgentMessage_Type()         {}
func (*AgentMessage_ImageRemoveRequest) isAgentMessage_Type()       {}
func (*AgentMessage_ImagePruneRequest) isAgentMessage_Type()        {}
func (*AgentMessage_VolumeListRequest) isAgentMessage_Type()        {}
func (*AgentMessage_VolumeCreateRequest) isAgentMessage_Type()      {}
func (*AgentMessage_VolumeRemoveRequest) isAgentMessage_Type()      {}
func (*AgentMessage_VolumePruneRequest) isAgentMessage_Type()       {}
func (*AgentMessage_VolumeListResponse) isAgentMessage_Type()       {}
func (*AgentMessage_VolumeActionResponse) isAgentMessage_Type()     {}
func (*AgentMessage_VolumePruneResponse) isAgentMessage_Type()      {}
func (*AgentMessage_NetworkListRequest) isAgentMessage_Type()       {}
func (*AgentMessage_NetworkCreateRequest) isAgentMessage_Type()     {}
func (*AgentMessage_NetworkRemoveRequest) isAgentMessage_Type()     {}
func (*AgentMessage_NetworkPruneRequest) isAgentMessage_Type()      {}
func (*AgentMessage_NetworkConnectRequest) isAgentMessage_Type()    {}
func (*AgentMessage_NetworkDisconnectRequest) isAgentMessage_Type() {}
func (*AgentMessage_NetworkListResponse) isAgentMessage_Type()      {}
func (*AgentMessage_NetworkActionResponse) isAgentMessage_Type()    {}
func (*AgentMessage_NetworkPruneResponse) isAgentMessage_Type()     {}
func (*AgentMessage_HostInfoRequest) isAgentMessage_Type()          {}
func (*AgentMessage_EventStreamSubscribe) isAgentMessage_Type()     {}
func (*AgentMessage_EventStreamUnsubscribe) isAgentMessage_Type()   {}
func (*AgentMessage_ContainerLogsChunk) isAgentMessage_Type()       {}
func (*AgentMessage_ContainerLogsError) isAgentMessage_Type()       {}
func (*AgentMessage_EventStreamData) isAgentMessage_Type()          {}
func (*AgentMessage_EventStreamError) isAgentMessage_Type()         {}
func (*AgentMessage_ErrorResponse) isAgentMessage_Type()            {}

func (m *AgentMessage) GetHeartbeat() *Heartbeat {
	if m == nil {
		return nil
	}
	if x, ok := m.Type.(*AgentMessage_Heartbeat); ok {
		return x.Heartbeat
	}
	return nil
}

func (m *ManagerMessage) GetHeartbeatAck() *HeartbeatAck {
	if m == nil {
		return nil
	}
	if x, ok := m.Type.(*ManagerMessage_HeartbeatAck); ok {
		return x.HeartbeatAck
	}
	return nil
}

func (m *ManagerMessage) GetErrorResponse() *ErrorResponse {
	if m == nil {
		return nil
	}
	if x, ok := m.Type.(*ManagerMessage_ErrorResponse); ok {
		return x.ErrorResponse
	}
	return nil
}

type ManagerMessage struct{ Type isManagerMessage_Type }

type isManagerMessage_Type interface{ isManagerMessage_Type() }

type ManagerMessage_HeartbeatAck struct{ HeartbeatAck *HeartbeatAck }
type ManagerMessage_ContainerListResponse struct{ ContainerListResponse *ContainerListResponse }
type ManagerMessage_ContainerInspectResponse struct{ ContainerInspectResponse *ContainerInspectResponse }
type ManagerMessage_ContainerActionResponse struct{ ContainerActionResponse *ContainerActionResponse }
type ManagerMessage_ContainerCreateResponse struct{ ContainerCreateResponse *ContainerCreateResponse }
type ManagerMessage_ContainerLogsChunk struct{ ContainerLogsChunk *ContainerLogsChunk }
type ManagerMessage_ContainerLogsError struct{ ContainerLogsError *ContainerLogsError }
type ManagerMessage_ImageListResponse struct{ ImageListResponse *ImageListResponse }
type ManagerMessage_ImageActionResponse struct{ ImageActionResponse *ImageActionResponse }
type ManagerMessage_ImagePruneResponse struct{ ImagePruneResponse *ImagePruneResponse }
type ManagerMessage_VolumeListResponse struct{ VolumeListResponse *VolumeListResponse }
type ManagerMessage_VolumeActionResponse struct{ VolumeActionResponse *VolumeActionResponse }
type ManagerMessage_VolumePruneResponse struct{ VolumePruneResponse *VolumePruneResponse }
type ManagerMessage_NetworkListResponse struct{ NetworkListResponse *NetworkListResponse }
type ManagerMessage_NetworkActionResponse struct{ NetworkActionResponse *NetworkActionResponse }
type ManagerMessage_NetworkPruneResponse struct{ NetworkPruneResponse *NetworkPruneResponse }
type ManagerMessage_HostInfoResponse struct{ HostInfoResponse *HostInfoResponse }
type ManagerMessage_EventStreamData struct{ EventStreamData *EventStreamData }
type ManagerMessage_EventStreamError struct{ EventStreamError *EventStreamError }
type ManagerMessage_EnrollChallenge struct{ EnrollChallenge *EnrollChallenge }
type ManagerMessage_ErrorResponse struct{ ErrorResponse *ErrorResponse }

func (*ManagerMessage_HeartbeatAck) isManagerMessage_Type()             {}
func (*ManagerMessage_ContainerListResponse) isManagerMessage_Type()    {}
func (*ManagerMessage_ContainerInspectResponse) isManagerMessage_Type() {}
func (*ManagerMessage_ContainerActionResponse) isManagerMessage_Type()  {}
func (*ManagerMessage_ContainerCreateResponse) isManagerMessage_Type()  {}
func (*ManagerMessage_ContainerLogsChunk) isManagerMessage_Type()       {}
func (*ManagerMessage_ContainerLogsError) isManagerMessage_Type()       {}
func (*ManagerMessage_ImageListResponse) isManagerMessage_Type()        {}
func (*ManagerMessage_ImageActionResponse) isManagerMessage_Type()      {}
func (*ManagerMessage_ImagePruneResponse) isManagerMessage_Type()       {}
func (*ManagerMessage_VolumeListResponse) isManagerMessage_Type()       {}
func (*ManagerMessage_VolumeActionResponse) isManagerMessage_Type()     {}
func (*ManagerMessage_VolumePruneResponse) isManagerMessage_Type()      {}
func (*ManagerMessage_NetworkListResponse) isManagerMessage_Type()      {}
func (*ManagerMessage_NetworkActionResponse) isManagerMessage_Type()    {}
func (*ManagerMessage_NetworkPruneResponse) isManagerMessage_Type()     {}
func (*ManagerMessage_HostInfoResponse) isManagerMessage_Type()         {}
func (*ManagerMessage_EventStreamData) isManagerMessage_Type()          {}
func (*ManagerMessage_EventStreamError) isManagerMessage_Type()         {}
func (*ManagerMessage_EnrollChallenge) isManagerMessage_Type()          {}
func (*ManagerMessage_ErrorResponse) isManagerMessage_Type()            {}

type Heartbeat struct {
	RequestId string
	Timestamp *timestamppb.Timestamp
}
type HeartbeatAck struct {
	RequestId string
	Timestamp *timestamppb.Timestamp
}
type ContainerListRequest struct {
	RequestId string
	All       bool
	Filters   map[string]string
}
type ContainerInspectRequest struct {
	RequestId   string
	ContainerId string
}
type ContainerStartRequest struct {
	RequestId   string
	ContainerId string
}
type ContainerStopRequest struct {
	RequestId      string
	ContainerId    string
	TimeoutSeconds int32
}
type ContainerRestartRequest struct {
	RequestId      string
	ContainerId    string
	TimeoutSeconds int32
}
type ContainerRemoveRequest struct {
	RequestId     string
	ContainerId   string
	Force         bool
	RemoveVolumes bool
}
type ContainerCreateRequest struct {
	RequestId, Name, Image, RestartPolicy, Hostname string
	Env                                             map[string]string
	Ports                                           []*PortMappingRequest
	Volumes                                         []*VolumeMountRequest
	Networks                                        []string
	Labels                                          map[string]string
	Entrypoint, Command                             []string
	CpuLimit                                        float64
	MemoryLimit                                     int64
}
type ContainerLogsRequest struct {
	RequestId              string
	ContainerId            string
	Follow, Stdout, Stderr bool
	Tail                   int32
	Since, Until           *timestamppb.Timestamp
}
type ContainerListResponse struct {
	RequestId  string
	Containers []*Container
}
type ContainerInspectResponse struct {
	RequestId string
	Container *ContainerDetail
}
type ContainerActionResponse struct {
	RequestId, ContainerId string
	Result                 *ActionResult
}
type ContainerCreateResponse struct {
	RequestId, ContainerId string
	Result                 *ActionResult
}
type ContainerLogsChunk struct {
	RequestId, ContainerId string
	Entries                []*LogEntry
}
type ContainerLogsError struct{ RequestId, ContainerId, Error string }
type ImageListRequest struct {
	RequestId string
	All       bool
	Filters   map[string]string
}
type ImagePullRequest struct{ RequestId, Image, Tag, Registry string }
type ImageRemoveRequest struct {
	RequestId, ImageId string
	Force              bool
}
type ImagePruneRequest struct {
	RequestId string
	All       bool
	Filters   map[string]string
}
type ImageListResponse struct {
	RequestId string
	Images    []*Image
}
type ImageActionResponse struct {
	RequestId, ImageId string
	Result             *ActionResult
}
type ImagePruneResponse struct {
	RequestId           string
	ImagesDeleted       []string
	SpaceReclaimedBytes int64
	Result              *ActionResult
}
type VolumeListRequest struct {
	RequestId string
	Filters   map[string]string
}
type VolumeCreateRequest struct {
	RequestId, Name, Driver string
	Labels, Options         map[string]string
}
type VolumeRemoveRequest struct {
	RequestId, Name string
	Force           bool
}
type VolumePruneRequest struct {
	RequestId string
	Filters   map[string]string
}
type VolumeListResponse struct {
	RequestId string
	Volumes   []*Volume
}
type VolumeActionResponse struct {
	RequestId, Name string
	Result          *ActionResult
}
type VolumePruneResponse struct {
	RequestId           string
	VolumesDeleted      []string
	SpaceReclaimedBytes int64
	Result              *ActionResult
}
type NetworkListRequest struct {
	RequestId string
	Filters   map[string]string
}
type NetworkCreateRequest struct {
	RequestId, Name, Driver, Subnet, Gateway string
	Ipv6                                     bool
	Labels, Options                          map[string]string
}
type NetworkRemoveRequest struct {
	RequestId, Name string
	Force           bool
}
type NetworkPruneRequest struct {
	RequestId string
	Filters   map[string]string
}
type NetworkConnectRequest struct{ RequestId, NetworkName, ContainerId, Alias string }
type NetworkDisconnectRequest struct {
	RequestId, NetworkName, ContainerId string
	Force                               bool
}
type NetworkListResponse struct {
	RequestId string
	Networks  []*Network
}
type NetworkActionResponse struct {
	RequestId, NetworkName, ContainerId string
	Result                              *ActionResult
}
type NetworkPruneResponse struct {
	RequestId       string
	NetworksDeleted []string
	Result          *ActionResult
}
type HostInfoRequest struct{ RequestId string }
type HostInfoResponse struct {
	RequestId string
	HostInfo  *HostInfo
}
type EventStreamSubscribe struct {
	RequestId  string
	EventTypes []string
}
type EventStreamUnsubscribe struct{ RequestId string }
type EventStreamData struct {
	RequestId string
	Event     *PodmanEvent
}
type EventStreamError struct{ RequestId, Error string }
type EnrollChallenge struct {
	ChallengeId, Nonce string
	ExpiresAt          *timestamppb.Timestamp
}
type ErrorResponse struct{ RequestId, Code, Message, Details string }
type Container struct {
	Id, Name, Image, State, Status string
	Created                        *timestamppb.Timestamp
	Ports                          []*PortMapping
	Networks                       []*NetworkInfo
	Mounts                         []*MountInfo
	Labels                         map[string]string
	Host, Manager, SystemdUnit     string
}
type ContainerDetail struct {
	Container                            *Container
	Env                                  []string
	Hostname, RestartPolicy, NetworkMode string
	Pid                                  int64
	StartedAt, FinishedAt                *timestamppb.Timestamp
	Stats                                *ContainerStats
}
type ContainerStats struct {
	CpuPercent                                                                     float64
	MemoryUsageBytes, MemoryLimitBytes                                             int64
	MemoryPercent                                                                  float64
	Pids, NetworkInputBytes, NetworkOutputBytes, BlockInputBytes, BlockOutputBytes int64
}
type PortMapping struct {
	HostIp                  string
	HostPort, ContainerPort int32
	Protocol                string
}
type PortMappingRequest struct {
	HostIp                  string
	HostPort, ContainerPort int32
	Protocol                string
}
type NetworkInfo struct{ Name, Ip, Gateway, Mac string }
type MountInfo struct {
	Type, Source, Destination string
	Rw                        bool
}
type VolumeMountRequest struct {
	Source, Destination string
	ReadOnly            bool
	Type                string
}
type Image struct {
	Id, Repository, Tag, Digest string
	Created                     *timestamppb.Timestamp
	CreatedAgo                  string
	Size                        int64
}
type Volume struct {
	Name, Driver, Mountpoint string
	Labels                   map[string]string
	Created                  *timestamppb.Timestamp
	Scope                    string
}
type Network struct {
	Name, Driver, Subnet, Gateway string
	Ipv6                          bool
	Labels, Containers            map[string]string
}
type HostInfo struct {
	Hostname, Os, Kernel                                                            string
	UptimeSeconds                                                                   int64
	CpuCores                                                                        int32
	Load_1, Load_5, Load_15                                                         float64
	MemoryTotalBytes, MemoryUsedBytes, DiskTotalBytes, DiskUsedBytes, DiskFreeBytes int64
	PodmanVersion, AgentVersion                                                     string
}
type ActionResult struct {
	Success        bool
	Message, Error string
}
type PodmanEvent struct {
	Host, EventType, ActorId, ActorName string
	Timestamp                           *timestamppb.Timestamp
	Attributes                          map[string]string
}
type LogEntry struct {
	Timestamp *timestamppb.Timestamp
	Message   string
}
type EnrollRequest struct {
	Token, AgentVersion, HostId string
	Capabilities                []string
}
type EnrollResponse struct {
	Success                    bool
	AgentId, Credential, Error string
}

func (m *HeartbeatAck) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ContainerListResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ContainerInspectResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ContainerActionResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ContainerCreateResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ContainerLogsChunk) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ContainerLogsError) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ImageListResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ImageActionResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ImagePruneResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *VolumeListResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *VolumeActionResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *VolumePruneResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *NetworkListResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *NetworkActionResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *NetworkPruneResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *HostInfoResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *EventStreamData) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *EventStreamError) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}
func (m *ErrorResponse) GetRequestId() string {
	if m == nil {
		return ""
	}
	return m.RequestId
}

type AgentServiceClient interface {
	Connect(ctx context.Context, opts ...grpc.CallOption) (AgentService_ConnectClient, error)
	Enroll(ctx context.Context, in *EnrollRequest, opts ...grpc.CallOption) (*EnrollResponse, error)
}

type agentServiceClient struct{ cc grpc.ClientConnInterface }

func NewAgentServiceClient(cc grpc.ClientConnInterface) AgentServiceClient {
	return &agentServiceClient{cc}
}

func (c *agentServiceClient) Connect(ctx context.Context, opts ...grpc.CallOption) (AgentService_ConnectClient, error) {
	stream, err := c.cc.NewStream(ctx, &AgentService_ServiceDesc.Streams[0], "/agent.AgentService/Connect", opts...)
	if err != nil {
		return nil, err
	}
	return &agentServiceConnectClient{ClientStream: stream}, nil
}

func (c *agentServiceClient) Enroll(ctx context.Context, in *EnrollRequest, opts ...grpc.CallOption) (*EnrollResponse, error) {
	out := new(EnrollResponse)
	if err := c.cc.Invoke(ctx, "/agent.AgentService/Enroll", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

type AgentService_ConnectClient interface {
	Send(*AgentMessage) error
	Recv() (*ManagerMessage, error)
	grpc.ClientStream
}

type agentServiceConnectClient struct{ grpc.ClientStream }

func (x *agentServiceConnectClient) Send(m *AgentMessage) error { return x.ClientStream.SendMsg(m) }
func (x *agentServiceConnectClient) Recv() (*ManagerMessage, error) {
	m := new(ManagerMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type AgentServiceServer interface {
	Connect(AgentService_ConnectServer) error
	Enroll(context.Context, *EnrollRequest) (*EnrollResponse, error)
}

type UnimplementedAgentServiceServer struct{}

func (UnimplementedAgentServiceServer) Connect(AgentService_ConnectServer) error {
	return status.Errorf(codes.Unimplemented, "method Connect not implemented")
}
func (UnimplementedAgentServiceServer) Enroll(context.Context, *EnrollRequest) (*EnrollResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Enroll not implemented")
}

type AgentService_ConnectServer interface {
	Send(*ManagerMessage) error
	Recv() (*AgentMessage, error)
	grpc.ServerStream
}

func RegisterAgentServiceServer(s grpc.ServiceRegistrar, srv AgentServiceServer) {
	s.RegisterService(&AgentService_ServiceDesc, srv)
}

var AgentService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "agent.AgentService",
	HandlerType: (*AgentServiceServer)(nil),
	Methods:     []grpc.MethodDesc{{MethodName: "Enroll", Handler: _AgentService_Enroll_Handler}},
	Streams:     []grpc.StreamDesc{{StreamName: "Connect", Handler: _AgentService_Connect_Handler, ServerStreams: true, ClientStreams: true}},
}

func _AgentService_Enroll_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(EnrollRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).Enroll(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/agent.AgentService/Enroll"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AgentServiceServer).Enroll(ctx, req.(*EnrollRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentService_Connect_Handler(srv any, stream grpc.ServerStream) error {
	return srv.(AgentServiceServer).Connect(&agentServiceConnectServer{ServerStream: stream})
}

type agentServiceConnectServer struct{ grpc.ServerStream }

func (x *agentServiceConnectServer) Send(m *ManagerMessage) error { return x.ServerStream.SendMsg(m) }
func (x *agentServiceConnectServer) Recv() (*AgentMessage, error) {
	m := new(AgentMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}
