package service

import (
	"fmt"
	"net"
	"strconv"

	"minedock/backend/internal/model"
)

func checkPortsAvailable(ports []model.PortMapping) error {
	for _, port := range ports {
		if err := checkPortAvailable(port.Host, port.Protocol); err != nil {
			return err
		}
	}
	return nil
}

func checkPortAvailable(hostPort int, protocol string) error {
	if hostPort <= 0 || hostPort > 65535 {
		return fmt.Errorf("invalid host port %d: %w", hostPort, model.ErrInvalidParams)
	}

	addr := ":" + strconv.Itoa(hostPort)
	switch normalizePortProtocol(protocol) {
	case "tcp":
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("host port %d/tcp is unavailable: %w", hostPort, model.ErrPortUnavailable)
		}
		return ln.Close()
	case "udp":
		conn, err := net.ListenPacket("udp", addr)
		if err != nil {
			return fmt.Errorf("host port %d/udp is unavailable: %w", hostPort, model.ErrPortUnavailable)
		}
		return conn.Close()
	default:
		return fmt.Errorf("unsupported port protocol %q: %w", protocol, model.ErrInvalidParams)
	}
}
