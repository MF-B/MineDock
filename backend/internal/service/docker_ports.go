package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"minedock/backend/internal/model"
)

func buildPortBindings(ports []model.PortMapping) (nat.PortSet, nat.PortMap) {
	if len(ports) == 0 {
		return nil, nil
	}

	exposedPorts := make(nat.PortSet, len(ports))
	portBindings := make(nat.PortMap, len(ports))

	for _, p := range ports {
		protocol := strings.ToLower(strings.TrimSpace(p.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}

		containerPort, err := nat.NewPort(protocol, strconv.Itoa(p.Container))
		if err != nil {
			continue
		}

		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = append(portBindings[containerPort], nat.PortBinding{HostPort: strconv.Itoa(p.Host)})
	}

	if len(exposedPorts) == 0 {
		return nil, nil
	}

	return exposedPorts, portBindings
}

func resolveConfigPorts(
	templatePorts []model.PortMapping,
	hostConfig *container.HostConfig,
	requestedPorts []model.PortMapping,
) ([]model.PortMapping, error) {
	if len(templatePorts) == 0 {
		return []model.PortMapping{}, nil
	}

	currentHosts := mapCurrentHostPorts(nil)
	if hostConfig != nil {
		currentHosts = mapCurrentHostPorts(hostConfig.PortBindings)
	}

	requestedHosts := make(map[string]int, len(requestedPorts))
	for _, p := range requestedPorts {
		if p.Container <= 0 || p.Host <= 0 {
			return nil, fmt.Errorf("invalid port mapping: %w", model.ErrInvalidParams)
		}
		protocol := normalizePortProtocol(p.Protocol)
		key := portConfigKey(p.Container, protocol)
		if _, exists := requestedHosts[key]; exists {
			return nil, fmt.Errorf("duplicate port mapping %q: %w", key, model.ErrInvalidParams)
		}
		requestedHosts[key] = p.Host
	}

	resolved := make([]model.PortMapping, 0, len(templatePorts))
	for _, p := range templatePorts {
		if p.Container <= 0 || p.Host <= 0 {
			return nil, fmt.Errorf("template port is invalid: %w", model.ErrTemplateInvalid)
		}

		protocol := normalizePortProtocol(p.Protocol)
		key := portConfigKey(p.Container, protocol)

		host := p.Host
		if currentHost, ok := currentHosts[key]; ok {
			host = currentHost
		}
		if requestedHost, ok := requestedHosts[key]; ok {
			host = requestedHost
			delete(requestedHosts, key)
		}

		resolved = append(resolved, model.PortMapping{Host: host, Container: p.Container, Protocol: protocol})
	}

	if len(requestedHosts) > 0 {
		return nil, fmt.Errorf("unknown port mapping: %w", model.ErrInvalidParams)
	}

	return resolved, nil
}

func mapCurrentHostPorts(portBindings nat.PortMap) map[string]int {
	out := make(map[string]int, len(portBindings))
	for containerPort, bindings := range portBindings {
		if len(bindings) == 0 {
			continue
		}

		hostPort, err := strconv.Atoi(strings.TrimSpace(bindings[0].HostPort))
		if err != nil || hostPort <= 0 {
			continue
		}

		containerValue, err := strconv.Atoi(containerPort.Port())
		if err != nil || containerValue <= 0 {
			continue
		}

		key := portConfigKey(containerValue, normalizePortProtocol(containerPort.Proto()))
		out[key] = hostPort
	}
	return out
}

func normalizePortProtocol(protocol string) string {
	normalized := strings.ToLower(strings.TrimSpace(protocol))
	if normalized == "" {
		return "tcp"
	}
	return normalized
}

func portConfigKey(containerPort int, protocol string) string {
	return fmt.Sprintf("%d/%s", containerPort, normalizePortProtocol(protocol))
}
