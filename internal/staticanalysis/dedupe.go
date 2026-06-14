package staticanalysis

import (
	"fmt"
	"sort"
	"strings"
)

func dedupeFindings(findings []Finding) []Finding {
	merged := make(map[string]Finding)
	for _, f := range findings {
		key := findingKey(f)
		existing, ok := merged[key]
		if !ok {
			merged[key] = f
			continue
		}
		existing.Tool = appendUnique(existing.Tool, f.Tool)
		existing.Evidence = appendEvidence(existing.Evidence, f)
		existing.Severity = maxSeverity(existing.Severity, f.Severity)
		merged[key] = existing
	}

	out := make([]Finding, 0, len(merged))
	for _, f := range merged {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return severityRank(out[i].Severity) > severityRank(out[j].Severity)
	})
	return out
}

func findingKey(f Finding) string {
	lineBucket := f.Line / 3
	msg := strings.ToLower(f.Message)
	if len(msg) > 80 {
		msg = msg[:80]
	}
	return fmt.Sprintf("%s:%d:%s:%s", f.Path, lineBucket, strings.ToLower(f.Category), msg)
}

func appendUnique(a, b string) string {
	parts := strings.Split(a, "+")
	for _, p := range parts {
		if p == b {
			return a
		}
	}
	return a + "+" + b
}

func appendEvidence(existing string, f Finding) string {
	next := f.Evidence
	if next == "" {
		next = f.Message
	}
	if existing == "" {
		return fmt.Sprintf("%s: %s", f.Tool, next)
	}
	return existing + "\n" + fmt.Sprintf("%s: %s", f.Tool, next)
}

func maxSeverity(a, b string) string {
	if severityRank(b) > severityRank(a) {
		return b
	}
	return a
}

func severityRank(s string) int {
	switch s {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
