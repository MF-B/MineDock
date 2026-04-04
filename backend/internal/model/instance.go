package model

// Instance describes a managed container instance.
type Instance struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	GameID      string `json:"game_id"`
	Status      string `json:"status"`
}
