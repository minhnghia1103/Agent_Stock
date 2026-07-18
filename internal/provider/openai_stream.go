package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ChatStream streams an OpenAI-compatible chat completion and invokes onChunk for text deltas.
// Tool call arguments are accumulated; the final ChatResponse includes complete ToolCalls.
func (p *OpenAICompat) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) (*ChatResponse, error) {
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
		"stream":   true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
	}
	if len(req.Tools) > 0 {
		body["tools"] = toOpenAITools(req.Tools)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal: %w", p.name, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBase+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: new request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	// Streaming needs a client without a hard Timeout (use context instead).
	client := &http.Client{Timeout: 0}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: request: %w", p.name, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
		return nil, &HTTPError{Provider: p.name, StatusCode: httpResp.StatusCode, Body: string(respBody)}
	}

	return p.readSSE(httpResp.Body, model, onChunk)
}

type streamToolAcc struct {
	id        string
	name      string
	arguments strings.Builder
}

func (p *OpenAICompat) readSSE(r io.Reader, model string, onChunk func(StreamChunk)) (*ChatResponse, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var content strings.Builder
	toolsByIndex := map[int]*streamToolAcc{}
	finishReason := ""
	outModel := model
	var usage *Usage

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Model != "" {
			outModel = chunk.Model
		}
		if chunk.Usage != nil {
			usage = &Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		ch := chunk.Choices[0]
		if ch.FinishReason != "" {
			finishReason = ch.FinishReason
		}
		if ch.Delta.Content != "" {
			content.WriteString(ch.Delta.Content)
			if onChunk != nil {
				onChunk(StreamChunk{Content: ch.Delta.Content})
			}
		}
		for _, tc := range ch.Delta.ToolCalls {
			acc, ok := toolsByIndex[tc.Index]
			if !ok {
				acc = &streamToolAcc{}
				toolsByIndex[tc.Index] = acc
			}
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				acc.arguments.WriteString(tc.Function.Arguments)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: stream read: %w", p.name, err)
	}

	out := &ChatResponse{
		Content:      content.String(),
		FinishReason: finishReason,
		Model:        outModel,
		Usage:        usage,
	}
	maxIdx := -1
	for idx := range toolsByIndex {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	for i := 0; i <= maxIdx; i++ {
		acc, ok := toolsByIndex[i]
		if !ok {
			continue
		}
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: acc.arguments.String(),
		})
	}
	return out, nil
}

type openAIStreamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
