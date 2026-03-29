package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"minedock/backend/internal/model"
	"minedock/backend/internal/store"
)

// ErrInstanceRunning indicates the instance is running and must be stopped before delete.
var ErrInstanceRunning = errors.New("instance is running, stop it before delete")

const (
	managedLabelKey   = "minedock.managed"
	managedLabelValue = "true"
	nameLabelKey      = "minedock.name"
	defaultImage      = "alpine:latest"
)

// DockerService contains business logic for container management.
type DockerService struct {
	cli   *client.Client
	store *store.SQLiteStore
	image string
}

// NewDockerService creates a DockerService with its dependencies and runtime image.
func NewDockerService(cli *client.Client, sqliteStore *store.SQLiteStore, imageName string) *DockerService {
	if strings.TrimSpace(imageName) == "" {
		imageName = defaultImage
	}
	return &DockerService{cli: cli, store: sqliteStore, image: imageName}
}

// CreateInstance creates a managed container and persists its metadata.
// TODO(minedock): Make create and save behavior atomic across Docker and SQLite.
func (s *DockerService) CreateInstance(ctx context.Context, name string) (string, error) {
	if err := s.ensureImage(ctx, s.image); err != nil {
		return "", err
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image: s.image,
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
		// NOTE: Best-effort cleanup uses a detached context if request context is canceled.
		_ = s.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		return "", err
	}

	return resp.ID, nil
}

// StartInstance starts a managed container and updates persisted state.
func (s *DockerService) StartInstance(ctx context.Context, containerID string) error {
	if err := s.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	inst, err := s.readInstance(ctx, containerID, "Running")
	if err != nil {
		return err
	}
	if err := s.store.Save(ctx, inst); err != nil {
		return err
	}

	return nil
}

// StopInstance stops a managed container and updates persisted state.
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
		return err
	}

	return nil
}

// ListInstances lists managed containers and synchronizes snapshot state into the store.
// TODO(minedock): Replace per-item Save with a batched or transactional sync path.
// TODO(minedock): Guard concurrent snapshot writes to avoid last-write-wins races.
func (s *DockerService) ListInstances(ctx context.Context) ([]model.Instance, error) {
	args := filters.NewArgs()
	args.Add("label", managedLabelKey+"="+managedLabelValue)

	containers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: args})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	instances := make([]model.Instance, 0, len(containers))
	for _, c := range containers {
		// NOTE: Name fallback order is label value first, then Docker container name.
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
		// NOTE: Save is currently best-effort to keep list responses available.
		// TODO(minedock): Report sync failures without breaking the listing path.
		_ = s.store.Save(ctx, inst)
		instances = append(instances, inst)
	}

	return instances, nil
}

// DeleteInstance removes a stopped managed container and its persisted metadata.
func (s *DockerService) DeleteInstance(ctx context.Context, containerID string) error {
	inspect, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}

	if inspect.State != nil && inspect.State.Running {
		return ErrInstanceRunning
	}

	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: false}); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	if err := s.store.Delete(ctx, containerID); err != nil {
		return err
	}
	return nil
}

// readInstance resolves instance metadata from store and reconciles runtime state.
// TODO(minedock): Split this function into store read and Docker reconcile helpers.
func (s *DockerService) readInstance(ctx context.Context, containerID string, fallbackStatus string) (model.Instance, error) {
	inst, ok, err := s.store.Get(ctx, containerID)
	if err != nil {
		return model.Instance{}, err
	}
	if ok {
		// NOTE: Store hit still applies caller-provided fallback status.
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
		// NOTE: Docker runtime state overrides fallback when inspect data is present.
		if inspect.State.Running {
			status = "Running"
		} else {
			status = "Stopped"
		}
	}

	return model.Instance{ContainerID: containerID, Name: name, Status: status}, nil
}

// ensureImage ensures imageName exists locally and pulls it when missing.
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
