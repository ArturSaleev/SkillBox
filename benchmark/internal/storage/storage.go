package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Session struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	ProviderID   string   `json:"provider_id"`
	Model        string   `json:"model"`
	MCPServerIDs []string `json:"mcp_server_ids"`
	Mode         string   `json:"mode"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type Message struct {
	ID             string     `json:"id"`
	SessionID      string     `json:"session_id"`
	Role           string     `json:"role"`
	Content        string     `json:"content"`
	ProviderID     string     `json:"provider_id,omitempty"`
	Model          string     `json:"model,omitempty"`
	DurationMS     *int64     `json:"duration_ms,omitempty"`
	InputTokens    *int       `json:"input_tokens,omitempty"`
	OutputTokens   *int       `json:"output_tokens,omitempty"`
	ToolCallsCount int        `json:"tool_calls_count"`
	ToolCalls      []ToolCall `json:"tool_calls,omitempty"`
	CreatedAt      string     `json:"created_at"`
}

type ToolCall struct {
	ID         string `json:"id"`
	MessageID  string `json:"message_id"`
	ServerID   string `json:"server_id"`
	ToolName   string `json:"tool_name"`
	Arguments  string `json:"arguments"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	CreatedAt  string `json:"created_at"`
}

func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) initialize(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS chat_sessions (
            id TEXT PRIMARY KEY,
            title TEXT NOT NULL,
            provider_id TEXT NOT NULL,
            model TEXT NOT NULL,
            mcp_server_ids TEXT NOT NULL DEFAULT '[]',
            mode TEXT NOT NULL DEFAULT 'no_skill',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_updated ON chat_sessions(updated_at DESC)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
            id TEXT PRIMARY KEY,
            session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
            role TEXT NOT NULL,
            content TEXT NOT NULL,
            provider_id TEXT NULL,
            model TEXT NULL,
            duration_ms INTEGER NULL,
            input_tokens INTEGER NULL,
            output_tokens INTEGER NULL,
            tool_calls_count INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_session ON chat_messages(session_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS chat_tool_calls (
            id TEXT PRIMARY KEY,
            message_id TEXT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
            server_id TEXT NOT NULL,
            tool_name TEXT NOT NULL,
            arguments TEXT NOT NULL,
            result TEXT NULL,
            error TEXT NULL,
            duration_ms INTEGER NOT NULL,
            created_at TEXT NOT NULL
        )`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, datetime('now'))`,
		`CREATE TABLE IF NOT EXISTS benchmark_cases (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            description TEXT NOT NULL DEFAULT '',
            task TEXT NOT NULL,
            mcp_server_ids TEXT NOT NULL DEFAULT '[]',
            skill_mcp_server_id TEXT NOT NULL,
            skill_id TEXT NOT NULL,
            repetitions INTEGER NOT NULL DEFAULT 3,
            temperature REAL NULL,
            max_tokens INTEGER NOT NULL DEFAULT 2048,
            required_phrases TEXT NOT NULL DEFAULT '[]',
            forbidden_phrases TEXT NOT NULL DEFAULT '[]',
            required_tools TEXT NOT NULL DEFAULT '[]',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS benchmark_targets (
            id TEXT PRIMARY KEY,
            case_id TEXT NOT NULL REFERENCES benchmark_cases(id) ON DELETE CASCADE,
            provider_id TEXT NOT NULL,
            model TEXT NOT NULL,
            position INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_benchmark_targets_case ON benchmark_targets(case_id, position)`,
		`CREATE TABLE IF NOT EXISTS benchmark_runs (
            id TEXT PRIMARY KEY,
            case_id TEXT NOT NULL REFERENCES benchmark_cases(id) ON DELETE RESTRICT,
            status TEXT NOT NULL,
            total_trials INTEGER NOT NULL,
            completed_trials INTEGER NOT NULL DEFAULT 0,
            error TEXT NULL,
            started_at TEXT NULL,
            finished_at TEXT NULL,
            case_snapshot TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_benchmark_runs_case ON benchmark_runs(case_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS benchmark_trials (
            id TEXT PRIMARY KEY,
            run_id TEXT NOT NULL REFERENCES benchmark_runs(id) ON DELETE CASCADE,
            provider_id TEXT NOT NULL,
            model TEXT NOT NULL,
            variant TEXT NOT NULL,
            repetition INTEGER NOT NULL,
            status TEXT NOT NULL,
            response TEXT NOT NULL DEFAULT '',
            error TEXT NULL,
            duration_ms INTEGER NOT NULL DEFAULT 0,
            prepare_duration_ms INTEGER NOT NULL DEFAULT 0,
            input_tokens INTEGER NOT NULL DEFAULT 0,
            output_tokens INTEGER NOT NULL DEFAULT 0,
            tool_calls_count INTEGER NOT NULL DEFAULT 0,
            trajectory TEXT NOT NULL DEFAULT '[]',
            quality_score REAL NOT NULL DEFAULT 0,
            passed INTEGER NOT NULL DEFAULT 0,
            score_checks TEXT NOT NULL DEFAULT '[]',
            skill_id TEXT NULL,
            skill_version INTEGER NULL,
            started_at TEXT NOT NULL,
            finished_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_benchmark_trials_run ON benchmark_trials(run_id, provider_id, model, variant, repetition)`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(2, datetime('now'))`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize benchmark database: %w", err)
		}
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, item *Session) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.Title == "" {
		item.Title = "New conversation"
	}
	item.CreatedAt, item.UpdatedAt = now, now
	servers, _ := json.Marshal(item.MCPServerIDs)
	_, err := s.db.ExecContext(ctx, `INSERT INTO chat_sessions(id,title,provider_id,model,mcp_server_ids,mode,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, item.ID, item.Title, item.ProviderID, item.Model, string(servers), item.Mode, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *Store) UpdateSession(ctx context.Context, item Session) error {
	item.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	servers, _ := json.Marshal(item.MCPServerIDs)
	result, err := s.db.ExecContext(ctx, `UPDATE chat_sessions SET title=?,provider_id=?,model=?,mcp_server_ids=?,mode=?,updated_at=? WHERE id=?`, item.Title, item.ProviderID, item.Model, string(servers), item.Mode, item.UpdatedAt, item.ID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	var item Session
	var servers string
	err := s.db.QueryRowContext(ctx, `SELECT id,title,provider_id,model,mcp_server_ids,mode,created_at,updated_at FROM chat_sessions WHERE id=?`, id).Scan(&item.ID, &item.Title, &item.ProviderID, &item.Model, &servers, &item.Mode, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	_ = json.Unmarshal([]byte(servers), &item.MCPServerIDs)
	return item, nil
}

func (s *Store) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,provider_id,model,mcp_server_ids,mode,created_at,updated_at FROM chat_sessions ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Session{}
	for rows.Next() {
		var item Session
		var servers string
		if err := rows.Scan(&item.ID, &item.Title, &item.ProviderID, &item.Model, &servers, &item.Mode, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(servers), &item.MCPServerIDs)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) AddMessage(ctx context.Context, item *Message) error {
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.CreatedAt == "" {
		item.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO chat_messages(id,session_id,role,content,provider_id,model,duration_ms,input_tokens,output_tokens,tool_calls_count,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.SessionID, item.Role, item.Content, nullable(item.ProviderID), nullable(item.Model), item.DurationMS, item.InputTokens, item.OutputTokens, item.ToolCallsCount, item.CreatedAt)
	if err != nil {
		return err
	}
	for i := range item.ToolCalls {
		call := &item.ToolCalls[i]
		if call.ID == "" {
			call.ID = uuid.NewString()
		}
		call.MessageID = item.ID
		if call.CreatedAt == "" {
			call.CreatedAt = item.CreatedAt
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO chat_tool_calls(id,message_id,server_id,tool_name,arguments,result,error,duration_ms,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, call.ID, call.MessageID, call.ServerID, call.ToolName, call.Arguments, nullable(call.Result), nullable(call.Error), call.DurationMS, call.CreatedAt)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE chat_sessions SET updated_at=? WHERE id=?`, item.CreatedAt, item.SessionID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,role,content,provider_id,model,duration_ms,input_tokens,output_tokens,tool_calls_count,created_at FROM chat_messages WHERE session_id=? ORDER BY created_at,id`, sessionID)
	if err != nil {
		return nil, err
	}
	result := []Message{}
	for rows.Next() {
		var item Message
		var providerID, model sql.NullString
		var duration sql.NullInt64
		var inputTokens, outputTokens sql.NullInt64
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Role, &item.Content, &providerID, &model, &duration, &inputTokens, &outputTokens, &item.ToolCallsCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ProviderID = providerID.String
		item.Model = model.String
		item.DurationMS = int64Pointer(duration)
		item.InputTokens = intPointer(inputTokens)
		item.OutputTokens = intPointer(outputTokens)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range result {
		calls, err := s.listToolCalls(ctx, result[i].ID)
		if err != nil {
			return nil, err
		}
		result[i].ToolCalls = calls
	}
	return result, nil
}

func (s *Store) listToolCalls(ctx context.Context, messageID string) ([]ToolCall, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,message_id,server_id,tool_name,arguments,result,error,duration_ms,created_at FROM chat_tool_calls WHERE message_id=? ORDER BY created_at,id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ToolCall{}
	for rows.Next() {
		var item ToolCall
		var callResult, callError sql.NullString
		if err := rows.Scan(&item.ID, &item.MessageID, &item.ServerID, &item.ToolName, &item.Arguments, &callResult, &callError, &item.DurationMS, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Result, item.Error = callResult.String, callError.String
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM chat_sessions WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func int64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func intPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func IsNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
