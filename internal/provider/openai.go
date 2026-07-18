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

	out := &ChatResponse{
		Content:      parsed.Choices[0].Message.Content,
		FinishReason: parsed.Choices[0].FinishReason,
		Model:        firstNonEmpty(parsed.Model, model),
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
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func toOpenAIMessages(msgs []Message) []map[string]string {
	out := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, map[string]string{
			"role":    m.Role,
			"content": m.Content,
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
