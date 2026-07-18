package provider

import (
	"context"
	"errors"
	"time"
)

// RetryConfig controls simple retry with linear backoff.
type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{MaxAttempts: 3, Backoff: 300 * time.Millisecond}
}

// RetryDo runs fn up to MaxAttempts times on retryable errors.
func RetryDo[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var zero T
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	var last error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		v, err := fn()
		if err == nil {
			return v, nil
		}
		last = err
		if !isRetryable(err) || attempt == cfg.MaxAttempts {
			return zero, err
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(cfg.Backoff * time.Duration(attempt)):
		}
	}
	return zero, last
}

func isRetryable(err error) bool {
	var re *HTTPError
	if errors.As(err, &re) {
		return re.StatusCode == 429 || re.StatusCode >= 500
	}
	return false
}

// HTTPError is a non-2xx response from an LLM HTTP API.
type HTTPError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return e.Provider + ": http " + itoa(e.StatusCode) + ": " + truncate(e.Body, 300)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
