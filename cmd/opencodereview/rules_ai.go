package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/open-code-review/open-code-review/internal/llm"
	"github.com/open-code-review/open-code-review/internal/ruleengine"
)

const defaultAIExplainLimit = 20

type aiFindingInput struct {
	Index       int                   `json:"index"`
	RuleID      string                `json:"rule_id"`
	Title       string                `json:"title"`
	Severity    string                `json:"severity"`
	Disposition string                `json:"disposition"`
	Path        string                `json:"path"`
	Line        int                   `json:"line"`
	Function    string                `json:"function,omitempty"`
	Role        string                `json:"role,omitempty"`
	Message     string                `json:"message"`
	Evidence    []ruleengine.Evidence `json:"evidence,omitempty"`
}

type aiFindingOutput struct {
	Index          int    `json:"index"`
	Summary        string `json:"summary"`
	Risk           string `json:"risk"`
	Recommendation string `json:"recommendation"`
	Confidence     string `json:"confidence"`
}

func enrichFindingsWithAI(ctx context.Context, findings []ruleengine.Finding, limit int) ([]ruleengine.Finding, string, error) {
	selected := selectFindingsForAI(findings, limit)
	if len(selected) == 0 {
		return findings, "", nil
	}
	cfgPath, err := defaultConfigPath()
	if err != nil {
		return nil, "", err
	}
	ep, err := llm.ResolveEndpoint(cfgPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve LLM endpoint for --ai: %w; configure llm.url/auth_token/model and run `ocr llm test`", err)
	}
	client := llm.NewLLMClient(ep)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return explainFindingsWithClient(ctx, client, ep.Model, findings, selected)
}

func explainFindingsWithClient(ctx context.Context, client llm.LLMClient, model string, findings []ruleengine.Finding, selected []int) ([]ruleengine.Finding, string, error) {
	inputs := make([]aiFindingInput, 0, len(selected))
	for _, idx := range selected {
		finding := findings[idx]
		inputs = append(inputs, aiFindingInput{
			Index:       idx,
			RuleID:      finding.RuleID,
			Title:       finding.Title,
			Severity:    finding.Severity,
			Disposition: finding.Disposition,
			Path:        finding.Path,
			Line:        finding.Line,
			Function:    finding.Function,
			Role:        finding.Role,
			Message:     finding.Message,
			Evidence:    finding.Evidence,
		})
	}
	data, err := json.Marshal(inputs)
	if err != nil {
		return nil, "", err
	}
	temp := 0.0
	resp, err := client.CompletionsWithCtx(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			llm.NewTextMessage("system", "你是嵌入式 C 代码审查助手。请只返回 JSON 数组，不要 Markdown。每个元素包含 index、summary、risk、recommendation、confidence。confidence 只能是 high、medium、low。"),
			llm.NewTextMessage("user", "请解释以下静态分析 finding，面向嵌入式 C 开发者，中文输出。输入 JSON:\n"+string(data)),
		},
		Temperature: &temp,
		MaxTokens:   2048,
	})
	if err != nil {
		return nil, "", err
	}
	out, warning := applyAIResponse(findings, selected, resp.Content())
	return out, warning, nil
}

func selectFindingsForAI(findings []ruleengine.Finding, limit int) []int {
	if limit <= 0 {
		return nil
	}
	var selected []int
	for _, disposition := range []string{"blocking", "review"} {
		for i, finding := range findings {
			if len(selected) >= limit {
				return selected
			}
			if finding.Disposition != disposition {
				continue
			}
			if finding.Disposition == "suppressed" || finding.Disposition == "ignored" {
				continue
			}
			selected = append(selected, i)
		}
	}
	return selected
}

func applyAIResponse(findings []ruleengine.Finding, selected []int, content string) ([]ruleengine.Finding, string) {
	out := make([]ruleengine.Finding, len(findings))
	copy(out, findings)
	var parsed []aiFindingOutput
	clean := strings.TrimSpace(content)
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		for _, idx := range selected {
			out[idx].AI = &ruleengine.FindingAI{
				Summary:    "AI response parse failed",
				Risk:       strings.TrimSpace(content),
				Confidence: "low",
			}
		}
		return out, "AI response was not valid JSON; stored raw response in ai.risk"
	}
	for _, item := range parsed {
		if item.Index < 0 || item.Index >= len(out) {
			continue
		}
		out[item.Index].AI = &ruleengine.FindingAI{
			Summary:        item.Summary,
			Risk:           item.Risk,
			Recommendation: item.Recommendation,
			Confidence:     normalizeAIConfidence(item.Confidence),
		}
	}
	return out, ""
}

func normalizeAIConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		return "medium"
	}
}
