package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type rulesAutoDiscovery struct {
	Root                string
	ProjectConfigPath   string
	CompileCommandsPath string
	RoleConfigPath      string
	BaselinePath        string
	IncludePaths        []string
	ScanPaths           []string
}

func runRulesAuto(args []string) error {
	a := newOcrFlagSet("ocr scan")
	var format, failOn string
	var ai bool
	var aiLimit int
	a.StringVarP(&format, "format", "f", "text", "output format: json, text, sarif, or html")
	a.StringVar(&failOn, "fail-on", "", "exit with failure when findings reach disposition: none, blocking, review, report_only, any")
	a.BoolVar(&ai, "ai", false, "add LLM-generated explanations for blocking/review findings")
	a.IntVar(&aiLimit, "ai-limit", defaultAIExplainLimit, "maximum number of findings to explain when --ai is enabled")
	if err := a.Parse(args); err != nil {
		return err
	}
	if a.showHelp {
		printRulesAutoUsage()
		return nil
	}

	rest := a.fs.Args()
	if len(rest) > 1 {
		return fmt.Errorf("ocr scan accepts at most one project directory")
	}
	root := "."
	if len(rest) == 1 {
		root = rest[0]
	}
	discovery, err := discoverRulesAuto(root)
	if err != nil {
		return err
	}

	scanArgs := []string{"--format", format}
	if ai {
		scanArgs = append(scanArgs, "--ai", "--ai-limit", fmt.Sprintf("%d", aiLimit))
	}
	if failOn != "" {
		scanArgs = append(scanArgs, "--fail-on", failOn)
	}
	if discovery.ProjectConfigPath != "" {
		scanArgs = append(scanArgs, "--project", discovery.ProjectConfigPath)
	}
	if discovery.CompileCommandsPath != "" {
		scanArgs = append(scanArgs, "--compile-commands", discovery.CompileCommandsPath)
	}
	if discovery.RoleConfigPath != "" {
		scanArgs = append(scanArgs, "--role-config", discovery.RoleConfigPath)
	}
	if discovery.BaselinePath != "" {
		scanArgs = append(scanArgs, "--baseline", discovery.BaselinePath)
	}
	for _, includePath := range discovery.IncludePaths {
		scanArgs = append(scanArgs, "--include", includePath)
	}
	scanArgs = append(scanArgs, discovery.ScanPaths...)
	return runRulesScan(scanArgs)
}

func discoverRulesAuto(root string) (rulesAutoDiscovery, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return rulesAutoDiscovery{}, fmt.Errorf("resolve project directory %s: %w", root, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return rulesAutoDiscovery{}, fmt.Errorf("stat project directory %s: %w", absRoot, err)
	}
	if !info.IsDir() {
		absRoot = filepath.Dir(absRoot)
	}

	discovery := rulesAutoDiscovery{Root: absRoot}
	discovery.ProjectConfigPath = firstExistingPath(
		filepath.Join(absRoot, ".opencodereview", "project.json"),
	)
	discovery.CompileCommandsPath = firstExistingPath(
		filepath.Join(absRoot, "compile_commands.json"),
		filepath.Join(absRoot, "build", "compile_commands.json"),
		filepath.Join(absRoot, "Build", "compile_commands.json"),
	)
	discovery.RoleConfigPath = firstExistingPath(
		filepath.Join(absRoot, ".opencodereview", "roles.json"),
	)
	discovery.BaselinePath = firstExistingPath(
		filepath.Join(absRoot, ".opencodereview", "baseline.json"),
	)
	discovery.IncludePaths = existingPaths(
		filepath.Join(absRoot, "Include"),
		filepath.Join(absRoot, "Includes"),
		filepath.Join(absRoot, "include"),
		filepath.Join(absRoot, "Inc"),
		filepath.Join(absRoot, "inc"),
	)
	if discovery.CompileCommandsPath == "" {
		discovery.ScanPaths = existingPaths(
			filepath.Join(absRoot, "Src"),
			filepath.Join(absRoot, "src"),
			filepath.Join(absRoot, "Source"),
			filepath.Join(absRoot, "source"),
		)
		if len(discovery.ScanPaths) == 0 {
			discovery.ScanPaths = []string{absRoot}
		}
	}
	return discovery, nil
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func existingPaths(paths ...string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			key := filepath.Clean(path)
			if os.PathSeparator == '\\' {
				key = filepath.Clean(filepath.ToSlash(path))
				key = strings.ToLower(key)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, path)
		}
	}
	return out
}

func printRulesAutoUsage() {
	fmt.Println(`Usage:
  ocr scan [flags] [project-dir]
  ocr rules auto [flags] [project-dir]

Automatically scan an embedded C project. The command discovers project config, compile_commands.json, Include/Src directories, roles, and baseline when present.

Flags:
  --ai                add LLM-generated explanations for blocking/review findings
  --ai-limit int      maximum number of findings to explain when --ai is enabled (default 20)
  -f, --format string   output format: json, text, sarif, or html (default "text")
  --fail-on string      exit with failure when findings reach disposition: none, blocking, review, report_only, any

Examples:
  ocr scan .
  ocr scan C:\Projects\Q2760
  ocr scan --format html C:\Projects\Q2760 > report.html
  ocr scan --ai --format html C:\Projects\Q2760 > report-ai.html
  ocr scan --fail-on review .`)
}
