package staticanalysis

import (
	"strings"
	"testing"

	"github.com/open-code-review/open-code-review/internal/model"
)

func TestParseCppcheckXML(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<results version="2">
  <errors>
    <error id="nullPointer" severity="error" msg="Null pointer dereference" verbose="Null pointer dereference: ctx">
      <location file="drivers/uart.c" line="12" column="5"/>
    </error>
  </errors>
</results>`)

	findings, err := parseCppcheckXML("", xml)
	if err != nil {
		t.Fatalf("parseCppcheckXML: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Tool != "cppcheck" || findings[0].RuleID != "nullPointer" {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
	if findings[0].Path != "drivers/uart.c" || findings[0].Line != 12 {
		t.Fatalf("unexpected location: %+v", findings[0])
	}
}

func TestParseSemgrepJSON(t *testing.T) {
	raw := []byte(`{
  "results": [
    {
      "check_id": "embedded.no-blocking-in-isr",
      "path": "src/isr.c",
      "start": {"line": 20, "col": 3},
      "extra": {
        "message": "Do not call blocking API in ISR",
        "severity": "ERROR",
        "metadata": {"category": "embedded"}
      }
    }
  ]
}`)

	findings, err := parseSemgrepJSON("", raw)
	if err != nil {
		t.Fatalf("parseSemgrepJSON: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Category != "embedded" || findings[0].Severity != "ERROR" {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
}

func TestFilterDiffRelated(t *testing.T) {
	diffs := []model.Diff{{
		NewPath: "drivers/uart.c",
		Diff: strings.Join([]string{
			"diff --git a/drivers/uart.c b/drivers/uart.c",
			"--- a/drivers/uart.c",
			"+++ b/drivers/uart.c",
			"@@ -10,3 +10,4 @@",
			" context",
			"+added_call();",
			" unchanged",
		}, "\n"),
	}}
	findings := []Finding{
		{Tool: "cppcheck", Path: "drivers/uart.c", Line: 11, Message: "related", Severity: "warning"},
		{Tool: "cppcheck", Path: "drivers/uart.c", Line: 50, Message: "old issue", Severity: "warning"},
		{Tool: "cppcheck", Path: "other.c", Line: 11, Message: "other", Severity: "warning"},
	}

	kept := filterDiffRelated(findings, diffs, 0)
	if len(kept) != 1 {
		t.Fatalf("expected 1 kept finding, got %d: %+v", len(kept), kept)
	}
	if kept[0].Message != "related" {
		t.Fatalf("unexpected kept finding: %+v", kept[0])
	}
}

func TestRenderForPrompt(t *testing.T) {
	out := RenderForPrompt([]Finding{{
		Tool:     "cppcheck+semgrep",
		RuleID:   "memleak",
		Category: "memory",
		Severity: "high",
		Path:     "src/main.c",
		Line:     42,
		Message:  "possible leak",
	}})
	if !strings.Contains(out, "[HIGH][cppcheck+semgrep] src/main.c:42") {
		t.Fatalf("prompt missing finding: %s", out)
	}
}
