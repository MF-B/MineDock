package service

import (
	"fmt"
	"strconv"
	"strings"

	"minedock/backend/internal/model"
)

const (
	minecraftJavaGameID = "minecraft-java"

	mcParamVersion       = "MC_VERSION"
	mcParamServerType    = "SERVER_TYPE"
	mcParamServerVersion = "SERVER_VERSION"
	mcParamLoaderVersion = "LOADER_VERSION"
	mcParamJavaTag       = "JAVA_TAG"
	mcParamJavaTagSource = "JAVA_TAG_SOURCE"

	mcServerVanilla  = "VANILLA"
	mcServerPaper    = "PAPER"
	mcServerFabric   = "FABRIC"
	mcServerForge    = "FORGE"
	mcServerNeoForge = "NEOFORGE"
)

func resolveInstanceCreateConfig(
	tpl model.GameTemplate,
	game model.Game,
	params map[string]string,
	ports []model.PortMapping,
	resources *model.ResourceLimits,
) (*model.StoredInstanceConfig, error) {
	if game.ID == minecraftJavaGameID && shouldUseMinecraftJavaResolver(params) {
		return resolveMinecraftJavaConfig(tpl, params, ports, resources)
	}
	return resolveTemplateInstanceConfig(tpl, game.ID, params, ports, resources)
}

func resolveTemplateInstanceConfig(
	tpl model.GameTemplate,
	gameID string,
	params map[string]string,
	ports []model.PortMapping,
	resources *model.ResourceLimits,
) (*model.StoredInstanceConfig, error) {
	env, err := mergeTemplateEnv(tpl, params)
	if err != nil {
		return nil, err
	}

	resolvedPorts, err := resolveConfigPorts(tpl.Container.Ports, nil, ports)
	if err != nil {
		return nil, err
	}

	imageRef := tpl.Image.FullImageRef()
	if strings.TrimSpace(imageRef) == "" {
		return nil, model.ErrTemplateInvalid
	}

	effectiveResources := tpl.Container.Resources
	if resources != nil {
		effectiveResources = resources
	}

	return &model.StoredInstanceConfig{
		SchemaVersion: 1,
		GameID:        gameID,
		Source:        "manual",
		Image:         imageRef,
		Env:           env,
		Ports:         resolvedPorts,
		Resources:     effectiveResources,
		GameConfig:    map[string]string{"kind": "template"},
	}, nil
}

func shouldUseMinecraftJavaResolver(params map[string]string) bool {
	for _, key := range []string{
		mcParamVersion,
		mcParamServerVersion,
		mcParamLoaderVersion,
		mcParamJavaTag,
		mcParamJavaTagSource,
		"FORGE_VERSION",
		"NEOFORGE_VERSION",
		"FABRIC_LOADER_VERSION",
	} {
		if _, exists := params[key]; exists {
			return true
		}
	}
	return false
}

func resolveMinecraftJavaConfig(
	tpl model.GameTemplate,
	params map[string]string,
	ports []model.PortMapping,
	resources *model.ResourceLimits,
) (*model.StoredInstanceConfig, error) {
	imageName := strings.TrimSpace(tpl.Image.Name)
	if imageName == "" {
		return nil, model.ErrTemplateInvalid
	}

	mcVersion := strings.TrimSpace(params[mcParamVersion])
	if mcVersion == "" {
		mcVersion = "LATEST"
	}

	serverType := strings.ToUpper(strings.TrimSpace(params[mcParamServerType]))
	if serverType == "" {
		serverType = mcServerVanilla
	}
	if !isSupportedMinecraftServerType(serverType) {
		return nil, fmt.Errorf("unsupported minecraft server type %q: %w", serverType, model.ErrInvalidParams)
	}

	serverVersion := minecraftServerVersionFromParams(params, serverType)
	if requiresMinecraftServerVersion(serverType) && strings.TrimSpace(serverVersion) == "" {
		return nil, fmt.Errorf("server version is required for %s: %w", serverType, model.ErrInvalidParams)
	}

	recommendedJava := recommendMinecraftJavaTag(mcVersion, serverType)
	javaTag := strings.TrimSpace(params[mcParamJavaTag])
	javaSource := "auto"
	if javaTag == "" {
		javaTag = recommendedJava
	} else {
		javaSource = "manual"
	}
	if source := strings.TrimSpace(params[mcParamJavaTagSource]); source != "" {
		javaSource = source
	}
	if !isAllowedMinecraftJavaTag(javaTag) {
		return nil, fmt.Errorf("unsupported java tag %q: %w", javaTag, model.ErrInvalidParams)
	}

	env, err := minecraftJavaEnv(tpl, params, mcVersion, serverType, serverVersion)
	if err != nil {
		return nil, err
	}

	resolvedPorts, err := resolveConfigPorts(tpl.Container.Ports, nil, ports)
	if err != nil {
		return nil, err
	}

	effectiveResources := tpl.Container.Resources
	if resources != nil {
		effectiveResources = resources
	}

	gameConfig := map[string]string{
		"kind":              "minecraft_java",
		"mode":              "managed",
		"minecraft_version": mcVersion,
		"server_type":       serverType,
		"java_tag":          javaTag,
		"java_tag_source":   javaSource,
	}
	if serverVersion != "" {
		gameConfig["server_version"] = serverVersion
	}

	return &model.StoredInstanceConfig{
		SchemaVersion: 1,
		GameID:        minecraftJavaGameID,
		Source:        "manual",
		Image:         imageName + ":" + javaTag,
		Env:           env,
		Ports:         resolvedPorts,
		Resources:     effectiveResources,
		GameConfig:    gameConfig,
	}, nil
}

