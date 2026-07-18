package provider

import "context"

// Roles for chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is a chat message sent to / returned from an LLM.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is input for Provider.Chat (Phase 3: no tools yet).
type ChatRequest struct {
	Messages []Message
	Model    string
}

// Usage holds token counts when the provider reports them.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is a non-streaming LLM reply.
type ChatResponse struct {
	Content      string
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
