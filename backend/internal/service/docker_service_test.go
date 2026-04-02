package service

import (
	"testing"

	"github.com/docker/go-connections/nat"

	"minedock/backend/internal/model"
)

func TestBuildPortBindings(t *testing.T) {
	t.Run("empty ports", func(t *testing.T) {
		exposed, bindings := buildPortBindings(nil)
		if len(exposed) != 0 {
			t.Fatalf("expected empty exposed ports, got %d", len(exposed))
		}
		if len(bindings) != 0 {
			t.Fatalf("expected empty port bindings, got %d", len(bindings))
		}
	})

	t.Run("tcp mapping", func(t *testing.T) {
		exposed, bindings := buildPortBindings([]model.PortMapping{{
			Host:      25565,
			Container: 25565,
			Protocol:  "tcp",
		}})

		port, err := nat.NewPort("tcp", "25565")
		if err != nil {
			t.Fatalf("new port: %v", err)
		}

		if _, ok := exposed[port]; !ok {
			t.Fatalf("expected exposed port %q", port)
		}
		bound, ok := bindings[port]
		if !ok {
			t.Fatalf("expected binding for port %q", port)
		}
		if len(bound) != 1 {
			t.Fatalf("expected 1 binding, got %d", len(bound))
		}
		if bound[0].HostPort != "25565" {
			t.Fatalf("expected host port 25565, got %s", bound[0].HostPort)
		}
	})

	t.Run("udp mapping", func(t *testing.T) {
		exposed, bindings := buildPortBindings([]model.PortMapping{{
			Host:      19132,
			Container: 19132,
			Protocol:  "udp",
		}})

		port, err := nat.NewPort("udp", "19132")
		if err != nil {
			t.Fatalf("new port: %v", err)
		}

		if _, ok := exposed[port]; !ok {
			t.Fatalf("expected exposed port %q", port)
		}
		bound, ok := bindings[port]
		if !ok {
			t.Fatalf("expected binding for port %q", port)
		}
		if len(bound) != 1 {
			t.Fatalf("expected 1 binding, got %d", len(bound))
		}
		if bound[0].HostPort != "19132" {
			t.Fatalf("expected host port 19132, got %s", bound[0].HostPort)
		}
	})

	t.Run("default protocol is tcp", func(t *testing.T) {
		exposed, bindings := buildPortBindings([]model.PortMapping{{
			Host:      8080,
			Container: 8080,
			Protocol:  "   ",
		}})

		port, err := nat.NewPort("tcp", "8080")
		if err != nil {
			t.Fatalf("new port: %v", err)
		}

		if _, ok := exposed[port]; !ok {
			t.Fatalf("expected exposed port %q", port)
		}
		if _, ok := bindings[port]; !ok {
			t.Fatalf("expected binding for port %q", port)
		}
	})
}

func TestBuildVolumeBinds(t *testing.T) {
	t.Run("empty volumes", func(t *testing.T) {
		binds := buildVolumeBinds("my-server", nil)
		if binds != nil {
			t.Fatalf("expected nil binds, got %v", binds)
		}
	})

	t.Run("normal volume", func(t *testing.T) {
		binds := buildVolumeBinds("my-server", []model.VolumeMount{{
			Name:          "server-data",
			ContainerPath: "/data",
		}})

		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		if binds[0] != "minedock-my-server-server-data:/data" {
			t.Fatalf("unexpected bind: %s", binds[0])
		}
	})

	t.Run("readonly volume", func(t *testing.T) {
		binds := buildVolumeBinds("my-server", []model.VolumeMount{{
			Name:          "server-data",
			ContainerPath: "/data",
			ReadOnly:      true,
		}})

		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		if binds[0] != "minedock-my-server-server-data:/data:ro" {
			t.Fatalf("unexpected bind: %s", binds[0])
		}
	})

	t.Run("instance name with special chars", func(t *testing.T) {
		binds := buildVolumeBinds("My Server (US)#1", []model.VolumeMount{{
			Name:          "World Data@Prod",
			ContainerPath: "/data",
		}})

		if len(binds) != 1 {
			t.Fatalf("expected 1 bind, got %d", len(binds))
		}
		if binds[0] != "minedock-my-server-us-1-world-data-prod:/data" {
			t.Fatalf("unexpected bind: %s", binds[0])
		}
	})
}
