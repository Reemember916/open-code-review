package ruleengine

import (
	"github.com/open-code-review/open-code-review/internal/diff"
	"github.com/open-code-review/open-code-review/internal/model"
)

// ChangedLinesFromDiffs returns new-file line numbers touched by a diff. Added
// lines are expanded by contextLines so nearby analyzer positions can still be
// considered relevant to the current change.
func ChangedLinesFromDiffs(diffs []model.Diff, contextLines int) map[string]map[int]bool {
	if contextLines < 0 {
		contextLines = 0
	}
	result := make(map[string]map[int]bool)
	for _, d := range diffs {
		if d.IsDeleted || d.NewPath == "" || d.NewPath == "/dev/null" {
			continue
		}
		path := normalizePath(d.NewPath)
		if result[path] == nil {
			result[path] = make(map[int]bool)
		}
		for _, hunk := range diff.ParseHunks(d.Diff) {
			lineNo := hunk.NewStart
			for _, line := range hunk.Lines {
				switch line.Type {
				case diff.HunkAdded:
					for line := lineNo - contextLines; line <= lineNo+contextLines; line++ {
						if line > 0 {
							result[path][line] = true
						}
					}
					lineNo++
				case diff.HunkContext:
					lineNo++
				case diff.HunkDeleted:
				}
			}
		}
	}
	return result
}

func isChangedLine(req Request, path string, line int) bool {
	if len(req.ChangedLines) == 0 {
		return true
	}
	lines := req.ChangedLines[normalizePath(path)]
	if lines == nil {
		return false
	}
	return lines[line]
}
