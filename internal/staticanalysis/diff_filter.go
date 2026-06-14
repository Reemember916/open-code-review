package staticanalysis

import (
	"math"

	"github.com/open-code-review/open-code-review/internal/diff"
	"github.com/open-code-review/open-code-review/internal/model"
)

type changedLineSet map[string]map[int]bool

func buildChangedLineSet(diffs []model.Diff, contextLines int) changedLineSet {
	if contextLines < 0 {
		contextLines = 0
	}
	result := make(changedLineSet)
	for _, d := range diffs {
		if d.IsDeleted || d.NewPath == "" || d.NewPath == "/dev/null" {
			continue
		}
		path := normalizePath("", d.NewPath)
		if result[path] == nil {
			result[path] = make(map[int]bool)
		}
		for _, h := range diff.ParseHunks(d.Diff) {
			newLine := h.NewStart
			for _, line := range h.Lines {
				switch line.Type {
				case diff.HunkAdded:
					for i := newLine - contextLines; i <= newLine+contextLines; i++ {
						if i > 0 {
							result[path][i] = true
						}
					}
					newLine++
				case diff.HunkContext:
					newLine++
				case diff.HunkDeleted:
				}
			}
		}
	}
	return result
}

func filterDiffRelated(findings []Finding, diffs []model.Diff, contextLines int) []Finding {
	changed := buildChangedLineSet(diffs, contextLines)
	var kept []Finding
	for _, f := range findings {
		if f.Line <= 0 {
			continue
		}
		lines := changed[f.Path]
		if lines == nil {
			continue
		}
		if lines[f.Line] {
			kept = append(kept, f)
			continue
		}
		// Defensive fallback if the caller passed contextLines=0 but the analyzer
		// points at a nearby expression instead of the exact changed line.
		for line := range lines {
			if int(math.Abs(float64(line-f.Line))) <= contextLines {
				kept = append(kept, f)
				break
			}
		}
	}
	return kept
}
