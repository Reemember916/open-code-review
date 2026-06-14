package staticanalysis

import (
	"bytes"
	"encoding/xml"
	"os/exec"
	"strconv"
	"strings"
)

type CppcheckAnalyzer struct{}

func (CppcheckAnalyzer) Name() string { return "cppcheck" }

func (CppcheckAnalyzer) Available() bool {
	_, err := exec.LookPath("cppcheck")
	return err == nil
}

func (CppcheckAnalyzer) Analyze(req AnalyzeRequest) ([]Finding, error) {
	if len(req.Files) == 0 {
		return nil, nil
	}
	args := []string{
		"--enable=warning,style,performance,portability",
		"--inconclusive",
		"--xml",
		"--xml-version=2",
	}
	args = append(args, req.Files...)

	cmd := exec.Command("cppcheck", args...)
	cmd.Dir = req.RepoDir
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
	return parseCppcheckXML(req.RepoDir, data)
}

type cppcheckResults struct {
	Errors []cppcheckError `xml:"errors>error"`
}

type cppcheckError struct {
	ID        string             `xml:"id,attr"`
	Severity  string             `xml:"severity,attr"`
	Msg       string             `xml:"msg,attr"`
	Verbose   string             `xml:"verbose,attr"`
	CWE       string             `xml:"cwe,attr"`
	Locations []cppcheckLocation `xml:"location"`
}

type cppcheckLocation struct {
	File   string `xml:"file,attr"`
	Line   string `xml:"line,attr"`
	Column string `xml:"column,attr"`
}

func parseCppcheckXML(repoDir string, data []byte) ([]Finding, error) {
	var parsed cppcheckResults
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	var findings []Finding
	for _, e := range parsed.Errors {
		msg := e.Msg
		if msg == "" {
			msg = e.Verbose
		}
		category := "quality"
		if e.CWE != "" || strings.Contains(strings.ToLower(e.ID), "buffer") {
			category = "security"
		}
		if len(e.Locations) == 0 {
			continue
		}
		for _, loc := range e.Locations {
			line, _ := strconv.Atoi(loc.Line)
			col, _ := strconv.Atoi(loc.Column)
			findings = append(findings, Finding{
				Tool:       "cppcheck",
				RuleID:     e.ID,
				Category:   category,
				Severity:   e.Severity,
				Path:       normalizePath(repoDir, loc.File),
				Line:       line,
				Column:     col,
				Message:    msg,
				Evidence:   e.Verbose,
				Confidence: "medium",
			})
		}
	}
	return findings, nil
}
