package model

// ServerMetrics describes one host-level resource usage snapshot.
type ServerMetrics struct {
	Timestamp int64                `json:"timestamp"`
	CPU       ServerCPUMetrics     `json:"cpu"`
	Memory    ServerMemoryMetrics  `json:"memory"`
	Disks     []ServerDiskMetrics  `json:"disks"`
	Network   ServerNetworkMetrics `json:"network"`
}

// ServerCPUMetrics describes host CPU usage.
type ServerCPUMetrics struct {
	Percent      float64   `json:"percent"`
	Cores        []float64 `json:"cores"`
	LogicalCores int       `json:"logical_cores"`
	Model        string    `json:"model"`
}

// ServerMemoryMetrics describes host memory usage.
type ServerMemoryMetrics struct {
	Percent        float64 `json:"percent"`
	UsedBytes      uint64  `json:"used_bytes"`
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	SwapInBps      float64 `json:"swap_in_bps"`
	SwapOutBps     float64 `json:"swap_out_bps"`
	Model          string  `json:"model"`
}

// ServerDiskMetrics describes one mounted filesystem and its block I/O rate.
type ServerDiskMetrics struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Name       string  `json:"name"`
	Mountpoint string  `json:"mountpoint"`
	Percent    float64 `json:"percent"`
	UsedBytes  uint64  `json:"used_bytes"`
	TotalBytes uint64  `json:"total_bytes"`
	ReadBps    float64 `json:"read_bps"`
	WriteBps   float64 `json:"write_bps"`
}

// ServerNetworkMetrics describes aggregate host network throughput.
type ServerNetworkMetrics struct {
	Name  string  `json:"name"`
	RxBps float64 `json:"rx_bps"`
	TxBps float64 `json:"tx_bps"`
}
