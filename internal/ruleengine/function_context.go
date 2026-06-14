package ruleengine

import (
	"regexp"
	"strings"
)

type FunctionContext struct {
	Name      string
	Role      string
	StartLine int
	EndLine   int
	Calls     []CallFact
}

type CallFact struct {
	Name    string
	Path    string
	Line    int
	Column  int
	Snippet string
}

var functionDeclRe = regexp.MustCompile(`^\s*(?:static\s+)?(?:interrupt\s+)?[A-Za-z_][A-Za-z0-9_\s\*]*\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^;]*\)\s*(?:\{|$)`)
var callRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

func parseFunctionContexts(path string, lines []string, hints []RoleHint) []FunctionContext {
	var funcs []FunctionContext
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		matches := functionDeclRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		name := matches[1]
		if isControlKeyword(name) {
			continue
		}
		startLine := i + 1
		endLine := findFunctionEnd(lines, i)
		role := resolveRoleFromHints(path, name, hints)
		if role == "" {
			role = inferFunctionRole(path, name)
		}
		funcs = append(funcs, FunctionContext{
			Name:      name,
			Role:      role,
			StartLine: startLine,
			EndLine:   endLine,
			Calls:     extractCalls(path, lines, startLine, endLine, name),
		})
		if endLine > startLine {
			i = endLine - 1
		}
	}
	return funcs
}

func extractCalls(path string, lines []string, startLine, endLine int, functionName string) []CallFact {
	var calls []CallFact
	if startLine <= 0 || endLine < startLine {
		return calls
	}
	for lineNo := startLine; lineNo <= endLine && lineNo <= len(lines); lineNo++ {
		line := lines[lineNo-1]
		for _, match := range callRe.FindAllStringSubmatchIndex(line, -1) {
			if len(match) < 4 {
				continue
			}
			name := line[match[2]:match[3]]
			if isControlKeyword(name) || name == "sizeof" {
				continue
			}
			if lineNo == startLine && name == functionName {
				continue
			}
			calls = append(calls, CallFact{
				Name:    name,
				Path:    normalizePath(path),
				Line:    lineNo,
				Column:  match[2] + 1,
				Snippet: strings.TrimSpace(line),
			})
		}
	}
	return calls
}

func isControlKeyword(name string) bool {
	switch name {
	case "if", "while", "for", "switch", "return":
		return true
	default:
		return false
	}
}

func findFunctionEnd(lines []string, start int) int {
	braceDepth := 0
	seenOpen := false
	for i := start; i < len(lines); i++ {
		for _, ch := range lines[i] {
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
			return i + 1
		}
	}
	return len(lines)
}

func inferFunctionRole(_ string, name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "isr") || strings.Contains(lower, "interrupt") {
		return "isr"
	}
	return ""
}

func functionAtLine(functions []FunctionContext, line int) FunctionContext {
	for _, fn := range functions {
		if line >= fn.StartLine && line <= fn.EndLine {
			return fn
		}
	}
	return FunctionContext{}
}
