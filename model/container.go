package model

type ContainerView struct {
	ID     string `json:"id"`
	Name   string `json:"names"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
}
