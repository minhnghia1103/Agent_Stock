package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent_stock/internal/security"
)

// stockQuoteTool fetches a lightweight quote via Yahoo chart API (no API key).
type stockQuoteTool struct {
	client *http.Client
	policy *security.Policy
}

func NewStockQuoteTool(pol *security.Policy) Tool {
	return &stockQuoteTool{
		client: &http.Client{Timeout: 12 * time.Second},
		policy: pol,
	}
}

func (t *stockQuoteTool) Name() string { return "get_stock_quote" }
func (t *stockQuoteTool) Description() string {
	return "Get a rough latest price for a stock/ETF symbol (e.g. AAPL, VNM.VN) via public market data."
}
func (t *stockQuoteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"symbol": map[string]any{"type": "string", "description": "Ticker symbol"},
		},
		"required": []string{"symbol"},
	}
}

func (t *stockQuoteTool) Execute(ctx context.Context, args map[string]any) Result {
	symbol := strings.ToUpper(strings.TrimSpace(StringArg(args, "symbol")))
	if symbol == "" {
		return Err("symbol is required")
	}

	endpoint := "https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(symbol) + "?interval=1d&range=1d"
	if err := security.AssertPublicURL(t.policy, endpoint); err != nil {
		return Err(err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Err(err.Error())
	}
	req.Header.Set("User-Agent", "agent_stock/0.1")

	resp, err := t.client.Do(req)
	if err != nil {
		return Err(err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Err(err.Error())
	}
	if resp.StatusCode >= 300 {
		return Err(fmt.Sprintf("quote http %d: %s", resp.StatusCode, truncate(string(body), 200)))
	}

	var parsed yahooChart
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Err("decode quote: " + err.Error())
	}
	if len(parsed.Chart.Result) == 0 {
		return Err("no quote data for symbol " + symbol)
	}
	meta := parsed.Chart.Result[0].Meta
	return OK(fmt.Sprintf(
		"symbol=%s currency=%s price=%v previous_close=%v exchange=%s",
		meta.Symbol, meta.Currency, meta.RegularMarketPrice, meta.ChartPreviousClose, meta.ExchangeName,
	))
}

type yahooChart struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
				ExchangeName       string  `json:"exchangeName"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
