package service

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/image"
)

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
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("read image pull stream: %w", err)
	}
	return nil
}
