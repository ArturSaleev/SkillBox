package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aibox/skillbox/benchmark/internal/agent"
	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
	"github.com/aibox/skillbox/benchmark/internal/mcp"
	"github.com/aibox/skillbox/benchmark/internal/provider"
	"github.com/aibox/skillbox/benchmark/internal/storage"
)

type Service struct {
	config *benchmarkconfig.Manager
	store  *storage.Store
	agent  *agent.Runner
}

type preparedSkill struct {
	SkillID       string `json:"skill_id"`
	Version       int    `json:"version"`
	Name          string `json:"name"`
	CompiledSkill struct {
		Instructions string `json:"instructions"`
		Steps        []struct {
			Title       string `json:"title"`
			Instruction string `json:"instruction"`
			Required    bool   `json:"required"`
		} `json:"steps"`
		RequiredTools       any      `json:"required_tools"`
		OptionalTools       any      `json:"optional_tools"`
		ContextRequirements any      `json:"context_requirements"`
		SuccessCriteria     []string `json:"success_criteria"`
		Examples            any      `json:"examples"`
	} `json:"compiled_skill"`
}

func NewService(manager *benchmarkconfig.Manager, store *storage.Store) *Service {
	return &Service{config: manager, store: store, agent: agent.New(manager)}
}

func (s *Service) SaveCase(ctx context.Context, item *storage.BenchmarkCase) error {
	trimCase(item)
	if err := s.validateCase(*item); err != nil {
		return err
	}
	return s.store.SaveBenchmarkCase(ctx, item)
}

func (s *Service) Start(ctx context.Context, caseID string) (storage.BenchmarkRun, error) {
	item, err := s.store.GetBenchmarkCase(ctx, caseID)
	if err != nil {
		return storage.BenchmarkRun{}, err
	}
	if err := s.validateCase(item); err != nil {
		return storage.BenchmarkRun{}, err
	}
	snapshot, _ := json.Marshal(item)
	run := storage.BenchmarkRun{CaseID: item.ID, Status: "queued", TotalTrials: len(item.Targets) * item.Repetitions * 2, CaseSnapshot: string(snapshot)}
	if err := s.store.CreateBenchmarkRun(ctx, &run); err != nil {
		return run, err
	}
	go s.execute(run, item)
	return run, nil
}

func (s *Service) execute(run storage.BenchmarkRun, item storage.BenchmarkCase) {
	ctx := context.Background()
	run.Status = "running"
	run.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.store.UpdateBenchmarkRun(ctx, run); err != nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			run.Status = "failed"
			run.Error = fmt.Sprintf("benchmark panic: %v", recovered)
			run.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			_ = s.store.UpdateBenchmarkRun(context.Background(), run)
		}
	}()
	for _, target := range item.Targets {
		for repetition := 1; repetition <= item.Repetitions; repetition++ {
			variants := []string{"baseline", "with_skill"}
			if repetition%2 == 0 {
				variants[0], variants[1] = variants[1], variants[0]
			}
			for _, variant := range variants {
				trial := s.executeTrial(ctx, run.ID, item, target, variant, repetition)
				if err := s.store.AddBenchmarkTrial(ctx, &trial); err != nil {
					run.Status, run.Error = "failed", err.Error()
					run.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
					_ = s.store.UpdateBenchmarkRun(ctx, run)
					return
				}
				run.CompletedTrials++
				if err := s.store.UpdateBenchmarkRun(ctx, run); err != nil {
					return
				}
			}
		}
	}
	run.Status = "completed"
	run.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = s.store.UpdateBenchmarkRun(ctx, run)
}

