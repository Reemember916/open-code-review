package staticanalysis

import (
	"fmt"
	"strings"
)

// Run executes analyzers, normalizes their output, filters to diff-related
// findings, and deduplicates overlapping reports.
func Run(req AnalyzeRequest, analyzers []Analyzer) ([]Finding, []string) {
	var all []Finding
	var warnings []string
	if req.DiffContextLines == 0 {
		req.DiffContextLines = 3
	}

	for _, analyzer := range analyzers {
		if !analyzer.Available() {
			warnings = append(warnings, fmt.Sprintf("%s is not available", analyzer.Name()))
			continue
		}
		findings, err := analyzer.Analyze(req)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s failed: %v", analyzer.Name(), err))
			continue
		}
		for _, f := range findings {
			all = append(all, normalizeFinding(req.RepoDir, f))
		}
	}

	related := filterDiffRelated(all, req.Diffs, req.DiffContextLines)
	return dedupeFindings(related), warnings
}

func SelectAnalyzers(names string, semgrepConfig string) []Analyzer {
	var analyzers []Analyzer
	for _, name := range strings.Split(names, ",") {
		switch strings.TrimSpace(strings.ToLower(name)) {
		case "", "none":
			continue
		case "cppcheck":
			analyzers = append(analyzers, CppcheckAnalyzer{})
		case "semgrep":
			analyzers = append(analyzers, SemgrepAnalyzer{Config: semgrepConfig})
		default:
			analyzers = append(analyzers, UnsupportedAnalyzer{NameValue: strings.TrimSpace(name)})
		}
	}
	return analyzers
}

type UnsupportedAnalyzer struct {
	NameValue string
}

func (a UnsupportedAnalyzer) Name() string { return a.NameValue }

func (UnsupportedAnalyzer) Available() bool { return true }

func (a UnsupportedAnalyzer) Analyze(AnalyzeRequest) ([]Finding, error) {
	return nil, fmt.Errorf("static analyzer %q is not implemented yet", a.NameValue)
}
