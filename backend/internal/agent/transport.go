package agent

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	agentpb "github.com/brdweb/podman-manager/agent/proto"
	"github.com/brdweb/podman-manager/internal/host"
)

// AgentTransport implements host.Transport for agent-connected hosts.
type AgentTransport struct {
	name       string
	agentID    string
	grpcServer *Server
	timeout    time.Duration
}

var requestCounter uint64

func NewAgentTransport(name, agentID string, grpcServer *Server, timeout time.Duration) *AgentTransport {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &AgentTransport{name: name, agentID: agentID, grpcServer: grpcServer, timeout: timeout}
}

func (t *AgentTransport) Name() string { return t.name }

// Each method sends a command via the gRPC server and waits for the response.
func (t *AgentTransport) ListContainers(ctx context.Context) ([]host.Container, error) {
	requestID := nextRequestID()
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ContainerListRequest{ContainerListRequest: &agentpb.ContainerListRequest{RequestId: requestID, All: true}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_ContainerListResponse)
	if !ok {
		return nil, responseError(resp, "container list")
	}
	containers := make([]host.Container, 0, len(body.ContainerListResponse.Containers))
	for _, container := range body.ContainerListResponse.Containers {
		containers = append(containers, convertContainer(container))
	}
	return containers, nil
}

func (t *AgentTransport) InspectContainer(ctx context.Context, id string) (*host.ContainerDetail, error) {
	requestID := nextRequestID()
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ContainerInspectRequest{ContainerInspectRequest: &agentpb.ContainerInspectRequest{RequestId: requestID, ContainerId: id}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_ContainerInspectResponse)
	if !ok {
		return nil, responseError(resp, "container inspect")
	}
	detail := convertContainerDetail(body.ContainerInspectResponse.Container)
	return &detail, nil
}

func (t *AgentTransport) StartContainer(ctx context.Context, id string) (*host.ActionResult, error) {
	return t.containerAction(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ContainerStartRequest{ContainerStartRequest: &agentpb.ContainerStartRequest{RequestId: nextRequestID(), ContainerId: id}}})
}

func (t *AgentTransport) StopContainer(ctx context.Context, id string) (*host.ActionResult, error) {
	return t.containerAction(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ContainerStopRequest{ContainerStopRequest: &agentpb.ContainerStopRequest{RequestId: nextRequestID(), ContainerId: id}}})
}

func (t *AgentTransport) RestartContainer(ctx context.Context, id string) (*host.ActionResult, error) {
	return t.containerAction(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ContainerRestartRequest{ContainerRestartRequest: &agentpb.ContainerRestartRequest{RequestId: nextRequestID(), ContainerId: id}}})
}

func (t *AgentTransport) RemoveContainer(ctx context.Context, id string, force bool) (*host.ActionResult, error) {
	return t.containerAction(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ContainerRemoveRequest{ContainerRemoveRequest: &agentpb.ContainerRemoveRequest{RequestId: nextRequestID(), ContainerId: id, Force: force}}})
}

func (t *AgentTransport) CreateContainer(ctx context.Context, req *host.CreateContainerRequest) (*host.ActionResult, error) {
	if req == nil {
		return nil, fmt.Errorf("container create request is required")
	}
	msg := &AgentMessage{Type: &agentpb.AgentMessage_ContainerCreateRequest{ContainerCreateRequest: &agentpb.ContainerCreateRequest{RequestId: nextRequestID(), Name: req.Name, Image: req.Image, Command: req.Command, Labels: req.Labels, Ports: convertPortRequests(req.Ports), Volumes: convertVolumeMountRequests(req.Mounts), Networks: []string{req.Network}}}}
	resp, err := t.send(ctx, msg)
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_ContainerCreateResponse)
	if !ok {
		return nil, responseError(resp, "container create")
	}
	return convertActionResult(body.ContainerCreateResponse.Result), nil
}

func (t *AgentTransport) CheckForUpdate(ctx context.Context, id string) (*host.UpdateCheckResult, error) {
	return nil, fmt.Errorf("not yet supported for agent transport")
}

