package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BenchmarkCase struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Task             string            `json:"task"`
	MCPServerIDs     []string          `json:"mcp_server_ids"`
	SkillMCPServerID string            `json:"skill_mcp_server_id"`
	SkillID          string            `json:"skill_id"`
	Repetitions      int               `json:"repetitions"`
	Temperature      *float64          `json:"temperature,omitempty"`
	MaxTokens        int               `json:"max_tokens"`
	RequiredPhrases  []string          `json:"required_phrases"`
	ForbiddenPhrases []string          `json:"forbidden_phrases"`
	RequiredTools    []string          `json:"required_tools"`
	Targets          []BenchmarkTarget `json:"targets"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
}

type BenchmarkTarget struct {
	ID         string `json:"id"`
	CaseID     string `json:"case_id"`
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	Position   int    `json:"position"`
}

type BenchmarkRun struct {
	ID              string           `json:"id"`
	CaseID          string           `json:"case_id"`
	Status          string           `json:"status"`
	TotalTrials     int              `json:"total_trials"`
	CompletedTrials int              `json:"completed_trials"`
	Error           string           `json:"error,omitempty"`
	StartedAt       string           `json:"started_at,omitempty"`
	FinishedAt      string           `json:"finished_at,omitempty"`
	CaseSnapshot    string           `json:"case_snapshot,omitempty"`
	CreatedAt       string           `json:"created_at"`
	Trials          []BenchmarkTrial `json:"trials,omitempty"`
}

type BenchmarkTrial struct {
	ID                string          `json:"id"`
	RunID             string          `json:"run_id"`
	ProviderID        string          `json:"provider_id"`
	Model             string          `json:"model"`
	Variant           string          `json:"variant"`
	Repetition        int             `json:"repetition"`
	Status            string          `json:"status"`
	Response          string          `json:"response"`
	Error             string          `json:"error,omitempty"`
	DurationMS        int64           `json:"duration_ms"`
	PrepareDurationMS int64           `json:"prepare_duration_ms"`
	InputTokens       int             `json:"input_tokens"`
	OutputTokens      int             `json:"output_tokens"`
	ToolCallsCount    int             `json:"tool_calls_count"`
	Trajectory        []TrialToolCall `json:"trajectory"`
	QualityScore      float64         `json:"quality_score"`
	Passed            bool            `json:"passed"`
	ScoreChecks       []ScoreCheck    `json:"score_checks"`
	SkillID           string          `json:"skill_id,omitempty"`
	SkillVersion      *int            `json:"skill_version,omitempty"`
	StartedAt         string          `json:"started_at"`
	FinishedAt        string          `json:"finished_at"`
}

type TrialToolCall struct {
	ServerID   string `json:"server_id"`
	ToolName   string `json:"tool_name"`
	Arguments  string `json:"arguments"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type ScoreCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details,omitempty"`
}

