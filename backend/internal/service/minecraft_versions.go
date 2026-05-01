package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	minecraftManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
	fabricLoaderURL      = "https://meta.fabricmc.net/v2/versions/loader/%s"
	forgeMetadataURL     = "https://maven.minecraftforge.net/net/minecraftforge/forge/maven-metadata.xml"
	neoForgeMetadataURL  = "https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml"
	versionCacheTTL      = 6 * time.Hour
)

// MinecraftVersionOption describes a selectable Minecraft version.
type MinecraftVersionOption struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// MinecraftLoaderVersionOption describes a selectable loader version.
type MinecraftLoaderVersionOption struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable,omitempty"`
}

// MinecraftVersionService loads Minecraft and loader version metadata.
type MinecraftVersionService struct {
	client *http.Client

	mu               sync.Mutex
	minecraftCache   cachedMinecraftVersions
	loaderCache      map[string]cachedLoaderVersions
	forgeMetadata    cachedStringSlice
	neoForgeMetadata cachedStringSlice
}

type cachedMinecraftVersions struct {
	values    []MinecraftVersionOption
	expiresAt time.Time
}

type cachedLoaderVersions struct {
	values    []MinecraftLoaderVersionOption
	expiresAt time.Time
}

type cachedStringSlice struct {
	values    []string
	expiresAt time.Time
}

// NewMinecraftVersionService creates a Minecraft version metadata service.
func NewMinecraftVersionService() *MinecraftVersionService {
	return &MinecraftVersionService{
		client:      &http.Client{Timeout: 10 * time.Second},
		loaderCache: map[string]cachedLoaderVersions{},
	}
}

