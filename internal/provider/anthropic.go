package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Anthropic talks to Anthropic Messages API (not OpenAI-compat).
type Anthropic struct {
	name         string
	apiKey       string
	apiBase      string
	defaultModel string
	client       *http.Client
	retry        RetryConfig
}

// AnthropicConfig configures the Anthropic provider.
type AnthropicConfig struct {
	Name         string
	APIKey       string
	BaseURL      string
	DefaultModel string
	Timeout      time.Duration
	Retry        RetryConfig
}

func NewAnthropic(cfg AnthropicConfig) *Anthropic {
	if cfg.Name == "" {
		cfg.Name = "anthropic"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry = DefaultRetryConfig()
	}
	return &Anthropic{
		name:         cfg.Name,
		apiKey:       cfg.APIKey,
		apiBase:      strings.TrimRight(cfg.BaseURL, "/"),
		defaultModel: cfg.DefaultModel,
		client:       &http.Client{Timeout: cfg.Timeout},
		retry:        cfg.Retry,
	}
}

func (p *Anthropic) Name() string         { return p.name }
func (p *Anthropic) DefaultModel() string { return p.defaultModel }

func (p *Anthropic) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("%s: model is required", p.name)
	}

	system, messages := splitSystem(req.Messages)
	body := map[string]any{
		"model":      model,
		"max_tokens": 4096,
		"messages":   toAnthropicMessages(messages),
	}
	if system != "" {
		body["system"] = system
	}

	return RetryDo(ctx, p.retry, func() (*ChatResponse, error) {
		return p.doChat(ctx, body, model)
	})
}

func (p *Anthropic) doChat(ctx context.Context, body map[string]any, model string) (*ChatResponse, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal: %w", p.name, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBase+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: new request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: request: %w", p.name, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%s: read body: %w", p.name, err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, &HTTPError{Provider: p.name, StatusCode: httpResp.StatusCode, Body: string(respBody)}
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", p.name, err)
	}

	var content strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	out := &ChatResponse{
		Content:      content.String(),
		FinishReason: parsed.StopReason,
		Model:        firstNonEmpty(parsed.Model, model),
	}
	if parsed.Usage != nil {
		out.Usage = &Usage{
			PromptTokens:     parsed.Usage.InputTokens,
			CompletionTokens: parsed.Usage.OutputTokens,
			TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
		}
	}
	return out, nil
}

type anthropicResponse struct {
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func splitSystem(msgs []Message) (system string, rest []Message) {
	rest = make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == RoleSystem {
			if system != "" {
				system += "\n"
			}
			system += m.Content
			continue
		}
		rest = append(rest, m)
	}
	return system, rest
}

func toAnthropicMessages(msgs []Message) []map[string]string {
	out := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		role := m.Role
		if role != RoleUser && role != RoleAssistant {
			role = RoleUser
		}
		out = append(out, map[string]string{
			"role":    role,
			"content": m.Content,
		})
	}
	return out
}