func minecraftJavaEnv(
	tpl model.GameTemplate,
	params map[string]string,
	mcVersion string,
	serverType string,
	serverVersion string,
) (map[string]string, error) {
	env := copyStringMap(tpl.Container.Env)
	env["EULA"] = "TRUE"
	env["VERSION"] = mcVersion
	env["TYPE"] = serverType

	switch serverType {
	case mcServerForge:
		env["FORGE_VERSION"] = serverVersion
	case mcServerNeoForge:
		env["NEOFORGE_VERSION"] = serverVersion
	case mcServerFabric:
		env["FABRIC_LOADER_VERSION"] = serverVersion
	}

	allowedExtraParams := map[string]struct{}{
		mcParamVersion:          {},
		mcParamServerType:       {},
		mcParamServerVersion:    {},
		mcParamLoaderVersion:    {},
		mcParamJavaTag:          {},
		mcParamJavaTagSource:    {},
		"FORGE_VERSION":         {},
		"NEOFORGE_VERSION":      {},
		"FABRIC_LOADER_VERSION": {},
	}

	paramDefs := make(map[string]model.TemplateParam, len(tpl.Params))
	for _, param := range tpl.Params {
		paramDefs[param.Key] = param
	}
	for key, rawValue := range params {
		paramKey := strings.TrimSpace(key)
		if _, exists := allowedExtraParams[paramKey]; exists {
			continue
		}
		paramDef, exists := paramDefs[paramKey]
		if !exists {
			return nil, fmt.Errorf("unknown param key %q: %w", paramKey, model.ErrInvalidParams)
		}
		normalized, err := normalizeParamValue(paramDef, rawValue)
		if err != nil {
			return nil, err
		}
		envKey := strings.TrimSpace(paramDef.EnvVar)
		if envKey == "" {
			envKey = paramDef.Key
		}
		if envKey == "TYPE" || envKey == "VERSION" {
			continue
		}
		env[envKey] = normalized
	}

	for _, param := range tpl.Params {
		envKey := strings.TrimSpace(param.EnvVar)
		if envKey == "" {
			envKey = param.Key
		}
		if _, exists := env[envKey]; exists || envKey == "TYPE" || envKey == "VERSION" {
			continue
		}
		defaultValue, ok := stringifyTemplateDefault(param)
		if ok {
			env[envKey] = defaultValue
		}
	}

	return env, nil
}

func minecraftServerVersionFromParams(params map[string]string, serverType string) string {
	candidates := []string{mcParamServerVersion, mcParamLoaderVersion}
	switch serverType {
	case mcServerForge:
		candidates = append(candidates, "FORGE_VERSION")
	case mcServerNeoForge:
		candidates = append(candidates, "NEOFORGE_VERSION")
	case mcServerFabric:
		candidates = append(candidates, "FABRIC_LOADER_VERSION")
	}
	for _, key := range candidates {
		if value := strings.TrimSpace(params[key]); value != "" {
			return value
		}
	}
	return ""
}

func requiresMinecraftServerVersion(serverType string) bool {
	return serverType == mcServerForge || serverType == mcServerNeoForge || serverType == mcServerFabric
}

func isSupportedMinecraftServerType(serverType string) bool {
	switch serverType {
	case mcServerVanilla, mcServerPaper, mcServerFabric, mcServerForge, mcServerNeoForge:
		return true
	default:
		return false
	}
}

func recommendMinecraftJavaTag(mcVersion string, serverType string) string {
	version := parseMinecraftVersion(mcVersion)
	if serverType == mcServerForge && compareMinecraftVersion(version, minecraftVersion{major: 1, minor: 18, patch: 0}) < 0 {
		return "java8"
	}
	if compareMinecraftVersion(version, minecraftVersion{major: 1, minor: 16, patch: 5}) <= 0 {
		return "java8"
	}
	if version.major == 1 && version.minor == 17 {
		return "java16"
	}
	if compareMinecraftVersion(version, minecraftVersion{major: 1, minor: 20, patch: 4}) <= 0 {
		return "java17"
	}
	return "java21"
}

func isAllowedMinecraftJavaTag(tag string) bool {
	switch strings.TrimSpace(tag) {
	case "java8", "java16", "java17", "java21", "java25":
		return true
	default:
		return false
	}
}

type minecraftVersion struct {
	major int
	minor int
	patch int
}

func parseMinecraftVersion(raw string) minecraftVersion {
	trimmed := strings.TrimSpace(strings.ToUpper(raw))
	if trimmed == "" || trimmed == "LATEST" {
		return minecraftVersion{major: 1, minor: 21, patch: 0}
	}
	parts := strings.Split(trimmed, ".")
	values := [3]int{1, 21, 0}
	for i := 0; i < len(parts) && i < len(values); i++ {
		value, err := strconv.Atoi(parts[i])
		if err != nil {
			return minecraftVersion{major: 1, minor: 21, patch: 0}
		}
		values[i] = value
	}
	return minecraftVersion{major: values[0], minor: values[1], patch: values[2]}
}

func compareMinecraftVersion(a, b minecraftVersion) int {
	if a.major != b.major {
		return a.major - b.major
	}
	if a.minor != b.minor {
		return a.minor - b.minor
	}
	return a.patch - b.patch
}
