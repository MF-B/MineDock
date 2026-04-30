package model

// StoredInstanceConfig is the desired configuration persisted for an instance.
type StoredInstanceConfig struct {
	SchemaVersion int               `json:"schema_version"`
	GameID        string            `json:"game_id"`
	Source        string            `json:"source"`
	Image         string            `json:"image"`
	Env           map[string]string `json:"env"`
	Ports         []PortMapping     `json:"ports"`
	Resources     *ResourceLimits   `json:"resources,omitempty"`
	GameConfig    map[string]string `json:"game_config,omitempty"`
}
