package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ErrContainerNotRunning 表示容器当前不处于运行状态，无法建立控制台 Attach。
var ErrContainerNotRunning = errors.New("container is not running")

// consoleDockerClient 定义 ConsoleService 依赖的最小 Docker 能力集合。
type consoleDockerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerAttach(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error)
}

// ConsoleService 封装容器控制台 Attach 的业务逻辑。
type ConsoleService struct {
	cli consoleDockerClient
}

// NewConsoleService 创建 ConsoleService。
func NewConsoleService(cli *client.Client) *ConsoleService {
	return &ConsoleService{cli: cli}
}

// Attach 连接到运行中容器的 stdin/stdout/stderr。
func (s *ConsoleService) Attach(ctx context.Context, containerID string) (types.HijackedResponse, error) {
	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return types.HijackedResponse{}, fmt.Errorf("inspect container: %w", err)
	}

	if inspect.State == nil || !inspect.State.Running {
		return types.HijackedResponse{}, ErrContainerNotRunning
	}

	hijacked, err := s.cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Logs:   true,
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return types.HijackedResponse{}, fmt.Errorf("attach container: %w", err)
	}

	return hijacked, nil
}
