package service

import (
	"context"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	netio "github.com/shirou/gopsutil/v4/net"

	"minedock/backend/internal/model"
)

type metricsProvider interface {
	CPUPercent(ctx context.Context, percpu bool) ([]float64, error)
	CPUInfo(ctx context.Context) ([]cpuInfoStat, error)
	LogicalCPUCount(ctx context.Context) (int, error)
	VirtualMemory(ctx context.Context) (*virtualMemoryStat, error)
	SwapMemory(ctx context.Context) (*swapMemoryStat, error)
	DiskPartitions(ctx context.Context) ([]diskPartitionStat, error)
	DiskUsage(ctx context.Context, mountpoint string) (*diskUsageStat, error)
	DiskIOCounters(ctx context.Context) (map[string]diskIOCountersStat, error)
	NetIOCounters(ctx context.Context) ([]netIOCountersStat, error)
}

type cpuInfoStat struct {
	ModelName string
}

type virtualMemoryStat struct {
	Total       uint64
	Available   uint64
	Used        uint64
	UsedPercent float64
}

type swapMemoryStat struct {
	Sin  uint64
	Sout uint64
}

type diskPartitionStat struct {
	Device     string
	Mountpoint string
}

type diskUsageStat struct {
	Path        string
	Total       uint64
	Used        uint64
	UsedPercent float64
}

type diskIOCountersStat struct {
	Name       string
	ReadBytes  uint64
	WriteBytes uint64
}

type netIOCountersStat struct {
	Name      string
	BytesRecv uint64
	BytesSent uint64
}

type gopsutilProvider struct{}

func (gopsutilProvider) CPUPercent(ctx context.Context, percpu bool) ([]float64, error) {
	return cpu.PercentWithContext(ctx, 0, percpu)
}

func (gopsutilProvider) CPUInfo(ctx context.Context) ([]cpuInfoStat, error) {
	stats, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]cpuInfoStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, cpuInfoStat{ModelName: stat.ModelName})
	}
	return result, nil
}

func (gopsutilProvider) LogicalCPUCount(ctx context.Context) (int, error) {
	return cpu.CountsWithContext(ctx, true)
}

func (gopsutilProvider) VirtualMemory(ctx context.Context) (*virtualMemoryStat, error) {
	stat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &virtualMemoryStat{
		Total:       stat.Total,
		Available:   stat.Available,
		Used:        stat.Used,
		UsedPercent: stat.UsedPercent,
	}, nil
}

func (gopsutilProvider) SwapMemory(ctx context.Context) (*swapMemoryStat, error) {
	stat, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &swapMemoryStat{Sin: stat.Sin, Sout: stat.Sout}, nil
}

func (gopsutilProvider) DiskPartitions(ctx context.Context) ([]diskPartitionStat, error) {
	stats, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}
	result := make([]diskPartitionStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, diskPartitionStat{
			Device:     stat.Device,
			Mountpoint: stat.Mountpoint,
		})
	}
	return result, nil
}

func (gopsutilProvider) DiskUsage(ctx context.Context, mountpoint string) (*diskUsageStat, error) {
	stat, err := disk.UsageWithContext(ctx, mountpoint)
	if err != nil {
		return nil, err
	}
	return &diskUsageStat{
		Path:        stat.Path,
		Total:       stat.Total,
		Used:        stat.Used,
		UsedPercent: stat.UsedPercent,
	}, nil
}

func (gopsutilProvider) DiskIOCounters(ctx context.Context) (map[string]diskIOCountersStat, error) {
	stats, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]diskIOCountersStat, len(stats))
	for name, stat := range stats {
		result[name] = diskIOCountersStat{
			Name:       stat.Name,
			ReadBytes:  stat.ReadBytes,
			WriteBytes: stat.WriteBytes,
		}
	}
	return result, nil
}

func (gopsutilProvider) NetIOCounters(ctx context.Context) ([]netIOCountersStat, error) {
	stats, err := netio.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}
	result := make([]netIOCountersStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, netIOCountersStat{
			Name:      stat.Name,
			BytesRecv: stat.BytesRecv,
			BytesSent: stat.BytesSent,
		})
	}
	return result, nil
}

