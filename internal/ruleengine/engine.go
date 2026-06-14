package ruleengine

import "fmt"

// Engine dispatches rules to registered backends and combines the findings.
type Engine struct {
	backends []Backend
}

func New(backends ...Backend) *Engine {
	return &Engine{backends: backends}
}

func (e *Engine) Analyze(req Request, rules []Rule) ([]Finding, error) {
	var findings []Finding
	var errs []error
	for _, backend := range e.backends {
		var selected []Rule
		for _, rule := range rules {
			if backend.Supports(rule) {
				selected = append(selected, rule)
			}
		}
		if len(selected) == 0 {
			continue
		}
		got, err := backend.Analyze(req, selected)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", backend.Name(), err))
			continue
		}
		findings = append(findings, got...)
	}
	if len(errs) > 0 {
		return applyInlineSuppressions(req, dedupeFindings(findings)), errorsJoin(errs)
	}
	return applyInlineSuppressions(req, dedupeFindings(findings)), nil
}

func errorsJoin(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	msg := errs[0].Error()
	for i := 1; i < len(errs); i++ {
		msg += "; " + errs[i].Error()
	}
	return fmt.Errorf("%s", msg)
}

func dedupeFindings(findings []Finding) []Finding {
	seen := make(map[string]bool)
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		key := fmt.Sprintf("%s|%s|%d|%s", finding.RuleID, finding.Path, finding.Line, finding.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}
