package ruleengine

// Rule describes one review rule. The MVP supports line-oriented regex
// matching first; later backends can reuse the same metadata and Finding model.
type Rule struct {
	ID                   string                `json:"id"`
	Title                string                `json:"title"`
	Severity             string                `json:"severity"`
	Scope                string                `json:"scope"`
	Backend              string                `json:"backend"`
	Where                Where                 `json:"where,omitempty"`
	Match                Match                 `json:"match"`
	Disposition          string                `json:"disposition,omitempty"`
	DispositionOverrides []DispositionOverride `json:"disposition_overrides,omitempty"`
	Message              string                `json:"message"`
	Suggestion           string                `json:"suggestion,omitempty"`
	Tags                 []string              `json:"tags,omitempty"`
	Metadata             map[string]string     `json:"metadata,omitempty"`
}

// DispositionOverride adjusts a finding's presentation/gate policy when the
// backend evidence text matches a project or domain-specific pattern.
type DispositionOverride struct {
	IfMessageContainsAny []string `json:"if_message_contains_any,omitempty"`
	Disposition          string   `json:"disposition"`
}

// Where describes contextual constraints for a rule.
type Where struct {
	FunctionRole       string   `json:"function_role,omitempty"`
	FunctionNameAny    []string `json:"function_name_any,omitempty"`
	FunctionNameNotAny []string `json:"function_name_not_any,omitempty"`
}

// Match holds backend-specific matching conditions.
type Match struct {
	Pattern     string   `json:"pattern,omitempty"`
	Any         []string `json:"any,omitempty"`
	Not         []string `json:"not,omitempty"`
	BlockNotAny []string `json:"block_not_any,omitempty"`
	CallsAny    []string `json:"calls_any,omitempty"`
}

// Evidence is the concrete code fact that made a rule fire.
type Evidence struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column,omitempty"`
	Snippet string `json:"snippet,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Finding is the normalized result of applying a rule.
type Finding struct {
	RuleID      string     `json:"rule_id"`
	Title       string     `json:"title"`
	Severity    string     `json:"severity"`
	Path        string     `json:"path"`
	Line        int        `json:"line"`
	Column      int        `json:"column,omitempty"`
	Function    string     `json:"function,omitempty"`
	Role        string     `json:"role,omitempty"`
	Disposition string     `json:"disposition"`
	Message     string     `json:"message"`
	Suggestion  string     `json:"suggestion,omitempty"`
	Evidence    []Evidence `json:"evidence,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Backend     string     `json:"backend"`
	AI          *FindingAI `json:"ai,omitempty"`
}

// FindingAI is optional LLM-generated explanatory context for a finding.
type FindingAI struct {
	Summary        string `json:"summary,omitempty"`
	Risk           string `json:"risk,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
	Confidence     string `json:"confidence,omitempty"`
}

// FileInput is the source unit analyzed by the rule engine.
type FileInput struct {
	Path    string
	Content string
}

// Request contains all data needed for one rule-engine run.
type Request struct {
	Files            []FileInput
	ChangedLines     map[string]map[int]bool
	DiffContextLines int
	RoleHints        []RoleHint
	COptions         CScanOptions
}

// CScanOptions carries C/C++ project context needed by analyzer backends.
type CScanOptions struct {
	IncludePaths []string
	Defines      []string
	Undefines    []string
	Standard     string
	Platform     string
}

// Backend executes a subset of rules.
type Backend interface {
	Name() string
	Supports(rule Rule) bool
	Analyze(req Request, rules []Rule) ([]Finding, error)
}
