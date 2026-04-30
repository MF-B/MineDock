package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"

	"minedock/backend/internal/model"
)

func TestBuildPortBindings(t *testing.T) {
	t.Run("empty ports", func(t *testing.T) {
		exposed, bindings := buildPortBindings(nil)
		if len(exposed) != 0 {
			t.Fatalf("expected empty exposed ports, got %d", len(exposed))
		}
		if len(bindings) != 0 {
			t.Fatalf("expected empty port bindings, got %d", len(bindings))
		}
	})

	t.Run("tcp mapping", func(t *testing.T) {
		exposed, bindings := buildPortBindings([]model.PortMapping{{
			Host:      25565,
			Container: 25565,
			Protocol:  "tcp",
		}})

		port, err := nat.NewPort("tcp", "25565")
		if err != nil {
			t.Fatalf("new port: %v", err)
		}

		if _, ok := exposed[port]; !ok {
			t.Fatalf("expected exposed port %q", port)
		}
		bound, ok := bindings[port]
		if !ok {
			t.Fatalf("expected binding for port %q", port)
		}
		if len(bound) != 1 {
			t.Fatalf("expected 1 binding, got %d", len(bound))
		}
		if bound[0].HostPort != "25565" {
			t.Fatalf("expected host port 25565, got %s", bound[0].HostPort)
		}
	})

	t.Run("udp mapping", func(t *testing.T) {
		exposed, bindings := buildPortBindings([]model.PortMapping{{
			Host:      19132,
			Container: 19132,
			Protocol:  "udp",
		}})

		port, err := nat.NewPort("udp", "19132")
		if err != nil {
			t.Fatalf("new port: %v", err)
		}

		if _, ok := exposed[port]; !ok {
			t.Fatalf("expected exposed port %q", port)
		}
		bound, ok := bindings[port]
		if !ok {
			t.Fatalf("expected binding for port %q", port)
		}
		if len(bound) != 1 {
			t.Fatalf("expected 1 binding, got %d", len(bound))
		}
		if bound[0].HostPort != "19132" {
			t.Fatalf("expected host port 19132, got %s", bound[0].HostPort)
		}
	})

	t.Run("default protocol is tcp", func(t *testing.T) {
		exposed, bindings := buildPortBindings([]model.PortMapping{{
			Host:      8080,
			Container: 8080,
			Protocol:  "   ",
		}})

		port, err := nat.NewPort("tcp", "8080")
		if err != nil {
			t.Fatalf("new port: %v", err)
		}

		if _, ok := exposed[port]; !ok {
			t.Fatalf("expected exposed port %q", port)
		}
		if _, ok := bindings[port]; !ok {
			t.Fatalf("expected binding for port %q", port)
		}
	})
}

func TestBuildBindMounts(t *testing.T) {
	t.Run("empty volumes", func(t *testing.T) {
		binds, err := buildBindMounts(t.TempDir(), "my-server", nil)
		if err != nil {
			t.Fatalf("buildBindMounts: %v", err)
		}
		if binds != nil {
			t.Fatalf("expected nil binds, got %v", binds)
		}
	})

	t.Run("normal volume", func(t *testing.T) {
		baseDir := t.TempDir()
		binds, err := buildBindMounts(baseDir, "my-server", []model.VolumeMount{{
			Name:          "server-data",
			ContainerPath: "/data",
		}})
		if err != nil {
			t.Fatalf("buildBindMounts: %v", err)
		}

		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		hostPath := filepath.Join(baseDir, "my-server", "volumes", "server-data")
		if binds[0] != fmt.Sprintf("%s:/data", hostPath) {
			t.Fatalf("unexpected bind: %s", binds[0])
		}
		if _, err := os.Stat(hostPath); err != nil {
			t.Fatalf("expected host path to exist: %v", err)
		}
	})

	t.Run("readonly volume", func(t *testing.T) {
		baseDir := t.TempDir()
		binds, err := buildBindMounts(baseDir, "my-server", []model.VolumeMount{{
			Name:          "server-data",
			ContainerPath: "/data",
			ReadOnly:      true,
		}})
		if err != nil {
			t.Fatalf("buildBindMounts: %v", err)
		}

		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		hostPath := filepath.Join(baseDir, "my-server", "volumes", "server-data")
		if binds[0] != fmt.Sprintf("%s:/data:ro", hostPath) {
			t.Fatalf("unexpected bind: %s", binds[0])
		}
	})

	t.Run("instance name with special chars", func(t *testing.T) {
		baseDir := t.TempDir()
		binds, err := buildBindMounts(baseDir, "My Server (US)#1", []model.VolumeMount{{
			Name:          "World Data@Prod",
			ContainerPath: "/data",
		}})
		if err != nil {
			t.Fatalf("buildBindMounts: %v", err)
		}

		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		hostPath := filepath.Join(baseDir, "my-server-us-1", "volumes", "world-data-prod")
		if binds[0] != fmt.Sprintf("%s:/data", hostPath) {
			t.Fatalf("unexpected bind: %s", binds[0])
		}
	})
}

