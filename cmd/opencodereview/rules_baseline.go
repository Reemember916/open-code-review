package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/open-code-review/open-code-review/internal/ruleengine"
)

type rulesScanBaseline struct {
	Version  string                  `json:"version,omitempty"`
	Findings []rulesScanBaselineItem `json:"findings"`
}

type rulesScanBaselineItem struct {
	Fingerprint string `json:"fingerprint"`
	RuleID      string `json:"rule_id"`
	Path        string `json:"path"`
	Line        int    `json:"line"`
	Message     string `json:"message,omitempty"`
}

func loadRulesScanBaseline(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline %s: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var baseline rulesScanBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("unmarshal baseline %s: %w", path, err)
	}
	out := make(map[string]bool, len(baseline.Findings))
	for _, item := range baseline.Findings {
		if item.Fingerprint != "" {
			out[item.Fingerprint] = true
		}
	}
	return out, nil
}

func writeRulesScanBaseline(path string, findings []ruleengine.Finding) error {
	if path == "" {
		return nil
	}
	entries := make([]rulesScanBaselineItem, 0, len(findings))
	for _, finding := range findings {
		entries = append(entries, rulesScanBaselineItem{
			Fingerprint: rulesScanFindingFingerprint(finding),
			RuleID:      finding.RuleID,
			Path:        normalizeBaselinePath(finding.Path),
			Line:        finding.Line,
			Message:     finding.Message,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Fingerprint < entries[j].Fingerprint
	})

	data, err := json.MarshalIndent(rulesScanBaseline{
		Version:  "0.1.0",
		Findings: entries,
	}, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create baseline directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write baseline %s: %w", path, err)
	}
	return nil
}

func applyRulesScanBaseline(findings []ruleengine.Finding, baseline map[string]bool) []ruleengine.Finding {
	if len(baseline) == 0 {
		return findings
	}
	out := make([]ruleengine.Finding, len(findings))
	copy(out, findings)
	for i := range out {
		if baseline[rulesScanFindingFingerprint(out[i])] {
			out[i].Disposition = "suppressed"
			out[i].Tags = appendTagOnce(out[i].Tags, "baseline")
		}
	}
	return out
}

func rulesScanFindingFingerprint(finding ruleengine.Finding) string {
	parts := []string{
		finding.RuleID,
		normalizeBaselinePath(finding.Path),
		strconv.Itoa(finding.Line),
		finding.Message,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func normalizeBaselinePath(path string) string {
	return strings.ReplaceAll(filepath.Clean(path), "\\", "/")
}

func appendTagOnce(tags []string, tag string) []string {
	for _, existing := range tags {
		if existing == tag {
			return tags
		}
	}
	return append(tags, tag)
}