type diskMetricReading struct {
	metrics    model.ServerDiskMetrics
	readBytes  uint64
	writeBytes uint64
}

type counterSnapshot struct {
	at       time.Time
	swapIn   uint64
	swapOut  uint64
	disks    map[string]diskCounterSnapshot
	netRx    uint64
	netTx    uint64
	hasValue bool
}

type diskCounterSnapshot struct {
	readBytes  uint64
	writeBytes uint64
}

// MonitorService collects host-level resource metrics for the monitor page.
type MonitorService struct {
	provider metricsProvider
	now      func() time.Time

	mu       sync.Mutex
	previous counterSnapshot
}

// NewMonitorService creates a monitor service backed by local OS counters.
func NewMonitorService() *MonitorService {
	return newMonitorServiceWithProvider(gopsutilProvider{})
}

func newMonitorServiceWithProvider(provider metricsProvider) *MonitorService {
	if provider == nil {
		provider = gopsutilProvider{}
	}
	return &MonitorService{
		provider: provider,
		now:      time.Now,
	}
}

// GetServerMetrics returns one host resource metrics snapshot.
func (s *MonitorService) GetServerMetrics(ctx context.Context) (*model.ServerMetrics, error) {
	if s == nil || s.provider == nil {
		return nil, fmt.Errorf("monitor service is not configured")
	}

	now := s.now()
	corePercents, err := s.provider.CPUPercent(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("read cpu percent: %w", err)
	}
	corePercents = clampPercentSlice(corePercents)

	logicalCores, err := s.provider.LogicalCPUCount(ctx)
	if err != nil || logicalCores <= 0 {
		logicalCores = len(corePercents)
	}
	if logicalCores <= 0 {
		logicalCores = 1
	}

	cpuPercent := averageFloat(corePercents)
	if len(corePercents) == 0 {
		totalPercent, err := s.provider.CPUPercent(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("read total cpu percent: %w", err)
		}
		if len(totalPercent) > 0 {
			cpuPercent = clampPercent(totalPercent[0])
		}
	}

	memory, err := s.provider.VirtualMemory(ctx)
	if err != nil {
		return nil, fmt.Errorf("read memory: %w", err)
	}

	swap := &swapMemoryStat{}
	if nextSwap, err := s.provider.SwapMemory(ctx); err == nil && nextSwap != nil {
		swap = nextSwap
	}

	disks := s.readDisks(ctx)
	netName, netRx, netTx := s.readNetwork(ctx)

	current := counterSnapshot{
		at:       now,
		swapIn:   swap.Sin,
		swapOut:  swap.Sout,
		disks:    make(map[string]diskCounterSnapshot, len(disks)),
		netRx:    netRx,
		netTx:    netTx,
		hasValue: true,
	}
	for _, disk := range disks {
		current.disks[disk.metrics.ID] = diskCounterSnapshot{
			readBytes:  disk.readBytes,
			writeBytes: disk.writeBytes,
		}
	}

	s.mu.Lock()
	previous := s.previous
	elapsed := now.Sub(previous.at)
	swapInBps := rateBytes(swap.Sin, previous.swapIn, elapsed, previous.hasValue)
	swapOutBps := rateBytes(swap.Sout, previous.swapOut, elapsed, previous.hasValue)
	netRxBps := rateBytes(netRx, previous.netRx, elapsed, previous.hasValue)
	netTxBps := rateBytes(netTx, previous.netTx, elapsed, previous.hasValue)
	for index := range disks {
		if previousDisk, ok := previous.disks[disks[index].metrics.ID]; ok && previous.hasValue {
			disks[index].metrics.ReadBps = rateBytes(disks[index].readBytes, previousDisk.readBytes, elapsed, true)
			disks[index].metrics.WriteBps = rateBytes(disks[index].writeBytes, previousDisk.writeBytes, elapsed, true)
		}
	}
	s.previous = current
	s.mu.Unlock()

	resultDisks := make([]model.ServerDiskMetrics, 0, len(disks))
	for _, disk := range disks {
		resultDisks = append(resultDisks, disk.metrics)
	}

	return &model.ServerMetrics{
		Timestamp: now.UnixMilli(),
		CPU: model.ServerCPUMetrics{
			Percent:      roundMetric(cpuPercent),
			Cores:        roundMetricSlice(corePercents),
			LogicalCores: logicalCores,
			Model:        s.cpuModel(ctx),
		},
		Memory: model.ServerMemoryMetrics{
			Percent:        roundMetric(clampPercent(memory.UsedPercent)),
			UsedBytes:      memory.Used,
			TotalBytes:     memory.Total,
			AvailableBytes: memory.Available,
			SwapInBps:      roundMetric(swapInBps),
			SwapOutBps:     roundMetric(swapOutBps),
			Model:          memoryModel(memory.Total),
		},
		Disks: resultDisks,
		Network: model.ServerNetworkMetrics{
			Name:  netName,
			RxBps: roundMetric(netRxBps),
			TxBps: roundMetric(netTxBps),
		},
	}, nil
}

