package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/open-code-review/open-code-review/internal/ruleengine"
)

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID                   string                    `json:"id"`
	Name                 string                    `json:"name,omitempty"`
	ShortDescription     sarifMessage              `json:"shortDescription,omitempty"`
	DefaultConfiguration sarifDefaultConfiguration `json:"defaultConfiguration,omitempty"`
	Properties           map[string]any            `json:"properties,omitempty"`
}

type sarifDefaultConfiguration struct {
	Level string `json:"level,omitempty"`
}

type sarifResult struct {
	RuleID     string          `json:"ruleId"`
	Level      string          `json:"level,omitempty"`
	Message    sarifMessage    `json:"message"`
	Locations  []sarifLocation `json:"locations,omitempty"`
	Properties map[string]any  `json:"properties,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

func outputRulesScanSARIF(findings []ruleengine.Finding) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(buildRulesScanSARIF(findings))
}

func buildRulesScanSARIF(findings []ruleengine.Finding) sarifLog {
	return sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "OpenCodeReview",
					InformationURI: "https://github.com/open-code-review/open-code-review",
					Rules:          buildSARIFRules(findings),
				},
			},
			Results: buildSARIFResults(findings),
		}},
	}
}

func buildSARIFRules(findings []ruleengine.Finding) []sarifRule {
	byID := make(map[string]ruleengine.Finding)
	for _, finding := range findings {
		if _, ok := byID[finding.RuleID]; !ok {
			byID[finding.RuleID] = finding
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rules := make([]sarifRule, 0, len(ids))
	for _, id := range ids {
		finding := byID[id]
		properties := map[string]any{
			"severity":    finding.Severity,
			"disposition": finding.Disposition,
			"backend":     finding.Backend,
		}
		if len(finding.Tags) > 0 {
			properties["tags"] = finding.Tags
		}
		rules = append(rules, sarifRule{
			ID:               finding.RuleID,
			Name:             finding.Title,
			ShortDescription: sarifMessage{Text: finding.Title},
			DefaultConfiguration: sarifDefaultConfiguration{
				Level: sarifLevel(finding.Severity),
			},
			Properties: properties,
		})
	}
	return rules
}

func buildSARIFResults(findings []ruleengine.Finding) []sarifResult {
	results := make([]sarifResult, 0, len(findings))
	for _, finding := range findings {
		properties := map[string]any{
			"severity":    finding.Severity,
			"disposition": finding.Disposition,
			"backend":     finding.Backend,
		}
		if finding.Function != "" {
			properties["function"] = finding.Function
		}
		if finding.Role != "" {
			properties["role"] = finding.Role
		}
		if finding.Suggestion != "" {
			properties["suggestion"] = finding.Suggestion
		}
		if len(finding.Tags) > 0 {
			properties["tags"] = finding.Tags
		}
		if finding.AI != nil {
			properties["ai"] = finding.AI
		}
		results = append(results, sarifResult{
			RuleID:  finding.RuleID,
			Level:   sarifLevel(finding.Severity),
			Message: sarifMessage{Text: finding.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{
						URI: strings.ReplaceAll(finding.Path, "\\", "/"),
					},
					Region: sarifRegion{
						StartLine:   finding.Line,
						StartColumn: finding.Column,
					},
				},
			}},
			Properties: properties,
		})
	}
	return results
}

func sarifLevel(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high":
		return "error"
	case "medium":
		return "warning"
	case "low":
		return "note"
	default:
		return "warning"
	}
}
