package provider

import "context"

// Roles for chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is a chat message sent to / returned from an LLM.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // for role=tool
	Name       string     `json:"name,omitempty"`         // tool name (optional)
}

// ToolCall is a model-requested function invocation.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON object string
}

// ToolDefinition is an OpenAI-style tool schema exposed to the model.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// ChatRequest is input for Provider.Chat.
type ChatRequest struct {
	Messages []Message
	Model    string
	Tools    []ToolDefinition
}

// Usage holds token counts when the provider reports them.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is a non-streaming LLM reply (may include tool calls).
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Model        string
	Usage        *Usage
}

// Provider is the Strategy interface for LLM backends.
type Provider interface {
	Name() string
	DefaultModel() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ToolCapable providers can accept Tools in ChatRequest.
type ToolCapable interface {
	Provider
	SupportsTools() bool
}

// SupportsTools reports whether p can run tool-calling loops.
func SupportsTools(p Provider) bool {
	if tc, ok := p.(ToolCapable); ok {
		return tc.SupportsTools()
	}
	return false
}
