package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-code-review/open-code-review/internal/ruleengine"
	embeddedc "github.com/open-code-review/open-code-review/internal/rulesets/embedded_c"
)

type rulesScanOutput struct {
	Status   string               `json:"status"`
	Files    int                  `json:"files"`
	Summary  rulesScanSummary     `json:"summary"`
	Findings []ruleengine.Finding `json:"findings"`
}

type rulesScanSummary struct {
	Total           int            `json:"total"`
	Severity        map[string]int `json:"severity,omitempty"`
	Disposition     map[string]int `json:"disposition,omitempty"`
	Rules           map[string]int `json:"rules,omitempty"`
	Roles           map[string]int `json:"roles,omitempty"`
	Files           map[string]int `json:"files,omitempty"`
	HighCount       int            `json:"high_count"`
	MediumCount     int            `json:"medium_count"`
	LowCount        int            `json:"low_count"`
	BlockingCount   int            `json:"blocking_count"`
	ReviewCount     int            `json:"review_count"`
	ReportOnlyCount int            `json:"report_only_count"`
	SuppressedCount int            `json:"suppressed_count"`
	IgnoredCount    int            `json:"ignored_count"`
}

func runRulesScan(args []string) error {
	a := newOcrFlagSet("ocr rules scan")
	var projectConfigPath, rulesetPath, roleConfigPath, baselinePath, writeBaselinePath, compileCommandsPath, format, failOn string
	var includePaths, defines, undefines []string
	var cStandard, cPlatform string
	var ai bool
	var aiLimit int
	a.StringVar(&projectConfigPath, "project", "", "path to project scan config JSON (default: .opencodereview/project.json when present)")
	a.StringVar(&rulesetPath, "ruleset", "", "path to ruleengine JSON ruleset (default: embedded C rules)")
	a.StringVar(&roleConfigPath, "role-config", "", "path to function role config JSON")
	a.StringVar(&baselinePath, "baseline", "", "path to rules scan baseline JSON; matching findings are marked suppressed")
	a.StringVar(&writeBaselinePath, "write-baseline", "", "write current findings to a baseline JSON file")
	a.StringVar(&compileCommandsPath, "compile-commands", "", "path to compile_commands.json for analyzer include/define/std context")
	a.StringVarP(&format, "format", "f", "json", "output format: json, text, sarif, or html")
	a.BoolVar(&ai, "ai", false, "add LLM-generated explanations for blocking/review findings")
	a.IntVar(&aiLimit, "ai-limit", defaultAIExplainLimit, "maximum number of findings to explain when --ai is enabled")
	a.StringListVarP(&includePaths, "include", "I", nil, "C include directory for analyzer backends; repeat or comma-separate")
	a.StringListVarP(&defines, "define", "D", nil, "C preprocessor define for analyzer backends; repeat or comma-separate")
	a.StringListVarP(&undefines, "undefine", "U", nil, "C preprocessor undefine for analyzer backends; repeat or comma-separate")
	a.StringVar(&cStandard, "std", "", "C/C++ language standard passed to analyzer backends (default: c99)")
	a.StringVar(&cPlatform, "platform", "", "target platform passed to analyzer backends, e.g. native, unix32, win64, avr8")
	a.StringVar(&failOn, "fail-on", "", "exit with failure when findings reach disposition: none, blocking, review, report_only, any")
	if err := a.Parse(args); err != nil {
		return err
	}
	if a.showHelp {
		printRulesScanUsage()
		return nil
	}
	if aiLimit < 0 {
		return fmt.Errorf("--ai-limit must be non-negative")
	}

	projectConfig, projectRoot, err := loadRulesScanProjectConfig(projectConfigPath)
	if err != nil {
		return err
	}
	if projectConfig != nil {
		if rulesetPath == "" {
			rulesetPath = resolveRulesProjectPath(projectRoot, projectConfig.Ruleset)
		}
		if roleConfigPath == "" {
			roleConfigPath = resolveRulesProjectPath(projectRoot, projectConfig.RoleConfig)
		}
		if baselinePath == "" {
			baselinePath = resolveRulesProjectPath(projectRoot, projectConfig.Baseline)
		}
		if compileCommandsPath == "" {
			compileCommandsPath = resolveRulesProjectPath(projectRoot, projectConfig.CompileCommands)
		}
		configuredIncludes := resolveRulesProjectPaths(projectRoot, projectConfig.Include)
		includePaths = append(configuredIncludes, includePaths...)
		defines = append(append([]string{}, projectConfig.Define...), defines...)
		undefines = append(append([]string{}, projectConfig.Undefine...), undefines...)
		if cStandard == "" {
			cStandard = projectConfig.Std
		}
		if cPlatform == "" {
			cPlatform = projectConfig.Platform
		}
		if failOn == "" {
			failOn = projectConfig.FailOn
		}
	}
	failOn, err = normalizeRulesScanFailOn(failOn)
	if err != nil {
		return err
	}
	compileOptions, err := loadRulesCompileCommands(compileCommandsPath)
	if err != nil {
		return err
	}
	if len(compileOptions.IncludePaths) > 0 {
		includePaths = append(compileOptions.IncludePaths, includePaths...)
	}
	if len(compileOptions.Defines) > 0 {
		defines = append(compileOptions.Defines, defines...)
	}
	if len(compileOptions.Undefines) > 0 {
		undefines = append(compileOptions.Undefines, undefines...)
	}
	if cStandard == "" {
		cStandard = compileOptions.Standard
	}

	paths := a.fs.Args()
	if len(paths) == 0 && projectConfig != nil {
		paths = resolveRulesProjectPaths(projectRoot, projectConfig.Paths)
	}
	if len(paths) == 0 && len(compileOptions.Files) > 0 {
		paths = compileOptions.Files
	}
	if len(paths) == 0 {
		printRulesScanUsage()
		return nil
	}

	files, err := collectRuleScanFiles(paths)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .c/.h files found")
	}

	ruleset, err := loadRuleScanRuleSet(rulesetPath)
	if err != nil {
		return err
	}
	roleHints, err := ruleengine.LoadRoleConfig(roleConfigPath)
	if err != nil {
		return err
	}

	findings, err := ruleengine.New(ruleengine.RegexBackend{}, ruleengine.CallBackend{}, ruleengine.CppcheckBackend{}).Analyze(ruleengine.Request{
		Files:     files,
		RoleHints: roleHints,
		COptions: ruleengine.CScanOptions{
			IncludePaths: includePaths,
			Defines:      defines,
			Undefines:    undefines,
			Standard:     cStandard,
			Platform:     cPlatform,
		},
	}, ruleset.Rules)
	if err != nil {
		return err
	}
	if writeBaselinePath != "" {
		if err := writeRulesScanBaseline(writeBaselinePath, findings); err != nil {
			return err
		}
	}
	baseline, err := loadRulesScanBaseline(baselinePath)
	if err != nil {
		return err
	}
	findings = applyRulesScanBaseline(findings, baseline)
	if ai {
		enriched, warning, err := enrichFindingsWithAI(context.Background(), findings, aiLimit)
		if err != nil {
			return err
		}
		if warning != "" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}
		findings = enriched
	}

	switch format {
	case "json":
		if err := outputRulesScanJSON(len(files), findings); err != nil {
			return err
		}
	case "text":
		outputRulesScanText(len(files), findings)
	case "sarif":
		if err := outputRulesScanSARIF(findings); err != nil {
			return err
		}
	case "html":
		if err := outputRulesScanHTML(len(files), findings); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid --format %q: must be json, text, sarif, or html", format)
	}
	return applyRulesScanGate(failOn, buildRulesScanSummary(findings))
}

