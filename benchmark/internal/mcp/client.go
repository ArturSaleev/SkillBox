package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
)

type Client struct {
	config    benchmarkconfig.MCPServer
	http      *http.Client
	sessionID string
	nextID    atomic.Int64
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

type CallResult struct {
	Content           []Content `json:"content"`
	IsError           bool      `json:"isError,omitempty"`
	StructuredContent any       `json:"structuredContent,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

func New(cfg benchmarkconfig.MCPServer) *Client {
	client := &Client{config: cfg, http: &http.Client{Timeout: benchmarkconfig.MCPTimeout(cfg.RequestTimeout)}}
	client.nextID.Store(1)
	return client
}

func (c *Client) Initialize(ctx context.Context) error {
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "skillbox-bench", "version": "0.1.0"},
	}, &result)
	if err != nil {
		return err
	}
	return c.notify(ctx, "notifications/initialized", map[string]any{})
}

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.call(ctx, "tools/list", map[string]any{}, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (CallResult, error) {
	var result CallResult
	var args any = map[string]any{}
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return result, fmt.Errorf("decode tool arguments: %w", err)
		}
	}
	if err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	id := c.nextID.Add(1)
	response, err := c.request(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return err
	}
	if response.Error != nil {
		return fmt.Errorf("MCP %s failed (%d): %s", method, response.Error.Code, response.Error.Message)
	}
	if result != nil && len(response.Result) > 0 {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("decode MCP %s result: %w", method, err)
		}
	}
	return nil
}

func (c *Client) notify(ctx context.Context, method string, params any) error {
	_, err := c.request(ctx, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	return err
}

func (c *Client) request(ctx context.Context, payload any) (rpcResponse, error) {
	var result rpcResponse
	raw, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(raw))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	if c.config.AuthHeader != "" {
		if value := c.config.ResolvedAuthValue(); value != "" {
			req.Header.Set(c.config.AuthHeader, value)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.sessionID = sessionID
	}
	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent {
		return result, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return result, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 1000 {
			message = message[:1000]
		}
		return result, fmt.Errorf("MCP returned HTTP %d: %s", resp.StatusCode, message)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		body = lastSSEData(body)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("decode MCP response: %w", err)
	}
	return result, nil
}

func lastSSEData(body []byte) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var result []byte
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			result = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return result
}

func Text(result CallResult) string {
	if result.StructuredContent != nil {
		raw, _ := json.Marshal(result.StructuredContent)
		return string(raw)
	}
	var parts []string
	for _, item := range result.Content {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	if len(parts) == 0 {
		if result.IsError {
			return `{"error":"MCP tool returned an error"}`
		}
		return "{}"
	}
	return strings.Join(parts, "\n")
}

func ValidateTools(tools []Tool) error {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			return errors.New("MCP server returned a tool without a name")
		}
	}
	return nil
}