func (s *MonitorService) cpuModel(ctx context.Context) string {
	info, err := s.provider.CPUInfo(ctx)
	if err != nil {
		return "CPU"
	}
	for _, stat := range info {
		modelName := strings.TrimSpace(stat.ModelName)
		if modelName != "" {
			return modelName
		}
	}
	return "CPU"
}

func (s *MonitorService) readDisks(ctx context.Context) []diskMetricReading {
	partitions, err := s.provider.DiskPartitions(ctx)
	if err != nil {
		return nil
	}

	ioCounters, err := s.provider.DiskIOCounters(ctx)
	if err != nil {
		ioCounters = map[string]diskIOCountersStat{}
	}
	diskDisplayNames := hostDiskNamesByMountpoint()

	disks := make([]diskMetricReading, 0, len(partitions))
	seen := map[string]struct{}{}
	for _, partition := range partitions {
		mountpoint := strings.TrimSpace(partition.Mountpoint)
		if mountpoint == "" {
			continue
		}
		if _, ok := seen[mountpoint]; ok {
			continue
		}
		seen[mountpoint] = struct{}{}

		usage, err := s.provider.DiskUsage(ctx, mountpoint)
		if err != nil || usage == nil {
			continue
		}

		id := diskID(partition, usage)
		diskMetrics := model.ServerDiskMetrics{
			ID:         id,
			Label:      fmt.Sprintf("Disk %d", len(disks)+1),
			Name:       diskName(partition, usage, diskDisplayNames),
			Mountpoint: mountpoint,
			Percent:    roundMetric(clampPercent(usage.UsedPercent)),
			UsedBytes:  usage.Used,
			TotalBytes: usage.Total,
		}

		reading := diskMetricReading{metrics: diskMetrics}
		if counter, ok := diskCounterForPartition(partition, ioCounters); ok {
			reading.readBytes = counter.ReadBytes
			reading.writeBytes = counter.WriteBytes
		}
		disks = append(disks, reading)
	}

	sort.Slice(disks, func(i, j int) bool {
		return disks[i].metrics.Mountpoint < disks[j].metrics.Mountpoint
	})
	for index := range disks {
		disks[index].metrics.Label = fmt.Sprintf("Disk %d", index+1)
	}
	return disks
}

func (s *MonitorService) readNetwork(ctx context.Context) (string, uint64, uint64) {
	counters, err := s.provider.NetIOCounters(ctx)
	if err != nil {
		return "Network", 0, 0
	}

	selected := make([]netIOCountersStat, 0, len(counters))
	for _, counter := range counters {
		if isLoopbackInterface(counter.Name) {
			continue
		}
		selected = append(selected, counter)
	}
	if len(selected) == 0 {
		selected = counters
	}

	var rx uint64
	var tx uint64
	names := make([]string, 0, len(selected))
	for _, counter := range selected {
		rx += counter.BytesRecv
		tx += counter.BytesSent
		if strings.TrimSpace(counter.Name) != "" {
			names = append(names, counter.Name)
		}
	}

	switch len(names) {
	case 0:
		return "Network", rx, tx
	case 1:
		return names[0], rx, tx
	default:
		return "All interfaces", rx, tx
	}
}

