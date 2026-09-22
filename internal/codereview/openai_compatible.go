package codereview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OpenAICompatible struct {
	endpoint    string
	model       string
	apiKey      string
	client      *http.Client
	destination Destination
}

func NewOpenAICompatible(endpoint, model, apiKey string, client *http.Client) (*OpenAICompatible, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" || model == "" {
		return nil, errors.New("code-review endpoint and model are required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("code-review endpoint must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("code-review endpoint must use HTTP or HTTPS")
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenAICompatible{endpoint: endpoint, model: model, apiKey: apiKey, client: client, destination: Destination{Endpoint: endpoint, External: !isLoopback(parsed.Hostname())}}, nil
}

func (p *OpenAICompatible) Name() string             { return "openai-compatible" }
func (p *OpenAICompatible) Model() string            { return p.model }
func (p *OpenAICompatible) Destination() Destination { return p.destination }

func (p *OpenAICompatible) Review(ctx context.Context, request Request) (Result, error) {
	if p.destination.External && !request.ExternalTransmissionApproved {
		return Result{}, ErrExternalConsentRequired
	}
	payload, err := reviewPayload(p.model, request)
	if err != nil {
		return Result{}, err
	}
	endpoint := p.endpoint
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		if strings.HasSuffix(endpoint, "/v1") {
			endpoint += "/chat/completions"
		} else {
			endpoint += "/v1/chat/completions"
		}
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	response, err := p.client.Do(httpRequest)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return Result{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Result{}, fmt.Errorf("code-review provider returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	content, err := responseContent(body)
	if err != nil {
		return Result{}, err
	}
	result := Result{Provider: p.Name(), Model: p.model, RawResponse: content, Recommendation: "Manual review required. AI review can miss risks and false positives are possible."}
	clean := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(content), "```json"), "```"))
	var structured struct {
		Summary        string    `json:"summary"`
		Findings       []Finding `json:"findings"`
		Recommendation string    `json:"recommendation"`
	}
	if json.Unmarshal([]byte(clean), &structured) == nil {
		result.Summary, result.Findings = structured.Summary, structured.Findings
		if structured.Recommendation != "" {
			result.Recommendation = structured.Recommendation
		}
	} else {
		result.Summary = content
	}
	return result, nil
}

func reviewPayload(model string, request Request) ([]byte, error) {
	code, err := json.Marshal(struct {
		PackageName  string `json:"package_name"`
		Files        []File `json:"files"`
		StaticReport any    `json:"static_report"`
	}{request.PackageName, request.Files, request.StaticReport})
	if err != nil {
		return nil, err
	}
	if len(code) > 2<<20 {
		return nil, errors.New("code-review payload exceeds 2 MiB")
	}
	body := map[string]any{"model": model, "temperature": 0, "messages": []map[string]string{
		{"role": "system", "content": "Review untrusted Skill package code without executing it. Return JSON with summary, findings, and recommendation. Each finding has path, line, severity, category, title, explanation, recommendation. Never claim the package is safe; recommend manual review."},
		{"role": "user", "content": string(code)},
	}}
	return json.Marshal(body)
}

func responseContent(body []byte) (string, error) {
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("decode code-review response: %w", err)
	}
	if len(envelope.Choices) == 0 || strings.TrimSpace(envelope.Choices[0].Message.Content) == "" {
		return "", errors.New("code-review provider returned no content")
	}
	return envelope.Choices[0].Message.Content, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
