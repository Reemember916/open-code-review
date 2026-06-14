package staticanalysis

import (
	"path/filepath"
	"strings"
)

func normalizePath(repoDir, path string) string {
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if filepath.IsAbs(path) && repoDir != "" {
		if rel, err := filepath.Rel(repoDir, path); err == nil {
			path = rel
		}
	}
	return strings.ReplaceAll(path, "\\", "/")
}

func normalizeSeverity(sev string) string {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "error", "critical", "high":
		return "high"
	case "warning", "medium", "moderate":
		return "medium"
	case "style", "performance", "portability", "info", "information", "low":
		return "low"
	default:
		return "medium"
	}
}

func normalizeFinding(repoDir string, f Finding) Finding {
	f.Path = normalizePath(repoDir, f.Path)
	f.Severity = normalizeSeverity(f.Severity)
	if f.Confidence == "" {
		f.Confidence = "medium"
	}
	return f
}
