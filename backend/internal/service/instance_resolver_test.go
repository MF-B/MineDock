package service

import (
	"testing"

	"minedock/backend/internal/model"
)

func TestResolveMinecraftJavaConfig_ForgeLegacyUsesJava8(t *testing.T) {
	tpl := model.GameTemplate{
		Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"},
		Container: model.ContainerConfig{
			Env:   map[string]string{"EULA": "TRUE", "TYPE": "PAPER"},
			Ports: []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
		},
	}

	cfg, err := resolveMinecraftJavaConfig(tpl, map[string]string{
		"MC_VERSION":     "1.12.1",
		"SERVER_TYPE":    "FORGE",
		"SERVER_VERSION": "14.22.1.2478",
	}, nil, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if cfg.Image != "itzg/minecraft-server:java8" {
		t.Fatalf("unexpected image: %s", cfg.Image)
	}
	if cfg.Env["TYPE"] != "FORGE" || cfg.Env["VERSION"] != "1.12.1" || cfg.Env["FORGE_VERSION"] != "14.22.1.2478" {
		t.Fatalf("unexpected env: %+v", cfg.Env)
	}
	if cfg.GameConfig["java_tag_source"] != "auto" {
		t.Fatalf("unexpected java tag source: %+v", cfg.GameConfig)
	}
}

func TestResolveMinecraftJavaConfig_ManualJavaOverride(t *testing.T) {
	tpl := model.GameTemplate{
		Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"},
	}

	cfg, err := resolveMinecraftJavaConfig(tpl, map[string]string{
		"MC_VERSION":      "1.12.1",
		"SERVER_TYPE":     "FORGE",
		"SERVER_VERSION":  "14.22.1.2478",
		"JAVA_TAG":        "java17",
		"JAVA_TAG_SOURCE": "manual",
	}, nil, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if cfg.Image != "itzg/minecraft-server:java17" {
		t.Fatalf("unexpected image: %s", cfg.Image)
	}
	if cfg.GameConfig["java_tag"] != "java17" || cfg.GameConfig["java_tag_source"] != "manual" {
		t.Fatalf("unexpected game config: %+v", cfg.GameConfig)
	}
}

func TestResolveMinecraftJavaConfig_DefaultsToVanilla(t *testing.T) {
	tpl := model.GameTemplate{
		Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"},
	}

	cfg, err := resolveMinecraftJavaConfig(tpl, map[string]string{"MC_VERSION": "1.21.4"}, nil, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if cfg.Image != "itzg/minecraft-server:java21" {
		t.Fatalf("unexpected image: %s", cfg.Image)
	}
	if cfg.Env["TYPE"] != "VANILLA" {
		t.Fatalf("unexpected env: %+v", cfg.Env)
	}
}

func TestNeoForgeVersionPrefix(t *testing.T) {
	if got := neoForgeVersionPrefix("26.1"); got != "26.1." {
		t.Fatalf("unexpected prefix: %s", got)
	}
	if got := neoForgeVersionPrefix("1.26.1"); got != "26.1." {
		t.Fatalf("unexpected prefix: %s", got)
	}
	if got := neoForgeVersionPrefix("1.21.4"); got != "21.4." {
		t.Fatalf("unexpected prefix: %s", got)
	}
}
