package host

import (
	"context"
	"time"
)

type Transport interface {
	// Host identity
	Name() string

	// Container operations
	ListContainers(ctx context.Context) ([]Container, error)
	InspectContainer(ctx context.Context, id string) (*ContainerDetail, error)
	StartContainer(ctx context.Context, id string) (*ActionResult, error)
	StopContainer(ctx context.Context, id string) (*ActionResult, error)
	RestartContainer(ctx context.Context, id string) (*ActionResult, error)
	RemoveContainer(ctx context.Context, id string, force bool) (*ActionResult, error)
	CreateContainer(ctx context.Context, req *CreateContainerRequest) (*ActionResult, error)
	CheckForUpdate(ctx context.Context, id string) (*UpdateCheckResult, error)
	UpdateContainer(ctx context.Context, id string) (*UpdateResult, error)

	// Container logs
	ContainerLogs(ctx context.Context, id string, tail int) (string, error)
	StreamLogs(ctx context.Context, id string, tail int, output chan<- string) error

	// Event streaming
	StreamEvents(ctx context.Context, output chan<- PodmanEvent) error

	// Image operations
	ListImages(ctx context.Context) ([]Image, error)
	PullImage(ctx context.Context, imageRef string) error
	RemoveImage(ctx context.Context, imageID string, force bool) error
	PruneImages(ctx context.Context) (int, error)

	// Volume operations (new)
	ListVolumes(ctx context.Context) ([]Volume, error)
	CreateVolume(ctx context.Context, req *CreateVolumeRequest) (*Volume, error)
	RemoveVolume(ctx context.Context, name string, force bool) error
	PruneVolumes(ctx context.Context) (int, error)

	// Network operations (new)
	ListNetworks(ctx context.Context) ([]Network, error)
	CreateNetwork(ctx context.Context, req *CreateNetworkRequest) (*Network, error)
	RemoveNetwork(ctx context.Context, name string) error
	PruneNetworks(ctx context.Context) (int, error)
	ConnectNetwork(ctx context.Context, networkName, containerID string) error
	DisconnectNetwork(ctx context.Context, networkName, containerID string) error

	// Host info
	HostSystemInfo(ctx context.Context) (*HostSystemInfo, error)
	Ping(ctx context.Context) (time.Duration, error)

	// Lifecycle
	Close() error
}
