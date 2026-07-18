package agent

import (
	"context"
	"fmt"
	"log/slog"

	"agent_stock/internal/progress"
	"agent_stock/internal/provider"
	"agent_stock/internal/tools"
)

// Loop runs a ReAct-style think→act→observe cycle with tools.
type Loop struct {
	LLM           provider.Provider
	Tools         *tools.Registry
	MaxIterations int
	OnEvent       progress.Emitter
}

// RunResult is the final assistant answer after the loop.
type RunResult struct {
	Content      string
	Model        string
	Usage        *provider.Usage
	Iterations   int
	ToolCallsRan int
}

// Run executes the agent loop over the given messages (already includes system/history/user).
func (l *Loop) Run(ctx context.Context, msgs []provider.Message) (*RunResult, error) {
	if l.LLM == nil {
		return nil, fmt.Errorf("llm provider not configured")
	}
	maxIter := l.MaxIterations
	if maxIter <= 0 {
		maxIter = 8
	}

	var toolDefs []provider.ToolDefinition
	if l.Tools != nil && provider.SupportsTools(l.LLM) {
		toolDefs = l.Tools.Definitions()
	}

	working := append([]provider.Message(nil), msgs...)
	var lastUsage *provider.Usage
	var model string
	toolCallsRan := 0

	for i := 0; i < maxIter; i++ {
		resp, err := l.chatOnce(ctx, provider.ChatRequest{
			Messages: working,
			Tools:    toolDefs,
		})
		if err != nil {
			progress.Emit(l.OnEvent, progress.EventError, map[string]any{"message": err.Error()})
			return nil, fmt.Errorf("llm chat (iter %d): %w", i+1, err)
		}
		model = resp.Model
		lastUsage = mergeUsage(lastUsage, resp.Usage)

		if len(resp.ToolCalls) == 0 {
			if resp.Content == "" {
				err := fmt.Errorf("llm returned empty content")
				progress.Emit(l.OnEvent, progress.EventError, map[string]any{"message": err.Error()})
				return nil, err
			}
			return &RunResult{
				Content:      resp.Content,
				Model:        model,
				Usage:        lastUsage,
				Iterations:   i + 1,
				ToolCallsRan: toolCallsRan,
			}, nil
		}

		if l.Tools == nil {
			return nil, fmt.Errorf("model requested tools but registry is nil")
		}

		working = append(working, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			slog.Info("tool call",
				"iteration", i+1,
				"tool", tc.Name,
				"id", tc.ID,
			)
			progress.Emit(l.OnEvent, progress.EventToolCall, map[string]any{
				"id":        tc.ID,
				"name":      tc.Name,
				"arguments": tc.Arguments,
			})
			result := l.Tools.Execute(ctx, tc.Name, tc.Arguments)
			toolCallsRan++
			content := result.Content
			if result.IsError {
				content = "ERROR: " + content
			}
			progress.Emit(l.OnEvent, progress.EventToolResult, map[string]any{
				"id":       tc.ID,
				"name":     tc.Name,
				"content":  truncate(content, 2000),
				"is_error": result.IsError,
			})
			working = append(working, provider.Message{
				Role:       provider.RoleTool,
				Content:    content,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}
	}

	err := fmt.Errorf("max tool iterations reached (%d)", maxIter)
	progress.Emit(l.OnEvent, progress.EventError, map[string]any{"message": err.Error()})
	return nil, err
}

func (l *Loop) chatOnce(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	if streamer, ok := provider.AsStreamer(l.LLM); ok {
		return streamer.ChatStream(ctx, req, func(c provider.StreamChunk) {
			if c.Content == "" {
				return
			}
			progress.Emit(l.OnEvent, progress.EventChunk, map[string]any{"text": c.Content})
		})
	}
	resp, err := l.LLM.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Content != "" && len(resp.ToolCalls) == 0 {
		progress.Emit(l.OnEvent, progress.EventChunk, map[string]any{"text": resp.Content})
	}
	return resp, nil
}

func mergeUsage(a, b *provider.Usage) *provider.Usage {
	if a == nil && b == nil {
		return nil
	}
	out := &provider.Usage{}
	if a != nil {
		out.PromptTokens += a.PromptTokens
		out.CompletionTokens += a.CompletionTokens
		out.TotalTokens += a.TotalTokens
	}
	if b != nil {
		out.PromptTokens += b.PromptTokens
		out.CompletionTokens += b.CompletionTokens
		out.TotalTokens += b.TotalTokens
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
