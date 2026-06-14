package ruleengine

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bmatcuk/doublestar/v4"
)

type RoleHint struct {
	Role     string `json:"role"`
	Path     string `json:"path,omitempty"`
	Function string `json:"function,omitempty"`
}

type RoleConfig struct {
	Roles []RoleHint `json:"roles"`
}

func LoadRoleConfig(path string) ([]RoleHint, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read role config %s: %w", path, err)
	}
	var cfg RoleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal role config: %w", err)
	}
	if err := validateRoleHints(cfg.Roles); err != nil {
		return nil, err
	}
	return cfg.Roles, nil
}

func validateRoleHints(hints []RoleHint) error {
	for i, hint := range hints {
		if hint.Role == "" {
			return fmt.Errorf("role hint[%d] missing role", i)
		}
		if hint.Path == "" && hint.Function == "" {
			return fmt.Errorf("role hint[%d] requires path or function", i)
		}
	}
	return nil
}

func resolveRoleFromHints(path, functionName string, hints []RoleHint) string {
	path = normalizePath(path)
	for _, hint := range hints {
		if hint.Path != "" {
			matched, _ := doublestar.Match(normalizePath(hint.Path), path)
			if !matched {
				continue
			}
		}
		if hint.Function != "" {
			matched, _ := doublestar.Match(hint.Function, functionName)
			if !matched {
				continue
			}
		}
		return hint.Role
	}
	return ""
}

func patternListMatches(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		matched, _ := doublestar.Match(pattern, value)
		if matched {
			return true
		}
	}
	return false
}
