//go:build windows

package service

import "testing"

func TestFormatWindowsMemoryModelDDRSpeedAndCapacity(t *testing.T) {
	got := formatWindowsMemoryModel([]win32PhysicalMemory{
		{
			Capacity:             16 * 1024 * 1024 * 1024,
			SMBIOSMemoryType:     26,
			ConfiguredClockSpeed: 3200,
		},
		{
			Capacity:             16 * 1024 * 1024 * 1024,
			SMBIOSMemoryType:     26,
			ConfiguredClockSpeed: 3200,
		},
	}, 0)

	if got != "DDR4-3200 32 GB" {
		t.Fatalf("unexpected memory model: %q", got)
	}
}

func TestFormatWindowsMemoryModelMixedSpeed(t *testing.T) {
	got := formatWindowsMemoryModel([]win32PhysicalMemory{
		{
			Capacity:             8 * 1024 * 1024 * 1024,
			SMBIOSMemoryType:     34,
			ConfiguredClockSpeed: 4800,
		},
		{
			Capacity:             8 * 1024 * 1024 * 1024,
			SMBIOSMemoryType:     34,
			ConfiguredClockSpeed: 5200,
		},
	}, 0)

	if got != "DDR5 4800-5200 MHz 16 GB" {
		t.Fatalf("unexpected memory model: %q", got)
	}
}

func TestFormatWindowsMemoryModelFallbacks(t *testing.T) {
	got := formatWindowsMemoryModel([]win32PhysicalMemory{
		{
			Capacity: 8 * 1024 * 1024 * 1024,
		},
	}, 0)

	if got != "System Memory 8 GB" {
		t.Fatalf("unexpected fallback memory model: %q", got)
	}
}
