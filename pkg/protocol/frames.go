package protocol

import "encoding/json"

// Frame is a WebSocket RPC envelope (req / res / event).
type Frame struct {
	Type    string          `json:"type"`              // req | res | event
	ID      string          `json:"id,omitempty"`      // correlates req/res
	Method  string          `json:"method,omitempty"`  // for type=req
	Params  json.RawMessage `json:"params,omitempty"`  // for type=req
	OK      *bool           `json:"ok,omitempty"`      // for type=res
	Payload any             `json:"payload,omitempty"` // for type=res
	Error   string          `json:"error,omitempty"`   // for type=res failure
	Event   string          `json:"event,omitempty"`   // for type=event (progress type)
}

const (
	TypeReq   = "req"
	TypeRes   = "res"
	TypeEvent = "event"

	MethodConnect  = "connect"
	MethodChatSend = "chat.send"
	MethodPing     = "ping"
)

func Bool(v bool) *bool { return &v }

func ResOK(id string, payload any) Frame {
	return Frame{Type: TypeRes, ID: id, OK: Bool(true), Payload: payload}
}

func ResErr(id, message string) Frame {
	return Frame{Type: TypeRes, ID: id, OK: Bool(false), Error: message}
}

func EventFrame(event string, payload any) Frame {
	return Frame{Type: TypeEvent, Event: event, Payload: payload}
}
