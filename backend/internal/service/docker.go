package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"minedock/backend/internal/model"
)

const (
	managedLabelKey    = "minedock.managed"
	managedLabelValue  = "true"
	nameLabelKey       = "minedock.name"
	gameIDLabelKey     = "minedock.game_id"
	configPathLabelKey = "minedock.config_path"
	defaultDataDir     = "data/instances"
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
	ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerKill(ctx context.Context, containerID, signal string) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
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
	GameID     string                `json:"game_id"`
	Status     string                `json:"status"`
	Image      string                `json:"image,omitempty"`
	Ports      []model.PortMapping   `json:"ports"`
	Params     map[string]string     `json:"params"`
	Resources  *model.ResourceLimits `json:"resources,omitempty"`
	GameConfig map[string]string     `json:"game_config,omitempty"`
}

// DockerService 封装容器管理相关业务逻辑。
type DockerService struct {
	cli         dockerClient
	store       InstanceStore
	registry    GameRegistry
	dataDir     string
	configStore *InstanceConfigFileStore
	checkPorts  func([]model.PortMapping) error
}

// NewDockerService 使用依赖项创建 DockerService。
func NewDockerService(cli dockerClient, s InstanceStore, registry GameRegistry) *DockerService {
	return NewDockerServiceWithDataDir(cli, s, registry, defaultDataDir)
}