func (t *AgentTransport) UpdateContainer(ctx context.Context, id string) (*host.UpdateResult, error) {
	return nil, fmt.Errorf("not yet supported for agent transport")
}

func (t *AgentTransport) ContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ContainerLogsRequest{ContainerLogsRequest: &agentpb.ContainerLogsRequest{RequestId: nextRequestID(), ContainerId: id, Stdout: true, Stderr: true, Tail: int32(tail)}}})
	if err != nil {
		return "", err
	}
	switch body := resp.Type.(type) {
	case *agentpb.ManagerMessage_ContainerLogsChunk:
		lines := make([]string, 0, len(body.ContainerLogsChunk.Entries))
		for _, entry := range body.ContainerLogsChunk.Entries {
			lines = append(lines, entry.Message)
		}
		return strings.Join(lines, "\n"), nil
	case *agentpb.ManagerMessage_ContainerLogsError:
		return "", fmt.Errorf("%s", body.ContainerLogsError.Error)
	default:
		return "", responseError(resp, "container logs")
	}
}

func (t *AgentTransport) StreamLogs(ctx context.Context, id string, tail int, output chan<- string) error {
	if t.grpcServer == nil {
		return fmt.Errorf("agent gRPC server is not configured")
	}
	requestID := nextRequestID()
	callback := make(chan *ManagerMessage, 64)
	t.grpcServer.RegisterStreamCallback(requestID, callback)
	defer t.grpcServer.UnregisterStreamCallback(requestID)

	if tail < 0 {
		tail = 0
	}
	startCtx, cancel := context.WithTimeout(ctx, t.timeout)
	err := t.grpcServer.SendStreamCommand(startCtx, t.agentID, streamCommand(requestID, "container_logs_start", id, fmt.Sprintf("%d", tail)))
	cancel()
	if err != nil {
		return err
	}
	defer t.stopStream(requestID, "container_logs_stop")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-callback:
			if !ok {
				return nil
			}
			switch body := msg.Type.(type) {
			case *agentpb.ManagerMessage_ContainerLogsChunk:
				for _, entry := range body.ContainerLogsChunk.Entries {
					select {
					case output <- entry.Message:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			case *agentpb.ManagerMessage_ContainerLogsError:
				return fmt.Errorf("%s", body.ContainerLogsError.Error)
			}
		}
	}
}

func (t *AgentTransport) StreamEvents(ctx context.Context, output chan<- host.PodmanEvent) error {
	if t.grpcServer == nil {
		return fmt.Errorf("agent gRPC server is not configured")
	}
	requestID := nextRequestID()
	callback := make(chan *ManagerMessage, 64)
	t.grpcServer.RegisterStreamCallback(requestID, callback)
	defer t.grpcServer.UnregisterStreamCallback(requestID)

	startCtx, cancel := context.WithTimeout(ctx, t.timeout)
	err := t.grpcServer.SendStreamCommand(startCtx, t.agentID, streamCommand(requestID, "event_stream_start", "", ""))
	cancel()
	if err != nil {
		return err
	}
	defer t.stopStream(requestID, "event_stream_stop")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-callback:
			if !ok {
				return nil
			}
			switch body := msg.Type.(type) {
			case *agentpb.ManagerMessage_EventStreamData:
				select {
				case output <- t.podmanEventFromProto(body.EventStreamData.Event):
				case <-ctx.Done():
					return ctx.Err()
				}
			case *agentpb.ManagerMessage_EventStreamError:
				return fmt.Errorf("%s", body.EventStreamError.Error)
			}
		}
	}
}

func (t *AgentTransport) ListImages(ctx context.Context) ([]host.Image, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ImageListRequest{ImageListRequest: &agentpb.ImageListRequest{RequestId: nextRequestID(), All: true}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_ImageListResponse)
	if !ok {
		return nil, responseError(resp, "image list")
	}
	images := make([]host.Image, 0, len(body.ImageListResponse.Images))
	for _, image := range body.ImageListResponse.Images {
		images = append(images, convertImage(image))
	}
	return images, nil
}

