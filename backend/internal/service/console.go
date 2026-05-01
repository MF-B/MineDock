package service

import (
	"context"
	"errors"
	"fmt"
	"io"

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
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
}

// ConsoleService 封装容器控制台输出的业务逻辑。
type ConsoleService struct {
	cli consoleDockerClient
}

// ConsoleSession 表示一个控制台输出会话。
type ConsoleSession struct {
	Output io.ReadCloser
	Input  io.Writer
	TTY    bool
	Live   bool
}

// Close 释放控制台输出会话。
func (s *ConsoleSession) Close() error {
	if s == nil || s.Output == nil {
		return nil
	}
	return s.Output.Close()
}

// NewConsoleService 创建 ConsoleService。
func NewConsoleService(cli *client.Client) *ConsoleService {
	return &ConsoleService{cli: cli}
}

// Open 打开容器控制台会话，运行中容器连接实时 Attach，停止容器返回历史日志。
func (s *ConsoleService) Open(ctx context.Context, containerID string) (*ConsoleSession, error) {
	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}

	tty := inspect.Config != nil && inspect.Config.Tty
	if inspect.State != nil && inspect.State.Running {
		hijacked, err := s.cli.ContainerAttach(ctx, containerID, container.AttachOptions{
			Logs:   true,
			Stream: true,
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		})
		if err != nil {
			return nil, fmt.Errorf("attach container: %w", err)
		}

		return &ConsoleSession{
			Output: &hijackedReadCloser{hijacked: &hijacked},
			Input:  hijacked.Conn,
			TTY:    tty,
			Live:   true,
		}, nil
	}

	logs, err := s.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Tail:       "all",
	})
	if err != nil {
		return nil, fmt.Errorf("read container logs: %w", err)
	}

	return &ConsoleSession{
		Output: logs,
		TTY:    tty,
		Live:   false,
	}, nil
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

type hijackedReadCloser struct {
	hijacked *types.HijackedResponse
}

func (h *hijackedReadCloser) Read(p []byte) (int, error) {
	return h.hijacked.Reader.Read(p)
}

func (h *hijackedReadCloser) Close() error {
	h.hijacked.Close()
	return nil
}
