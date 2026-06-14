package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-code-review/open-code-review/internal/ruleengine"
)

func TestBuildRulesScanSummary(t *testing.T) {
	findings := []ruleengine.Finding{
		{Severity: "high", Disposition: "blocking", RuleID: "r1", Path: "a.c", Role: "isr"},
		{Severity: "medium", Disposition: "review", RuleID: "r1", Path: "a.c", Role: "isr"},
		{Severity: "low", Disposition: "report_only", RuleID: "r2", Path: "b.c", Role: "driver"},
	}

	summary := buildRulesScanSummary(findings)
	if summary.Total != 3 {
		t.Fatalf("expected total 3, got %d", summary.Total)
	}
	if summary.HighCount != 1 || summary.MediumCount != 1 || summary.LowCount != 1 {
		t.Fatalf("unexpected severity counts: %+v", summary)
	}
	if summary.Rules["r1"] != 2 || summary.Rules["r2"] != 1 {
		t.Fatalf("unexpected rule counts: %+v", summary.Rules)
	}
	if summary.BlockingCount != 1 || summary.ReviewCount != 1 || summary.ReportOnlyCount != 1 {
		t.Fatalf("unexpected disposition counts: %+v", summary)
	}
	if summary.Roles["isr"] != 2 || summary.Roles["driver"] != 1 {
		t.Fatalf("unexpected role counts: %+v", summary.Roles)
	}
	if summary.Files["a.c"] != 2 || summary.Files["b.c"] != 1 {
		t.Fatalf("unexpected file counts: %+v", summary.Files)
	}
}

