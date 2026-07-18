package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"agent_stock/internal/security"
)

type webFetchTool struct {
	client *http.Client
	policy *security.Policy
}

func NewWebFetchTool(pol *security.Policy) Tool {
	return &webFetchTool{
		client: &http.Client{Timeout: 15 * time.Second},
		policy: pol,
	}
}

func (t *webFetchTool) Name() string { return "web_fetch" }
func (t *webFetchTool) Description() string {
	return "Fetch a public http(s) URL and return text body (truncated). Blocks private/loopback hosts."
}
func (t *webFetchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "http or https URL"},
		},
		"required": []string{"url"},
	}
}

func (t *webFetchTool) Execute(ctx context.Context, args map[string]any) Result {
	raw := StringArg(args, "url")
	u, err := url.Parse(raw)
	if err != nil || u.String() == "" {
		return Err("url must be http or https")
	}
	if err := security.AssertPublicURL(t.policy, raw); err != nil {
		return Err(err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Err(err.Error())
	}
	req.Header.Set("User-Agent", "agent_stock/0.1")

	resp, err := t.client.Do(req)
	if err != nil {
		return Err(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 200_000))
	if err != nil {
		return Err(err.Error())
	}
	return OK(fmt.Sprintf("status=%d\n%s", resp.StatusCode, string(body)))
}
