package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"minedock/backend/internal/model"
)

const (
	managedLabelKey   = "minedock.managed"
	managedLabelValue = "true"
	nameLabelKey      = "minedock.name"
	gameIDLabelKey    = "minedock.game_id"
)

// dockerClient 定义 DockerService 依赖的 Docker API。
type dockerClient interface {
	ContainerCreate(
		ctx context.Context,
		config *container.Config,
		hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig,
		platform *ocispec.Platform,
		containerName string,
	) (container.CreateResponse, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
}

// InstanceStore 定义 DockerService 依赖的持久化操作。
type InstanceStore interface {
	Save(ctx context.Context, inst model.Instance) error
	Get(ctx context.Context, containerID string) (model.Instance, bool, error)
	Delete(ctx context.Context, containerID string) error
}

// GameRegistry 定义 DockerService 依赖的游戏与模板查询能力。
type GameRegistry interface {
	GetGame(ctx context.Context, id string) (model.Game, error)
	GetTemplate(ctx context.Context, id string) (model.GameTemplate, error)
}

// InstanceConfig 描述容器当前生效的可编辑配置。
type InstanceConfig struct {
	GameID    string                `json:"game_id"`
	Status    string                `json:"status"`
	Ports     []model.PortMapping   `json:"ports"`
	Params    map[string]string     `json:"params"`
	Resources *model.ResourceLimits `json:"resources,omitempty"`
}

// DockerService 封装容器管理相关业务逻辑。
type DockerService struct {
	cli      dockerClient
	store    InstanceStore
	registry GameRegistry
}

// NewDockerService 使用依赖项创建 DockerService。
func NewDockerService(cli dockerClient, s InstanceStore, registry GameRegistry) *DockerService {
	return &DockerService{cli: cli, store: s, registry: registry}
}

// CreateInstance 创建托管容器并持久化实例元数据。
// TODO: 让 Docker 创建与 SQLite 保存具备原子性。
func (s *DockerService) CreateInstance(
	ctx context.Context,
	name, gameID string,
	params map[string]string,
	ports []model.PortMapping,
	resources *model.ResourceLimits,
) (string, error) {
	if s.registry == nil {
		return "", fmt.Errorf("game registry is not configured")
	}

	game, err := s.registry.GetGame(ctx, gameID)
	if err != nil {
		return "", err
	}

	tpl, err := s.registry.GetTemplate(ctx, game.ID)
	if err != nil {
		return "", err
	}

	env, err := mergeTemplateEnv(tpl, params)
	if err != nil {
		return "", err
	}

	imageRef := tpl.Image.FullImageRef()
	if strings.TrimSpace(imageRef) == "" {
		return "", model.ErrTemplateInvalid
	}

	if err := s.ensureImage(ctx, imageRef); err != nil {
		return "", err
	}

	resolvedPorts, err := resolveConfigPorts(tpl.Container.Ports, nil, ports)
	if err != nil {
		return "", err
	}

	exposedPorts, portBindings := buildPortBindings(resolvedPorts)
	cmd := []string(nil)
	if len(tpl.Container.Command) > 0 {
		cmd = append(cmd, tpl.Container.Command...)
	}
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        buildVolumeBinds(name, tpl.Container.Volumes),
	}
	effectiveResources := tpl.Container.Resources
	if resources != nil {
		effectiveResources = resources
	}
	if err := applyResourceLimits(hostConfig, effectiveResources); err != nil {
		return "", err
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image:        imageRef,
		Cmd:          cmd,
		Env:          mapToDockerEnv(env),
		ExposedPorts: exposedPorts,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    false,
		Labels: map[string]string{
			managedLabelKey: managedLabelValue,
			nameLabelKey:    name,
			gameIDLabelKey:  game.ID,
		},
	}, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	inst := model.Instance{ContainerID: resp.ID, Name: name, GameID: game.ID, Status: "Stopped"}
	if err := s.store.Save(ctx, inst); err != nil {
		// 说明：请求上下文取消时，清理逻辑会使用独立上下文做尽力回收。
		_ = s.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("save instance record: %w", err)
	}

	return resp.ID, nil
}

