package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"minedock/backend/internal/model"
)

// RegistryService 提供可用镜像注册表的查询能力。
type RegistryService struct {
	images   []model.RegistryImage
	imageMap map[string]model.RegistryImage
}

// NewRegistryService 从 JSON 文件加载镜像数据并构建查找索引。
func NewRegistryService(filePath string) (*RegistryService, error) {
	path := strings.TrimSpace(filePath)
	if path == "" {
		return nil, fmt.Errorf("registry path is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read registry file: %w", err)
	}

	var images []model.RegistryImage
	if err := json.Unmarshal(content, &images); err != nil {
		return nil, fmt.Errorf("decode registry file: %w", err)
	}

	imageMap := make(map[string]model.RegistryImage, len(images))
	for i, img := range images {
		id := strings.TrimSpace(img.ID)
		if id == "" {
			return nil, fmt.Errorf("registry image at index %d has empty id", i)
		}
		if strings.TrimSpace(img.Image) == "" {
			return nil, fmt.Errorf("registry image %q has empty image", id)
		}
		if _, exists := imageMap[id]; exists {
			return nil, fmt.Errorf("duplicate registry image id: %s", id)
		}
		img.ID = id
		img.Image = strings.TrimSpace(img.Image)
		imageMap[id] = img
		images[i] = img
	}

	return &RegistryService{images: images, imageMap: imageMap}, nil
}

// ListImages 返回注册表中全部可用镜像。
func (s *RegistryService) ListImages(_ context.Context) []model.RegistryImage {
	if s == nil || len(s.images) == 0 {
		return []model.RegistryImage{}
	}
	out := make([]model.RegistryImage, 0, len(s.images))
	for _, img := range s.images {
		out = append(out, cloneRegistryImage(img))
	}
	return out
}

// GetImage 按 ID 查找镜像，未找到时返回 model.ErrImageNotFound。
func (s *RegistryService) GetImage(_ context.Context, id string) (model.RegistryImage, error) {
	if s == nil {
		return model.RegistryImage{}, model.ErrImageNotFound
	}

	key := strings.TrimSpace(id)
	if key == "" {
		return model.RegistryImage{}, model.ErrImageNotFound
	}

	img, ok := s.imageMap[key]
	if !ok {
		return model.RegistryImage{}, model.ErrImageNotFound
	}
	return cloneRegistryImage(img), nil
}

func cloneRegistryImage(img model.RegistryImage) model.RegistryImage {
	clone := img
	if len(img.DefaultEnv) > 0 {
		clone.DefaultEnv = make(map[string]string, len(img.DefaultEnv))
		for k, v := range img.DefaultEnv {
			clone.DefaultEnv[k] = v
		}
	} else {
		clone.DefaultEnv = map[string]string{}
	}

	if len(img.DefaultPorts) > 0 {
		clone.DefaultPorts = append([]string(nil), img.DefaultPorts...)
	} else {
		clone.DefaultPorts = []string{}
	}

	return clone
}
