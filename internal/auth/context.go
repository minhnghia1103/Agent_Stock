package auth

import "context"

type ctxKey int

const identityKey ctxKey = 1

// WithIdentity attaches the authenticated identity to ctx.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// FromContext returns identity if present.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// MustFromContext returns identity or zero value.
func MustFromContext(ctx context.Context) Identity {
	id, _ := FromContext(ctx)
	return id
}