// NewDockerServiceWithDataDir 使用指定数据目录创建 DockerService。
func NewDockerServiceWithDataDir(cli dockerClient, s InstanceStore, registry GameRegistry, dataDir string) *DockerService {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	return &DockerService{
		cli:         cli,
		store:       s,
		registry:    registry,
		dataDir:     dataDir,
		configStore: NewInstanceConfigFileStore(dataDir),
		checkPorts:  checkPortsAvailable,
	}
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

	desired, err := resolveInstanceCreateConfig(tpl, game, params, ports, resources)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(desired.Image) == "" {
		return "", model.ErrTemplateInvalid
	}

	if err := s.checkPorts(desired.Ports); err != nil {
		return "", err
	}

	configPath, err := s.configStore.ConfigPath(name)
	if err != nil {
		return "", err
	}
	if err := s.configStore.Save(ctx, configPath, desired); err != nil {
		return "", err
	}

	if err := s.ensureImage(ctx, desired.Image); err != nil {
		return "", err
	}

	exposedPorts, portBindings := buildPortBindings(desired.Ports)
	cmd := []string(nil)
	if len(tpl.Container.Command) > 0 {
		cmd = append(cmd, tpl.Container.Command...)
	}
	binds, err := buildBindMounts(s.dataDir, name, tpl.Container.Volumes)
	if err != nil {
		return "", err
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        binds,
	}
	if err := applyResourceLimits(hostConfig, desired.Resources); err != nil {
		return "", err
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image:        desired.Image,
		Cmd:          cmd,
		Env:          mapToDockerEnv(desired.Env),
		ExposedPorts: exposedPorts,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    false,
		Labels: map[string]string{
			managedLabelKey:    managedLabelValue,
			nameLabelKey:       name,
			gameIDLabelKey:     game.ID,
			configPathLabelKey: configPath,
		},
	}, hostConfig, nil, nil, "")
	if err != nil {
		slog.Error("create instance container failed", "name", name, "game_id", game.ID, "error", err)
		return "", fmt.Errorf("create container: %w", err)
	}

	inst := model.Instance{ContainerID: resp.ID, Name: name, GameID: game.ID, Status: "Stopped", ConfigPath: configPath}
	if err := s.store.Save(ctx, inst); err != nil {
		// 说明：请求上下文取消时，清理逻辑会使用独立上下文做尽力回收。
		_ = s.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		slog.Error("save instance record failed", "container_id", resp.ID, "name", name, "game_id", game.ID, "error", err)
		return "", fmt.Errorf("save instance record: %w", err)
	}

	slog.Info("instance created", "container_id", resp.ID, "name", name, "game_id", game.ID)
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

	if stored, ok := s.loadStoredConfig(ctx, inspect, containerID); ok {
		return editableConfigFromStored(stored, tpl, instanceStatusFromState(inspect.State)), nil
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
		Image:     inspect.Config.Image,
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
	if strings.TrimSpace(tpl.Image.Name) == "" && strings.TrimSpace(inspect.Config.Image) != "" {
		tpl.Image = templateImageFromRef(inspect.Config.Image)
	}

	containerName := strings.TrimPrefix(inspect.Name, "/")
	instanceName := strings.TrimSpace(inspect.Config.Labels[nameLabelKey])
	if instanceName == "" {
		instanceName = containerName
	}
	if instanceName == "" {
		instanceName = containerID
	}

	ports := newPorts
	if len(ports) == 0 {
		ports, err = resolveConfigPorts(tpl.Container.Ports, inspect.HostConfig, nil)
		if err != nil {
			return "", err
		}
	}

	resolvedResources := newResources
	if resolvedResources == nil {
		if stored, ok := s.loadStoredConfig(ctx, inspect, containerID); ok && stored.Resources != nil {
			resolvedResources = stored.Resources
		} else {
			resolvedResources = readResourceLimits(inspect.HostConfig)
		}
	}

	desired, err := resolveInstanceCreateConfig(tpl, model.Game{ID: gameID}, newParams, ports, resolvedResources)
	if err != nil {
		return "", err
	}

	if err := s.checkPorts(desired.Ports); err != nil {
		return "", err
	}

	configPath := strings.TrimSpace(inspect.Config.Labels[configPathLabelKey])
	if configPath == "" {
		if inst, ok, err := s.store.Get(ctx, containerID); err == nil && ok {
			configPath = strings.TrimSpace(inst.ConfigPath)
		}
	}
	if configPath == "" {
		configPath, err = s.configStore.ConfigPath(instanceName)
		if err != nil {
			return "", err
		}
	}
	if err := s.configStore.Save(ctx, configPath, desired); err != nil {
		return "", err
	}

	if err := s.ensureImage(ctx, desired.Image); err != nil {
		return "", err
	}

	exposedPorts, portBindings := buildPortBindings(desired.Ports)

	binds, err := buildBindMounts(s.dataDir, instanceName, tpl.Container.Volumes)
	if err != nil {
		return "", err
	}
	hostConfig := &container.HostConfig{Binds: binds}
	hostConfig.PortBindings = portBindings
	if err := applyResourceLimits(hostConfig, desired.Resources); err != nil {
		return "", err
	}

	labels := copyStringMap(inspect.Config.Labels)
	labels[managedLabelKey] = managedLabelValue
	labels[nameLabelKey] = instanceName
	labels[gameIDLabelKey] = gameID
	labels[configPathLabelKey] = configPath

	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: false}); err != nil {
		slog.Error("remove old container failed", "container_id", containerID, "error", err)
		return "", fmt.Errorf("remove old container: %w", err)
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image:        desired.Image,
		Cmd:          append([]string(nil), inspect.Config.Cmd...),
		Env:          mapToDockerEnv(desired.Env),
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
		slog.Error("create replacement container failed", "old_container_id", containerID, "name", instanceName, "game_id", gameID, "error", err)
		return "", fmt.Errorf("create replacement container: %w", err)
	}

	newInst := model.Instance{
		ContainerID: resp.ID,
		Name:        instanceName,
		GameID:      gameID,
		Status:      "Stopped",
		ConfigPath:  configPath,
	}

	if err := s.store.Delete(ctx, containerID); err != nil {
		slog.Error("delete old instance record failed", "container_id", containerID, "error", err)
		return "", fmt.Errorf("delete old instance record: %w", err)
	}
	if err := s.store.Save(ctx, newInst); err != nil {
		slog.Error("save new instance record failed", "container_id", resp.ID, "name", instanceName, "game_id", gameID, "error", err)
		return "", fmt.Errorf("save new instance record: %w", err)
	}

	slog.Info("instance config updated", "old_container_id", containerID, "new_container_id", resp.ID, "name", instanceName, "game_id", gameID)
	return resp.ID, nil
}

// StartInstance 启动托管容器并更新持久化记录。
func (s *DockerService) StartInstance(ctx context.Context, containerID string) error {
	if err := s.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		slog.Error("start instance failed", "container_id", containerID, "error", err)
		return fmt.Errorf("start container: %w", err)
	}

	inst, err := s.readInstance(ctx, containerID, "Running")
	if err != nil {
		return err
	}
	if err := s.store.Save(ctx, inst); err != nil {
		slog.Error("save started instance state failed", "container_id", containerID, "error", err)
		return fmt.Errorf("save instance state: %w", err)
	}

	slog.Info("instance started", "container_id", containerID, "name", inst.Name, "game_id", inst.GameID)
	return nil
}