type fakeDockerClient struct {
	inspectResp container.InspectResponse
	inspectErr  error

	createResp   container.CreateResponse
	createErr    error
	createCfg    *container.Config
	createHost   *container.HostConfig
	createName   string
	removeErr    error
	removedIDs   []string
	listResp     []container.Summary
	listErr      error
	startErr     error
	stopErr      error
	imageList    []image.Summary
	imageListErr error
	imagePullErr error
}

func (f *fakeDockerClient) ContainerCreate(
	_ context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	_ *network.NetworkingConfig,
	_ *ocispec.Platform,
	containerName string,
) (container.CreateResponse, error) {
	f.createCfg = config
	f.createHost = hostConfig
	f.createName = containerName
	if f.createErr != nil {
		return container.CreateResponse{}, f.createErr
	}
	if f.createResp.ID == "" {
		f.createResp.ID = "new-container-id"
	}
	return f.createResp, nil
}

func (f *fakeDockerClient) ContainerInspect(_ context.Context, _ string) (container.InspectResponse, error) {
	if f.inspectErr != nil {
		return container.InspectResponse{}, f.inspectErr
	}
	return f.inspectResp, nil
}

func (f *fakeDockerClient) ContainerRemove(_ context.Context, containerID string, _ container.RemoveOptions) error {
	f.removedIDs = append(f.removedIDs, containerID)
	if f.removeErr != nil {
		return f.removeErr
	}
	return nil
}

func (f *fakeDockerClient) ContainerStart(_ context.Context, _ string, _ container.StartOptions) error {
	return f.startErr
}

func (f *fakeDockerClient) ContainerStop(_ context.Context, _ string, _ container.StopOptions) error {
	return f.stopErr
}

func (f *fakeDockerClient) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResp, nil
}

func (f *fakeDockerClient) ImageList(_ context.Context, _ image.ListOptions) ([]image.Summary, error) {
	if f.imageListErr != nil {
		return nil, f.imageListErr
	}
	return f.imageList, nil
}

func (f *fakeDockerClient) ImagePull(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
	if f.imagePullErr != nil {
		return nil, f.imagePullErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

type fakeInstanceStore struct {
	instances map[string]model.Instance

	saveErr   error
	getErr    error
	deleteErr error
}

func newFakeInstanceStore(initial ...model.Instance) *fakeInstanceStore {
	instances := make(map[string]model.Instance, len(initial))
	for _, inst := range initial {
		instances[inst.ContainerID] = inst
	}
	return &fakeInstanceStore{instances: instances}
}

func (f *fakeInstanceStore) Save(_ context.Context, inst model.Instance) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.instances == nil {
		f.instances = map[string]model.Instance{}
	}
	f.instances[inst.ContainerID] = inst
	return nil
}

func (f *fakeInstanceStore) Get(_ context.Context, containerID string) (model.Instance, bool, error) {
	if f.getErr != nil {
		return model.Instance{}, false, f.getErr
	}
	inst, ok := f.instances[containerID]
	return inst, ok, nil
}

func (f *fakeInstanceStore) Delete(_ context.Context, containerID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.instances, containerID)
	return nil
}

