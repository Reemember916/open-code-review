package staticanalysis

import (
	"fmt"
	"strings"
)

// RenderForPrompt formats findings for injection into the LLM review prompt.
func RenderForPrompt(findings []Finding) string {
	if len(findings) == 0 {
		return "No static analysis findings related to the current diff."
	}
	var sb strings.Builder
	sb.WriteString("Use these static analysis findings as evidence. Only create a code comment when the finding is relevant to the current diff and not an obvious false positive.\n\n")
	for _, f := range findings {
		sb.WriteString(fmt.Sprintf("- [%s][%s] %s:%d", strings.ToUpper(f.Severity), f.Tool, f.Path, f.Line))
		if f.RuleID != "" {
			sb.WriteString(fmt.Sprintf(" rule=%s", f.RuleID))
		}
		if f.Category != "" {
			sb.WriteString(fmt.Sprintf(" category=%s", f.Category))
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  Message: %s\n", f.Message))
		if f.Evidence != "" {
			sb.WriteString(fmt.Sprintf("  Evidence: %s\n", strings.ReplaceAll(f.Evidence, "\n", "\n            ")))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