func (s *Store) SaveBenchmarkCase(ctx context.Context, item *BenchmarkCase) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if item.ID == "" {
		item.ID = uuid.NewString()
		item.CreatedAt = now
	}
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	mcpIDs, _ := json.Marshal(item.MCPServerIDs)
	requiredPhrases, _ := json.Marshal(item.RequiredPhrases)
	forbiddenPhrases, _ := json.Marshal(item.ForbiddenPhrases)
	requiredTools, _ := json.Marshal(item.RequiredTools)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO benchmark_cases(id,name,description,task,mcp_server_ids,skill_mcp_server_id,skill_id,repetitions,temperature,max_tokens,required_phrases,forbidden_phrases,required_tools,created_at,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description,task=excluded.task,mcp_server_ids=excluded.mcp_server_ids,skill_mcp_server_id=excluded.skill_mcp_server_id,skill_id=excluded.skill_id,repetitions=excluded.repetitions,temperature=excluded.temperature,max_tokens=excluded.max_tokens,required_phrases=excluded.required_phrases,forbidden_phrases=excluded.forbidden_phrases,required_tools=excluded.required_tools,updated_at=excluded.updated_at`, item.ID, item.Name, item.Description, item.Task, string(mcpIDs), item.SkillMCPServerID, item.SkillID, item.Repetitions, item.Temperature, item.MaxTokens, string(requiredPhrases), string(forbiddenPhrases), string(requiredTools), item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM benchmark_targets WHERE case_id=?`, item.ID); err != nil {
		return err
	}
	for position := range item.Targets {
		target := &item.Targets[position]
		if target.ID == "" {
			target.ID = uuid.NewString()
		}
		target.CaseID, target.Position = item.ID, position
		if _, err := tx.ExecContext(ctx, `INSERT INTO benchmark_targets(id,case_id,provider_id,model,position) VALUES(?,?,?,?,?)`, target.ID, target.CaseID, target.ProviderID, target.Model, target.Position); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetBenchmarkCase(ctx context.Context, id string) (BenchmarkCase, error) {
	var item BenchmarkCase
	var mcpIDs, requiredPhrases, forbiddenPhrases, requiredTools string
	var temperature sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `SELECT id,name,description,task,mcp_server_ids,skill_mcp_server_id,skill_id,repetitions,temperature,max_tokens,required_phrases,forbidden_phrases,required_tools,created_at,updated_at FROM benchmark_cases WHERE id=?`, id).Scan(&item.ID, &item.Name, &item.Description, &item.Task, &mcpIDs, &item.SkillMCPServerID, &item.SkillID, &item.Repetitions, &temperature, &item.MaxTokens, &requiredPhrases, &forbiddenPhrases, &requiredTools, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	decodeStrings(mcpIDs, &item.MCPServerIDs)
	decodeStrings(requiredPhrases, &item.RequiredPhrases)
	decodeStrings(forbiddenPhrases, &item.ForbiddenPhrases)
	decodeStrings(requiredTools, &item.RequiredTools)
	if temperature.Valid {
		item.Temperature = &temperature.Float64
	}
	targets, err := s.listBenchmarkTargets(ctx, item.ID)
	item.Targets = targets
	return item, err
}

func (s *Store) ListBenchmarkCases(ctx context.Context) ([]BenchmarkCase, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM benchmark_cases ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]BenchmarkCase, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetBenchmarkCase(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Store) listBenchmarkTargets(ctx context.Context, caseID string) ([]BenchmarkTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,case_id,provider_id,model,position FROM benchmark_targets WHERE case_id=? ORDER BY position`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []BenchmarkTarget{}
	for rows.Next() {
		var item BenchmarkTarget
		if err := rows.Scan(&item.ID, &item.CaseID, &item.ProviderID, &item.Model, &item.Position); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) CreateBenchmarkRun(ctx context.Context, item *BenchmarkRun) error {
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.Status == "" {
		item.Status = "queued"
	}
	item.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO benchmark_runs(id,case_id,status,total_trials,completed_trials,error,started_at,finished_at,case_snapshot,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, item.ID, item.CaseID, item.Status, item.TotalTrials, item.CompletedTrials, nullable(item.Error), nullable(item.StartedAt), nullable(item.FinishedAt), item.CaseSnapshot, item.CreatedAt)
	return err
}

func (s *Store) UpdateBenchmarkRun(ctx context.Context, item BenchmarkRun) error {
	_, err := s.db.ExecContext(ctx, `UPDATE benchmark_runs SET status=?,completed_trials=?,error=?,started_at=?,finished_at=? WHERE id=?`, item.Status, item.CompletedTrials, nullable(item.Error), nullable(item.StartedAt), nullable(item.FinishedAt), item.ID)
	return err
}

func (s *Store) AddBenchmarkTrial(ctx context.Context, item *BenchmarkTrial) error {
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	trajectory, _ := json.Marshal(item.Trajectory)
	checks, _ := json.Marshal(item.ScoreChecks)
	_, err := s.db.ExecContext(ctx, `INSERT INTO benchmark_trials(id,run_id,provider_id,model,variant,repetition,status,response,error,duration_ms,prepare_duration_ms,input_tokens,output_tokens,tool_calls_count,trajectory,quality_score,passed,score_checks,skill_id,skill_version,started_at,finished_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.RunID, item.ProviderID, item.Model, item.Variant, item.Repetition, item.Status, item.Response, nullable(item.Error), item.DurationMS, item.PrepareDurationMS, item.InputTokens, item.OutputTokens, item.ToolCallsCount, string(trajectory), item.QualityScore, item.Passed, string(checks), nullable(item.SkillID), item.SkillVersion, item.StartedAt, item.FinishedAt)
	return err
}

func (s *Store) GetBenchmarkRun(ctx context.Context, id string) (BenchmarkRun, error) {
	var item BenchmarkRun
	var runError, started, finished sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,case_id,status,total_trials,completed_trials,error,started_at,finished_at,case_snapshot,created_at FROM benchmark_runs WHERE id=?`, id).Scan(&item.ID, &item.CaseID, &item.Status, &item.TotalTrials, &item.CompletedTrials, &runError, &started, &finished, &item.CaseSnapshot, &item.CreatedAt)
	if err != nil {
		return item, err
	}
	item.Error, item.StartedAt, item.FinishedAt = runError.String, started.String, finished.String
	trials, err := s.listBenchmarkTrials(ctx, id)
	item.Trials = trials
	return item, err
}

func (s *Store) ListBenchmarkRuns(ctx context.Context, caseID string, limit int) ([]BenchmarkRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query, args := `SELECT id FROM benchmark_runs`, []any{}
	if caseID != "" {
		query, args = query+` WHERE case_id=?`, append(args, caseID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	result := make([]BenchmarkRun, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetBenchmarkRun(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Store) listBenchmarkTrials(ctx context.Context, runID string) ([]BenchmarkTrial, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,run_id,provider_id,model,variant,repetition,status,response,error,duration_ms,prepare_duration_ms,input_tokens,output_tokens,tool_calls_count,trajectory,quality_score,passed,score_checks,skill_id,skill_version,started_at,finished_at FROM benchmark_trials WHERE run_id=? ORDER BY provider_id,model,repetition,variant`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []BenchmarkTrial{}
	for rows.Next() {
		var item BenchmarkTrial
		var trialError, skillID sql.NullString
		var skillVersion sql.NullInt64
		var trajectory, checks string
		if err := rows.Scan(&item.ID, &item.RunID, &item.ProviderID, &item.Model, &item.Variant, &item.Repetition, &item.Status, &item.Response, &trialError, &item.DurationMS, &item.PrepareDurationMS, &item.InputTokens, &item.OutputTokens, &item.ToolCallsCount, &trajectory, &item.QualityScore, &item.Passed, &checks, &skillID, &skillVersion, &item.StartedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		item.Error, item.SkillID = trialError.String, skillID.String
		if skillVersion.Valid {
			version := int(skillVersion.Int64)
			item.SkillVersion = &version
		}
		_ = json.Unmarshal([]byte(trajectory), &item.Trajectory)
		_ = json.Unmarshal([]byte(checks), &item.ScoreChecks)
		result = append(result, item)
	}
	return result, rows.Err()
}

func decodeStrings(raw string, target *[]string) {
	*target = []string{}
	_ = json.Unmarshal([]byte(raw), target)
}
