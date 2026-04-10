package service

import (
	"fmt"
	"strings"

	"minedock/backend/internal/model"
)

func buildVolumeBinds(instanceName string, volumes []model.VolumeMount) []string {
	if len(volumes) == 0 {
		return nil
	}

	instanceToken := sanitizeVolumeNameToken(instanceName)
	binds := make([]string, 0, len(volumes))
	for _, v := range volumes {
		volumeToken := sanitizeVolumeNameToken(v.Name)
		dockerVolName := fmt.Sprintf("minedock-%s-%s", instanceToken, volumeToken)
		bind := fmt.Sprintf("%s:%s", dockerVolName, strings.TrimSpace(v.ContainerPath))
		if v.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	return binds
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