type fakeRegistry struct {
	game     model.Game
	gameErr  error
	template model.GameTemplate
	tplErr   error
}

func (f *fakeRegistry) GetGame(_ context.Context, _ string) (model.Game, error) {
	if f.gameErr != nil {
		return model.Game{}, f.gameErr
	}
	return f.game, nil
}

func (f *fakeRegistry) GetTemplate(_ context.Context, _ string) (model.GameTemplate, error) {
	if f.tplErr != nil {
		return model.GameTemplate{}, f.tplErr
	}
	return f.template, nil
}

func newTestDockerService(t *testing.T, cli dockerClient, store InstanceStore, registry GameRegistry) *DockerService {
	t.Helper()
	svc := NewDockerServiceWithDataDir(cli, store, registry, t.TempDir())
	svc.checkPorts = func([]model.PortMapping) error { return nil }
	return svc
}

func TestGetInstanceConfig_Success(t *testing.T) {
	port, err := nat.NewPort("tcp", "25565")
	if err != nil {
		t.Fatalf("new port: %v", err)
	}

	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: true},
				HostConfig: &container.HostConfig{
					Resources: container.Resources{
						Memory:    2 * 1024 * 1024 * 1024,
						CPUPeriod: defaultCPUPeriod,
						CPUQuota:  2 * defaultCPUPeriod,
					},
					PortBindings: nat.PortMap{port: []nat.PortBinding{{HostPort: "25575"}}},
				},
			},
			Config: &container.Config{
				Labels: map[string]string{
					gameIDLabelKey: "minecraft-java",
				},
				Env: []string{
					"EULA=TRUE",
					"TYPE=FABRIC",
					"MAX_PLAYERS=50",
				},
			},
		},
	}
	registry := &fakeRegistry{
		template: model.GameTemplate{
			Container: model.ContainerConfig{
				Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
				Env:   map[string]string{"EULA": "TRUE"},
			},
			Params: []model.TemplateParam{
				{Key: "SERVER_TYPE", Type: "select", Default: "PAPER", EnvVar: "TYPE", Options: []model.ParamOption{{Value: "PAPER", Label: "Paper"}, {Value: "FABRIC", Label: "Fabric"}}},
				{Key: "MAX_PLAYERS", Type: "number", Default: 20},
			},
		},
	}

	svc := newTestDockerService(t, cli, newFakeInstanceStore(), registry)

	cfg, err := svc.GetInstanceConfig(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetInstanceConfig: %v", err)
	}
	if cfg.GameID != "minecraft-java" {
		t.Fatalf("unexpected game id: %s", cfg.GameID)
	}
	if cfg.Status != "Running" {
		t.Fatalf("unexpected status: %s", cfg.Status)
	}
	if cfg.Params["SERVER_TYPE"] != "FABRIC" {
		t.Fatalf("unexpected SERVER_TYPE: %s", cfg.Params["SERVER_TYPE"])
	}
	if cfg.Params["MAX_PLAYERS"] != "50" {
		t.Fatalf("unexpected MAX_PLAYERS: %s", cfg.Params["MAX_PLAYERS"])
	}
	if len(cfg.Ports) != 1 {
		t.Fatalf("expected 1 port mapping, got %+v", cfg.Ports)
	}
	if cfg.Ports[0].Host != 25575 || cfg.Ports[0].Container != 25565 || cfg.Ports[0].Protocol != "tcp" {
		t.Fatalf("unexpected config ports: %+v", cfg.Ports)
	}
	if cfg.Resources == nil {
		t.Fatal("expected resources in config")
	}
	if cfg.Resources.Memory != "2g" || cfg.Resources.CPU != 2 {
		t.Fatalf("unexpected resources: %+v", cfg.Resources)
	}
}