func (t *AgentTransport) PullImage(ctx context.Context, imageRef string) error {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ImagePullRequest{ImagePullRequest: &agentpb.ImagePullRequest{RequestId: nextRequestID(), Image: imageRef}}})
	if err != nil {
		return err
	}
	return actionResponseError(resp, "image pull")
}

func (t *AgentTransport) RemoveImage(ctx context.Context, imageID string, force bool) error {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ImageRemoveRequest{ImageRemoveRequest: &agentpb.ImageRemoveRequest{RequestId: nextRequestID(), ImageId: imageID, Force: force}}})
	if err != nil {
		return err
	}
	return actionResponseError(resp, "image remove")
}

func (t *AgentTransport) PruneImages(ctx context.Context) (int, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_ImagePruneRequest{ImagePruneRequest: &agentpb.ImagePruneRequest{RequestId: nextRequestID(), All: true}}})
	if err != nil {
		return 0, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_ImagePruneResponse)
	if !ok {
		return 0, responseError(resp, "image prune")
	}
	if result := convertActionResult(body.ImagePruneResponse.Result); result != nil && !result.Success {
		return 0, fmt.Errorf("%s", result.Error)
	}
	return len(body.ImagePruneResponse.ImagesDeleted), nil
}

func (t *AgentTransport) ListVolumes(ctx context.Context) ([]host.Volume, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_VolumeListRequest{VolumeListRequest: &agentpb.VolumeListRequest{RequestId: nextRequestID()}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_VolumeListResponse)
	if !ok {
		return nil, responseError(resp, "volume list")
	}
	volumes := make([]host.Volume, 0, len(body.VolumeListResponse.Volumes))
	for _, volume := range body.VolumeListResponse.Volumes {
		volumes = append(volumes, volumeFromProto(volume))
	}
	return volumes, nil
}

func (t *AgentTransport) CreateVolume(ctx context.Context, req *host.CreateVolumeRequest) (*host.Volume, error) {
	if req == nil {
		return nil, fmt.Errorf("volume create request is required")
	}
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_VolumeCreateRequest{VolumeCreateRequest: &agentpb.VolumeCreateRequest{RequestId: nextRequestID(), Name: req.Name, Driver: req.Driver, Labels: req.Labels, Options: req.Options}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_VolumeActionResponse)
	if !ok {
		return nil, responseError(resp, "volume create")
	}
	if result := convertActionResult(body.VolumeActionResponse.Result); result != nil && !result.Success {
		return nil, actionResultError(result)
	}
	name := body.VolumeActionResponse.Name
	if name == "" {
		name = req.Name
	}
	return &host.Volume{Name: name, Driver: req.Driver, Labels: req.Labels, Options: req.Options}, nil
}

func (t *AgentTransport) RemoveVolume(ctx context.Context, name string, force bool) error {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_VolumeRemoveRequest{VolumeRemoveRequest: &agentpb.VolumeRemoveRequest{RequestId: nextRequestID(), Name: name, Force: force}}})
	if err != nil {
		return err
	}
	return actionResponseError(resp, "volume remove")
}

func (t *AgentTransport) PruneVolumes(ctx context.Context) (int, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_VolumePruneRequest{VolumePruneRequest: &agentpb.VolumePruneRequest{RequestId: nextRequestID()}}})
	if err != nil {
		return 0, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_VolumePruneResponse)
	if !ok {
		return 0, responseError(resp, "volume prune")
	}
	if result := convertActionResult(body.VolumePruneResponse.Result); result != nil && !result.Success {
		return 0, actionResultError(result)
	}
	return len(body.VolumePruneResponse.VolumesDeleted), nil
}

func (t *AgentTransport) ListNetworks(ctx context.Context) ([]host.Network, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_NetworkListRequest{NetworkListRequest: &agentpb.NetworkListRequest{RequestId: nextRequestID()}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_NetworkListResponse)
	if !ok {
		return nil, responseError(resp, "network list")
	}
	networks := make([]host.Network, 0, len(body.NetworkListResponse.Networks))
	for _, network := range body.NetworkListResponse.Networks {
		networks = append(networks, networkFromProto(network))
	}
	return networks, nil
}

