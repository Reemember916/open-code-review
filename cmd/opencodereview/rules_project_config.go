package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const defaultRulesProjectConfigPath = ".opencodereview/project.json"

type rulesScanProjectConfig struct {
	Paths           []string `json:"paths,omitempty"`
	Ruleset         string   `json:"ruleset,omitempty"`
	RoleConfig      string   `json:"role_config,omitempty"`
	Baseline        string   `json:"baseline,omitempty"`
	CompileCommands string   `json:"compile_commands,omitempty"`
	Include         []string `json:"include,omitempty"`
	Define          []string `json:"define,omitempty"`
	Undefine        []string `json:"undefine,omitempty"`
	Std             string   `json:"std,omitempty"`
	Platform        string   `json:"platform,omitempty"`
	FailOn          string   `json:"fail_on,omitempty"`
}

func loadRulesScanProjectConfig(path string) (*rulesScanProjectConfig, string, error) {
	explicit := path != ""
	if path == "" {
		path = defaultRulesProjectConfigPath
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve project config %s: %w", path, err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read project config %s: %w", absPath, err)
	}

	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var cfg rulesScanProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, "", fmt.Errorf("unmarshal project config %s: %w", absPath, err)
	}
	return &cfg, rulesProjectRoot(absPath), nil
}

func rulesProjectRoot(configPath string) string {
	dir := filepath.Dir(configPath)
	if filepath.Base(dir) == ".opencodereview" {
		return filepath.Dir(dir)
	}
	return dir
}

func resolveRulesProjectPath(root, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func resolveRulesProjectPaths(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != "" {
			out = append(out, resolveRulesProjectPath(root, path))
		}
	}
	return out
}