func loadRuleScanRuleSet(path string) (*ruleengine.RuleSet, error) {
	if path == "" {
		return embeddedc.Load()
	}
	return ruleengine.LoadRuleSet(path)
}

func collectRuleScanFiles(paths []string) ([]ruleengine.FileInput, error) {
	var filePaths []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		if !info.IsDir() {
			if isCSourcePath(path) {
				filePaths = append(filePaths, path)
			}
			continue
		}
		err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if isCSourcePath(p) {
				filePaths = append(filePaths, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", path, err)
		}
	}
	sort.Strings(filePaths)

	files := make([]ruleengine.FileInput, 0, len(filePaths))
	for _, path := range filePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		files = append(files, ruleengine.FileInput{
			Path:    path,
			Content: string(data),
		})
	}
	return files, nil
}

func isCSourcePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".c" || ext == ".h"
}

func outputRulesScanJSON(fileCount int, findings []ruleengine.Finding) error {
	out := rulesScanOutput{
		Status:   "success",
		Files:    fileCount,
		Summary:  buildRulesScanSummary(findings),
		Findings: findings,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func outputRulesScanText(fileCount int, findings []ruleengine.Finding) {
	summary := buildRulesScanSummary(findings)
	fmt.Printf("Scanned %d file(s), %d finding(s)\n", fileCount, len(findings))
	fmt.Printf("Severity: high=%d medium=%d low=%d\n", summary.HighCount, summary.MediumCount, summary.LowCount)
	fmt.Printf("Disposition: blocking=%d review=%d report_only=%d suppressed=%d ignored=%d\n",
		summary.BlockingCount,
		summary.ReviewCount,
		summary.ReportOnlyCount,
		summary.SuppressedCount,
		summary.IgnoredCount)
	for _, finding := range findings {
		context := ""
		if finding.Function != "" {
			context = " function=" + finding.Function
			if finding.Role != "" {
				context += " role=" + finding.Role
			}
		}
		disposition := normalizeFindingDisposition(finding.Disposition)
		fmt.Printf("%s:%d [%s][%s]%s %s (%s)\n",
			finding.Path,
			finding.Line,
			strings.ToUpper(finding.Severity),
			disposition,
			context,
			finding.Message,
			finding.RuleID)
		if finding.AI != nil {
			fmt.Printf("  AI: %s | risk: %s | fix: %s\n", finding.AI.Summary, finding.AI.Risk, finding.AI.Recommendation)
		}
	}
}

func buildRulesScanSummary(findings []ruleengine.Finding) rulesScanSummary {
	summary := rulesScanSummary{
		Total:       len(findings),
		Severity:    make(map[string]int),
		Disposition: make(map[string]int),
		Rules:       make(map[string]int),
		Roles:       make(map[string]int),
		Files:       make(map[string]int),
	}
	for _, finding := range findings {
		severity := strings.ToLower(finding.Severity)
		disposition := normalizeFindingDisposition(finding.Disposition)
		summary.Severity[severity]++
		summary.Disposition[disposition]++
		summary.Rules[finding.RuleID]++
		summary.Files[finding.Path]++
		if finding.Role != "" {
			summary.Roles[finding.Role]++
		}
		switch severity {
		case "high":
			summary.HighCount++
		case "medium":
			summary.MediumCount++
		case "low":
			summary.LowCount++
		}
		switch disposition {
		case "blocking":
			summary.BlockingCount++
		case "review":
			summary.ReviewCount++
		case "report_only":
			summary.ReportOnlyCount++
		case "suppressed":
			summary.SuppressedCount++
		case "ignored":
			summary.IgnoredCount++
		}
	}
	return summary
}

func normalizeFindingDisposition(disposition string) string {
	switch strings.ToLower(strings.TrimSpace(disposition)) {
	case "blocking":
		return "blocking"
	case "review":
		return "review"
	case "report_only", "report-only", "reportonly":
		return "report_only"
	case "suppressed":
		return "suppressed"
	case "ignored":
		return "ignored"
	default:
		return "review"
	}
}

func normalizeRulesScanFailOn(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none":
		return "none", nil
	case "blocking":
		return "blocking", nil
	case "review":
		return "review", nil
	case "report_only", "report-only", "reportonly":
		return "report_only", nil
	case "any":
		return "any", nil
	default:
		return "", fmt.Errorf("invalid --fail-on %q: must be none, blocking, review, report_only, or any", value)
	}
}

func applyRulesScanGate(failOn string, summary rulesScanSummary) error {
	count := rulesScanGateCount(failOn, summary)
	if count == 0 {
		return nil
	}
	return fmt.Errorf("rules scan gate failed: --fail-on %s matched %d finding(s)", failOn, count)
}

func rulesScanGateCount(failOn string, summary rulesScanSummary) int {
	switch failOn {
	case "blocking":
		return summary.BlockingCount
	case "review":
		return summary.BlockingCount + summary.ReviewCount
	case "report_only":
		return summary.BlockingCount + summary.ReviewCount + summary.ReportOnlyCount
	case "any":
		return summary.Total - summary.SuppressedCount - summary.IgnoredCount
	default:
		return 0
	}
}

func printRulesScanUsage() {
	fmt.Println(`Usage:
  ocr rules scan [flags] <file-or-dir>...

Scan C/H files with the embedded rule engine. This command does not require Git or LLM configuration.

Flags:
  --ai                add LLM-generated explanations for blocking/review findings
  --ai-limit int      maximum number of findings to explain when --ai is enabled (default 20)
  -f, --format string   output format: json, text, sarif, or html (default "json")
  --baseline string     path to rules scan baseline JSON; matching findings are marked suppressed
  --compile-commands string path to compile_commands.json for analyzer include/define/std context
  -D, --define string   C preprocessor define for analyzer backends; repeat or comma-separate
  --fail-on string      exit with failure when findings reach disposition: none, blocking, review, report_only, any
  -I, --include string  C include directory for analyzer backends; repeat or comma-separate
  -U, --undefine string C preprocessor undefine for analyzer backends; repeat or comma-separate
  --platform string     target platform passed to analyzer backends, e.g. native, unix32, win64, avr8
  --project string      path to project scan config JSON (default: .opencodereview/project.json when present)
  --role-config string  path to function role config JSON
  --ruleset string      path to ruleengine JSON ruleset (default: embedded C rules)
  --std string          C/C++ language standard passed to analyzer backends (default: c99)
  --write-baseline string write current findings to a baseline JSON file

Examples:
  ocr rules scan Src
  ocr rules scan --project .opencodereview\project.json
  ocr rules scan --compile-commands build\compile_commands.json
  ocr rules scan --format sarif Src > ocr.sarif
  ocr rules scan --format html Src > ocr.html
  ocr rules scan --ai --format html Src > ocr-ai.html
  ocr rules scan --write-baseline .opencodereview\baseline.json Src
  ocr rules scan --baseline .opencodereview\baseline.json --fail-on review Src
  ocr rules scan --fail-on review Src
  ocr rules scan --format text Src\bit\IFBIT.c
  ocr rules scan -I Inc -D CPU1 --std c99 Src
  ocr rules scan --role-config .opencodereview\roles.json Src
  ocr rules scan --ruleset .opencodereview\embedded-rules.json Src`)
}