func (t *AgentTransport) CreateNetwork(ctx context.Context, req *host.CreateNetworkRequest) (*host.Network, error) {
	if req == nil {
		return nil, fmt.Errorf("network create request is required")
	}
	subnet := ""
	if len(req.Subnets) > 0 {
		subnet = req.Subnets[0]
	}
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_NetworkCreateRequest{NetworkCreateRequest: &agentpb.NetworkCreateRequest{RequestId: nextRequestID(), Name: req.Name, Driver: req.Driver, Subnet: subnet, Gateway: req.Gateway, Ipv6: req.IPv6, Labels: req.Labels, Options: req.Options}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_NetworkActionResponse)
	if !ok {
		return nil, responseError(resp, "network create")
	}
	if result := convertActionResult(body.NetworkActionResponse.Result); result != nil && !result.Success {
		return nil, actionResultError(result)
	}
	name := body.NetworkActionResponse.NetworkName
	if name == "" {
		name = req.Name
	}
	return &host.Network{Name: name, Driver: req.Driver, Subnets: req.Subnets, Gateway: req.Gateway, IPv6: req.IPv6, DNSEnabled: req.DNSEnabled, Internal: req.Internal, Labels: req.Labels, Options: req.Options}, nil
}

func (t *AgentTransport) RemoveNetwork(ctx context.Context, name string) error {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_NetworkRemoveRequest{NetworkRemoveRequest: &agentpb.NetworkRemoveRequest{RequestId: nextRequestID(), Name: name}}})
	if err != nil {
		return err
	}
	return actionResponseError(resp, "network remove")
}

func (t *AgentTransport) PruneNetworks(ctx context.Context) (int, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_NetworkPruneRequest{NetworkPruneRequest: &agentpb.NetworkPruneRequest{RequestId: nextRequestID()}}})
	if err != nil {
		return 0, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_NetworkPruneResponse)
	if !ok {
		return 0, responseError(resp, "network prune")
	}
	if result := convertActionResult(body.NetworkPruneResponse.Result); result != nil && !result.Success {
		return 0, actionResultError(result)
	}
	return len(body.NetworkPruneResponse.NetworksDeleted), nil
}

func (t *AgentTransport) ConnectNetwork(ctx context.Context, networkName, containerID string) error {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_NetworkConnectRequest{NetworkConnectRequest: &agentpb.NetworkConnectRequest{RequestId: nextRequestID(), NetworkName: networkName, ContainerId: containerID}}})
	if err != nil {
		return err
	}
	return actionResponseError(resp, "network connect")
}

func (t *AgentTransport) DisconnectNetwork(ctx context.Context, networkName, containerID string) error {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_NetworkDisconnectRequest{NetworkDisconnectRequest: &agentpb.NetworkDisconnectRequest{RequestId: nextRequestID(), NetworkName: networkName, ContainerId: containerID, Force: true}}})
	if err != nil {
		return err
	}
	return actionResponseError(resp, "network disconnect")
}

func (t *AgentTransport) HostSystemInfo(ctx context.Context) (*host.HostSystemInfo, error) {
	resp, err := t.send(ctx, &AgentMessage{Type: &agentpb.AgentMessage_HostInfoRequest{HostInfoRequest: &agentpb.HostInfoRequest{RequestId: nextRequestID()}}})
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_HostInfoResponse)
	if !ok {
		return nil, responseError(resp, "host info")
	}
	info := convertHostInfo(body.HostInfoResponse.HostInfo)
	return &info, nil
}

func (t *AgentTransport) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	if _, ok := t.grpcServer.registry.Get(t.agentID); !ok {
		return 0, fmt.Errorf("agent %s is not connected", t.agentID)
	}
	return time.Since(start), nil
}

func (t *AgentTransport) Close() error { return nil }

func (t *AgentTransport) containerAction(ctx context.Context, msg *AgentMessage) (*host.ActionResult, error) {
	resp, err := t.send(ctx, msg)
	if err != nil {
		return nil, err
	}
	body, ok := resp.Type.(*agentpb.ManagerMessage_ContainerActionResponse)
	if !ok {
		return nil, responseError(resp, "container action")
	}
	return convertActionResult(body.ContainerActionResponse.Result), nil
}