func diskID(partition diskPartitionStat, usage *diskUsageStat) string {
	for _, candidate := range []string{partition.Mountpoint, partition.Device, usage.Path} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return "disk"
}

func diskName(partition diskPartitionStat, usage *diskUsageStat, displayNames map[string]string) string {
	device := strings.TrimSpace(partition.Device)
	mountpoint := strings.TrimSpace(partition.Mountpoint)
	if displayName := strings.TrimSpace(displayNames[normalizeMountpoint(mountpoint)]); displayName != "" {
		return displayName
	}
	if device != "" && mountpoint != "" {
		return fmt.Sprintf("%s (%s)", mountpoint, device)
	}
	if device != "" {
		return device
	}
	if mountpoint != "" {
		return mountpoint
	}
	if usage != nil && strings.TrimSpace(usage.Path) != "" {
		return usage.Path
	}
	return "Disk"
}

func normalizeMountpoint(mountpoint string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(mountpoint, "\\", "/"))
	normalized = strings.TrimRight(normalized, "/")
	return strings.ToUpper(normalized)
}

func diskCounterForPartition(partition diskPartitionStat, counters map[string]diskIOCountersStat) (diskIOCountersStat, bool) {
	for _, key := range diskCounterKeys(partition.Device) {
		if counter, ok := counters[key]; ok {
			return counter, true
		}
		for name, counter := range counters {
			if strings.EqualFold(name, key) || strings.EqualFold(counter.Name, key) {
				return counter, true
			}
		}
	}
	return diskIOCountersStat{}, false
}

func diskCounterKeys(device string) []string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(device, "\\", "/"))
	if cleaned == "" {
		return nil
	}

	keys := []string{cleaned, strings.TrimSuffix(cleaned, "/")}
	base := path.Base(strings.TrimSuffix(cleaned, "/"))
	if base != "." && base != "/" && base != "" {
		keys = append(keys, base)
		if parent := parentBlockDevice(base); parent != "" && parent != base {
			keys = append(keys, parent)
		}
	}
	if len(cleaned) >= 2 && cleaned[1] == ':' {
		keys = append(keys, cleaned[:2])
	}

	return uniqueStrings(keys)
}

func parentBlockDevice(device string) string {
	trimmed := strings.TrimRightFunc(device, func(r rune) bool {
		return r >= '0' && r <= '9'
	})
	trimmed = strings.TrimSuffix(trimmed, "p")
	if trimmed == "" {
		return device
	}
	return trimmed
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func isLoopbackInterface(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "lo" ||
		strings.HasPrefix(normalized, "lo:") ||
		strings.Contains(normalized, "loopback")
}

func memoryModel(totalBytes uint64) string {
	if model := strings.TrimSpace(hostMemoryModel(totalBytes)); model != "" {
		return model
	}
	if totalBytes == 0 {
		return "System Memory"
	}
	return "System Memory " + formatMemoryCapacity(totalBytes)
}

func formatMemoryCapacity(totalBytes uint64) string {
	const gib = 1024 * 1024 * 1024

	if totalBytes == 0 {
		return ""
	}
	value := float64(totalBytes) / gib
	rounded := math.Round(value)
	if math.Abs(value-rounded) < 0.05 {
		return fmt.Sprintf("%.0f GB", rounded)
	}
	return fmt.Sprintf("%.1f GB", value)
}

func rateBytes(current, previous uint64, elapsed time.Duration, hasPrevious bool) float64 {
	if !hasPrevious || elapsed <= 0 || current < previous {
		return 0
	}
	return float64(current-previous) / elapsed.Seconds()
}

func averageFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func clampPercentSlice(values []float64) []float64 {
	result := make([]float64, 0, len(values))
	for _, value := range values {
		result = append(result, clampPercent(value))
	}
	return result
}

func clampPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Min(100, math.Max(0, value))
}

func roundMetricSlice(values []float64) []float64 {
	result := make([]float64, 0, len(values))
	for _, value := range values {
		result = append(result, roundMetric(value))
	}
	return result
}

func roundMetric(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*100) / 100
}