// GetInstanceConfig 读取容器当前生效的用户可调参数。
func (s *DockerService) GetInstanceConfig(ctx context.Context, containerID string) (*InstanceConfig, error) {
	if s.registry == nil {
		return nil, fmt.Errorf("game registry is not configured")
	}

	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}
	if inspect.Config == nil {
		return nil, fmt.Errorf("inspect container config is empty")
	}

	gameID := strings.TrimSpace(inspect.Config.Labels[gameIDLabelKey])
	if gameID == "" {
		return nil, fmt.Errorf("container %q is missing %s label", containerID, gameIDLabelKey)
	}

	tpl, err := s.registry.GetTemplate(ctx, gameID)
	if err != nil {
		return nil, err
	}

	ports, err := resolveConfigPorts(tpl.Container.Ports, inspect.HostConfig, nil)
	if err != nil {
		return nil, err
	}

	containerEnv := dockerEnvToMap(inspect.Config.Env)
	params := make(map[string]string, len(tpl.Params))
	for _, param := range tpl.Params {
		envKey := strings.TrimSpace(param.EnvVar)
		if envKey == "" {
			envKey = param.Key
		}

		value, ok := containerEnv[envKey]
		if !ok {
			if defaultValue, hasDefault := stringifyTemplateDefault(param); hasDefault {
				value = defaultValue
			}
		}

		params[param.Key] = value
	}

	return &InstanceConfig{
		GameID:    gameID,
		Status:    instanceStatusFromState(inspect.State),
		Ports:     ports,
		Params:    params,
		Resources: readResourceLimits(inspect.HostConfig),
	}, nil
}

// UpdateInstanceConfig 通过重建容器应用新的用户参数。
func (s *DockerService) UpdateInstanceConfig(
	ctx context.Context,
	containerID string,
	newParams map[string]string,
	newPorts []model.PortMapping,
	newResources *model.ResourceLimits,
) (string, error) {
	if s.registry == nil {
		return "", fmt.Errorf("game registry is not configured")
	}

	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspect container: %w", err)
	}
	if inspect.Config == nil {
		return "", fmt.Errorf("inspect container config is empty")
	}
	if inspect.State != nil && inspect.State.Running {
		return "", model.ErrContainerNotStopped
	}

	gameID := strings.TrimSpace(inspect.Config.Labels[gameIDLabelKey])
	if gameID == "" {
		return "", fmt.Errorf("container %q is missing %s label", containerID, gameIDLabelKey)
	}

	tpl, err := s.registry.GetTemplate(ctx, gameID)
	if err != nil {
		return "", err
	}

	env, err := mergeTemplateEnv(tpl, newParams)
	if err != nil {
		return "", err
	}

	ports, err := resolveConfigPorts(tpl.Container.Ports, inspect.HostConfig, newPorts)
	if err != nil {
		return "", err
	}

	exposedPorts, portBindings := buildPortBindings(ports)

	containerName := strings.TrimPrefix(inspect.Name, "/")
	instanceName := strings.TrimSpace(inspect.Config.Labels[nameLabelKey])
	if instanceName == "" {
		instanceName = containerName
	}
	if instanceName == "" {
		instanceName = containerID
	}

	hostConfig := &container.HostConfig{}
	if inspect.HostConfig != nil {
		hostConfig.Binds = append([]string(nil), inspect.HostConfig.Binds...)
	}
	hostConfig.PortBindings = portBindings
	resolvedResources := readResourceLimits(inspect.HostConfig)
	if newResources != nil {
		resolvedResources = newResources
	}
	if err := applyResourceLimits(hostConfig, resolvedResources); err != nil {
		return "", err
	}

	labels := copyStringMap(inspect.Config.Labels)
	labels[managedLabelKey] = managedLabelValue
	labels[nameLabelKey] = instanceName
	labels[gameIDLabelKey] = gameID

	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: false}); err != nil {
		return "", fmt.Errorf("remove old container: %w", err)
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image:        inspect.Config.Image,
		Cmd:          append([]string(nil), inspect.Config.Cmd...),
		Env:          mapToDockerEnv(env),
		ExposedPorts: exposedPorts,
		Tty:          inspect.Config.Tty,
		AttachStdin:  inspect.Config.AttachStdin,
		AttachStdout: inspect.Config.AttachStdout,
		AttachStderr: inspect.Config.AttachStderr,
		OpenStdin:    inspect.Config.OpenStdin,
		StdinOnce:    inspect.Config.StdinOnce,
		Labels:       labels,
	}, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("create replacement container: %w", err)
	}

	newInst := model.Instance{
		ContainerID: resp.ID,
		Name:        instanceName,
		GameID:      gameID,
		Status:      "Stopped",
	}

	if err := s.store.Delete(ctx, containerID); err != nil {
		return "", fmt.Errorf("delete old instance record: %w", err)
	}
	if err := s.store.Save(ctx, newInst); err != nil {
		return "", fmt.Errorf("save new instance record: %w", err)
	}

	return resp.ID, nil
}

