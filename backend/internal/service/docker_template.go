package service

import (
	"fmt"
	"strconv"
	"strings"

	"minedock/backend/internal/model"
)

func mergeTemplateEnv(tpl model.GameTemplate, params map[string]string) (map[string]string, error) {
	merged := make(map[string]string, len(tpl.Container.Env)+len(tpl.Params))
	for key, value := range tpl.Container.Env {
		merged[key] = value
	}

	paramDefs := make(map[string]model.TemplateParam, len(tpl.Params))
	for _, param := range tpl.Params {
		paramDefs[param.Key] = param

		defaultValue, ok := stringifyTemplateDefault(param)
		if !ok {
			continue
		}
		envKey := strings.TrimSpace(param.EnvVar)
		if envKey == "" {
			envKey = param.Key
		}
		merged[envKey] = defaultValue
	}

	for key, rawValue := range params {
		paramKey := strings.TrimSpace(key)
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
		merged[envKey] = normalized
	}

	return merged, nil
}

func normalizeParamValue(param model.TemplateParam, raw string) (string, error) {
	switch param.Type {
	case "string":
		return raw, nil
	case "number":
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return "", fmt.Errorf("param %q requires number value: %w", param.Key, model.ErrInvalidParams)
		}
		if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
			return "", fmt.Errorf("param %q has invalid number %q: %w", param.Key, raw, model.ErrInvalidParams)
		}
		return trimmed, nil
	case "boolean":
		trimmed := strings.TrimSpace(strings.ToLower(raw))
		if trimmed == "" {
			return "", fmt.Errorf("param %q requires boolean value: %w", param.Key, model.ErrInvalidParams)
		}
		v, err := strconv.ParseBool(trimmed)
		if err != nil {
			return "", fmt.Errorf("param %q has invalid boolean %q: %w", param.Key, raw, model.ErrInvalidParams)
		}
		if v {
			return "true", nil
		}
		return "false", nil
	case "select":
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return "", fmt.Errorf("param %q requires selected value: %w", param.Key, model.ErrInvalidParams)
		}
		for _, option := range param.Options {
			if option.Value == trimmed {
				return trimmed, nil
			}
		}
		return "", fmt.Errorf("param %q has unsupported value %q: %w", param.Key, raw, model.ErrInvalidParams)
	default:
		return "", fmt.Errorf("param %q has unsupported type %q: %w", param.Key, param.Type, model.ErrTemplateInvalid)
	}
}

func stringifyTemplateDefault(param model.TemplateParam) (string, bool) {
	if param.Default == nil {
		return "", false
	}

	switch param.Type {
	case "string", "select", "number":
		value := strings.TrimSpace(fmt.Sprint(param.Default))
		if value == "" {
			return "", false
		}
		return value, true
	case "boolean":
		v, err := strconv.ParseBool(strings.TrimSpace(strings.ToLower(fmt.Sprint(param.Default))))
		if err != nil {
			return "", false
		}
		if v {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}
