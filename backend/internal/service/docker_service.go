package service

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"minedock/backend/internal/model"
)

const (
	managedLabelKey   = "minedock.managed"
	managedLabelValue = "true"
	nameLabelKey      = "minedock.name"
	gameIDLabelKey    = "minedock.game_id"
)

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

// DockerService 封装容器管理相关业务逻辑。
type DockerService struct {
	cli      *client.Client
	store    InstanceStore
	registry GameRegistry
}

// NewDockerService 使用依赖项创建 DockerService。
func NewDockerService(cli *client.Client, s InstanceStore, registry GameRegistry) *DockerService {
	return &DockerService{cli: cli, store: s, registry: registry}
}

// CreateInstance 创建托管容器并持久化实例元数据。
// TODO: 让 Docker 创建与 SQLite 保存具备原子性。
func (s *DockerService) CreateInstance(ctx context.Context, name, gameID string, params map[string]string) (string, error) {
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

	exposedPorts, portBindings := buildPortBindings(tpl.Container.Ports)
	cmd := []string(nil)
	if len(tpl.Container.Command) > 0 {
		cmd = append(cmd, tpl.Container.Command...)
	}
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        buildVolumeBinds(name, tpl.Container.Volumes),
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image:        imageRef,
		Cmd:          cmd,
		Env:          mapToDockerEnv(env),
		ExposedPorts: exposedPorts,
		Labels: map[string]string{
			managedLabelKey: managedLabelValue,
			nameLabelKey:    name,
			gameIDLabelKey:  game.ID,
		},
	}, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	inst := model.Instance{ContainerID: resp.ID, Name: name, Status: "Stopped"}
	if err := s.store.Save(ctx, inst); err != nil {
		// 说明：请求上下文取消时，清理逻辑会使用独立上下文做尽力回收。
		_ = s.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("save instance record: %w", err)
	}

	return resp.ID, nil
}

func mergeTemplateEnv(tpl model.GameTemplate, params map[string]string) (map[string]string, error) {
	merged := make(map[string]string, len(tpl.Container.Env)+len(tpl.Params))
	for key, value := range tpl.Container.Env {
		merged[key] = value
	}

	paramDefs := make(map[string]model.TemplateParam, len(tpl.Params))
	for _, param := range tpl.Params {
		paramDefs[param.Key] = param

		defaultValue, ok := stringifyTemplateDefault(param)
		if !ok {
			continue
		}
		envKey := strings.TrimSpace(param.EnvVar)
		if envKey == "" {
			envKey = param.Key
		}
		merged[envKey] = defaultValue
	}

	for key, rawValue := range params {
		paramKey := strings.TrimSpace(key)
		paramDef, exists := paramDefs[paramKey]
		if !exists {
			return nil, fmt.Errorf("unknown param key %q: %w", paramKey, model.ErrInvalidParams)
		}

		normalized, err := normalizeParamValue(paramDef, rawValue)
		if err != nil {
			return nil, err
		}

		envKey := strings.TrimSpace(paramDef.EnvVar)
		if envKey == "" {
			envKey = paramDef.Key
		}
		merged[envKey] = normalized
	}

	return merged, nil
}

func normalizeParamValue(param model.TemplateParam, raw string) (string, error) {
	switch param.Type {
	case "string":
		return raw, nil
	case "number":
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return "", fmt.Errorf("param %q requires number value: %w", param.Key, model.ErrInvalidParams)
		}
		if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
			return "", fmt.Errorf("param %q has invalid number %q: %w", param.Key, raw, model.ErrInvalidParams)
		}
		return trimmed, nil
	case "boolean":
		trimmed := strings.TrimSpace(strings.ToLower(raw))
		if trimmed == "" {
			return "", fmt.Errorf("param %q requires boolean value: %w", param.Key, model.ErrInvalidParams)
		}
		v, err := strconv.ParseBool(trimmed)
		if err != nil {
			return "", fmt.Errorf("param %q has invalid boolean %q: %w", param.Key, raw, model.ErrInvalidParams)
		}
		if v {
			return "true", nil
		}
		return "false", nil
	case "select":
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return "", fmt.Errorf("param %q requires selected value: %w", param.Key, model.ErrInvalidParams)
		}
		for _, option := range param.Options {
			if option.Value == trimmed {
				return trimmed, nil
			}
		}
		return "", fmt.Errorf("param %q has unsupported value %q: %w", param.Key, raw, model.ErrInvalidParams)
	default:
		return "", fmt.Errorf("param %q has unsupported type %q: %w", param.Key, param.Type, model.ErrTemplateInvalid)
	}
}

