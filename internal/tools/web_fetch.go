package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type webFetchTool struct {
	client *http.Client
}

func NewWebFetchTool() Tool {
	return &webFetchTool{
		client: &http.Client{Timeout: 15 * time.Second},
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
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return Err("url must be http or https")
	}
	if err := assertPublicHost(u.Hostname()); err != nil {
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

func assertPublicHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("empty host")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("blocked host: %s", host)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup: %w", err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("blocked private/loopback IP for host %s", host)
		}
	}
	return nil
}
