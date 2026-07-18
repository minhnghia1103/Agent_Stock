package progress

// Event types shared by SSE and WebSocket RPC.
const (
	EventRunStarted   = "run.started"
	EventChunk        = "chunk"
	EventToolCall     = "tool.call"
	EventToolResult   = "tool.result"
	EventRunCompleted = "run.completed"
	EventError        = "error"
)

// Event is the unified progress envelope.
type Event struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Emitter receives progress events (Observer).
type Emitter func(Event)

// Emit is a nil-safe helper.
func Emit(em Emitter, typ string, payload map[string]any) {
	if em == nil {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	em(Event{Type: typ, Payload: payload})
}