func stringifyTemplateDefault(param model.TemplateParam) (string, bool) {
	if param.Default == nil {
		return "", false
	}

	switch param.Type {
	case "string", "select", "number":
		value := strings.TrimSpace(fmt.Sprint(param.Default))
		if value == "" {
			return "", false
		}
		return value, true
	case "boolean":
		v, err := strconv.ParseBool(strings.TrimSpace(strings.ToLower(fmt.Sprint(param.Default))))
		if err != nil {
			return "", false
		}
		if v {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}

func mapToDockerEnv(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	env := make([]string, 0, len(m))
	for key, value := range m {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func buildPortBindings(ports []model.PortMapping) (nat.PortSet, nat.PortMap) {
	if len(ports) == 0 {
		return nil, nil
	}

	exposedPorts := make(nat.PortSet, len(ports))
	portBindings := make(nat.PortMap, len(ports))

	for _, p := range ports {
		protocol := strings.ToLower(strings.TrimSpace(p.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}

		containerPort, err := nat.NewPort(protocol, strconv.Itoa(p.Container))
		if err != nil {
			continue
		}

		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = append(portBindings[containerPort], nat.PortBinding{
			HostPort: strconv.Itoa(p.Host),
		})
	}

	if len(exposedPorts) == 0 {
		return nil, nil
	}

	return exposedPorts, portBindings
}

func buildVolumeBinds(instanceName string, volumes []model.VolumeMount) []string {
	if len(volumes) == 0 {
		return nil
	}

	instanceToken := sanitizeVolumeNameToken(instanceName)
	binds := make([]string, 0, len(volumes))
	for _, v := range volumes {
		volumeToken := sanitizeVolumeNameToken(v.Name)
		dockerVolName := fmt.Sprintf("minedock-%s-%s", instanceToken, volumeToken)
		bind := fmt.Sprintf("%s:%s", dockerVolName, strings.TrimSpace(v.ContainerPath))
		if v.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	return binds
}

func sanitizeVolumeNameToken(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "default"
	}

	var b strings.Builder
	b.Grow(len(s))
	lastSeparator := false

	for _, r := range s {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum {
			b.WriteRune(r)
			lastSeparator = false
			continue
		}

		if r == '-' || r == '_' || r == '.' {
			if !lastSeparator {
				b.WriteRune(r)
				lastSeparator = true
			}
			continue
		}

		if !lastSeparator {
			b.WriteByte('-')
			lastSeparator = true
		}
	}

	token := strings.Trim(b.String(), "-_.")
	if token == "" {
		return "default"
	}

	first := token[0]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		token = "v-" + token
	}

	return token
}

// StartInstance 启动托管容器并更新持久化状态。
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

// StopInstance 停止托管容器并更新持久化状态。
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

// ListInstances 列出托管容器并将快照状态同步到存储层。
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
		status := "Stopped"
		if strings.EqualFold(c.State, "running") {
			status = "Running"
		}
		inst := model.Instance{
			ContainerID: c.ID,
			Name:        name,
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
	if ok {
		// 说明：命中存储后仍会应用调用方传入的兜底状态。
		inst.Status = fallbackStatus
		return inst, nil
	}

	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return model.Instance{}, fmt.Errorf("inspect container: %w", err)
	}

	name := ""
	if inspect.Config != nil && inspect.Config.Labels != nil {
		name = strings.TrimSpace(inspect.Config.Labels[nameLabelKey])
	}
	if name == "" {
		name = strings.TrimPrefix(inspect.Name, "/")
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

	return model.Instance{ContainerID: containerID, Name: name, Status: status}, nil
}

// ensureImage 确保本地存在目标镜像，缺失时自动拉取。
func (s *DockerService) ensureImage(ctx context.Context, imageName string) error {
	list, err := s.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return fmt.Errorf("list images: %w", err)
	}

	for _, img := range list {
		for _, tag := range img.RepoTags {
			if tag == imageName {
				return nil
			}
		}
	}

	rc, err := s.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
	return nil
}