func (t *AgentTransport) send(ctx context.Context, msg *AgentMessage) (*ManagerMessage, error) {
	if t.grpcServer == nil {
		return nil, fmt.Errorf("agent gRPC server is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.grpcServer.SendCommand(ctx, t.agentID, msg)
}

func (t *AgentTransport) stopStream(requestID, code string) {
	if t.grpcServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = t.grpcServer.SendStreamCommand(ctx, t.agentID, streamCommand(requestID, code, "", ""))
}

func streamCommand(requestID, code, message, details string) *ManagerMessage {
	return &ManagerMessage{Type: &agentpb.ManagerMessage_ErrorResponse{ErrorResponse: &agentpb.ErrorResponse{RequestId: requestID, Code: code, Message: message, Details: details}}}
}

func (t *AgentTransport) podmanEventFromProto(event *agentpb.PodmanEvent) host.PodmanEvent {
	payload := map[string]any{}
	if event != nil {
		payload["type"] = event.EventType
		payload["actor_id"] = event.ActorId
		payload["actor_name"] = event.ActorName
		if event.Timestamp != nil {
			payload["time"] = event.Timestamp.AsTime().Format(time.RFC3339Nano)
		}
		if len(event.Attributes) > 0 {
			attrs := make(map[string]string, len(event.Attributes))
			for key, value := range event.Attributes {
				attrs[key] = value
			}
			payload["attributes"] = attrs
		}
	}
	return host.PodmanEvent{Host: t.name, Event: payload}
}

func nextRequestID() string {
	return fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), atomic.AddUint64(&requestCounter, 1))
}

func responseError(resp *ManagerMessage, operation string) error {
	if resp == nil || resp.Type == nil {
		return fmt.Errorf("%s failed: empty response", operation)
	}
	if errResp, ok := resp.Type.(*agentpb.ManagerMessage_ErrorResponse); ok {
		return fmt.Errorf("%s failed: %s", operation, errResp.ErrorResponse.Message)
	}
	return fmt.Errorf("%s failed: unexpected response %T", operation, resp.Type)
}

