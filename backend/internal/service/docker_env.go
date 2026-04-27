package service

import (
	"fmt"
	"strings"
)

func mapToDockerEnv(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	env := make([]string, 0, len(m))
	for key, value := range m {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func dockerEnvToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
