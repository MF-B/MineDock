//go:build windows

package service

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/yusufpapurcu/wmi"
)

type win32DiskDrive struct {
	DeviceID string
	Model    string
	Size     uint64
}

type win32DiskDriveToDiskPartition struct {
	Antecedent string
	Dependent  string
}

type win32LogicalDiskToPartition struct {
	Antecedent string
	Dependent  string
}

type win32PhysicalMemory struct {
	Capacity             uint64
	MemoryType           uint16
	SMBIOSMemoryType     uint16
	Speed                uint32
	ConfiguredClockSpeed uint32
}

var windowsDiskNameCache struct {
	once  sync.Once
	names map[string]string
}

var windowsMemoryModelCache struct {
	once  sync.Once
	model string
}

func hostDiskNamesByMountpoint() map[string]string {
	windowsDiskNameCache.once.Do(func() {
		windowsDiskNameCache.names = loadWindowsDiskNamesByMountpoint()
	})

	result := make(map[string]string, len(windowsDiskNameCache.names))
	for mountpoint, name := range windowsDiskNameCache.names {
		result[mountpoint] = name
	}
	return result
}

func hostMemoryModel(totalBytes uint64) string {
	windowsMemoryModelCache.once.Do(func() {
		windowsMemoryModelCache.model = loadWindowsMemoryModel(totalBytes)
	})
	return windowsMemoryModelCache.model
}

func loadWindowsDiskNamesByMountpoint() map[string]string {
	var drives []win32DiskDrive
	if err := wmi.Query("SELECT DeviceID, Model, Size FROM Win32_DiskDrive", &drives); err != nil {
		return map[string]string{}
	}

	var drivePartitions []win32DiskDriveToDiskPartition
	if err := wmi.Query("SELECT Antecedent, Dependent FROM Win32_DiskDriveToDiskPartition", &drivePartitions); err != nil {
		return map[string]string{}
	}

	var logicalPartitions []win32LogicalDiskToPartition
	if err := wmi.Query("SELECT Antecedent, Dependent FROM Win32_LogicalDiskToPartition", &logicalPartitions); err != nil {
		return map[string]string{}
	}

	drivesByID := make(map[string]win32DiskDrive, len(drives))
	for _, drive := range drives {
		key := normalizeWMIValue(drive.DeviceID)
		if key != "" {
			drivesByID[key] = drive
		}
	}

	partitionToDrive := make(map[string]string, len(drivePartitions))
	for _, relation := range drivePartitions {
		driveID := normalizeWMIValue(wmiReferenceValue(relation.Antecedent, "DeviceID"))
		partitionID := normalizeWMIValue(wmiReferenceValue(relation.Dependent, "DeviceID"))
		if driveID != "" && partitionID != "" {
			partitionToDrive[partitionID] = driveID
		}
	}

	result := map[string]string{}
	for _, relation := range logicalPartitions {
		partitionID := normalizeWMIValue(wmiReferenceValue(relation.Antecedent, "DeviceID"))
		logicalDiskID := normalizeMountpoint(wmiReferenceValue(relation.Dependent, "DeviceID"))
		driveID := partitionToDrive[partitionID]
		drive, ok := drivesByID[driveID]
		if !ok || logicalDiskID == "" {
			continue
		}
		if name := formatWindowsDiskDriveName(drive); name != "" {
			result[logicalDiskID] = name
		}
	}

	return result
}

func loadWindowsMemoryModel(totalBytes uint64) string {
	var modules []win32PhysicalMemory
	if err := wmi.Query("SELECT Capacity, MemoryType, SMBIOSMemoryType, Speed, ConfiguredClockSpeed FROM Win32_PhysicalMemory", &modules); err != nil {
		return ""
	}
	return formatWindowsMemoryModel(modules, totalBytes)
}

