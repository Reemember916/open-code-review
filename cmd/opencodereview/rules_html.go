package main

import (
	"html/template"
	"io"
	"os"
	"strings"

	"github.com/open-code-review/open-code-review/internal/ruleengine"
)

type rulesScanHTMLData struct {
	FileCount int
	Summary   rulesScanSummary
	Findings  []ruleengine.Finding
}

func outputRulesScanHTML(fileCount int, findings []ruleengine.Finding) error {
	return renderRulesScanHTML(os.Stdout, fileCount, findings)
}

func renderRulesScanHTML(w io.Writer, fileCount int, findings []ruleengine.Finding) error {
	tmpl, err := template.New("rules-report").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(rulesScanHTMLTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, rulesScanHTMLData{
		FileCount: fileCount,
		Summary:   buildRulesScanSummary(findings),
		Findings:  findings,
	})
}

const rulesScanHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>OpenCodeReview Rules Report</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #1f2937;
      --muted: #667085;
      --line: #d7dce3;
      --high: #b42318;
      --medium: #b54708;
      --low: #175cd3;
      --review: #7a2e0e;
      --blocking: #912018;
      --report: #344054;
      --suppressed: #475467;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font-family: "Segoe UI", Arial, sans-serif;
      font-size: 14px;
      line-height: 1.45;
    }
    header {
      padding: 24px 32px 16px;
      border-bottom: 1px solid var(--line);
      background: var(--panel);
    }
    h1 {
      margin: 0 0 6px;
      font-size: 24px;
      font-weight: 650;
      letter-spacing: 0;
    }
    .subtle { color: var(--muted); }
    main { padding: 24px 32px 36px; }
    .summary {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
      gap: 12px;
      margin-bottom: 20px;
    }
    .metric {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px 14px;
    }
    .metric .label {
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
    }
    .metric .value {
      margin-top: 4px;
      font-size: 24px;
      font-weight: 650;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      overflow: hidden;
    }
    th, td {
      padding: 9px 10px;
      border-bottom: 1px solid var(--line);
      vertical-align: top;
      text-align: left;
    }
    th {
      background: #eef1f5;
      color: #344054;
      font-size: 12px;
      text-transform: uppercase;
      white-space: nowrap;
    }
    tr:last-child td { border-bottom: 0; }
    code {
      font-family: Consolas, "Courier New", monospace;
      font-size: 12px;
    }
    .badge {
      display: inline-block;
      border-radius: 999px;
      padding: 2px 8px;
      font-size: 12px;
      font-weight: 600;
      border: 1px solid var(--line);
      white-space: nowrap;
    }
    .severity-high { color: var(--high); }
    .severity-medium { color: var(--medium); }
    .severity-low { color: var(--low); }
    .disp-blocking { color: var(--blocking); }
    .disp-review { color: var(--review); }
    .disp-report_only { color: var(--report); }
    .disp-suppressed, .disp-ignored { color: var(--suppressed); }
    .path {
      max-width: 340px;
      word-break: break-word;
    }
    .message {
      min-width: 260px;
    }
  </style>
</head>
<body>
  <header>
    <h1>OpenCodeReview Rules Report</h1>
    <div class="subtle">Scanned {{.FileCount}} file(s), {{.Summary.Total}} finding(s)</div>
  </header>
  <main>
    <section class="summary">
      <div class="metric"><div class="label">Total</div><div class="value">{{.Summary.Total}}</div></div>
      <div class="metric"><div class="label">High</div><div class="value severity-high">{{.Summary.HighCount}}</div></div>
      <div class="metric"><div class="label">Medium</div><div class="value severity-medium">{{.Summary.MediumCount}}</div></div>
      <div class="metric"><div class="label">Low</div><div class="value severity-low">{{.Summary.LowCount}}</div></div>
      <div class="metric"><div class="label">Blocking</div><div class="value disp-blocking">{{.Summary.BlockingCount}}</div></div>
      <div class="metric"><div class="label">Review</div><div class="value disp-review">{{.Summary.ReviewCount}}</div></div>
      <div class="metric"><div class="label">Report Only</div><div class="value disp-report_only">{{.Summary.ReportOnlyCount}}</div></div>
      <div class="metric"><div class="label">Suppressed</div><div class="value disp-suppressed">{{.Summary.SuppressedCount}}</div></div>
    </section>
    <table>
      <thead>
        <tr>
          <th>Severity</th>
          <th>Disposition</th>
          <th>Location</th>
          <th>Rule</th>
          <th>Context</th>
          <th>Message</th>
          <th>AI</th>
        </tr>
      </thead>
      <tbody>
      {{range .Findings}}
        <tr>
          <td><span class="badge severity-{{.Severity}}">{{upper .Severity}}</span></td>
          <td><span class="badge disp-{{.Disposition}}">{{.Disposition}}</span></td>
          <td class="path"><code>{{.Path}}:{{.Line}}</code></td>
          <td><code>{{.RuleID}}</code><br><span class="subtle">{{.Backend}}</span></td>
          <td>{{if .Function}}<code>{{.Function}}</code>{{end}}{{if .Role}}<br><span class="subtle">{{.Role}}</span>{{end}}</td>
          <td class="message">{{.Message}}{{if .Suggestion}}<br><span class="subtle">{{.Suggestion}}</span>{{end}}</td>
          <td class="message">{{if .AI}}<strong>{{.AI.Summary}}</strong><br><span class="subtle">{{.AI.Risk}}</span><br>{{.AI.Recommendation}}{{if .AI.Confidence}}<br><span class="subtle">confidence: {{.AI.Confidence}}</span>{{end}}{{else}}<span class="subtle">-</span>{{end}}</td>
        </tr>
      {{else}}
        <tr><td colspan="7" class="subtle">No findings.</td></tr>
      {{end}}
      </tbody>
    </table>
  </main>
</body>
</html>
`
