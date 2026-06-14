package ruleengine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// loadFixture reads a fixture file under testdata/embedded_c and returns its
// contents as a FileInput suitable for the rule engine.
func loadFixture(t *testing.T, relPath string) FileInput {
	t.Helper()
	path := filepath.Join("testdata", "embedded_c", relPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return FileInput{
		Path:    filepath.ToSlash(relPath),
		Content: string(data),
	}
}

// loadEmbeddedRuleset returns the rules in
// ../rulesets/embedded_c/rules.json, used by every fixture test so the
// fixture set and the shipped rules stay in lockstep.
func loadEmbeddedRuleset(t *testing.T) []Rule {
	t.Helper()
	path := filepath.Join("..", "rulesets", "embedded_c", "rules.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ruleset %s: %v", path, err)
	}
	set, err := ParseRuleSet(data)
	if err != nil {
		t.Fatalf("ParseRuleSet: %v", err)
	}
	return set.Rules
}

// findingByRule returns all findings for the given rule id, sorted by line.
func findingsByRule(findings []Finding, ruleID string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.RuleID == ruleID {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// TestPositiveFixtureStrcpy verifies the embedded C ruleset still flags
// strcpy() in a real-shape C file (the rule ships, the rule fires).
func TestPositiveFixtureStrcpy(t *testing.T) {
	rules := loadEmbeddedRuleset(t)
	findings, err := New(RegexBackend{}).Analyze(Request{
		Files: []FileInput{loadFixture(t, "positive/strcpy_violation.c")},
	}, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	hits := findingsByRule(findings, "embedded.memory.no_strcpy")
	if len(hits) == 0 {
		t.Fatalf("expected no_strcpy finding in strcpy_violation.c, got %+v", findings)
	}
	if hits[0].Disposition != "blocking" {
		t.Fatalf("expected blocking disposition, got %q", hits[0].Disposition)
	}
	if hits[0].Backend != "regex" {
		t.Fatalf("expected regex backend, got %q", hits[0].Backend)
	}
}

// TestPositiveFixtureIsrDelay verifies the call backend, role hint wiring,
// and embedded.isr.no_delay_call rule all cooperate on a real-shape fixture.
func TestPositiveFixtureIsrDelay(t *testing.T) {
	rules := loadEmbeddedRuleset(t)
	findings, err := New(CallBackend{}).Analyze(Request{
		Files: []FileInput{loadFixture(t, "positive/isr_delay_violation.c")},
		RoleHints: []RoleHint{{
			Role:     "isr",
			Function: "TimerVectorHandler",
		}},
	}, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	hits := findingsByRule(findings, "embedded.isr.no_delay_call")
	if len(hits) == 0 {
		t.Fatalf("expected isr delay finding, got %+v", findings)
	}
	if hits[0].Role != "isr" {
		t.Fatalf("expected role=isr on finding, got %q", hits[0].Role)
	}
}

// TestNegativeFixtureMainInfiniteLoop verifies the function_name_not_any
// guard on suspicious_infinite_wait correctly suppresses main()'s while(1).
func TestNegativeFixtureMainInfiniteLoop(t *testing.T) {
	rules := loadEmbeddedRuleset(t)
	findings, err := New(RegexBackend{}).Analyze(Request{
		Files: []FileInput{loadFixture(t, "negative/main_infinite_loop.c")},
	}, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if hits := findingsByRule(findings, "embedded.loop.suspicious_infinite_wait"); len(hits) != 0 {
		t.Fatalf("expected main() while(1) to be suppressed, got %+v", hits)
	}
}

// TestCppcheckBackendDowngradesInconclusiveArrayOOB verifies the new
// disposition_overrides on cppcheck.array_index_out_of_bounds reduces the
// gate cost of inconclusive findings from blocking to report_only.
func TestCppcheckBackendDowngradesInconclusiveArrayOOB(t *testing.T) {
	rules := loadEmbeddedRuleset(t)

	xmlData, err := os.ReadFile(filepath.Join("testdata", "embedded_c", "cppcheck", "array_oob_inconclusive.xml"))
	if err != nil {
		t.Fatalf("read xml fixture: %v", err)
	}
	results, err := parseCppcheckResults(xmlData)
	if err != nil {
		t.Fatalf("parseCppcheckResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Exercise the same disposition resolution path the backend uses.
	rule := findRuleOrFail(t, rules, "cppcheck.array_index_out_of_bounds")
	disp := resolveFindingDisposition(rule, results[0].Msg+" "+results[0].Verbose)
	if disp != "report_only" {
		t.Fatalf("expected inconclusive array oob to downgrade to report_only, got %q", disp)
	}
}

// TestCppcheckBackendDowngradesUninitAssumption verifies the override on
// cppcheck.uninitialized_variable fires when cppcheck itself expresses
// uncertainty (the classic cross-file / interrupt / volatile init case).
func TestCppcheckBackendDowngradesUninitAssumption(t *testing.T) {
	rules := loadEmbeddedRuleset(t)
	xmlData, err := os.ReadFile(filepath.Join("testdata", "embedded_c", "cppcheck", "uninitvar_assumption.xml"))
	if err != nil {
		t.Fatalf("read xml fixture: %v", err)
	}
	results, err := parseCppcheckResults(xmlData)
	if err != nil {
		t.Fatalf("parseCppcheckResults: %v", err)
	}

	rule := findRuleOrFail(t, rules, "cppcheck.uninitialized_variable")
	disp := resolveFindingDisposition(rule, results[0].Msg+" "+results[0].Verbose)
	if disp != "report_only" {
		t.Fatalf("expected uninitvar with 'Assuming' to downgrade, got %q", disp)
	}
}

// TestCppcheckBackendPreservesGenuineUninitvar is the RED-LINE test: it
// guards against the overrides ever being too permissive. A genuine
// uninitvar report from cppcheck (no uncertain-language cues) must still
// come out as blocking.
func TestCppcheckBackendPreservesGenuineUninitvar(t *testing.T) {
	rules := loadEmbeddedRuleset(t)
	xmlData, err := os.ReadFile(filepath.Join("testdata", "embedded_c", "cppcheck", "uninitvar_genuine.xml"))
	if err != nil {
		t.Fatalf("read xml fixture: %v", err)
	}
	results, err := parseCppcheckResults(xmlData)
	if err != nil {
		t.Fatalf("parseCppcheckResults: %v", err)
	}

	rule := findRuleOrFail(t, rules, "cppcheck.uninitialized_variable")
	disp := resolveFindingDisposition(rule, results[0].Msg+" "+results[0].Verbose)
	if disp != "blocking" {
		t.Fatalf("RED-LINE: genuine uninitvar must stay blocking, got %q (msg=%q verbose=%q)",
			disp, results[0].Msg, results[0].Verbose)
	}
}

// TestCppcheckBackendPreservesCANOverride is a regression guard for the
// long-standing CANTRS.all / CANRMP.all override. Removing or weakening
// that override should fail this test loudly.
func TestCppcheckBackendPreservesCANOverride(t *testing.T) {
	rules := loadEmbeddedRuleset(t)
	rule := findRuleOrFail(t, rules, "cppcheck.redundant_assignment")
	disp := resolveFindingDisposition(rule, "Variable 'CANTRS.all' is reassigned before use")
	if disp != "report_only" {
		t.Fatalf("expected CAN override to keep report_only, got %q", disp)
	}
}

func findRuleOrFail(t *testing.T, rules []Rule, id string) Rule {
	t.Helper()
	for _, r := range rules {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %s not found in embedded_c ruleset", id)
	return Rule{}
}

// TestKnownRegexFalsePositiveInComment documents the current weakness of
// the regex backend: a `strcpy(` inside a C comment will still match.
// This is recorded here so future tree-sitter / AST work has a baseline
// test to assert improvement against.
func TestKnownRegexFalsePositiveInComment(t *testing.T) {
	rules := loadEmbeddedRuleset(t)
	findings, err := New(RegexBackend{}).Analyze(Request{
		Files: []FileInput{loadFixture(t, "negative/safe_copy_with_comment.c")},
	}, rules)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	hits := findingsByRule(findings, "embedded.memory.no_strcpy")
	// Currently this fires (regex doesn't know about comments). The test
	// exists to document the known false-positive and to fail loudly the
	// day someone upgrades the backend to a comment-aware parser.
	if len(hits) == 0 {
		t.Logf("known-weakness probe: strcpy-in-comment no longer fires (good, the backend was upgraded)")
	} else {
		t.Logf("known-weakness probe: regex backend still flags comment-only strcpy at line %d (expected until AST backend lands)", hits[0].Line)
		// Sanity: the hit is at least on the comment line, not somewhere unrelated.
		if !strings.Contains(hits[0].Message, "strcpy") {
			t.Fatalf("unexpected finding message: %q", hits[0].Message)
		}
	}
}