func TestUpdateInstanceConfig_RejectRunning(t *testing.T) {
	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: true}},
			Config: &container.Config{Labels: map[string]string{
				gameIDLabelKey: "minecraft-java",
			}},
		},
	}

	svc := newTestDockerService(t, cli, newFakeInstanceStore(), &fakeRegistry{})

	_, err := svc.UpdateInstanceConfig(context.Background(), "c1", map[string]string{"MAX_PLAYERS": "50"}, nil, nil)
	if !errors.Is(err, model.ErrContainerNotStopped) {
		t.Fatalf("expected ErrContainerNotStopped, got %v", err)
	}
}

func TestUpdateInstanceConfig_Success(t *testing.T) {
	port, err := nat.NewPort("tcp", "25565")
	if err != nil {
		t.Fatalf("new port: %v", err)
	}

	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Name:       "/docker-name",
				HostConfig: &container.HostConfig{Binds: []string{"minedock-server-data:/data"}, PortBindings: nat.PortMap{port: []nat.PortBinding{{HostPort: "25565"}}}},
				State:      &container.State{Running: false},
			},
			Config: &container.Config{
				Image:        "itzg/minecraft-server:latest",
				Cmd:          []string{"/start"},
				Env:          []string{"EULA=TRUE", "TYPE=PAPER", "MAX_PLAYERS=20"},
				ExposedPorts: nat.PortSet{port: struct{}{}},
				Labels: map[string]string{
					managedLabelKey: "true",
					nameLabelKey:    "server-1",
					gameIDLabelKey:  "minecraft-java",
				},
			},
		},
		createResp: container.CreateResponse{ID: "new-id"},
	}
	registry := &fakeRegistry{
		template: model.GameTemplate{
			Container: model.ContainerConfig{
				Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
				Env:   map[string]string{"EULA": "TRUE"},
			},
			Params: []model.TemplateParam{
				{Key: "SERVER_TYPE", Type: "select", Default: "PAPER", EnvVar: "TYPE", Options: []model.ParamOption{{Value: "PAPER", Label: "Paper"}, {Value: "FABRIC", Label: "Fabric"}}},
				{Key: "MAX_PLAYERS", Type: "number", Default: 20},
			},
		},
	}
	store := newFakeInstanceStore(model.Instance{ContainerID: "old-id", Name: "server-1", GameID: "minecraft-java", Status: "Stopped"})

	svc := newTestDockerService(t, cli, store, registry)

	newID, err := svc.UpdateInstanceConfig(context.Background(), "old-id", map[string]string{
		"SERVER_TYPE": "FABRIC",
		"MAX_PLAYERS": "50",
	}, []model.PortMapping{{
		Host:      25575,
		Container: 25565,
		Protocol:  "tcp",
	}}, nil)
	if err != nil {
		t.Fatalf("UpdateInstanceConfig: %v", err)
	}
	if newID != "new-id" {
		t.Fatalf("unexpected new container id: %s", newID)
	}
	if !slices.Contains(cli.removedIDs, "old-id") {
		t.Fatalf("expected old container removed, got %+v", cli.removedIDs)
	}
	if cli.createName != "docker-name" {
		t.Fatalf("unexpected recreated docker name: %s", cli.createName)
	}

	env := dockerEnvToMap(cli.createCfg.Env)
	if env["TYPE"] != "FABRIC" {
		t.Fatalf("unexpected TYPE env: %s", env["TYPE"])
	}
	if env["MAX_PLAYERS"] != "50" {
		t.Fatalf("unexpected MAX_PLAYERS env: %s", env["MAX_PLAYERS"])
	}
	if env["EULA"] != "TRUE" {
		t.Fatalf("unexpected EULA env: %s", env["EULA"])
	}

	bindings := cli.createHost.PortBindings[port]
	if len(bindings) != 1 || bindings[0].HostPort != "25575" {
		t.Fatalf("unexpected recreated host bindings: %+v", cli.createHost.PortBindings)
	}
	if _, ok := cli.createCfg.ExposedPorts[port]; !ok {
		t.Fatalf("expected exposed port %q, got %+v", port, cli.createCfg.ExposedPorts)
	}

	if _, ok := store.instances["old-id"]; ok {
		t.Fatal("expected old instance to be deleted from store")
	}
	newInst, ok := store.instances["new-id"]
	if !ok {
		t.Fatal("expected new instance in store")
	}
	if newInst.GameID != "minecraft-java" || newInst.Name != "server-1" {
		t.Fatalf("unexpected stored instance: %+v", newInst)
	}
}