func actionResponseError(resp *ManagerMessage, operation string) error {
	var result *host.ActionResult
	switch body := resp.Type.(type) {
	case *agentpb.ManagerMessage_ImageActionResponse:
		result = convertActionResult(body.ImageActionResponse.Result)
	case *agentpb.ManagerMessage_VolumeActionResponse:
		result = convertActionResult(body.VolumeActionResponse.Result)
	case *agentpb.ManagerMessage_NetworkActionResponse:
		result = convertActionResult(body.NetworkActionResponse.Result)
	default:
		return responseError(resp, operation)
	}
	if result == nil || result.Success {
		return nil
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	return fmt.Errorf("%s", result.Message)
}

func convertActionResult(result *agentpb.ActionResult) *host.ActionResult {
	if result == nil {
		return nil
	}
	return &host.ActionResult{Success: result.Success, Message: result.Message, Error: result.Error}
}

func actionResultError(result *host.ActionResult) error {
	if result == nil || result.Success {
		return nil
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	return fmt.Errorf("%s", result.Message)
}

func convertContainer(container *agentpb.Container) host.Container {
	if container == nil {
		return host.Container{}
	}
	return host.Container{ID: container.Id, Name: container.Name, Image: container.Image, State: container.State, Status: container.Status, Created: timestamp(container.Created), Ports: convertPorts(container.Ports), Networks: convertNetworkInfos(container.Networks), Mounts: convertMounts(container.Mounts), Labels: container.Labels, Host: container.Host, Manager: container.Manager, SystemdUnit: container.SystemdUnit}
}

func convertContainerDetail(detail *agentpb.ContainerDetail) host.ContainerDetail {
	if detail == nil {
		return host.ContainerDetail{}
	}
	container := convertContainer(detail.Container)
	return host.ContainerDetail{Container: container, Env: detail.Env, Hostname: detail.Hostname, RestartPolicy: detail.RestartPolicy, NetworkMode: detail.NetworkMode, Pid: int(detail.Pid), StartedAt: timestamp(detail.StartedAt), FinishedAt: timestamp(detail.FinishedAt)}
}

func convertPorts(ports []*agentpb.PortMapping) []host.PortMapping {
	converted := make([]host.PortMapping, 0, len(ports))
	for _, port := range ports {
		converted = append(converted, host.PortMapping{HostIP: port.HostIp, HostPort: uint16(port.HostPort), ContainerPort: uint16(port.ContainerPort), Protocol: port.Protocol})
	}
	return converted
}

func convertPortRequests(ports []host.PortMapping) []*agentpb.PortMappingRequest {
	converted := make([]*agentpb.PortMappingRequest, 0, len(ports))
	for _, port := range ports {
		converted = append(converted, &agentpb.PortMappingRequest{HostIp: port.HostIP, HostPort: int32(port.HostPort), ContainerPort: int32(port.ContainerPort), Protocol: port.Protocol})
	}
	return converted
}

func convertNetworkInfos(networks []*agentpb.NetworkInfo) []host.NetworkInfo {
	converted := make([]host.NetworkInfo, 0, len(networks))
	for _, network := range networks {
		converted = append(converted, host.NetworkInfo{Name: network.Name, IP: network.Ip, Gateway: network.Gateway, MAC: network.Mac})
	}
	return converted
}

func convertMounts(mounts []*agentpb.MountInfo) []host.MountInfo {
	converted := make([]host.MountInfo, 0, len(mounts))
	for _, mount := range mounts {
		converted = append(converted, host.MountInfo{Type: mount.Type, Source: mount.Source, Destination: mount.Destination, RW: mount.Rw})
	}
	return converted
}

func convertVolumeMountRequests(mounts []host.MountInfo) []*agentpb.VolumeMountRequest {
	converted := make([]*agentpb.VolumeMountRequest, 0, len(mounts))
	for _, mount := range mounts {
		converted = append(converted, &agentpb.VolumeMountRequest{Source: mount.Source, Destination: mount.Destination, ReadOnly: !mount.RW, Type: mount.Type})
	}
	return converted
}

func convertImage(image *agentpb.Image) host.Image {
	if image == nil {
		return host.Image{}
	}
	return host.Image{ID: image.Id, Repository: image.Repository, Tag: image.Tag, Digest: image.Digest, Created: timestamp(image.Created), CreatedAgo: image.CreatedAgo, Size: fmt.Sprintf("%d", image.Size)}
}

func volumeFromProto(volume *agentpb.Volume) host.Volume {
	if volume == nil {
		return host.Volume{}
	}
	return host.Volume{Name: volume.Name, Driver: volume.Driver, Mountpoint: volume.Mountpoint, CreatedAt: timestamp(volume.Created), Labels: volume.Labels, Scope: volume.Scope}
}

func networkFromProto(network *agentpb.Network) host.Network {
	if network == nil {
		return host.Network{}
	}
	subnets := []string(nil)
	if network.Subnet != "" {
		subnets = []string{network.Subnet}
	}
	return host.Network{Name: network.Name, Driver: network.Driver, Subnets: subnets, Gateway: network.Gateway, IPv6: network.Ipv6, Labels: network.Labels}
}

func convertHostInfo(info *agentpb.HostInfo) host.HostSystemInfo {
	if info == nil {
		return host.HostSystemInfo{}
	}
	return host.HostSystemInfo{Hostname: info.Hostname, OS: info.Os, Kernel: info.Kernel, UptimeSeconds: info.UptimeSeconds, CPUCores: int(info.CpuCores), Load1: info.Load_1, Load5: info.Load_5, Load15: info.Load_15, MemoryTotalBytes: uint64(info.MemoryTotalBytes), MemoryUsedBytes: uint64(info.MemoryUsedBytes), DiskTotalBytes: uint64(info.DiskTotalBytes), DiskUsedBytes: uint64(info.DiskUsedBytes), DiskFreeBytes: uint64(info.DiskFreeBytes)}
}

func timestamp(ts interface{ AsTime() time.Time }) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
