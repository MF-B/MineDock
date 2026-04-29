package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"minedock/backend/internal/model"
)

func buildBindMounts(baseDir, instanceName string, volumes []model.VolumeMount) ([]string, error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	binds := make([]string, 0, len(volumes))
	for _, v := range volumes {
		hostPath, err := safeVolumeDataDir(baseDir, instanceName, v.Name)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(hostPath, 0o755); err != nil {
			return nil, fmt.Errorf("create volume data dir: %w", err)
		}
		bind := fmt.Sprintf("%s:%s", hostPath, strings.TrimSpace(v.ContainerPath))
		if v.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	return binds, nil
}

func safeVolumeDataDir(baseDir, instanceName, volumeName string) (string, error) {
	instanceDir, err := safeInstanceDataDir(baseDir, instanceName)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(instanceDir, "volumes", sanitizeVolumeNameToken(volumeName)))
	if err != nil {
		return "", fmt.Errorf("resolve volume data dir: %w", err)
	}
	rel, err := filepath.Rel(instanceDir, targetAbs)
	if err != nil {
		return "", fmt.Errorf("validate volume data dir: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("invalid volume data dir")
	}

	return targetAbs, nil
}

func sanitizeVolumeNameToken(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "default"
	}

	var b strings.Builder
	b.Grow(len(s))
	lastSeparator := false

	for _, r := range s {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum {
			b.WriteRune(r)
			lastSeparator = false
			continue
		}

		if r == '-' || r == '_' || r == '.' {
			if !lastSeparator {
				b.WriteRune(r)
				lastSeparator = true
			}
			continue
		}

		if !lastSeparator {
			b.WriteByte('-')
			lastSeparator = true
		}
	}

	token := strings.Trim(b.String(), "-_.")
	if token == "" {
		return "default"
	}

	first := token[0]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		token = "v-" + token
	}

	return token
}
