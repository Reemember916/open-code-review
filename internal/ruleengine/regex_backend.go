package ruleengine

import (
	"fmt"
	"regexp"
	"strings"
)

type RegexBackend struct{}

func (RegexBackend) Name() string { return "regex" }

func (RegexBackend) Supports(rule Rule) bool {
	return rule.Backend == "regex"
}

func (RegexBackend) Analyze(req Request, rules []Rule) ([]Finding, error) {
	compiled := make(map[string]*regexp.Regexp)
	for _, rule := range rules {
		pattern := rule.Match.Pattern
		if pattern == "" && len(rule.Match.Any) > 0 {
			pattern = strings.Join(rule.Match.Any, "|")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile rule %s: %w", rule.ID, err)
		}
		compiled[rule.ID] = re
	}

	var findings []Finding
	for _, file := range req.Files {
		path := normalizePath(file.Path)
		lines := splitLines(file.Content)
		functions := parseFunctionContexts(path, lines, req.RoleHints)
		for lineIdx, line := range lines {
			lineNo := lineIdx + 1
			if !isChangedLine(req, path, lineNo) {
				continue
			}
			for _, rule := range rules {
				if anyLiteralPresent(line, rule.Match.Not) {
					continue
				}
				re := compiled[rule.ID]
				match := re.FindStringIndex(line)
				if match == nil {
					continue
				}
				if !matchesWhere(rule, functions, lineNo) {
					continue
				}
				fn := functionAtLine(functions, lineNo)
				block := extractBlock(lines, lineIdx)
				if anyLiteralPresentInLines(block, rule.Match.BlockNotAny) {
					continue
				}
				findings = append(findings, Finding{
					RuleID:      rule.ID,
					Title:       rule.Title,
					Severity:    normalizeSeverity(rule.Severity),
					Path:        path,
					Line:        lineNo,
					Column:      match[0] + 1,
					Function:    fn.Name,
					Role:        fn.Role,
					Disposition: resolveFindingDisposition(rule, line),
					Message:     renderMessage(rule, line),
					Suggestion:  rule.Suggestion,
					Tags:        rule.Tags,
					Backend:     "regex",
					Evidence: []Evidence{{
						Kind:    "regex_match",
						Path:    path,
						Line:    lineNo,
						Column:  match[0] + 1,
						Snippet: strings.TrimSpace(line),
						Detail:  re.String(),
					}},
				})
			}
		}
	}
	return findings, nil
}

func matchesWhere(rule Rule, functions []FunctionContext, lineNo int) bool {
	fn := functionAtLine(functions, lineNo)
	if rule.Where.FunctionRole != "" && fn.Role != rule.Where.FunctionRole {
		return false
	}
	if len(rule.Where.FunctionNameAny) > 0 && !patternListMatches(rule.Where.FunctionNameAny, fn.Name) {
		return false
	}
	if patternListMatches(rule.Where.FunctionNameNotAny, fn.Name) {
		return false
	}
	return true
}

func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.Split(content, "\n")
}

func anyLiteralPresent(line string, literals []string) bool {
	for _, literal := range literals {
		if literal != "" && strings.Contains(line, literal) {
			return true
		}
	}
	return false
}

func anyLiteralPresentInLines(lines []string, literals []string) bool {
	if len(literals) == 0 {
		return false
	}
	for _, line := range lines {
		if anyLiteralPresent(line, literals) {
			return true
		}
	}
	return false
}

func extractBlock(lines []string, start int) []string {
	if start < 0 || start >= len(lines) {
		return nil
	}
	var block []string
	braceDepth := 0
	seenOpen := false
	for i := start; i < len(lines); i++ {
		line := lines[i]
		block = append(block, line)
		for _, ch := range line {
			switch ch {
			case '{':
				braceDepth++
				seenOpen = true
			case '}':
				if braceDepth > 0 {
					braceDepth--
				}
			}
		}
		if seenOpen && braceDepth == 0 {
			break
		}
		if !seenOpen && i > start+4 {
			break
		}
	}
	return block
}

func renderMessage(rule Rule, line string) string {
	if rule.Message != "" {
		return rule.Message
	}
	return fmt.Sprintf("%s matched `%s`", rule.Title, strings.TrimSpace(line))
}
