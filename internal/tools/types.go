package tools

import (
	"context"
	"encoding/json"
	"strings"
)

// Tool is the Command interface for agent-callable functions.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any // JSON Schema
	Execute(ctx context.Context, args map[string]any) Result
}

// Result is the outcome of a tool execution.
type Result struct {
	Content string
	IsError bool
}

func OK(content string) Result { return Result{Content: content} }

func Err(msg string) Result { return Result{Content: msg, IsError: true} }

// ParseArgs unmarshals raw JSON arguments into a map.
func ParseArgs(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	return args, nil
}

// StringArg reads a string argument.
func StringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
