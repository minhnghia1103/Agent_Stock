package tools

import "context"

type jailKey struct{}

// WithJailRoot overrides the workspace jail for this request (Phase 10A).
func WithJailRoot(ctx context.Context, root string) context.Context {
	return context.WithValue(ctx, jailKey{}, root)
}

// JailRootFromContext returns per-request jail root if set.
func JailRootFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(jailKey{}).(string); ok {
		return v
	}
	return ""
}
