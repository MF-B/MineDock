package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"

	"minedock/backend/internal/model"
)

const defaultCPUPeriod int64 = 100000

func applyResourceLimits(hostConfig *container.HostConfig, limits *model.ResourceLimits) error {
	if hostConfig == nil || limits == nil {
		return nil
	}

	memoryBytes, err := parseMemoryLimit(limits.Memory)
	if err != nil {
		return err
	}
	cpuQuota, err := parseCPUQuota(limits.CPU)
	if err != nil {
		return err
	}

	hostConfig.Memory = memoryBytes
	hostConfig.CPUPeriod = defaultCPUPeriod
	hostConfig.CPUQuota = cpuQuota
	return nil
}

func readResourceLimits(hostConfig *container.HostConfig) *model.ResourceLimits {
	if hostConfig == nil {
		return nil
	}

	memoryBytes := hostConfig.Memory
	cpuPeriod := hostConfig.CPUPeriod
	cpuQuota := hostConfig.CPUQuota
	if memoryBytes <= 0 || cpuPeriod <= 0 || cpuQuota <= 0 {
		return nil
	}

	cpuCores := float64(cpuQuota) / float64(cpuPeriod)
	if cpuCores <= 0 {
		return nil
	}

	return &model.ResourceLimits{
		Memory: formatMemoryLimit(memoryBytes),
		CPU:    roundCPU(cpuCores),
	}
}

func parseMemoryLimit(memory string) (int64, error) {
	trimmed := strings.TrimSpace(memory)
	if trimmed == "" {
		return 0, fmt.Errorf("memory is required: %w", model.ErrInvalidResourceLimits)
	}

	memoryBytes, err := units.RAMInBytes(trimmed)
	if err != nil || memoryBytes <= 0 {
		return 0, fmt.Errorf("invalid memory %q: %w", memory, model.ErrInvalidResourceLimits)
	}

	return memoryBytes, nil
}

func parseCPUQuota(cpuCores float64) (int64, error) {
	if cpuCores <= 0 {
		return 0, fmt.Errorf("cpu must be positive: %w", model.ErrInvalidResourceLimits)
	}

	quota := int64(math.Round(cpuCores * float64(defaultCPUPeriod)))
	if quota <= 0 {
		return 0, fmt.Errorf("cpu quota must be positive: %w", model.ErrInvalidResourceLimits)
	}

	return quota, nil
}

func formatMemoryLimit(memoryBytes int64) string {
	const (
		gib = 1024 * 1024 * 1024
		mib = 1024 * 1024
	)

	if memoryBytes <= 0 {
		return ""
	}
	if memoryBytes%gib == 0 {
		return strconv.FormatInt(memoryBytes/gib, 10) + "g"
	}
	if memoryBytes%mib == 0 {
		return strconv.FormatInt(memoryBytes/mib, 10) + "m"
	}

	return strconv.FormatInt(memoryBytes, 10) + "b"
}

func roundCPU(cpu float64) float64 {
	return math.Round(cpu*100) / 100
}
