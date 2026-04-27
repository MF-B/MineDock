package service

import (
	"context"
	"errors"
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

func TestConsoleServiceAttach_RunningContainer(t *testing.T) {
	fake := &fakeConsoleDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: true}},
			Config:            &container.Config{Tty: true},
		},
		attachResp: types.HijackedResponse{},
	}

	svc := &ConsoleService{cli: fake}

	_, err := svc.Attach(context.Background(), "c1")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if !fake.attachCalled {
		t.Fatal("expected attach to be called")
	}
	if fake.attachID != "c1" {
		t.Fatalf("unexpected attach id: %s", fake.attachID)
	}
	if !fake.attachOpts.Stream || !fake.attachOpts.Stdin || !fake.attachOpts.Stdout || !fake.attachOpts.Stderr {
		t.Fatalf("unexpected attach options: %+v", fake.attachOpts)
	}
}

func TestConsoleServiceAttach_StoppedContainer(t *testing.T) {
	fake := &fakeConsoleDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: false}},
			Config:            &container.Config{Tty: false},
		},
	}

	svc := &ConsoleService{cli: fake}

	_, err := svc.Attach(context.Background(), "c1")
	if !errors.Is(err, ErrContainerNotRunning) {
		t.Fatalf("expected ErrContainerNotRunning, got %v", err)
	}
	if fake.attachCalled {
		t.Fatal("attach should not be called for stopped container")
	}
}

func TestConsoleServiceAttach_ContainerNotFound(t *testing.T) {
	fake := &fakeConsoleDockerClient{inspectErr: errors.New("no such container")}
	svc := &ConsoleService{cli: fake}

	_, err := svc.Attach(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrContainerNotRunning) {
		t.Fatalf("unexpected not running error: %v", err)
	}
}
