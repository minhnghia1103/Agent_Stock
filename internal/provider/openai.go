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

// OpenAICompat talks to OpenAI-compatible Chat Completions APIs
// (OpenAI, LLMGate, DashScope compatible-mode, OpenRouter, vLLM, …).
type OpenAICompat struct {
	name         string
	apiKey       string
	apiBase      string
	defaultModel string
	client       *http.Client
	retry        RetryConfig
}

// OpenAICompatConfig configures an OpenAI-compatible provider.
type OpenAICompatConfig struct {
	Name         string
	APIKey       string
	BaseURL      string
	DefaultModel string
	Timeout      time.Duration
	Retry        RetryConfig
}

// NewOpenAICompat creates an OpenAI-compatible provider.
func NewOpenAICompat(cfg OpenAICompatConfig) *OpenAICompat {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry = DefaultRetryConfig()
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	return &OpenAICompat{
		name:         cfg.Name,
		apiKey:       cfg.APIKey,
		apiBase:      base,
		defaultModel: cfg.DefaultModel,
		client:       &http.Client{Timeout: cfg.Timeout},
		retry:        cfg.Retry,
	}
}

func (p *OpenAICompat) Name() string         { return p.name }
func (p *OpenAICompat) DefaultModel() string { return p.defaultModel }
func (p *OpenAICompat) SupportsTools() bool  { return true }

func (p *OpenAICompat) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("%s: model is required", p.name)
	}

	body := map[string]any{
		"model":    model,
		"messages": toOpenAIMessages(req.Messages),
	}
	if len(req.Tools) > 0 {
		body["tools"] = toOpenAITools(req.Tools)
	}

	return RetryDo(ctx, p.retry, func() (*ChatResponse, error) {
		return p.doChat(ctx, body, model)
	})
}

func (p *OpenAICompat) doChat(ctx context.Context, body map[string]any, model string) (*ChatResponse, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal: %w", p.name, err)
	}

	url := p.apiBase + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: new request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

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

	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", p.name, err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("%s: empty choices", p.name)
	}

	choice := parsed.Choices[0]
	out := &ChatResponse{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Model:        firstNonEmpty(parsed.Model, model),
	}
	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	if parsed.Usage != nil {
		out.Usage = &Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		}
	}
	return out, nil
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func toOpenAIMessages(msgs []Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		item := map[string]any{
			"role": m.Role,
		}
		switch m.Role {
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				calls := make([]map[string]any, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					calls = append(calls, map[string]any{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]any{
							"name":      tc.Name,
							"arguments": tc.Arguments,
						},
					})
				}
				item["tool_calls"] = calls
				if m.Content != "" {
					item["content"] = m.Content
				} else {
					item["content"] = nil
				}
			} else {
				item["content"] = m.Content
			}
		case RoleTool:
			item["content"] = m.Content
			item["tool_call_id"] = m.ToolCallID
			if m.Name != "" {
				item["name"] = m.Name
			}
		default:
			item["content"] = m.Content
		}
		out = append(out, item)
	}
	return out
}

func toOpenAITools(defs []ToolDefinition) []map[string]any {
	out := make([]map[string]any, 0, len(defs))
	for _, d := range defs {
		params := d.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        d.Name,
				"description": d.Description,
				"parameters":  params,
			},
		})
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