// StopInstance 停止托管容器并更新持久化记录。
func (s *DockerService) StopInstance(ctx context.Context, containerID string) error {
	timeout := 10
	if err := s.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		slog.Error("stop instance failed", "container_id", containerID, "error", err)
		return fmt.Errorf("stop container: %w", err)
	}

	inst, err := s.readInstance(ctx, containerID, "Stopped")
	if err != nil {
		return err
	}
	if err := s.store.Save(ctx, inst); err != nil {
		slog.Error("save stopped instance state failed", "container_id", containerID, "error", err)
		return fmt.Errorf("save instance state: %w", err)
	}

	slog.Info("instance stopped", "container_id", containerID, "name", inst.Name, "game_id", inst.GameID)
	return nil
}

// RestartInstance 重启运行中的托管容器并保持容器 ID 不变。
func (s *DockerService) RestartInstance(ctx context.Context, containerID string) error {
	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}
	if inspect.State == nil || !inspect.State.Running {
		slog.Warn("restart stopped instance rejected", "container_id", containerID)
		return model.ErrInstanceNotRunning
	}

	timeout := 10
	if err := s.cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		slog.Error("restart instance failed", "container_id", containerID, "error", err)
		return fmt.Errorf("restart container: %w", err)
	}

	inst, err := s.readInstance(ctx, containerID, "Running")
	if err != nil {
		return err
	}
	if err := s.store.Save(ctx, inst); err != nil {
		slog.Error("save restarted instance state failed", "container_id", containerID, "error", err)
		return fmt.Errorf("save instance state: %w", err)
	}

	slog.Info("instance restarted", "container_id", containerID, "name", inst.Name, "game_id", inst.GameID)
	return nil
}

// ForceStopInstance 使用 SIGKILL 强制终止运行中的托管容器。
func (s *DockerService) ForceStopInstance(ctx context.Context, containerID string) error {
	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}
	if inspect.State != nil && inspect.State.Running {
		if err := s.cli.ContainerKill(ctx, containerID, "SIGKILL"); err != nil {
			slog.Error("force stop instance failed", "container_id", containerID, "error", err)
			return fmt.Errorf("kill container: %w", err)
		}
	}

	inst, err := s.readInstance(ctx, containerID, "Stopped")
	if err != nil {
		return err
	}
	if err := s.store.Save(ctx, inst); err != nil {
		slog.Error("save force stopped instance state failed", "container_id", containerID, "error", err)
		return fmt.Errorf("save instance state: %w", err)
	}

	slog.Info("instance force stopped", "container_id", containerID, "name", inst.Name, "game_id", inst.GameID)
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
		configPath := strings.TrimSpace(c.Labels[configPathLabelKey])
		if configPath == "" {
			if existing, ok, err := s.store.Get(ctx, c.ID); err == nil && ok {
				configPath = strings.TrimSpace(existing.ConfigPath)
			}
		}
		inst := model.Instance{
			ContainerID: c.ID,
			Name:        name,
			GameID:      gameID,
			Status:      status,
			ConfigPath:  configPath,
		}
		// 说明：当前 Save 为尽力而为，避免影响列表返回。
		// TODO: 在不影响列表返回的前提下上报同步失败。
		_ = s.store.Save(ctx, inst)
		instances = append(instances, inst)
	}

	return instances, nil
}

// DeleteInstance 删除已停止的托管容器及其持久化记录。
func (s *DockerService) DeleteInstance(ctx context.Context, containerID string, purgeData bool) error {
	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}

	if inspect.State != nil && inspect.State.Running {
		slog.Warn("delete running instance rejected", "container_id", containerID)
		return model.ErrInstanceRunning
	}

	instanceName := instanceNameFromInspect(inspect, containerID)

	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: false}); err != nil {
		slog.Error("remove instance container failed", "container_id", containerID, "name", instanceName, "error", err)
		return fmt.Errorf("remove container: %w", err)
	}
	var purgeErr error
	if purgeData {
		purgeErr = s.removeInstanceData(instanceName)
	}
	if err := s.store.Delete(ctx, containerID); err != nil {
		slog.Error("delete instance record failed", "container_id", containerID, "name", instanceName, "error", err)
		return fmt.Errorf("delete instance record: %w", err)
	}
	if purgeErr != nil {
		slog.Error("purge instance data failed", "container_id", containerID, "name", instanceName, "error", purgeErr)
		return purgeErr
	}
	slog.Info("instance deleted", "container_id", containerID, "name", instanceName, "purge_data", purgeData)
	return nil
}

