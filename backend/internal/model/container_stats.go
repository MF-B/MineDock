package model

// ContainerStats describes a single container resource usage snapshot.
type ContainerStats struct {
	Timestamp       int64   `json:"timestamp"`
	CPUPercent      float64 `json:"cpu_percent"`
	MemoryUsedBytes uint64  `json:"memory_used_bytes"`
	MemoryMaxBytes  uint64  `json:"memory_max_bytes"`
	MemoryPercent   float64 `json:"memory_percent"`
	NetworkRxBytes  uint64  `json:"network_rx_bytes"`
	NetworkTxBytes  uint64  `json:"network_tx_bytes"`
	DiskReadBytes   uint64  `json:"disk_read_bytes"`
	DiskWriteBytes  uint64  `json:"disk_write_bytes"`
}
