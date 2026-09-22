package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/aibox/skillbox/benchmark/internal/agent"
	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
	"github.com/aibox/skillbox/benchmark/internal/provider"
	"github.com/aibox/skillbox/benchmark/internal/storage"
)

type Service struct {
	config *benchmarkconfig.Manager
	store  *storage.Store
	agent  *agent.Runner
}

type SendRequest struct {
	SessionID    string   `json:"session_id,omitempty"`
	ProviderID   string   `json:"provider_id"`
	Model        string   `json:"model"`
	MCPServerIDs []string `json:"mcp_server_ids,omitempty"`
	Mode         string   `json:"mode,omitempty"`
	Message      string   `json:"message"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	Temperature  *float64 `json:"temperature,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
}

type SendResponse struct {
	Session storage.Session `json:"session"`
	Message storage.Message `json:"message"`
}

func NewService(manager *benchmarkconfig.Manager, store *storage.Store) *Service {
	return &Service{config: manager, store: store, agent: agent.New(manager)}
}

func (s *Service) Send(ctx context.Context, request SendRequest) (SendResponse, error) {
	var output SendResponse
	request.Message = strings.TrimSpace(request.Message)
	request.ProviderID = strings.TrimSpace(request.ProviderID)
	request.Model = strings.TrimSpace(request.Model)
	if request.Message == "" || request.ProviderID == "" || request.Model == "" {
		return output, errors.New("provider_id, model and message are required")
	}
	if request.Mode == "" {
		request.Mode = "no_skill"
	}
	session, messages, err := s.session(ctx, request)
	if err != nil {
		return output, err
	}
	userMessage := storage.Message{SessionID: session.ID, Role: "user", Content: request.Message}
	if err := s.store.AddMessage(ctx, &userMessage); err != nil {
		return output, err
	}

	modelMessages := []provider.Message{}
	for _, item := range messages {
		if item.Role == "user" || item.Role == "assistant" {
			modelMessages = append(modelMessages, provider.Message{Role: item.Role, Content: item.Content})
		}
	}
	modelMessages = append(modelMessages, provider.Message{Role: "user", Content: request.Message})
	result, err := s.agent.Run(ctx, agent.Request{ProviderID: request.ProviderID, Model: request.Model, MCPServerIDs: request.MCPServerIDs, Messages: modelMessages, SystemPrompt: request.SystemPrompt, Temperature: request.Temperature, MaxTokens: request.MaxTokens})
	if err != nil {
		return output, err
	}
	duration, inputTokens, outputTokens := result.DurationMS, result.InputTokens, result.OutputTokens
	calls := make([]storage.ToolCall, 0, len(result.ToolCalls))
	for _, call := range result.ToolCalls {
		calls = append(calls, storage.ToolCall{ServerID: call.ServerID, ToolName: call.ToolName, Arguments: call.Arguments, Result: call.Result, Error: call.Error, DurationMS: call.DurationMS})
	}
	assistantMessage := storage.Message{SessionID: session.ID, Role: "assistant", Content: result.Content, ProviderID: request.ProviderID, Model: result.Model, DurationMS: &duration, InputTokens: &inputTokens, OutputTokens: &outputTokens, ToolCallsCount: len(calls), ToolCalls: calls}
	if err := s.store.AddMessage(ctx, &assistantMessage); err != nil {
		return output, err
	}
	output.Session, output.Message = session, assistantMessage
	return output, nil
}

func (s *Service) session(ctx context.Context, request SendRequest) (storage.Session, []storage.Message, error) {
	if request.SessionID == "" {
		title := []rune(request.Message)
		if len(title) > 56 {
			title = title[:56]
		}
		item := storage.Session{Title: string(title), ProviderID: request.ProviderID, Model: request.Model, MCPServerIDs: request.MCPServerIDs, Mode: request.Mode}
		if err := s.store.CreateSession(ctx, &item); err != nil {
			return item, nil, err
		}
		return item, nil, nil
	}
	item, err := s.store.GetSession(ctx, request.SessionID)
	if err != nil {
		return item, nil, err
	}
	item.ProviderID, item.Model, item.MCPServerIDs, item.Mode = request.ProviderID, request.Model, request.MCPServerIDs, request.Mode
	if err := s.store.UpdateSession(ctx, item); err != nil {
		return item, nil, err
	}
	messages, err := s.store.ListMessages(ctx, item.ID)
	return item, messages, err
}
