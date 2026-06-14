package ruleengine

import (
	"encoding/json"
	"fmt"
	"os"
)

// RuleSet is the top-level JSON document for project or built-in rules.
type RuleSet struct {
	Version string `json:"version,omitempty"`
	Rules   []Rule `json:"rules"`
}

// LoadRuleSet reads rules from a JSON file and validates the minimum required
// fields used by the engine.
func LoadRuleSet(path string) (*RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ruleset %s: %w", path, err)
	}
	return ParseRuleSet(data)
}

// ParseRuleSet parses rules from JSON bytes.
func ParseRuleSet(data []byte) (*RuleSet, error) {
	var set RuleSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("unmarshal ruleset: %w", err)
	}
	if err := ValidateRuleSet(&set); err != nil {
		return nil, err
	}
	return &set, nil
}

func ValidateRuleSet(set *RuleSet) error {
	if set == nil {
		return fmt.Errorf("ruleset is nil")
	}
	ids := make(map[string]bool)
	for i, rule := range set.Rules {
		if rule.ID == "" {
			return fmt.Errorf("rule[%d] missing id", i)
		}
		if ids[rule.ID] {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		ids[rule.ID] = true
		if rule.Title == "" {
			return fmt.Errorf("rule %q missing title", rule.ID)
		}
		if rule.Severity == "" {
			return fmt.Errorf("rule %q missing severity", rule.ID)
		}
		if rule.Backend == "" {
			return fmt.Errorf("rule %q missing backend", rule.ID)
		}
		if rule.Backend == "regex" && rule.Match.Pattern == "" && len(rule.Match.Any) == 0 {
			return fmt.Errorf("regex rule %q requires match.pattern or match.any", rule.ID)
		}
		if rule.Backend == "call" && len(rule.Match.CallsAny) == 0 {
			return fmt.Errorf("call rule %q requires match.calls_any", rule.ID)
		}
	}
	return nil
}