// ForceDeleteInstance 强制删除托管容器及其持久化记录，允许回收异常残留。
func (s *DockerService) ForceDeleteInstance(ctx context.Context, containerID string, purgeData bool) error {
	instanceName := containerID
	if inst, ok, err := s.store.Get(ctx, containerID); err != nil {
		return fmt.Errorf("read instance: %w", err)
	} else if ok && strings.TrimSpace(inst.Name) != "" {
		instanceName = inst.Name
	}

	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("inspect container: %w", err)
		}
	} else {
		instanceName = instanceNameFromInspect(inspect, instanceName)
	}

	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		if !errdefs.IsNotFound(err) {
			slog.Error("force remove instance container failed", "container_id", containerID, "name", instanceName, "error", err)
			return fmt.Errorf("remove container: %w", err)
		}
	}

	var purgeErr error
	if purgeData {
		purgeErr = s.removeInstanceData(instanceName)
	}
	if err := s.store.Delete(ctx, containerID); err != nil {
		slog.Error("delete force removed instance record failed", "container_id", containerID, "name", instanceName, "error", err)
		return fmt.Errorf("delete instance record: %w", err)
	}
	if purgeErr != nil {
		slog.Error("purge force removed instance data failed", "container_id", containerID, "name", instanceName, "error", purgeErr)
		return purgeErr
	}
	slog.Info("instance force deleted", "container_id", containerID, "name", instanceName, "purge_data", purgeData)
	return nil
}

func instanceNameFromInspect(inspect container.InspectResponse, fallback string) string {
	if inspect.Config != nil {
		if name := strings.TrimSpace(inspect.Config.Labels[nameLabelKey]); name != "" {
			return name
		}
	}
	if name := strings.Trim(strings.TrimSpace(inspect.Name), "/"); name != "" {
		return name
	}
	return fallback
}

func (s *DockerService) removeInstanceData(instanceName string) error {
	target, err := safeInstanceDataDir(s.dataDir, instanceName)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove instance data: %w", err)
	}
	return nil
}

func (s *DockerService) loadStoredConfig(
	ctx context.Context,
	inspect container.InspectResponse,
	containerID string,
) (*model.StoredInstanceConfig, bool) {
	configPath := ""
	if inspect.Config != nil {
		configPath = strings.TrimSpace(inspect.Config.Labels[configPathLabelKey])
	}
	if configPath == "" {
		inst, ok, err := s.store.Get(ctx, containerID)
		if err == nil && ok {
			configPath = strings.TrimSpace(inst.ConfigPath)
		}
	}
	if configPath == "" {
		return nil, false
	}

	cfg, err := s.configStore.Load(ctx, configPath)
	if err != nil {
		return nil, false
	}
	return cfg, true
}

func editableConfigFromStored(
	cfg *model.StoredInstanceConfig,
	tpl model.GameTemplate,
	status string,
) *InstanceConfig {
	params := make(map[string]string, len(tpl.Params))
	env := cfg.Env
	if env == nil {
		env = map[string]string{}
	}
	for _, param := range tpl.Params {
		envKey := strings.TrimSpace(param.EnvVar)
		if envKey == "" {
			envKey = param.Key
		}
		value, ok := env[envKey]
		if !ok {
			if defaultValue, hasDefault := stringifyTemplateDefault(param); hasDefault {
				value = defaultValue
			}
		}
		params[param.Key] = value
	}

	return &InstanceConfig{
		GameID:     cfg.GameID,
		Status:     status,
		Image:      cfg.Image,
		Ports:      append([]model.PortMapping(nil), cfg.Ports...),
		Params:     params,
		Resources:  cfg.Resources,
		GameConfig: copyStringMap(cfg.GameConfig),
	}
}

func templateImageFromRef(imageRef string) model.TemplateImage {
	ref := strings.TrimSpace(imageRef)
	if ref == "" {
		return model.TemplateImage{}
	}
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		return model.TemplateImage{Name: ref[:lastColon], Tag: ref[lastColon+1:]}
	}
	return model.TemplateImage{Name: ref}
}

func safeInstanceDataDir(baseDir, instanceName string) (string, error) {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve data dir: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(baseAbs, sanitizeVolumeNameToken(instanceName)))
	if err != nil {
		return "", fmt.Errorf("resolve instance data dir: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("validate instance data dir: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("invalid instance data dir")
	}
	return targetAbs, nil
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

	configPath := ""
	if inspect.Config != nil && inspect.Config.Labels != nil {
		configPath = strings.TrimSpace(inspect.Config.Labels[configPathLabelKey])
	}
	if configPath == "" && ok {
		configPath = strings.TrimSpace(inst.ConfigPath)
	}

	return model.Instance{ContainerID: containerID, Name: name, GameID: gameID, Status: status, ConfigPath: configPath}, nil
}
