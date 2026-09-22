package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
)

type Client struct {
	config benchmarkconfig.Provider
	http   *http.Client
}

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  any       `json:"tool_choice,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Model struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name,omitempty"`
	ContextLength       int      `json:"context_length,omitempty"`
	SupportedParameters []string `json:"supported_parameters,omitempty"`
	ToolCapable         bool     `json:"tool_capable"`
}

func New(cfg benchmarkconfig.Provider) *Client {
	return &Client{config: cfg, http: &http.Client{Timeout: benchmarkconfig.ProviderTimeout(cfg.RequestTimeout)}}
}

func (c *Client) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	var response ChatResponse
	if strings.TrimSpace(request.Model) == "" {
		return response, errors.New("model is required")
	}
	if len(request.Messages) == 0 {
		return response, errors.New("messages are required")
	}
	if len(request.Tools) > 0 && request.ToolChoice == nil {
		request.ToolChoice = "auto"
	}
	err := c.do(ctx, http.MethodPost, "chat/completions", request, &response)
	if err != nil {
		return response, err
	}
	if len(response.Choices) == 0 {
		return response, errors.New("provider returned no choices")
	}
	return response, nil
}

func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var response struct {
		Data []Model `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "models", nil, &response); err != nil {
		return nil, err
	}
	for i := range response.Data {
		for _, parameter := range response.Data[i].SupportedParameters {
			if parameter == "tools" {
				response.Data[i].ToolCapable = true
				break
			}
		}
	}
	sort.Slice(response.Data, func(i, j int) bool { return response.Data[i].ID < response.Data[j].ID })
	return response.Data, nil
}

func (c *Client) do(ctx context.Context, method, endpoint string, body any, result any) error {
	base, err := url.Parse(strings.TrimRight(c.config.BaseURL, "/") + "/")
	if err != nil {
		return err
	}
	relative, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base.ResolveReference(relative).String(), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := c.config.ResolvedAPIKey(); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError struct {
			Error any `json:"error"`
		}
		_ = json.Unmarshal(raw, &apiError)
		message := strings.TrimSpace(string(raw))
		if apiError.Error != nil {
			encoded, _ := json.Marshal(apiError.Error)
			message = string(encoded)
		}
		if len(message) > 1000 {
			message = message[:1000]
		}
		return fmt.Errorf("provider returned HTTP %d: %s", resp.StatusCode, message)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("decode provider response: %w", err)
	}
	return nil
}