// MinecraftVersions returns current Minecraft versions from Mojang's manifest.
func (s *MinecraftVersionService) MinecraftVersions(ctx context.Context) ([]MinecraftVersionOption, error) {
	now := time.Now()
	s.mu.Lock()
	if now.Before(s.minecraftCache.expiresAt) && len(s.minecraftCache.values) > 0 {
		out := append([]MinecraftVersionOption(nil), s.minecraftCache.values...)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	var payload struct {
		Versions []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"versions"`
	}
	if err := s.fetchJSON(ctx, minecraftManifestURL, &payload); err != nil {
		return nil, err
	}

	versions := make([]MinecraftVersionOption, 0, len(payload.Versions))
	for _, item := range payload.Versions {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		versions = append(versions, MinecraftVersionOption{ID: id, Type: item.Type})
	}

	s.mu.Lock()
	s.minecraftCache = cachedMinecraftVersions{values: append([]MinecraftVersionOption(nil), versions...), expiresAt: now.Add(versionCacheTTL)}
	s.mu.Unlock()
	return versions, nil
}

// LoaderVersions returns loader versions for a Minecraft version and server type.
func (s *MinecraftVersionService) LoaderVersions(
	ctx context.Context,
	mcVersion string,
	serverType string,
) ([]MinecraftLoaderVersionOption, error) {
	mcVersion = strings.TrimSpace(mcVersion)
	serverType = strings.ToUpper(strings.TrimSpace(serverType))
	if mcVersion == "" {
		return nil, fmt.Errorf("mc_version is required")
	}
	if serverType == "" || serverType == mcServerVanilla || serverType == mcServerPaper {
		return []MinecraftLoaderVersionOption{}, nil
	}

	cacheKey := serverType + ":" + mcVersion
	now := time.Now()
	s.mu.Lock()
	if cached, ok := s.loaderCache[cacheKey]; ok && now.Before(cached.expiresAt) {
		out := append([]MinecraftLoaderVersionOption(nil), cached.values...)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	var versions []MinecraftLoaderVersionOption
	var err error
	switch serverType {
	case mcServerFabric:
		versions, err = s.fabricLoaderVersions(ctx, mcVersion)
	case mcServerForge:
		versions, err = s.forgeVersions(ctx, mcVersion)
	case mcServerNeoForge:
		versions, err = s.neoForgeVersions(ctx, mcVersion)
	default:
		return nil, fmt.Errorf("unsupported server type %q", serverType)
	}
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.loaderCache[cacheKey] = cachedLoaderVersions{values: append([]MinecraftLoaderVersionOption(nil), versions...), expiresAt: now.Add(versionCacheTTL)}
	s.mu.Unlock()
	return versions, nil
}

func (s *MinecraftVersionService) fabricLoaderVersions(ctx context.Context, mcVersion string) ([]MinecraftLoaderVersionOption, error) {
	var payload []struct {
		Loader struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		} `json:"loader"`
	}
	if err := s.fetchJSON(ctx, fmt.Sprintf(fabricLoaderURL, mcVersion), &payload); err != nil {
		return nil, err
	}

	out := make([]MinecraftLoaderVersionOption, 0, len(payload))
	seen := map[string]struct{}{}
	for _, item := range payload {
		version := strings.TrimSpace(item.Loader.Version)
		if version == "" {
			continue
		}
		if _, exists := seen[version]; exists {
			continue
		}
		seen[version] = struct{}{}
		out = append(out, MinecraftLoaderVersionOption{Version: version, Stable: item.Loader.Stable})
	}
	return out, nil
}

func (s *MinecraftVersionService) forgeVersions(ctx context.Context, mcVersion string) ([]MinecraftLoaderVersionOption, error) {
	versions, err := s.mavenVersions(ctx, forgeMetadataURL, &s.forgeMetadata)
	if err != nil {
		return nil, err
	}
	prefix := mcVersion + "-"
	return loaderOptionsFromSuffix(filterVersionsByPrefix(versions, prefix), prefix), nil
}

func (s *MinecraftVersionService) neoForgeVersions(ctx context.Context, mcVersion string) ([]MinecraftLoaderVersionOption, error) {
	versions, err := s.mavenVersions(ctx, neoForgeMetadataURL, &s.neoForgeMetadata)
	if err != nil {
		return nil, err
	}
	prefix := neoForgeVersionPrefix(mcVersion)
	if prefix == "" {
		return []MinecraftLoaderVersionOption{}, nil
	}
	return loaderOptionsFromSuffix(filterVersionsByPrefix(versions, prefix), ""), nil
}

func (s *MinecraftVersionService) mavenVersions(
	ctx context.Context,
	url string,
	cache *cachedStringSlice,
) ([]string, error) {
	now := time.Now()
	s.mu.Lock()
	if now.Before(cache.expiresAt) && len(cache.values) > 0 {
		out := append([]string(nil), cache.values...)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create metadata request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch metadata: status %d", resp.StatusCode)
	}

	var metadata struct {
		Versioning struct {
			Versions []string `xml:"versions>version"`
		} `xml:"versioning"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}
	versions := append([]string(nil), metadata.Versioning.Versions...)
	for i, j := 0, len(versions)-1; i < j; i, j = i+1, j-1 {
		versions[i], versions[j] = versions[j], versions[i]
	}

	s.mu.Lock()
	*cache = cachedStringSlice{values: append([]string(nil), versions...), expiresAt: now.Add(versionCacheTTL)}
	s.mu.Unlock()
	return versions, nil
}

func (s *MinecraftVersionService) fetchJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("fetch json: status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func filterVersionsByPrefix(versions []string, prefix string) []string {
	out := make([]string, 0)
	for _, version := range versions {
		if strings.HasPrefix(version, prefix) {
			out = append(out, version)
		}
	}
	return out
}

func loaderOptionsFromSuffix(versions []string, trimPrefix string) []MinecraftLoaderVersionOption {
	out := make([]MinecraftLoaderVersionOption, 0, len(versions))
	seen := map[string]struct{}{}
	for _, version := range versions {
		value := strings.TrimPrefix(version, trimPrefix)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, MinecraftLoaderVersionOption{Version: value})
	}
	return out
}

func neoForgeVersionPrefix(mcVersion string) string {
	parts := strings.Split(strings.TrimSpace(mcVersion), ".")
	if len(parts) < 2 {
		return ""
	}
	if parts[0] != "1" {
		major := strings.TrimSpace(parts[0])
		minor := strings.TrimSpace(parts[1])
		if major == "" || minor == "" {
			return ""
		}
		return major + "." + minor + "."
	}

	minor := strings.TrimSpace(parts[1])
	patch := "0"
	if len(parts) >= 3 {
		patch = strings.TrimSpace(parts[2])
	}
	if minor == "" || patch == "" {
		return ""
	}
	return minor + "." + patch + "."
}
