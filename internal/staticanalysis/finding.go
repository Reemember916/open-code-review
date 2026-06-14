package staticanalysis

import "github.com/open-code-review/open-code-review/internal/model"

// Finding is a normalized static-analysis result from any analyzer.
type Finding struct {
	Tool       string         `json:"tool"`
	RuleID     string         `json:"rule_id,omitempty"`
	Category   string         `json:"category,omitempty"`
	Severity   string         `json:"severity"`
	Path       string         `json:"path"`
	Line       int            `json:"line"`
	Column     int            `json:"column,omitempty"`
	Message    string         `json:"message"`
	Evidence   string         `json:"evidence,omitempty"`
	Confidence string         `json:"confidence,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// AnalyzeRequest contains the repository state needed by analyzers and filters.
type AnalyzeRequest struct {
	RepoDir          string
	Files            []string
	Diffs            []model.Diff
	DiffContextLines int
	SemgrepConfig    string
}

// Analyzer runs one static-analysis tool and returns normalized findings.
type Analyzer interface {
	Name() string
	Available() bool
	Analyze(req AnalyzeRequest) ([]Finding, error)
}