func TestUpdateInstanceConfig_InvalidParams(t *testing.T) {
	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: false}},
			Config: &container.Config{
				Labels: map[string]string{gameIDLabelKey: "minecraft-java", nameLabelKey: "server-1"},
			},
		},
	}
	registry := &fakeRegistry{
		template: model.GameTemplate{
			Params: []model.TemplateParam{{Key: "MAX_PLAYERS", Type: "number", Default: 20}},
		},
	}

	svc := newTestDockerService(t, cli, newFakeInstanceStore(), registry)

	_, err := svc.UpdateInstanceConfig(context.Background(), "c1", map[string]string{"MAX_PLAYERS": "NaN??"}, nil, nil)
	if !errors.Is(err, model.ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestUpdateInstanceConfig_InvalidPorts(t *testing.T) {
	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: false}},
			Config: &container.Config{
				Labels: map[string]string{gameIDLabelKey: "minecraft-java", nameLabelKey: "server-1"},
			},
		},
	}
	registry := &fakeRegistry{
		template: model.GameTemplate{
			Container: model.ContainerConfig{Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}}, Env: map[string]string{}},
		},
	}

	svc := newTestDockerService(t, cli, newFakeInstanceStore(), registry)

	_, err := svc.UpdateInstanceConfig(context.Background(), "c1", map[string]string{}, []model.PortMapping{{
		Host:      19132,
		Container: 19132,
		Protocol:  "udp",
	}}, nil)
	if !errors.Is(err, model.ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams for unknown ports, got %v", err)
	}
}

