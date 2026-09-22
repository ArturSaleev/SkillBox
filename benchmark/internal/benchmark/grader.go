package benchmark

import (
	"strings"

	"github.com/aibox/skillbox/benchmark/internal/agent"
	"github.com/aibox/skillbox/benchmark/internal/storage"
)

func grade(item storage.BenchmarkCase, result agent.Result, runErr error) (float64, bool, []storage.ScoreCheck) {
	checks := []storage.ScoreCheck{{Name: "completed", Passed: runErr == nil && strings.TrimSpace(result.Content) != ""}}
	if runErr != nil {
		checks[0].Details = runErr.Error()
	}
	content := strings.ToLower(result.Content)
	for _, phrase := range item.RequiredPhrases {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		passed := strings.Contains(content, strings.ToLower(phrase))
		checks = append(checks, storage.ScoreCheck{Name: "required phrase", Passed: passed, Details: phrase})
	}
	for _, phrase := range item.ForbiddenPhrases {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		passed := !strings.Contains(content, strings.ToLower(phrase))
		checks = append(checks, storage.ScoreCheck{Name: "forbidden phrase absent", Passed: passed, Details: phrase})
	}
	for _, required := range item.RequiredTools {
		required = strings.TrimSpace(required)
		if required == "" {
			continue
		}
		passed := false
		for _, call := range result.ToolCalls {
			qualified := call.ServerID + "." + call.ToolName
			if required == call.ToolName || required == qualified || required == agent.ExternalToolName(call.ServerID, call.ToolName) {
				passed = true
				break
			}
		}
		checks = append(checks, storage.ScoreCheck{Name: "required tool", Passed: passed, Details: required})
	}
	passedCount := 0
	for _, check := range checks {
		if check.Passed {
			passedCount++
		}
	}
	score := float64(passedCount) / float64(len(checks)) * 100
	return score, passedCount == len(checks), checks
}
