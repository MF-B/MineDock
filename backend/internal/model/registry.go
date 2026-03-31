package model

// RegistryImage 描述注册表中的一个可用镜像条目。
type RegistryImage struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Description  string            `json:"description"`
	Category     string            `json:"category"`
	Icon         string            `json:"icon"`
	DefaultEnv   map[string]string `json:"default_env"`
	DefaultPorts []string          `json:"default_ports"`
}