func TestCreateInstance_UsesTemplateResources(t *testing.T) {
	cli := &fakeDockerClient{}
	store := newFakeInstanceStore()
	registry := &fakeRegistry{
		game: model.Game{ID: "minecraft-java", Name: "Minecraft Java"},
		template: model.GameTemplate{
			Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"},
			Container: model.ContainerConfig{
				Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
				Env:   map[string]string{"EULA": "TRUE"},
				Resources: &model.ResourceLimits{
					Memory: "2g",
					CPU:    2,
				},
			},
		},
	}

	svc := newTestDockerService(t, cli, store, registry)

	_, err := svc.CreateInstance(context.Background(), "server-1", "minecraft-java", map[string]string{}, nil, nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if cli.createHost == nil {
		t.Fatal("expected host config")
	}
	if cli.createHost.Memory != 2*1024*1024*1024 {
		t.Fatalf("unexpected memory: %d", cli.createHost.Memory)
	}
	if cli.createHost.CPUPeriod != defaultCPUPeriod || cli.createHost.CPUQuota != 2*defaultCPUPeriod {
		t.Fatalf("unexpected cpu limits: period=%d quota=%d", cli.createHost.CPUPeriod, cli.createHost.CPUQuota)
	}
}

func TestCreateInstance_UsesBindMountDataDir(t *testing.T) {
	dataDir := t.TempDir()
	cli := &fakeDockerClient{}
	store := newFakeInstanceStore()
	registry := &fakeRegistry{
		game: model.Game{ID: "minecraft-java", Name: "Minecraft Java"},
		template: model.GameTemplate{
			Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"},
			Container: model.ContainerConfig{
				Volumes: []model.VolumeMount{{
					Name:          "World Data",
					ContainerPath: "/data",
					ReadOnly:      true,
				}},
			},
		},
	}

	svc := NewDockerServiceWithDataDir(cli, store, registry, dataDir)

	_, err := svc.CreateInstance(context.Background(), "Server One", "minecraft-java", map[string]string{}, nil, nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	hostPath := filepath.Join(dataDir, "server-one", "volumes", "world-data")
	expectedBind := fmt.Sprintf("%s:/data:ro", hostPath)
	if len(cli.createHost.Binds) != 1 || cli.createHost.Binds[0] != expectedBind {
		t.Fatalf("unexpected binds: %+v", cli.createHost.Binds)
	}
	if _, err := os.Stat(hostPath); err != nil {
		t.Fatalf("expected bind mount host path to exist: %v", err)
	}
}

func TestCreateInstance_OverrideResources(t *testing.T) {
	cli := &fakeDockerClient{}
	store := newFakeInstanceStore()
	registry := &fakeRegistry{
		game: model.Game{ID: "minecraft-java", Name: "Minecraft Java"},
		template: model.GameTemplate{
			Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"},
			Container: model.ContainerConfig{
				Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
				Env:   map[string]string{"EULA": "TRUE"},
				Resources: &model.ResourceLimits{
					Memory: "2g",
					CPU:    2,
				},
			},
		},
	}

	svc := newTestDockerService(t, cli, store, registry)

	_, err := svc.CreateInstance(
		context.Background(),
		"server-1",
		"minecraft-java",
		map[string]string{},
		nil,
		&model.ResourceLimits{Memory: "1g", CPU: 1},
	)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if cli.createHost == nil {
		t.Fatal("expected host config")
	}
	if cli.createHost.Memory != 1024*1024*1024 {
		t.Fatalf("unexpected memory: %d", cli.createHost.Memory)
	}
	if cli.createHost.CPUQuota != defaultCPUPeriod {
		t.Fatalf("unexpected cpu quota: %d", cli.createHost.CPUQuota)
	}
}

func TestCreateInstance_InvalidResources(t *testing.T) {
	cli := &fakeDockerClient{}
	store := newFakeInstanceStore()
	registry := &fakeRegistry{
		game: model.Game{ID: "minecraft-java", Name: "Minecraft Java"},
		template: model.GameTemplate{
			Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"},
			Container: model.ContainerConfig{
				Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
				Env:   map[string]string{"EULA": "TRUE"},
			},
		},
	}

	svc := newTestDockerService(t, cli, store, registry)

	_, err := svc.CreateInstance(
		context.Background(),
		"server-1",
		"minecraft-java",
		map[string]string{},
		nil,
		&model.ResourceLimits{Memory: "bad", CPU: 0},
	)
	if !errors.Is(err, model.ErrInvalidResourceLimits) {
		t.Fatalf("expected ErrInvalidResourceLimits, got %v", err)
	}
}

func TestUpdateInstanceConfig_OverrideResources(t *testing.T) {
	port, err := nat.NewPort("tcp", "25565")
	if err != nil {
		t.Fatalf("new port: %v", err)
	}

	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Name: "/docker-name",
				HostConfig: &container.HostConfig{
					Binds:        []string{"minedock-server-data:/data"},
					PortBindings: nat.PortMap{port: []nat.PortBinding{{HostPort: "25565"}}},
					Resources: container.Resources{
						Memory:    2 * 1024 * 1024 * 1024,
						CPUPeriod: defaultCPUPeriod,
						CPUQuota:  2 * defaultCPUPeriod,
					},
				},
				State: &container.State{Running: false},
			},
			Config: &container.Config{
				Image: "itzg/minecraft-server:latest",
				Labels: map[string]string{
					managedLabelKey: "true",
					nameLabelKey:    "server-1",
					gameIDLabelKey:  "minecraft-java",
				},
			},
		},
		createResp: container.CreateResponse{ID: "new-id"},
	}
	registry := &fakeRegistry{
		template: model.GameTemplate{
			Container: model.ContainerConfig{
				Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
				Env:   map[string]string{},
			},
		},
	}
	store := newFakeInstanceStore(model.Instance{ContainerID: "old-id", Name: "server-1", GameID: "minecraft-java", Status: "Stopped"})

	svc := newTestDockerService(t, cli, store, registry)

	_, err = svc.UpdateInstanceConfig(
		context.Background(),
		"old-id",
		map[string]string{},
		nil,
		&model.ResourceLimits{Memory: "1g", CPU: 1},
	)
	if err != nil {
		t.Fatalf("UpdateInstanceConfig: %v", err)
	}
	if cli.createHost.Memory != 1024*1024*1024 {
		t.Fatalf("unexpected memory: %d", cli.createHost.Memory)
	}
	if cli.createHost.CPUPeriod != defaultCPUPeriod || cli.createHost.CPUQuota != defaultCPUPeriod {
		t.Fatalf("unexpected cpu limits: period=%d quota=%d", cli.createHost.CPUPeriod, cli.createHost.CPUQuota)
	}
}