func wmiReferenceValue(objectPath, key string) string {
	lowerPath := strings.ToLower(objectPath)
	marker := strings.ToLower(key + "=\"")
	index := strings.Index(lowerPath, marker)
	if index < 0 {
		return ""
	}

	start := index + len(marker)
	rest := objectPath[start:]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func normalizeWMIValue(value string) string {
	normalized := strings.TrimSpace(value)
	for strings.Contains(normalized, "\\\\") {
		normalized = strings.ReplaceAll(normalized, "\\\\", "\\")
	}
	return strings.ToUpper(normalized)
}

func formatWindowsDiskDriveName(drive win32DiskDrive) string {
	model := normalizeSpaces(drive.Model)
	if model == "" {
		return ""
	}
	if drive.Size == 0 || stringContainsCapacity(model) {
		return model
	}
	return model + " " + formatDiskSize(drive.Size)
}

func formatWindowsMemoryModel(modules []win32PhysicalMemory, fallbackTotalBytes uint64) string {
	if len(modules) == 0 {
		return ""
	}

	totalBytes := fallbackTotalBytes
	var moduleTotalBytes uint64
	typeNames := make([]string, 0, len(modules))
	speeds := make([]uint32, 0, len(modules))
	for _, module := range modules {
		moduleTotalBytes += module.Capacity
		if typeName := memoryTypeName(module); typeName != "" {
			typeNames = append(typeNames, typeName)
		}
		if speed := memoryClockSpeed(module); speed > 0 {
			speeds = append(speeds, speed)
		}
	}
	if moduleTotalBytes > 0 {
		totalBytes = moduleTotalBytes
	}

	typeName := commonString(typeNames)
	speedLabel := memorySpeedLabel(speeds)
	capacity := formatMemoryCapacity(totalBytes)

	switch {
	case typeName != "" && speedLabel != "" && capacity != "":
		if strings.Contains(speedLabel, "-") {
			return fmt.Sprintf("%s %s MHz %s", typeName, speedLabel, capacity)
		}
		return fmt.Sprintf("%s-%s %s", typeName, speedLabel, capacity)
	case typeName != "" && capacity != "":
		return fmt.Sprintf("%s %s", typeName, capacity)
	case speedLabel != "" && capacity != "":
		return fmt.Sprintf("Memory %s MHz %s", speedLabel, capacity)
	case capacity != "":
		return "System Memory " + capacity
	default:
		return ""
	}
}

func memoryTypeName(module win32PhysicalMemory) string {
	if name := memoryTypeValueName(module.SMBIOSMemoryType); name != "" {
		return name
	}
	return memoryTypeValueName(module.MemoryType)
}

func memoryTypeValueName(value uint16) string {
	switch value {
	case 20:
		return "DDR"
	case 21, 22:
		return "DDR2"
	case 24:
		return "DDR3"
	case 26:
		return "DDR4"
	case 27:
		return "LPDDR"
	case 28:
		return "LPDDR2"
	case 29:
		return "LPDDR3"
	case 30:
		return "LPDDR4"
	case 34:
		return "DDR5"
	case 35:
		return "LPDDR5"
	default:
		return ""
	}
}

func memoryClockSpeed(module win32PhysicalMemory) uint32 {
	if module.ConfiguredClockSpeed > 0 {
		return module.ConfiguredClockSpeed
	}
	return module.Speed
}

func commonString(values []string) string {
	values = uniqueStrings(values)
	if len(values) == 1 {
		return values[0]
	}
	return ""
}

func memorySpeedLabel(values []uint32) string {
	if len(values) == 0 {
		return ""
	}

	unique := make([]uint32, 0, len(values))
	seen := map[uint32]struct{}{}
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	if len(unique) == 0 {
		return ""
	}
	if len(unique) == 1 {
		return fmt.Sprintf("%d", unique[0])
	}

	var minSpeed uint32
	var maxSpeed uint32
	for index, value := range unique {
		if index == 0 || value < minSpeed {
			minSpeed = value
		}
		if value > maxSpeed {
			maxSpeed = value
		}
	}
	return fmt.Sprintf("%d-%d", minSpeed, maxSpeed)
}

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func stringContainsCapacity(value string) bool {
	compact := strings.ToLower(strings.ReplaceAll(value, " ", ""))
	return strings.Contains(compact, "tb") || strings.Contains(compact, "gb")
}

func formatDiskSize(bytes uint64) string {
	if bytes >= 900_000_000_000 {
		return formatDiskSizeValue(float64(bytes)/1_000_000_000_000, "TB")
	}
	return formatDiskSizeValue(float64(bytes)/1_000_000_000, "GB")
}

func formatDiskSizeValue(value float64, unit string) string {
	rounded := math.Round(value)
	if math.Abs(value-rounded) < 0.05 {
		return fmt.Sprintf("%.0f %s", rounded, unit)
	}
	return fmt.Sprintf("%.1f %s", value, unit)
}
