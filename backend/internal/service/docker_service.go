package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"minedock/backend/internal/model"
)

const (
	managedLabelKey   = "minedock.managed"
	managedLabelValue = "true"
	nameLabelKey      = "minedock.name"
)

// InstanceStore 定义 DockerService 依赖的持久化操作。
type InstanceStore interface {
	Save(ctx context.Context, inst model.Instance) error
	Get(ctx context.Context, containerID string) (model.Instance, bool, error)
	Delete(ctx context.Context, containerID string) error
}

// ImageRegistry 定义 DockerService 依赖的镜像注册表查询能力。
type ImageRegistry interface {
	GetImage(ctx context.Context, id string) (model.RegistryImage, error)
}

// DockerService 封装容器管理相关业务逻辑。
type DockerService struct {
	cli      *client.Client
	store    InstanceStore
	registry ImageRegistry
}

// NewDockerService 使用依赖项创建 DockerService。
func NewDockerService(cli *client.Client, s InstanceStore, registry ImageRegistry) *DockerService {
	return &DockerService{cli: cli, store: s, registry: registry}
}

// CreateInstance 创建托管容器并持久化实例元数据。
// TODO: 让 Docker 创建与 SQLite 保存具备原子性。
func (s *DockerService) CreateInstance(ctx context.Context, name, imageID string) (string, error) {
	if s.registry == nil {
		return "", fmt.Errorf("image registry is not configured")
	}

	regImage, err := s.registry.GetImage(ctx, imageID)
	if err != nil {
		return "", err
	}

	if err := s.ensureImage(ctx, regImage.Image); err != nil {
		return "", err
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image: regImage.Image,
		Cmd:   []string{"sleep", "3600"},
		Labels: map[string]string{
			managedLabelKey: managedLabelValue,
			nameLabelKey:    name,
		},
	}, nil, nil, nil, "")
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
