package ruleengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-code-review/open-code-review/internal/model"
)

func TestParseRuleSetValidatesRequiredFields(t *testing.T) {
	_, err := ParseRuleSet([]byte(`{"rules":[{"title":"missing id"}]}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "missing id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegexBackendFindsDangerousCall(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.memory.no_strcpy",
		Title:    "禁止使用 strcpy",
		Severity: "high",
		Scope:    "line",
		Backend:  "regex",
		Match: Match{
			Pattern: `\bstrcpy\s*\(`,
		},
		Message: "strcpy is not bounded",
	}}

	engine := New(RegexBackend{})
	findings, err := engine.Analyze(Request{
		Files: []FileInput{{
			Path:    "Src/main.c",
			Content: "void f(void) {\n  strcpy(dst, src);\n}\n",
		}},
	}, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	got := findings[0]
	if got.Path != "Src/main.c" || got.Line != 2 || got.Column != 3 {
		t.Fatalf("unexpected location: %+v", got)
	}
	if got.RuleID != "embedded.memory.no_strcpy" || got.Severity != "high" {
		t.Fatalf("unexpected finding: %+v", got)
	}
	if got.Disposition != "review" {
		t.Fatalf("expected default high-severity disposition to be review, got %q", got.Disposition)
	}
}

func TestRegexBackendFiltersToChangedLines(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.memory.no_sprintf",
		Title:    "禁止使用 sprintf",
		Severity: "high",
		Scope:    "line",
		Backend:  "regex",
		Match: Match{
			Pattern: `\bsprintf\s*\(`,
		},
		Message: "sprintf is not bounded",
	}}
	req := Request{
		Files: []FileInput{{
			Path: "Src/main.c",
			Content: strings.Join([]string{
				"void f(void) {",
				"  sprintf(a, \"%d\", x);",
				"  sprintf(b, \"%d\", y);",
				"}",
			}, "\n"),
		}},
		ChangedLines: map[string]map[int]bool{
			"Src/main.c": {3: true},
		},
	}

	findings, err := New(RegexBackend{}).Analyze(req, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 || findings[0].Line != 3 {
		t.Fatalf("expected only line 3 finding, got %+v", findings)
	}
}

func TestInlineSuppressionNextLineSuppressesSpecificRule(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.memory.no_strcpy",
		Title:    "no strcpy",
		Severity: "high",
		Scope:    "line",
		Backend:  "regex",
		Match: Match{
			Pattern: `\bstrcpy\s*\(`,
		},
		Message: "strcpy is not bounded",
	}}

	findings, err := New(RegexBackend{}).Analyze(Request{
		Files: []FileInput{{
			Path: "Src/main.c",
			Content: strings.Join([]string{
				"void f(void) {",
				"  // ocr-disable-next-line embedded.memory.no_strcpy",
				"  strcpy(dst, src);",
				"}",
			}, "\n"),
		}},
	}, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %+v", findings)
	}
	if findings[0].Disposition != "suppressed" {
		t.Fatalf("expected suppressed finding, got %+v", findings[0])
	}
	if !stringSliceContains(findings[0].Tags, inlineSuppressionTag) {
		t.Fatalf("expected inline suppression tag, got %+v", findings[0].Tags)
	}
}

func TestInlineSuppressionLineSuppressesAllRules(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.memory.no_sprintf",
		Title:    "no sprintf",
		Severity: "high",
		Scope:    "line",
		Backend:  "regex",
		Match: Match{
			Pattern: `\bsprintf\s*\(`,
		},
		Message: "sprintf is not bounded",
	}}

	findings, err := New(RegexBackend{}).Analyze(Request{
		Files: []FileInput{{
			Path:    "Src/main.c",
			Content: "void f(void) {\n  sprintf(a, \"%d\", x); // ocr-disable-line\n}\n",
		}},
	}, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 || findings[0].Disposition != "suppressed" {
		t.Fatalf("expected suppressed finding, got %+v", findings)
	}
}

func TestInlineSuppressionSupportsRuleWildcards(t *testing.T) {
	patterns := parseInlineSuppressionRules("cppcheck.* reason")
	if !suppressionMatches(patterns, "cppcheck.unread_variable") {
		t.Fatalf("expected wildcard to match cppcheck rule")
	}
	if suppressionMatches(patterns, "embedded.memory.no_strcpy") {
		t.Fatalf("did not expect wildcard to match embedded rule")
	}
}

func TestRegexBackendBlockNotAnySuppressesFinding(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.loop.suspicious_infinite_wait",
		Title:    "可疑无限等待循环",
		Severity: "medium",
		Scope:    "line",
		Backend:  "regex",
		Match: Match{
			Pattern:     `\bwhile\s*\(\s*1\s*\)`,
			BlockNotAny: []string{"break", "timeout"},
		},
		Message: "loop requires exit evidence",
	}}

	req := Request{
		Files: []FileInput{{
			Path: "Src/main.c",
			Content: strings.Join([]string{
				"void wait_ready(void) {",
				"  while(1) {",
				"    if (ready()) {",
				"      break;",
				"    }",
				"  }",
				"}",
				"void wait_forever(void) {",
				"  while(1) {",
				"    poll_status();",
				"  }",
				"}",
			}, "\n"),
		}},
	}

	findings, err := New(RegexBackend{}).Analyze(req, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %+v", findings)
	}
	if findings[0].Line != 9 {
		t.Fatalf("expected unsuppressed loop on line 9, got %+v", findings[0])
	}
}

func TestRegexBackendFiltersByFunctionNameNotAny(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.loop.suspicious_infinite_wait",
		Title:    "suspicious infinite loop",
		Severity: "medium",
		Scope:    "line",
		Backend:  "regex",
		Where: Where{
			FunctionNameNotAny: []string{"main"},
		},
		Match: Match{
			Pattern: `\bwhile\s*\(\s*1\s*\)`,
		},
		Message: "loop requires exit evidence",
	}}

	req := Request{
		Files: []FileInput{{
			Path: "Src/main.c",
			Content: strings.Join([]string{
				"void wait_forever(void) {",
				"  while(1) {",
				"    poll_status();",
				"  }",
				"}",
				"int main(void) {",
				"  while(1) {",
				"    scheduler_tick();",
				"  }",
				"}",
			}, "\n"),
		}},
	}

	findings, err := New(RegexBackend{}).Analyze(req, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected only non-main loop finding, got %+v", findings)
	}
	if findings[0].Function != "wait_forever" {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
}

func TestRegexBackendFiltersByFunctionRole(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.isr.no_delay_call",
		Title:    "ISR 中禁止调用延时函数",
		Severity: "high",
		Scope:    "line",
		Backend:  "regex",
		Where: Where{
			FunctionRole: "isr",
		},
		Match: Match{
			Pattern: `\bDelayMs\s*\(`,
		},
		Message: "delay in ISR",
	}}

	req := Request{
		Files: []FileInput{{
			Path: "Src/ISR/GPIOExIntISR.c",
			Content: strings.Join([]string{
				"void normal_task(void) {",
				"  DelayMs(10);",
				"}",
				"interrupt void GPIOExIntISR(void) {",
				"  DelayMs(1);",
				"}",
			}, "\n"),
		}},
	}

	findings, err := New(RegexBackend{}).Analyze(req, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 ISR finding, got %+v", findings)
	}
	if findings[0].Line != 5 {
		t.Fatalf("expected ISR delay on line 5, got %+v", findings[0])
	}
}

func TestCallBackendFindsCallInISR(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.isr.no_delay_call",
		Title:    "ISR 中禁止调用延时函数",
		Severity: "high",
		Scope:    "line",
		Backend:  "call",
		Where: Where{
			FunctionRole: "isr",
		},
		Match: Match{
			CallsAny: []string{"DelayMs"},
		},
		Message: "delay in ISR",
	}}

	req := Request{
		Files: []FileInput{{
			Path: "Src/ISR/GPIOExIntISR.c",
			Content: strings.Join([]string{
				"void normal_task(void) {",
				"  DelayMs(10);",
				"}",
				"interrupt void GPIOExIntISR(void) {",
				"  DelayMs(1);",
				"}",
			}, "\n"),
		}},
	}

	findings, err := New(CallBackend{}).Analyze(req, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 call finding, got %+v", findings)
	}
	if findings[0].Line != 5 || findings[0].Backend != "call" {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
}

func TestCallBackendUsesRoleHints(t *testing.T) {
	rules := []Rule{{
		ID:       "embedded.isr.no_delay_call",
		Title:    "ISR 中禁止调用延时函数",
		Severity: "high",
		Scope:    "line",
		Backend:  "call",
		Where: Where{
			FunctionRole: "isr",
		},
		Match: Match{
			CallsAny: []string{"DelayMs"},
		},
		Message: "delay in ISR",
	}}

	req := Request{
		Files: []FileInput{{
			Path: "Src/vector.c",
			Content: strings.Join([]string{
				"void TimerVectorHandler(void) {",
				"  DelayMs(1);",
				"}",
			}, "\n"),
		}},
		RoleHints: []RoleHint{{
			Role:     "isr",
			Function: "TimerVectorHandler",
		}},
	}

	findings, err := New(CallBackend{}).Analyze(req, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected role hint to enable ISR finding, got %+v", findings)
	}
}

func TestResolveRoleFromHints(t *testing.T) {
	hints := []RoleHint{
		{Role: "driver", Path: "Src/DSPDriver/**/*.c"},
		{Role: "isr", Function: "*VectorHandler"},
	}
	if got := resolveRoleFromHints("Src/DSPDriver/DSP_Clock.c", "ClockInit", hints); got != "driver" {
		t.Fatalf("expected driver role, got %q", got)
	}
	if got := resolveRoleFromHints("Src/vector.c", "TimerVectorHandler", hints); got != "isr" {
		t.Fatalf("expected isr role, got %q", got)
	}
}

func TestParseCppcheckResults(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<results version="2">
  <errors>
    <error id="redundantAssignment" severity="style" msg="Variable is reassigned" verbose="Variable is reassigned before use">
      <location file="Src/main.c" line="12" column="3"/>
    </error>
  </errors>
</results>`)

	results, err := parseCppcheckResults(raw)
	if err != nil {
		t.Fatalf("parseCppcheckResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "redundantAssignment" || results[0].Locations[0].Line != "12" {
		t.Fatalf("unexpected result: %+v", results[0])
	}
}

func TestCppcheckRuleMap(t *testing.T) {
	rules := []Rule{{
		ID:      "cppcheck.redundant_assignment",
		Backend: "cppcheck",
		Match: Match{
			Any: []string{"redundantAssignment"},
		},
	}}
	mapping := cppcheckRuleMap(rules)
	if mapping["redundantAssignment"].ID != "cppcheck.redundant_assignment" {
		t.Fatalf("unexpected mapping: %+v", mapping)
	}
}

func TestResolveFindingDispositionOverride(t *testing.T) {
	rule := Rule{
		Severity:    "medium",
		Disposition: "review",
		DispositionOverrides: []DispositionOverride{{
			IfMessageContainsAny: []string{"CANTRS.all"},
			Disposition:          "report_only",
		}},
	}
	if got := resolveFindingDisposition(rule, "Variable 'CANTRS.all' is reassigned"); got != "report_only" {
		t.Fatalf("expected report_only override, got %q", got)
	}
	if got := resolveFindingDisposition(rule, "Variable 'fanSwAx_u16' is reassigned"); got != "review" {
		t.Fatalf("expected default review disposition, got %q", got)
	}
}

func TestBuildCppcheckArgsUsesCScanOptions(t *testing.T) {
	args := buildCppcheckArgs(CScanOptions{
		IncludePaths: []string{" Inc ", "Drivers"},
		Defines:      []string{"CPU1", "DEBUG=1"},
		Undefines:    []string{"LEGACY"},
		Standard:     "c11",
		Platform:     "unix32",
	}, []string{"Src/main.c"})

	want := []string{
		"--std=c11",
		"--platform=unix32",
		"-IInc",
		"-IDrivers",
		"-DCPU1",
		"-DDEBUG=1",
		"-ULEGACY",
		"Src/main.c",
	}
	for _, item := range want {
		if !stringSliceContains(args, item) {
			t.Fatalf("expected cppcheck args to contain %q, got %+v", item, args)
		}
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestDedupeFindings(t *testing.T) {
	findings := []Finding{
		{RuleID: "r1", Path: "a.c", Line: 10, Message: "same"},
		{RuleID: "r1", Path: "a.c", Line: 10, Message: "same"},
		{RuleID: "r1", Path: "a.c", Line: 11, Message: "same"},
	}
	got := dedupeFindings(findings)
	if len(got) != 2 {
		t.Fatalf("expected 2 findings after dedupe, got %+v", got)
	}
}

func TestChangedLinesFromDiffs(t *testing.T) {
	diffs := []model.Diff{{
		NewPath: "Src/main.c",
		Diff: strings.Join([]string{
			"diff --git a/Src/main.c b/Src/main.c",
			"--- a/Src/main.c",
			"+++ b/Src/main.c",
			"@@ -10,2 +10,3 @@",
			" old",
			"+new_call();",
			" keep",
		}, "\n"),
	}}

	lines := ChangedLinesFromDiffs(diffs, 1)
	if !lines["Src/main.c"][10] || !lines["Src/main.c"][11] || !lines["Src/main.c"][12] {
		t.Fatalf("expected changed line with context, got %+v", lines)
	}
}

func TestEmbeddedRulesetParses(t *testing.T) {
	path := filepath.Join("..", "rulesets", "embedded_c", "rules.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ruleset: %v", err)
	}
	set, err := ParseRuleSet(data)
	if err != nil {
		t.Fatalf("ParseRuleSet: %v", err)
	}
	if len(set.Rules) == 0 {
		t.Fatal("expected embedded rules")
	}
	for _, rule := range set.Rules {
		if rule.Backend == "regex" {
			if _, err := New(RegexBackend{}).Analyze(Request{}, []Rule{rule}); err != nil {
				t.Fatalf("rule %s should compile: %v", rule.ID, err)
			}
		}
	}
}
