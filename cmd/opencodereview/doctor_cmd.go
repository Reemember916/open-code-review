package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/open-code-review/open-code-review/internal/llm"
)

type doctorCheck struct {
	Name     string
	Status   string
	Detail   string
	Required bool
}

func runDoctor(args []string) error {
	a := newOcrFlagSet("ocr doctor")
	if err := a.Parse(args); err != nil {
		return err
	}
	if a.showHelp {
		printDoctorUsage()
		return nil
	}

	checks := runDoctorChecks(exec.LookPath, commandOutput)
	hasRequiredFailure := false
	for _, check := range checks {
		fmt.Printf("[%s] %s: %s\n", check.Status, check.Name, check.Detail)
		if check.Required && check.Status != "OK" {
			hasRequiredFailure = true
		}
	}
	if hasRequiredFailure {
		return fmt.Errorf("doctor found missing required dependency")
	}
	return nil
}

func runDoctorChecks(lookPath func(string) (string, error), output func(string, ...string) (string, error)) []doctorCheck {
	var checks []doctorCheck
	checks = append(checks, doctorCheck{
		Name:   "OpenCodeReview",
		Status: "OK",
		Detail: fmt.Sprintf("%s %s/%s", Version, runtime.GOOS, runtime.GOARCH),
	})

	if _, err := lookPath("go"); err != nil {
		checks = append(checks, doctorCheck{
			Name:   "Go",
			Status: "WARN",
			Detail: "not found in PATH; required only when running from source with go run/go test",
		})
	} else if version, err := output("go", "version"); err == nil {
		checks = append(checks, doctorCheck{Name: "Go", Status: "OK", Detail: strings.TrimSpace(version)})
	} else {
		checks = append(checks, doctorCheck{Name: "Go", Status: "WARN", Detail: err.Error()})
	}

	if _, err := lookPath("cppcheck"); err != nil {
		checks = append(checks, doctorCheck{
			Name:     "cppcheck",
			Status:   "FAIL",
			Detail:   "not found in PATH; install cppcheck before running embedded C static scan",
			Required: true,
		})
	} else if version, err := output("cppcheck", "--version"); err == nil {
		checks = append(checks, doctorCheck{Name: "cppcheck", Status: "OK", Detail: strings.TrimSpace(version), Required: true})
	} else {
		checks = append(checks, doctorCheck{Name: "cppcheck", Status: "FAIL", Detail: err.Error(), Required: true})
	}

	cfgPath, err := defaultConfigPath()
	if err != nil {
		checks = append(checks, doctorCheck{Name: "LLM", Status: "WARN", Detail: err.Error()})
		return checks
	}
	ep, err := llm.ResolveEndpoint(cfgPath)
	if err != nil {
		checks = append(checks, doctorCheck{
			Name:   "LLM",
			Status: "WARN",
			Detail: "not configured; required only for --ai. Run `ocr llm test` after configuring llm.url/auth_token/model",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:   "LLM",
			Status: "OK",
			Detail: fmt.Sprintf("%s via %s", ep.Model, ep.Source),
		})
	}
	return checks
}

func commandOutput(name string, args ...string) (string, error) {
	data, err := exec.Command(name, args...).CombinedOutput()
	return string(data), err
}

func printDoctorUsage() {
	fmt.Println(`Usage:
  ocr doctor

Check trial-version dependencies and configuration.

The embedded C scanner requires cppcheck. LLM configuration is only required when using --ai.`)
}
