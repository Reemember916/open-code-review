package ruleengine

import (
	"path/filepath"
	"strings"
)

func normalizePath(path string) string {
	path = filepath.Clean(path)
	return strings.ReplaceAll(path, "\\", "/")
}

func normalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "error", "high":
		return "high"
	case "warning", "medium", "moderate":
		return "medium"
	case "style", "info", "information", "low":
		return "low"
	default:
		return "medium"
	}
}

func resolveDisposition(rule Rule) string {
	if rule.Disposition != "" {
		return normalizeDispositionValue(rule.Disposition)
	}
	switch normalizeSeverity(rule.Severity) {
	case "high", "medium":
		return "review"
	case "low":
		return "report_only"
	default:
		return "review"
	}
}

func resolveFindingDisposition(rule Rule, evidenceText string) string {
	for _, override := range rule.DispositionOverrides {
		if anyLiteralPresent(evidenceText, override.IfMessageContainsAny) {
			return normalizeDispositionValue(override.Disposition)
		}
	}
	return resolveDisposition(rule)
}

func normalizeDispositionValue(disposition string) string {
	switch strings.ToLower(strings.TrimSpace(disposition)) {
	case "blocking":
		return "blocking"
	case "review":
		return "review"
	case "report_only", "report-only", "reportonly":
		return "report_only"
	case "suppressed":
		return "suppressed"
	case "ignored":
		return "ignored"
	default:
		return "review"
	}
}