// StartInstance 启动托管容器并更新持久化记录。
func (s *DockerService) StartInstance(ctx context.Context, containerID string) error {
	if err := s.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	inst, err := s.readInstance(ctx, containerID, "Running")
	if err != nil {
		return err
	}
	if err := s.store.Save(ctx, inst); err != nil {
		return fmt.Errorf("save instance state: %w", err)
	}

	return nil
}

// StopInstance 停止托管容器并更新持久化记录。
func (s *DockerService) StopInstance(ctx context.Context, containerID string) error {
	timeout := 10
	if err := s.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}

	inst, err := s.readInstance(ctx, containerID, "Stopped")
	if err != nil {
		return err
	}
	if err := s.store.Save(ctx, inst); err != nil {
		return fmt.Errorf("save instance state: %w", err)
	}

	return nil
}

// ListInstances 列出托管容器并将快照元数据同步到存储层。
// TODO: 将逐条 Save 改为批量或事务化同步路径。
// TODO: 增加并发写保护，避免最后写入覆盖前写入。
func (s *DockerService) ListInstances(ctx context.Context) ([]model.Instance, error) {
	args := filters.NewArgs()
	args.Add("label", managedLabelKey+"="+managedLabelValue)

	containers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: args})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	instances := make([]model.Instance, 0, len(containers))
	for _, c := range containers {
		// 说明：名称回退顺序是先 label，再 Docker 容器名。
		name := c.Labels[nameLabelKey]
		if strings.TrimSpace(name) == "" && len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		gameID := strings.TrimSpace(c.Labels[gameIDLabelKey])
		status := "Stopped"
		if strings.EqualFold(c.State, "running") {
			status = "Running"
		}
		inst := model.Instance{
			ContainerID: c.ID,
			Name:        name,
			GameID:      gameID,
			Status:      status,
		}
		// 说明：当前 Save 为尽力而为，避免影响列表返回。
		// TODO: 在不影响列表返回的前提下上报同步失败。
		_ = s.store.Save(ctx, inst)
		instances = append(instances, inst)
	}

	return instances, nil
}

// DeleteInstance 删除已停止的托管容器及其持久化记录。
func (s *DockerService) DeleteInstance(ctx context.Context, containerID string) error {
	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}

	if inspect.State != nil && inspect.State.Running {
		return model.ErrInstanceRunning
	}

	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: false}); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	if err := s.store.Delete(ctx, containerID); err != nil {
		return fmt.Errorf("delete instance record: %w", err)
	}
	return nil
}

// readInstance 从存储层读取实例信息并与运行态进行对账。
// TODO: 将该函数拆分为存储读取与 Docker 对账两个辅助函数。
func (s *DockerService) readInstance(ctx context.Context, containerID string, fallbackStatus string) (model.Instance, error) {
	inst, ok, err := s.store.Get(ctx, containerID)
	if err != nil {
		return model.Instance{}, fmt.Errorf("read instance: %w", err)
	}
	if ok && strings.TrimSpace(inst.GameID) != "" {
		// 说明：命中存储后仍会应用调用方传入的兜底状态。
		inst.Status = fallbackStatus
		return inst, nil
	}

	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return model.Instance{}, fmt.Errorf("inspect container: %w", err)
	}

	name := ""
	gameID := ""
	if inspect.Config != nil && inspect.Config.Labels != nil {
		name = strings.TrimSpace(inspect.Config.Labels[nameLabelKey])
		gameID = strings.TrimSpace(inspect.Config.Labels[gameIDLabelKey])
	}
	if name == "" {
		name = strings.TrimPrefix(inspect.Name, "/")
	}
	if gameID == "" && ok {
		gameID = strings.TrimSpace(inst.GameID)
	}

	status := fallbackStatus
	if inspect.State != nil {
		// 说明：当 inspect 有运行态数据时，以 Docker 真实状态覆盖兜底状态。
		if inspect.State.Running {
			status = "Running"
		} else {
			status = "Stopped"
		}
	}

	return model.Instance{ContainerID: containerID, Name: name, GameID: gameID, Status: status}, nil
}
