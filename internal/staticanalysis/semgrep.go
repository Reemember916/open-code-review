package staticanalysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type SemgrepAnalyzer struct {
	Config string
}

func (SemgrepAnalyzer) Name() string { return "semgrep" }

func (SemgrepAnalyzer) Available() bool {
	_, err := exec.LookPath("semgrep")
	return err == nil
}

func (a SemgrepAnalyzer) Analyze(req AnalyzeRequest) ([]Finding, error) {
	if len(req.Files) == 0 {
		return nil, nil
	}
	config := strings.TrimSpace(a.Config)
	if config == "" {
		return nil, fmt.Errorf("semgrep config is required; pass --semgrep-config")
	}

	args := []string{"scan", "--json", "--quiet", "--config", config}
	args = append(args, req.Files...)
	cmd := exec.Command("semgrep", args...)
	cmd.Dir = req.RepoDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.Len() == 0 && err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, msg)
	}
	return parseSemgrepJSON(req.RepoDir, stdout.Bytes())
}

type semgrepOutput struct {
	Results []semgrepResult `json:"results"`
}

type semgrepResult struct {
	CheckID string          `json:"check_id"`
	Path    string          `json:"path"`
	Start   semgrepPosition `json:"start"`
	Extra   semgrepExtra    `json:"extra"`
}

type semgrepPosition struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type semgrepExtra struct {
	Message  string         `json:"message"`
	Severity string         `json:"severity"`
	Metadata map[string]any `json:"metadata"`
}

func parseSemgrepJSON(repoDir string, data []byte) ([]Finding, error) {
	var parsed semgrepOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	var findings []Finding
	for _, r := range parsed.Results {
		category := "custom"
		if cat, ok := r.Extra.Metadata["category"].(string); ok && cat != "" {
			category = cat
		}
		findings = append(findings, Finding{
			Tool:       "semgrep",
			RuleID:     r.CheckID,
			Category:   category,
			Severity:   r.Extra.Severity,
			Path:       normalizePath(repoDir, r.Path),
			Line:       r.Start.Line,
			Column:     r.Start.Col,
			Message:    r.Extra.Message,
			Confidence: "medium",
			Raw:        r.Extra.Metadata,
		})
	}
	return findings, nil
}
