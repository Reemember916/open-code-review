package ruleengine

import (
	"strings"
)

const (
	inlineDisableLine     = "ocr-disable-line"
	inlineDisableNextLine = "ocr-disable-next-line"
	inlineSuppressionTag  = "inline-suppression"
)

func applyInlineSuppressions(req Request, findings []Finding) []Finding {
	suppressions := collectInlineSuppressions(req.Files)
	if len(suppressions) == 0 {
		return findings
	}
	out := make([]Finding, len(findings))
	copy(out, findings)
	for i := range out {
		path := normalizePath(out[i].Path)
		lineRules := suppressions[path][out[i].Line]
		if suppressionMatches(lineRules, out[i].RuleID) {
			out[i].Disposition = "suppressed"
			out[i].Tags = appendRuleengineTagOnce(out[i].Tags, inlineSuppressionTag)
		}
	}
	return out
}

func collectInlineSuppressions(files []FileInput) map[string]map[int][]string {
	out := make(map[string]map[int][]string)
	for _, file := range files {
		path := normalizePath(file.Path)
		lines := splitLines(file.Content)
		for idx, line := range lines {
			lineNo := idx + 1
			if markerIdx := strings.Index(line, inlineDisableLine); markerIdx >= 0 {
				addInlineSuppression(out, path, lineNo, parseInlineSuppressionRules(line[markerIdx+len(inlineDisableLine):]))
			}
			if markerIdx := strings.Index(line, inlineDisableNextLine); markerIdx >= 0 {
				addInlineSuppression(out, path, lineNo+1, parseInlineSuppressionRules(line[markerIdx+len(inlineDisableNextLine):]))
			}
		}
	}
	return out
}

func addInlineSuppression(suppressions map[string]map[int][]string, path string, line int, rules []string) {
	if line <= 0 {
		return
	}
	if _, ok := suppressions[path]; !ok {
		suppressions[path] = make(map[int][]string)
	}
	suppressions[path][line] = append(suppressions[path][line], rules...)
}

func parseInlineSuppressionRules(text string) []string {
	text = strings.TrimSpace(strings.TrimPrefix(text, ":"))
	if text == "" || strings.HasPrefix(text, "*/") {
		return []string{"*"}
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return []string{"*"}
	}
	token := strings.Trim(fields[0], " ,;")
	if token == "" {
		return []string{"*"}
	}
	parts := strings.Split(token, ",")
	rules := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			rules = append(rules, part)
		}
	}
	if len(rules) == 0 {
		return []string{"*"}
	}
	return rules
}

func suppressionMatches(patterns []string, ruleID string) bool {
	for _, pattern := range patterns {
		switch strings.ToLower(strings.TrimSpace(pattern)) {
		case "", "*", "all":
			return true
		default:
			if patternListMatches([]string{pattern}, ruleID) {
				return true
			}
		}
	}
	return false
}

func appendRuleengineTagOnce(tags []string, tag string) []string {
	for _, existing := range tags {
		if existing == tag {
			return tags
		}
	}
	return append(tags, tag)
}