func TestListInstances_IncludesGameID(t *testing.T) {
	cli := &fakeDockerClient{
		listResp: []container.Summary{{
			ID:    "c1",
			State: "running",
			Names: []string{"/server-1"},
			Labels: map[string]string{
				nameLabelKey:   "server-1",
				gameIDLabelKey: "minecraft-java",
			},
		}},
	}
	store := newFakeInstanceStore()
	svc := newTestDockerService(t, cli, store, &fakeRegistry{})

	instances, err := svc.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0].GameID != "minecraft-java" {
		t.Fatalf("expected game id minecraft-java, got %s", instances[0].GameID)
	}
	if saved := store.instances["c1"]; saved.GameID != "minecraft-java" {
		t.Fatalf("expected saved game id minecraft-java, got %s", saved.GameID)
	}
}

func TestDeleteInstance_PurgeDataRemovesInstanceDirAndStore(t *testing.T) {
	dataDir := t.TempDir()
	instanceDir := filepath.Join(dataDir, "server-1")
	volumeDir := filepath.Join(instanceDir, "volumes", "world")
	if err := os.MkdirAll(volumeDir, 0o755); err != nil {
		t.Fatalf("mkdir volume dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(volumeDir, "level.dat"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: false},
			},
			Config: &container.Config{Labels: map[string]string{nameLabelKey: "server-1"}},
		},
	}
	store := newFakeInstanceStore(model.Instance{ContainerID: "c1", Name: "server-1", GameID: "minecraft-java"})
	svc := NewDockerServiceWithDataDir(cli, store, &fakeRegistry{}, dataDir)

	if err := svc.DeleteInstance(context.Background(), "c1", true); err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if !slices.Contains(cli.removedIDs, "c1") {
		t.Fatalf("expected container removed, got %+v", cli.removedIDs)
	}
	if _, ok := store.instances["c1"]; ok {
		t.Fatal("expected instance record to be deleted")
	}
	if _, err := os.Stat(instanceDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected instance dir removed, got %v", err)
	}
}

func TestDeleteInstance_PurgeErrorStillDeletesStore(t *testing.T) {
	cli := &fakeDockerClient{
		inspectResp: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: false},
			},
			Config: &container.Config{Labels: map[string]string{nameLabelKey: "server-1"}},
		},
	}
	store := newFakeInstanceStore(model.Instance{ContainerID: "c1", Name: "server-1", GameID: "minecraft-java"})
	svc := NewDockerServiceWithDataDir(cli, store, &fakeRegistry{}, string(rune(0)))

	err := svc.DeleteInstance(context.Background(), "c1", true)
	if err == nil {
		t.Fatal("expected purge error")
	}
	if _, ok := store.instances["c1"]; ok {
		t.Fatal("expected instance record to be deleted after container removal")
	}
}
