package ruleengine

type CallBackend struct{}

func (CallBackend) Name() string { return "call" }

func (CallBackend) Supports(rule Rule) bool {
	return rule.Backend == "call"
}

func (CallBackend) Analyze(req Request, rules []Rule) ([]Finding, error) {
	var findings []Finding
	for _, file := range req.Files {
		path := normalizePath(file.Path)
		lines := splitLines(file.Content)
		functions := parseFunctionContexts(path, lines, req.RoleHints)
		for _, fn := range functions {
			for _, rule := range rules {
				if !matchesFunctionWhere(rule, fn) {
					continue
				}
				for _, call := range fn.Calls {
					if !isChangedLine(req, path, call.Line) {
						continue
					}
					if !callNameIn(call.Name, rule.Match.CallsAny) {
						continue
					}
					findings = append(findings, Finding{
						RuleID:      rule.ID,
						Title:       rule.Title,
						Severity:    normalizeSeverity(rule.Severity),
						Path:        path,
						Line:        call.Line,
						Column:      call.Column,
						Function:    fn.Name,
						Role:        fn.Role,
						Disposition: resolveFindingDisposition(rule, call.Name+" "+call.Snippet),
						Message:     rule.Message,
						Suggestion:  rule.Suggestion,
						Tags:        rule.Tags,
						Backend:     "call",
						Evidence: []Evidence{{
							Kind:    "function_call",
							Path:    path,
							Line:    call.Line,
							Column:  call.Column,
							Snippet: call.Snippet,
							Detail:  fn.Name + " calls " + call.Name,
						}},
					})
				}
			}
		}
	}
	return findings, nil
}

func matchesFunctionWhere(rule Rule, fn FunctionContext) bool {
	if rule.Where.FunctionRole != "" && fn.Role != rule.Where.FunctionRole {
		return false
	}
	if len(rule.Where.FunctionNameAny) > 0 && !patternListMatches(rule.Where.FunctionNameAny, fn.Name) {
		return false
	}
	if patternListMatches(rule.Where.FunctionNameNotAny, fn.Name) {
		return false
	}
	return true
}

func callNameIn(name string, candidates []string) bool {
	for _, candidate := range candidates {
		if name == candidate {
			return true
		}
	}
	return false
}
