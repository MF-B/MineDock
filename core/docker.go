package core

import (
	"github.com/docker/docker/client"
)

func InitDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}
