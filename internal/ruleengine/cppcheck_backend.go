package ruleengine

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type CppcheckBackend struct{}

func (CppcheckBackend) Name() string { return "cppcheck" }

func (CppcheckBackend) Supports(rule Rule) bool {
	return rule.Backend == "cppcheck"
}

func (CppcheckBackend) Analyze(req Request, rules []Rule) ([]Finding, error) {
	if len(req.Files) == 0 || len(rules) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("cppcheck"); err != nil {
		return nil, fmt.Errorf("cppcheck is not available: %w", err)
	}

	filePaths := make([]string, 0, len(req.Files))
	functionsByPath := make(map[string][]FunctionContext)
	for _, file := range req.Files {
		path := normalizePath(file.Path)
		filePaths = append(filePaths, file.Path)
		functionsByPath[path] = parseFunctionContexts(path, splitLines(file.Content), req.RoleHints)
	}

	args := buildCppcheckArgs(req.COptions, filePaths)

	cmd := exec.Command("cppcheck", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	data := stderr.Bytes()
	if len(data) == 0 {
		data = stdout.Bytes()
	}
	if len(data) == 0 && err != nil {
		return nil, err
	}

	results, parseErr := parseCppcheckResults(data)
	if parseErr != nil {
		return nil, parseErr
	}
	ruleByID := cppcheckRuleMap(rules)
	var findings []Finding
	for _, item := range results {
		rule, ok := ruleByID[item.ID]
		if !ok {
			continue
		}
		locations := item.Locations
		if len(locations) > 1 {
			locations = locations[:1]
		}
		for _, loc := range locations {
			line, _ := strconv.Atoi(loc.Line)
			col, _ := strconv.Atoi(loc.Column)
			path := normalizePath(loc.File)
			if !isChangedLine(req, path, line) {
				continue
			}
			fn := functionAtLine(functionsByPath[path], line)
			message := rule.Message
			if message == "" {
				message = item.Msg
			}
			findings = append(findings, Finding{
				RuleID:      rule.ID,
				Title:       rule.Title,
				Severity:    normalizeSeverity(rule.Severity),
				Path:        path,
				Line:        line,
				Column:      col,
				Function:    fn.Name,
				Role:        fn.Role,
				Disposition: resolveFindingDisposition(rule, item.Msg+" "+item.Verbose),
				Message:     message,
				Suggestion:  rule.Suggestion,
				Tags:        rule.Tags,
				Backend:     "cppcheck",
				Evidence: []Evidence{{
					Kind:    "cppcheck",
					Path:    path,
					Line:    line,
					Column:  col,
					Snippet: item.Msg,
					Detail:  item.Verbose,
				}},
			})
		}
	}
	return findings, nil
}

func buildCppcheckArgs(options CScanOptions, filePaths []string) []string {
	standard := strings.TrimSpace(options.Standard)
	if standard == "" {
		standard = "c99"
	}

	args := []string{
		"--xml",
		"--xml-version=2",
		"--enable=warning,style,performance,portability",
		"--inconclusive",
		"--std=" + standard,
		"--suppress=missingIncludeSystem",
	}
	if platform := strings.TrimSpace(options.Platform); platform != "" {
		args = append(args, "--platform="+platform)
	}
	for _, includePath := range cleanStringList(options.IncludePaths) {
		args = append(args, "-I"+includePath)
	}
	for _, define := range cleanStringList(options.Defines) {
		args = append(args, "-D"+define)
	}
	for _, undefine := range cleanStringList(options.Undefines) {
		args = append(args, "-U"+undefine)
	}
	return append(args, filePaths...)
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func cppcheckRuleMap(rules []Rule) map[string]Rule {
	out := make(map[string]Rule)
	for _, rule := range rules {
		for _, id := range rule.Match.Any {
			out[id] = rule
		}
	}
	return out
}

type cppcheckXML struct {
	Errors []cppcheckXMLError `xml:"errors>error"`
}

type cppcheckXMLError struct {
	ID        string                `xml:"id,attr"`
	Severity  string                `xml:"severity,attr"`
	Msg       string                `xml:"msg,attr"`
	Verbose   string                `xml:"verbose,attr"`
	Locations []cppcheckXMLLocation `xml:"location"`
}

type cppcheckXMLLocation struct {
	File   string `xml:"file,attr"`
	Line   string `xml:"line,attr"`
	Column string `xml:"column,attr"`
}

func parseCppcheckResults(data []byte) ([]cppcheckXMLError, error) {
	var parsed cppcheckXML
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return parsed.Errors, nil
}