func TestStringListFlagAcceptsRepeatedAndCommaSeparatedValues(t *testing.T) {
	a := newOcrFlagSet("test")
	var includes []string
	a.StringListVarP(&includes, "include", "I", nil, "include path")

	if err := a.Parse([]string{"-I", "Inc", "--include=Drivers,HAL"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []string{"Inc", "Drivers", "HAL"}
	if !reflect.DeepEqual(includes, want) {
		t.Fatalf("unexpected includes: got %+v want %+v", includes, want)
	}
}

func TestLoadRulesScanProjectConfigResolvesProjectRoot(t *testing.T) {
	dir := t.TempDir()
	ocrDir := filepath.Join(dir, ".opencodereview")
	if err := os.MkdirAll(ocrDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	configPath := filepath.Join(ocrDir, "project.json")
	if err := os.WriteFile(configPath, append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{
  "paths": ["Src"],
  "role_config": ".opencodereview/roles.json",
  "baseline": ".opencodereview/baseline.json",
  "compile_commands": "build/compile_commands.json",
  "include": ["Include"],
  "define": ["CPU1"],
  "std": "c99",
  "fail_on": "review"
}`)...), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, root, err := loadRulesScanProjectConfig(configPath)
	if err != nil {
		t.Fatalf("loadRulesScanProjectConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if root != dir {
		t.Fatalf("expected root %q, got %q", dir, root)
	}
	if got := resolveRulesProjectPaths(root, cfg.Paths); !reflect.DeepEqual(got, []string{filepath.Join(dir, "Src")}) {
		t.Fatalf("unexpected resolved paths: %+v", got)
	}
	if got := resolveRulesProjectPaths(root, cfg.Include); !reflect.DeepEqual(got, []string{filepath.Join(dir, "Include")}) {
		t.Fatalf("unexpected resolved includes: %+v", got)
	}
	if cfg.FailOn != "review" {
		t.Fatalf("unexpected fail_on: %q", cfg.FailOn)
	}
	if got := resolveRulesProjectPath(root, cfg.Baseline); got != filepath.Join(dir, ".opencodereview", "baseline.json") {
		t.Fatalf("unexpected resolved baseline: %q", got)
	}
	if got := resolveRulesProjectPath(root, cfg.CompileCommands); got != filepath.Join(dir, "build", "compile_commands.json") {
		t.Fatalf("unexpected resolved compile_commands: %q", got)
	}
}

func TestNormalizeRulesScanFailOn(t *testing.T) {
	cases := map[string]string{
		"":            "none",
		"none":        "none",
		"blocking":    "blocking",
		"review":      "review",
		"report-only": "report_only",
		"reportonly":  "report_only",
		"any":         "any",
	}
	for input, want := range cases {
		got, err := normalizeRulesScanFailOn(input)
		if err != nil {
			t.Fatalf("normalizeRulesScanFailOn(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizeRulesScanFailOn(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := normalizeRulesScanFailOn("suppressed"); err == nil {
		t.Fatal("expected invalid fail-on value")
	}
}

func TestApplyRulesScanBaselineSuppressesMatchingFindings(t *testing.T) {
	finding := ruleengine.Finding{
		RuleID:      "r1",
		Path:        "Src/main.c",
		Line:        10,
		Message:     "same",
		Disposition: "review",
		Tags:        []string{"cppcheck"},
	}
	baseline := map[string]bool{
		rulesScanFindingFingerprint(finding): true,
	}

	got := applyRulesScanBaseline([]ruleengine.Finding{finding}, baseline)
	if got[0].Disposition != "suppressed" {
		t.Fatalf("expected suppressed disposition, got %+v", got[0])
	}
	if !reflect.DeepEqual(got[0].Tags, []string{"cppcheck", "baseline"}) {
		t.Fatalf("unexpected tags: %+v", got[0].Tags)
	}
	if finding.Disposition != "review" {
		t.Fatalf("expected input finding to remain unchanged, got %+v", finding)
	}
}

func TestWriteAndLoadRulesScanBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".opencodereview", "baseline.json")
	finding := ruleengine.Finding{
		RuleID:  "r1",
		Path:    "Src/main.c",
		Line:    10,
		Message: "same",
	}
	if err := writeRulesScanBaseline(path, []ruleengine.Finding{finding}); err != nil {
		t.Fatalf("writeRulesScanBaseline: %v", err)
	}

	baseline, err := loadRulesScanBaseline(path)
	if err != nil {
		t.Fatalf("loadRulesScanBaseline: %v", err)
	}
	if !baseline[rulesScanFindingFingerprint(finding)] {
		t.Fatalf("expected baseline to include finding fingerprint: %+v", baseline)
	}
}

func TestBuildRulesScanSARIF(t *testing.T) {
	findings := []ruleengine.Finding{{
		RuleID:      "r1",
		Title:       "Rule One",
		Severity:    "high",
		Path:        "Src\\main.c",
		Line:        12,
		Column:      3,
		Function:    "main",
		Disposition: "review",
		Message:     "problem",
		Tags:        []string{"embedded"},
		Backend:     "regex",
	}}

	log := buildRulesScanSARIF(findings)
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("unexpected SARIF header: %+v", log)
	}
	run := log.Runs[0]
	if len(run.Tool.Driver.Rules) != 1 || run.Tool.Driver.Rules[0].ID != "r1" {
		t.Fatalf("unexpected SARIF rules: %+v", run.Tool.Driver.Rules)
	}
	if len(run.Results) != 1 {
		t.Fatalf("unexpected SARIF results: %+v", run.Results)
	}
	result := run.Results[0]
	if result.Level != "error" || result.RuleID != "r1" {
		t.Fatalf("unexpected SARIF result: %+v", result)
	}
	if result.Locations[0].PhysicalLocation.ArtifactLocation.URI != "Src/main.c" {
		t.Fatalf("unexpected SARIF URI: %+v", result.Locations[0])
	}
	if result.Properties["disposition"] != "review" || result.Properties["function"] != "main" {
		t.Fatalf("unexpected SARIF properties: %+v", result.Properties)
	}
}

func TestRenderRulesScanHTMLEscapesFindingText(t *testing.T) {
	var buf bytes.Buffer
	findings := []ruleengine.Finding{{
		RuleID:      "r1",
		Title:       "Rule One",
		Severity:    "medium",
		Path:        "Src/main.c",
		Line:        7,
		Function:    "main",
		Disposition: "review",
		Message:     "bad <script>alert(1)</script>",
		Backend:     "regex",
	}}

	if err := renderRulesScanHTML(&buf, 1, findings); err != nil {
		t.Fatalf("renderRulesScanHTML: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "OpenCodeReview Rules Report") {
		t.Fatalf("expected report title in HTML")
	}
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected script text to be escaped: %s", html)
	}
	if !strings.Contains(html, "bad &lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("expected escaped message in HTML: %s", html)
	}
}

func TestSelectFindingsForAI(t *testing.T) {
	findings := []ruleengine.Finding{
		{Disposition: "report_only"},
		{Disposition: "review"},
		{Disposition: "blocking"},
		{Disposition: "suppressed"},
		{Disposition: "review"},
	}
	got := selectFindingsForAI(findings, 2)
	want := []int{2, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected AI selection: got %+v want %+v", got, want)
	}
}

func TestApplyAIResponse(t *testing.T) {
	findings := []ruleengine.Finding{
		{RuleID: "r1", Disposition: "review"},
		{RuleID: "r2", Disposition: "review"},
	}
	raw := `[{"index":1,"summary":"概要","risk":"风险","recommendation":"修复","confidence":"high"}]`

	got, warning := applyAIResponse(findings, []int{1}, raw)
	if warning != "" {
		t.Fatalf("unexpected warning: %s", warning)
	}
	if got[1].AI == nil || got[1].AI.Summary != "概要" || got[1].AI.Confidence != "high" {
		t.Fatalf("unexpected AI result: %+v", got[1].AI)
	}
	if got[0].AI != nil {
		t.Fatalf("did not expect AI on first finding: %+v", got[0].AI)
	}
}

func TestApplyAIResponseParseFailureKeepsFinding(t *testing.T) {
	findings := []ruleengine.Finding{{RuleID: "r1", Disposition: "review"}}
	got, warning := applyAIResponse(findings, []int{0}, "not json")
	if warning == "" {
		t.Fatal("expected parse warning")
	}
	if got[0].AI == nil || got[0].AI.Summary != "AI response parse failed" {
		t.Fatalf("expected fallback AI summary, got %+v", got[0].AI)
	}
}

func TestLoadRulesCompileCommandsFromArguments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compile_commands.json")
	raw := `[
  {
    "directory": "` + filepath.ToSlash(dir) + `",
    "file": "Src/main.c",
    "arguments": ["cc", "-I", "Include", "-DCPU1", "-U", "OLD", "-std=c11", "Src/main.c"]
  }
]`
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, []byte(raw)...), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	options, err := loadRulesCompileCommands(path)
	if err != nil {
		t.Fatalf("loadRulesCompileCommands: %v", err)
	}
	if !reflect.DeepEqual(options.Files, []string{filepath.Join(dir, "Src", "main.c")}) {
		t.Fatalf("unexpected files: %+v", options.Files)
	}
	if !reflect.DeepEqual(options.IncludePaths, []string{filepath.Join(dir, "Include")}) {
		t.Fatalf("unexpected includes: %+v", options.IncludePaths)
	}
	if !reflect.DeepEqual(options.Defines, []string{"CPU1"}) {
		t.Fatalf("unexpected defines: %+v", options.Defines)
	}
	if !reflect.DeepEqual(options.Undefines, []string{"OLD"}) {
		t.Fatalf("unexpected undefines: %+v", options.Undefines)
	}
	if options.Standard != "c11" {
		t.Fatalf("unexpected standard: %q", options.Standard)
	}
}

func TestLoadRulesCompileCommandsFromCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compile_commands.json")
	raw := `[
  {
    "directory": "` + filepath.ToSlash(dir) + `",
    "file": "Src/main.c",
    "command": "cc -I\"Inc Dir\" -DNAME=1 --std=c99 Src/main.c"
  }
]`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	options, err := loadRulesCompileCommands(path)
	if err != nil {
		t.Fatalf("loadRulesCompileCommands: %v", err)
	}
	if !reflect.DeepEqual(options.IncludePaths, []string{filepath.Join(dir, "Inc Dir")}) {
		t.Fatalf("unexpected includes: %+v", options.IncludePaths)
	}
	if !reflect.DeepEqual(options.Defines, []string{"NAME=1"}) {
		t.Fatalf("unexpected defines: %+v", options.Defines)
	}
	if options.Standard != "c99" {
		t.Fatalf("unexpected standard: %q", options.Standard)
	}
}

func TestDiscoverRulesAutoFindsProjectArtifacts(t *testing.T) {
	dir := t.TempDir()
	for _, subdir := range []string{".opencodereview", "build", "Include", "Src"} {
		if err := os.MkdirAll(filepath.Join(dir, subdir), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	for _, file := range []string{
		filepath.Join(dir, ".opencodereview", "project.json"),
		filepath.Join(dir, ".opencodereview", "roles.json"),
		filepath.Join(dir, ".opencodereview", "baseline.json"),
		filepath.Join(dir, "build", "compile_commands.json"),
	} {
		if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	discovery, err := discoverRulesAuto(dir)
	if err != nil {
		t.Fatalf("discoverRulesAuto: %v", err)
	}
	if discovery.ProjectConfigPath != filepath.Join(dir, ".opencodereview", "project.json") {
		t.Fatalf("unexpected project config: %+v", discovery)
	}
	if discovery.CompileCommandsPath != filepath.Join(dir, "build", "compile_commands.json") {
		t.Fatalf("unexpected compile commands: %+v", discovery)
	}
	if !reflect.DeepEqual(discovery.IncludePaths, []string{filepath.Join(dir, "Include")}) {
		t.Fatalf("unexpected includes: %+v", discovery.IncludePaths)
	}
	if len(discovery.ScanPaths) != 0 {
		t.Fatalf("expected compile_commands to own scan file list, got %+v", discovery.ScanPaths)
	}
}

func TestDiscoverRulesAutoFallsBackToSrc(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Src"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	discovery, err := discoverRulesAuto(dir)
	if err != nil {
		t.Fatalf("discoverRulesAuto: %v", err)
	}
	if !reflect.DeepEqual(discovery.ScanPaths, []string{filepath.Join(dir, "Src")}) {
		t.Fatalf("unexpected scan paths: %+v", discovery.ScanPaths)
	}
}

func TestRunDoctorChecksMarksMissingCppcheckAsRequiredFailure(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "cppcheck" {
			return "", os.ErrNotExist
		}
		return name, nil
	}
	output := func(name string, args ...string) (string, error) {
		return name + " ok", nil
	}

	checks := runDoctorChecks(lookPath, output)
	var found bool
	for _, check := range checks {
		if check.Name == "cppcheck" {
			found = true
			if check.Status != "FAIL" || !check.Required {
				t.Fatalf("unexpected cppcheck doctor status: %+v", check)
			}
		}
	}
	if !found {
		t.Fatalf("expected cppcheck doctor check, got %+v", checks)
	}
}

func TestRulesScanGateCount(t *testing.T) {
	summary := rulesScanSummary{
		Total:           15,
		BlockingCount:   1,
		ReviewCount:     2,
		ReportOnlyCount: 3,
		SuppressedCount: 4,
		IgnoredCount:    5,
	}

	cases := map[string]int{
		"none":        0,
		"blocking":    1,
		"review":      3,
		"report_only": 6,
		"any":         6,
	}
	for failOn, want := range cases {
		if got := rulesScanGateCount(failOn, summary); got != want {
			t.Fatalf("rulesScanGateCount(%q) = %d, want %d", failOn, got, want)
		}
	}
}
