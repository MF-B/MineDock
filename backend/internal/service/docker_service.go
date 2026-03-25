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
	"minedock/backend/internal/store"
)

const (
	managedLabelKey   = "minedock.managed"
	managedLabelValue = "true"
	nameLabelKey      = "minedock.name"
	defaultImage      = "alpine:latest"
)

// DockerService contains business logic for container management.
type DockerService struct {
	cli   *client.Client
	store *store.MemoryStore
	image string
}

func NewDockerService(cli *client.Client, memStore *store.MemoryStore, imageName string) *DockerService {
	if strings.TrimSpace(imageName) == "" {
		imageName = defaultImage
	}
	return &DockerService{cli: cli, store: memStore, image: imageName}
}

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

	if err := s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = s.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("start container: %w", err)
	}

	inst := model.Instance{ContainerID: resp.ID, Name: name, Status: "Running"}
	if err := s.store.Save(inst); err != nil {
		_ = s.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		return "", err
	}

	return resp.ID, nil
}

func (s *DockerService) ListInstances(ctx context.Context) ([]model.Instance, error) {
	args := filters.NewArgs()
	args.Add("label", managedLabelKey+"="+managedLabelValue)

	containers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: args})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	instances := make([]model.Instance, 0, len(containers))
	for _, c := range containers {
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
		_ = s.store.Save(inst)
		instances = append(instances, inst)
	}

	return instances, nil
}

func (s *DockerService) DeleteInstance(ctx context.Context, containerID string) error {
	timeout := 10
	_ = s.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	s.store.Delete(containerID)
	return nil
}

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
