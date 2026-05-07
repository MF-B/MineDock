package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"

	"minedock/backend/internal/model"
)

// containerStatsClient 定义获取容器资源指标依赖的 Docker API 子集。
type containerStatsClient interface {
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
}

// ContainerStatsService 通过 Docker Stats API 采集单个容器的资源快照。
type ContainerStatsService struct {
	cli containerStatsClient
}

// NewContainerStatsService 创建 ContainerStatsService。
func NewContainerStatsService(cli containerStatsClient) *ContainerStatsService {
	return &ContainerStatsService{cli: cli}
}

// dockerStatsJSON 对应 Docker Stats API 返回的 JSON 结构（仅解析所需字段）。
type dockerStatsJSON struct {
	Read    time.Time `json:"read"`
	PreRead time.Time `json:"preread"`

	CPUStats    dockerCPUStats    `json:"cpu_stats"`
	PreCPUStats dockerCPUStats    `json:"precpu_stats"`
	MemoryStats dockerMemoryStats `json:"memory_stats"`
	Networks    map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlkioStats struct {
		IoServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}

type dockerCPUStats struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemCPUUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs     uint32 `json:"online_cpus"`
}

type dockerMemoryStats struct {
	Usage uint64 `json:"usage"`
	Limit uint64 `json:"limit"`
	Stats struct {
		InactiveFile uint64 `json:"inactive_file"`
		Cache        uint64 `json:"cache"`
	} `json:"stats"`
}

// GetContainerStats 获取容器的单次资源快照。
func (s *ContainerStatsService) GetContainerStats(ctx context.Context, containerID string) (*model.ContainerStats, error) {
	if s == nil || s.cli == nil {
		return nil, fmt.Errorf("container stats service is not configured")
	}

	resp, err := s.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("get container stats: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read container stats body: %w", err)
	}

	var raw dockerStatsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse container stats: %w", err)
	}

	cpuPercent := calculateCPUPercent(raw)

	// Docker 内存使用量扣除 inactive_file / cache 以反映实际工作集。
	memUsed := raw.MemoryStats.Usage
	if raw.MemoryStats.Stats.InactiveFile > 0 {
		memUsed = safeSub(memUsed, raw.MemoryStats.Stats.InactiveFile)
	} else if raw.MemoryStats.Stats.Cache > 0 {
		memUsed = safeSub(memUsed, raw.MemoryStats.Stats.Cache)
	}

	memLimit := raw.MemoryStats.Limit
	var memPercent float64
	if memLimit > 0 {
		memPercent = clampPercent(float64(memUsed) / float64(memLimit) * 100)
	}

	var netRx, netTx uint64
	for _, iface := range raw.Networks {
		netRx += iface.RxBytes
		netTx += iface.TxBytes
	}

	var diskRead, diskWrite uint64
	for _, entry := range raw.BlkioStats.IoServiceBytesRecursive {
		switch entry.Op {
		case "read", "Read":
			diskRead += entry.Value
		case "write", "Write":
			diskWrite += entry.Value
		}
	}

	return &model.ContainerStats{
		Timestamp:       time.Now().UnixMilli(),
		CPUPercent:      roundMetric(cpuPercent),
		MemoryUsedBytes: memUsed,
		MemoryMaxBytes:  memLimit,
		MemoryPercent:   roundMetric(memPercent),
		NetworkRxBytes:  netRx,
		NetworkTxBytes:  netTx,
		DiskReadBytes:   diskRead,
		DiskWriteBytes:  diskWrite,
	}, nil
}

// calculateCPUPercent 按 Docker stats 规范计算 CPU 使用百分比。
func calculateCPUPercent(stats dockerStatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)

	if systemDelta <= 0 || cpuDelta < 0 {
		return 0
	}

	onlineCPUs := stats.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}

	percent := (cpuDelta / systemDelta) * float64(onlineCPUs) * 100
	return clampPercent(percent)
}

func safeSub(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}