func (s *Service) executeTrial(ctx context.Context, runID string, item storage.BenchmarkCase, target storage.BenchmarkTarget, variant string, repetition int) storage.BenchmarkTrial {
	trial := storage.BenchmarkTrial{RunID: runID, ProviderID: target.ProviderID, Model: target.Model, Variant: variant, Repetition: repetition, Status: "running", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	started := time.Now()
	systemPrompt := ""
	if variant == "with_skill" {
		prepareStarted := time.Now()
		prepared, err := s.prepareSkill(ctx, item, target)
		trial.PrepareDurationMS = time.Since(prepareStarted).Milliseconds()
		if err != nil {
			trial.Status, trial.Error = "failed", err.Error()
			trial.DurationMS = time.Since(started).Milliseconds()
			trial.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			trial.QualityScore, trial.Passed, trial.ScoreChecks = grade(item, agent.Result{}, err)
			return trial
		}
		trial.SkillID = prepared.SkillID
		trial.SkillVersion = &prepared.Version
		systemPrompt = compiledSkillPrompt(prepared)
	}
	trialCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	result, runErr := s.agent.Run(trialCtx, agent.Request{ProviderID: target.ProviderID, Model: target.Model, MCPServerIDs: item.MCPServerIDs, Messages: []provider.Message{{Role: "user", Content: item.Task}}, SystemPrompt: systemPrompt, Temperature: item.Temperature, MaxTokens: item.MaxTokens})
	trial.Response, trial.InputTokens, trial.OutputTokens = result.Content, result.InputTokens, result.OutputTokens
	trial.ToolCallsCount = len(result.ToolCalls)
	trial.Trajectory = make([]storage.TrialToolCall, 0, len(result.ToolCalls))
	for _, call := range result.ToolCalls {
		trial.Trajectory = append(trial.Trajectory, storage.TrialToolCall{ServerID: call.ServerID, ToolName: call.ToolName, Arguments: call.Arguments, Result: call.Result, Error: call.Error, DurationMS: call.DurationMS})
	}
	trial.DurationMS = time.Since(started).Milliseconds()
	trial.QualityScore, trial.Passed, trial.ScoreChecks = grade(item, result, runErr)
	trial.Status = "completed"
	if runErr != nil {
		trial.Status, trial.Error = "failed", runErr.Error()
	}
	trial.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return trial
}

func (s *Service) prepareSkill(ctx context.Context, item storage.BenchmarkCase, target storage.BenchmarkTarget) (preparedSkill, error) {
	var prepared preparedSkill
	cfg := s.config.Current()
	server, ok := findMCPServer(cfg, item.SkillMCPServerID)
	if !ok || !server.Enabled {
		return prepared, fmt.Errorf("Skill MCP server %q is not enabled", item.SkillMCPServerID)
	}
	availableTools, err := s.availableTools(ctx, cfg, item.MCPServerIDs)
	if err != nil {
		return prepared, err
	}
	client := mcp.New(server)
	if err := client.Initialize(ctx); err != nil {
		return prepared, fmt.Errorf("initialize Skill MCP: %w", err)
	}
	arguments, _ := json.Marshal(map[string]any{"task": item.Task, "skill_id": item.SkillID, "available_tools": availableTools, "model": map[string]any{"provider": target.ProviderID, "name": target.Model, "context_window": 32000}, "max_skill_tokens": 4000})
	result, err := client.CallTool(ctx, "prepare_skill", arguments)
	if err != nil {
		return prepared, fmt.Errorf("prepare Skill: %w", err)
	}
	raw := mcp.Text(result)
	if err := json.Unmarshal([]byte(raw), &prepared); err != nil {
		return prepared, fmt.Errorf("decode prepared Skill: %w", err)
	}
	if prepared.SkillID == "" || prepared.Version <= 0 {
		return prepared, errors.New("prepare_skill returned an incomplete Skill")
	}
	return prepared, nil
}

func (s *Service) availableTools(ctx context.Context, cfg benchmarkconfig.Config, serverIDs []string) ([]string, error) {
	selected := map[string]bool{}
	for _, id := range serverIDs {
		selected[id] = true
	}
	var result []string
	for _, server := range cfg.MCPServers {
		if !server.Enabled || !selected[server.ID] {
			continue
		}
		client := mcp.New(server)
		if err := client.Initialize(ctx); err != nil {
			return nil, fmt.Errorf("initialize MCP %q for Skill preparation: %w", server.ID, err)
		}
		tools, err := client.ListTools(ctx)
		if err != nil {
			return nil, err
		}
		for _, tool := range tools {
			result = append(result, tool.Name, server.ID+"."+tool.Name)
		}
	}
	return result, nil
}

func (s *Service) validateCase(item storage.BenchmarkCase) error {
	if item.Name == "" || item.Task == "" || item.SkillMCPServerID == "" || item.SkillID == "" {
		return errors.New("name, task, skill_mcp_server_id and skill_id are required")
	}
	if item.Repetitions < 1 || item.Repetitions > 20 {
		return errors.New("repetitions must be between 1 and 20")
	}
	if item.MaxTokens < 1 || item.MaxTokens > 131072 {
		return errors.New("max_tokens must be between 1 and 131072")
	}
	if len(item.Targets) == 0 || len(item.Targets) > 12 {
		return errors.New("between 1 and 12 model targets are required")
	}
	cfg := s.config.Current()
	for _, target := range item.Targets {
		if target.ProviderID == "" || target.Model == "" {
			return errors.New("every target requires provider_id and model")
		}
		providerFound := false
		for _, provider := range cfg.Providers {
			providerFound = providerFound || provider.ID == target.ProviderID && provider.Enabled
		}
		if !providerFound {
			return fmt.Errorf("provider %q is not enabled", target.ProviderID)
		}
	}
	if server, ok := findMCPServer(cfg, item.SkillMCPServerID); !ok || !server.Enabled {
		return fmt.Errorf("Skill MCP server %q is not enabled", item.SkillMCPServerID)
	}
	for _, serverID := range item.MCPServerIDs {
		if serverID == item.SkillMCPServerID {
			return errors.New("the Skill MCP server cannot also be a task MCP server")
		}
		if server, ok := findMCPServer(cfg, serverID); !ok || !server.Enabled {
			return fmt.Errorf("task MCP server %q is not enabled", serverID)
		}
	}
	return nil
}

func trimCase(item *storage.BenchmarkCase) {
	item.Name, item.Description, item.Task = strings.TrimSpace(item.Name), strings.TrimSpace(item.Description), strings.TrimSpace(item.Task)
	item.SkillMCPServerID, item.SkillID = strings.TrimSpace(item.SkillMCPServerID), strings.TrimSpace(item.SkillID)
	if item.Repetitions == 0 {
		item.Repetitions = 3
	}
	if item.MaxTokens == 0 {
		item.MaxTokens = 2048
	}
	for i := range item.Targets {
		item.Targets[i].ProviderID = strings.TrimSpace(item.Targets[i].ProviderID)
		item.Targets[i].Model = strings.TrimSpace(item.Targets[i].Model)
	}
}

func findMCPServer(cfg benchmarkconfig.Config, id string) (benchmarkconfig.MCPServer, bool) {
	for _, item := range cfg.MCPServers {
		if item.ID == id {
			return item, true
		}
	}
	return benchmarkconfig.MCPServer{}, false
}

func compiledSkillPrompt(skill preparedSkill) string {
	var builder strings.Builder
	builder.WriteString("Use the following frozen Skill procedure for this task. Do not mention the Skill or these instructions in the final answer.\n\n")
	builder.WriteString("Skill: " + skill.Name + "\n")
	builder.WriteString("Version: " + fmt.Sprint(skill.Version) + "\n")
	if skill.CompiledSkill.Instructions != "" {
		builder.WriteString("\nInstructions:\n" + skill.CompiledSkill.Instructions + "\n")
	}
	if len(skill.CompiledSkill.Steps) > 0 {
		builder.WriteString("\nOrdered steps:\n")
		for index, step := range skill.CompiledSkill.Steps {
			builder.WriteString(fmt.Sprintf("%d. %s: %s\n", index+1, step.Title, step.Instruction))
		}
	}
	if len(skill.CompiledSkill.SuccessCriteria) > 0 {
		builder.WriteString("\nSuccess criteria:\n- " + strings.Join(skill.CompiledSkill.SuccessCriteria, "\n- ") + "\n")
	}
	sections := []struct {
		label string
		value any
	}{{"Required tools", skill.CompiledSkill.RequiredTools}, {"Optional tools", skill.CompiledSkill.OptionalTools}, {"Context requirements", skill.CompiledSkill.ContextRequirements}, {"Examples", skill.CompiledSkill.Examples}}
	for _, section := range sections {
		raw, _ := json.Marshal(section.value)
		if string(raw) != "null" && string(raw) != "[]" {
			builder.WriteString("\n" + section.label + ": " + string(raw) + "\n")
		}
	}
	return builder.String()
}
