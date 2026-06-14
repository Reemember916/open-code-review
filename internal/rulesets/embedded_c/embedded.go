package embeddedc

import (
	_ "embed"

	"github.com/open-code-review/open-code-review/internal/ruleengine"
)

//go:embed rules.json
var rulesJSON []byte

func Load() (*ruleengine.RuleSet, error) {
	return ruleengine.ParseRuleSet(rulesJSON)
}
