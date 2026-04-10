package service

import "github.com/docker/docker/api/types/container"

func instanceStatusFromState(state *container.State) string {
	if state != nil && state.Running {
		return "Running"
	}
	return "Stopped"
}
