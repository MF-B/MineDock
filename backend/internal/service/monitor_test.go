package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMetricsProvider struct {
	cpuPerCore []float64
	cpuTotal   []float64
	cpuErr     error
	cpuInfo    []cpuInfoStat
	logicalCPU int
	memory     *virtualMemoryStat
	memoryErr  error
	swap       *swapMemoryStat
	partitions []diskPartitionStat
	usage      map[string]*diskUsageStat
	diskIO     map[string]diskIOCountersStat
	netIO      []netIOCountersStat
}

func (f *fakeMetricsProvider) CPUPercent(_ context.Context, percpu bool) ([]float64, error) {
	if f.cpuErr != nil {
		return nil, f.cpuErr
	}
	if percpu {
		return append([]float64(nil), f.cpuPerCore...), nil
	}
	return append([]float64(nil), f.cpuTotal...), nil
}

func (f *fakeMetricsProvider) CPUInfo(_ context.Context) ([]cpuInfoStat, error) {
	return append([]cpuInfoStat(nil), f.cpuInfo...), nil
}

func (f *fakeMetricsProvider) LogicalCPUCount(_ context.Context) (int, error) {
	return f.logicalCPU, nil
}

func (f *fakeMetricsProvider) VirtualMemory(_ context.Context) (*virtualMemoryStat, error) {
	if f.memoryErr != nil {
		return nil, f.memoryErr
	}
	return f.memory, nil
}

func (f *fakeMetricsProvider) SwapMemory(_ context.Context) (*swapMemoryStat, error) {
	if f.swap == nil {
		return &swapMemoryStat{}, nil
	}
	return f.swap, nil
}

func (f *fakeMetricsProvider) DiskPartitions(_ context.Context) ([]diskPartitionStat, error) {
	return append([]diskPartitionStat(nil), f.partitions...), nil
}

func (f *fakeMetricsProvider) DiskUsage(_ context.Context, mountpoint string) (*diskUsageStat, error) {
	usage, ok := f.usage[mountpoint]
	if !ok {
		return nil, errors.New("usage not found")
	}
	return usage, nil
}

func (f *fakeMetricsProvider) DiskIOCounters(_ context.Context) (map[string]diskIOCountersStat, error) {
	result := make(map[string]diskIOCountersStat, len(f.diskIO))
	for key, value := range f.diskIO {
		result[key] = value
	}
	return result, nil
}

func (f *fakeMetricsProvider) NetIOCounters(_ context.Context) ([]netIOCountersStat, error) {
	return append([]netIOCountersStat(nil), f.netIO...), nil
}

func TestMonitorServiceGetServerMetrics_RatesUsePreviousSnapshot(t *testing.T) {
	baseTime := time.Unix(100, 0)
	provider := &fakeMetricsProvider{
		cpuPerCore: []float64{10, 20},
		cpuInfo:    []cpuInfoStat{{ModelName: "Test CPU"}},
		logicalCPU: 2,
		memory: &virtualMemoryStat{
			Total:       8 * 1024 * 1024 * 1024,
			Available:   4 * 1024 * 1024 * 1024,
			Used:        4 * 1024 * 1024 * 1024,
			UsedPercent: 50,
		},
		swap:       &swapMemoryStat{Sin: 1000, Sout: 2000},
		partitions: []diskPartitionStat{{Device: "/dev/sda1", Mountpoint: "/"}},
		usage: map[string]*diskUsageStat{
			"/": {Path: "/", Total: 1000, Used: 250, UsedPercent: 25},
		},
		diskIO: map[string]diskIOCountersStat{
			"sda": {Name: "sda", ReadBytes: 1000, WriteBytes: 2000},
		},
		netIO: []netIOCountersStat{
			{Name: "lo", BytesRecv: 9999, BytesSent: 9999},
			{Name: "eth0", BytesRecv: 5000, BytesSent: 7000},
		},
	}

	svc := newMonitorServiceWithProvider(provider)
	svc.now = func() time.Time { return baseTime }

	first, err := svc.GetServerMetrics(context.Background())
	if err != nil {
		t.Fatalf("first metrics: %v", err)
	}
	if first.CPU.Percent != 15 || first.CPU.LogicalCores != 2 || first.CPU.Model != "Test CPU" {
		t.Fatalf("unexpected cpu metrics: %+v", first.CPU)
	}
	if first.Memory.SwapInBps != 0 || first.Memory.SwapOutBps != 0 {
		t.Fatalf("first sample should not have swap rates: %+v", first.Memory)
	}
	if len(first.Disks) != 1 || first.Disks[0].ReadBps != 0 || first.Network.RxBps != 0 {
		t.Fatalf("first sample should not have io rates: disks=%+v network=%+v", first.Disks, first.Network)
	}

	provider.swap = &swapMemoryStat{Sin: 2024, Sout: 3024}
	provider.diskIO["sda"] = diskIOCountersStat{Name: "sda", ReadBytes: 3048, WriteBytes: 4048}
	provider.netIO = []netIOCountersStat{{Name: "eth0", BytesRecv: 8072, BytesSent: 10072}}
	svc.now = func() time.Time { return baseTime.Add(2 * time.Second) }

	second, err := svc.GetServerMetrics(context.Background())
	if err != nil {
		t.Fatalf("second metrics: %v", err)
	}
	if second.Memory.SwapInBps != 512 || second.Memory.SwapOutBps != 512 {
		t.Fatalf("unexpected swap rates: %+v", second.Memory)
	}
	if len(second.Disks) != 1 || second.Disks[0].ReadBps != 1024 || second.Disks[0].WriteBps != 1024 {
		t.Fatalf("unexpected disk rates: %+v", second.Disks)
	}
	if second.Network.Name != "eth0" || second.Network.RxBps != 1536 || second.Network.TxBps != 1536 {
		t.Fatalf("unexpected network metrics: %+v", second.Network)
	}
}

func TestRateBytesInvalidInputsReturnZero(t *testing.T) {
	if got := rateBytes(10, 20, time.Second, true); got != 0 {
		t.Fatalf("expected reset rate 0, got %f", got)
	}
	if got := rateBytes(20, 10, 0, true); got != 0 {
		t.Fatalf("expected zero elapsed rate 0, got %f", got)
	}
	if got := rateBytes(20, 10, time.Second, false); got != 0 {
		t.Fatalf("expected missing previous rate 0, got %f", got)
	}
}

func TestDiskNamePrefersResolvedDisplayName(t *testing.T) {
	got := diskName(
		diskPartitionStat{Device: "C:", Mountpoint: "C:"},
		&diskUsageStat{Path: "C:"},
		map[string]string{"C:": "ZHITAI TiPlus5000 1 TB"},
	)

	if got != "ZHITAI TiPlus5000 1 TB" {
		t.Fatalf("unexpected disk name: %q", got)
	}
}

func TestMonitorServiceGetServerMetrics_ProviderError(t *testing.T) {
	svc := newMonitorServiceWithProvider(&fakeMetricsProvider{
		cpuErr: errors.New("cpu failed"),
	})

	if _, err := svc.GetServerMetrics(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
