package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type fakeConsoleDockerClient struct {
	inspectResp container.InspectResponse
	inspectErr  error

	attachResp   types.HijackedResponse
	attachErr    error
	attachCalled bool
	attachID     string
	attachOpts   container.AttachOptions

	logsResp   io.ReadCloser
	logsErr    error
	logsCalled bool
	logsID     string
	logsOpts   container.LogsOptions
}

func (f *fakeConsoleDockerClient) ContainerInspect(_ context.Context, _ string) (container.InspectResponse, error) {
	if f.inspectErr != nil {
		return container.InspectResponse{}, f.inspectErr
	}
	return f.inspectResp, nil
}

func (f *fakeConsoleDockerClient) ContainerAttach(
	_ context.Context,
	containerID string,
	options container.AttachOptions,
) (types.HijackedResponse, error) {
	f.attachCalled = true
	f.attachID = containerID
	f.attachOpts = options

	if f.attachErr != nil {
		return types.HijackedResponse{}, f.attachErr
	}
	return f.attachResp, nil
}

func (f *fakeConsoleDockerClient) ContainerLogs(
	_ context.Context,
	containerID string,
	options container.LogsOptions,
) (io.ReadCloser, error) {
	f.logsCalled = true
	f.logsID = containerID
	f.logsOpts = options

	if f.logsErr != nil {
		return nil, f.logsErr
	}
	return f.logsResp, nil
}

func TestConsoleServiceOpen_RunningContainer(t *testing.T) {
	fake := &fakeConsoleDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: true}},
			Config:            &container.Config{Tty: true},
		},
		attachResp: types.HijackedResponse{},
	}

	svc := &ConsoleService{cli: fake}

	session, err := svc.Open(context.Background(), "c1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if session == nil || !session.Live || !session.TTY {
		t.Fatalf("unexpected session: %+v", session)
	}
	if !fake.attachCalled {
		t.Fatal("expected attach to be called")
	}
	if fake.logsCalled {
		t.Fatal("logs should not be called for running container")
	}
	if fake.attachID != "c1" {
		t.Fatalf("unexpected attach id: %s", fake.attachID)
	}
	if !fake.attachOpts.Stream || !fake.attachOpts.Stdin || !fake.attachOpts.Stdout || !fake.attachOpts.Stderr {
		t.Fatalf("unexpected attach options: %+v", fake.attachOpts)
	}
}

func TestConsoleServiceOpen_StoppedContainerReturnsLogs(t *testing.T) {
	fake := &fakeConsoleDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: false}},
			Config:            &container.Config{Tty: false},
		},
		logsResp: io.NopCloser(strings.NewReader("old log\n")),
	}

	svc := &ConsoleService{cli: fake}

	session, err := svc.Open(context.Background(), "c1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if session == nil || session.Live || session.TTY {
		t.Fatalf("unexpected session: %+v", session)
	}
	if session.Input != nil {
		t.Fatal("stopped container log session should not expose stdin")
	}
	if fake.attachCalled {
		t.Fatal("attach should not be called for stopped container")
	}
	if !fake.logsCalled {
		t.Fatal("expected logs to be called")
	}
	if fake.logsID != "c1" {
		t.Fatalf("unexpected logs id: %s", fake.logsID)
	}
	if !fake.logsOpts.ShowStdout || !fake.logsOpts.ShowStderr || fake.logsOpts.Follow || fake.logsOpts.Tail != "all" {
		t.Fatalf("unexpected logs options: %+v", fake.logsOpts)
	}

	data, err := io.ReadAll(session.Output)
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	if string(data) != "old log\n" {
		t.Fatalf("unexpected logs: %q", string(data))
	}
}

func TestConsoleServiceOpen_ContainerNotFound(t *testing.T) {
	fake := &fakeConsoleDockerClient{inspectErr: errors.New("no such container")}
	svc := &ConsoleService{cli: fake}

	_, err := svc.Open(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if fake.attachCalled || fake.logsCalled {
		t.Fatal("attach/logs should not be called when inspect fails")
	}
}
