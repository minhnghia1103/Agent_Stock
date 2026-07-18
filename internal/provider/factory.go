package provider

import (
	"fmt"
	"strings"
	"time"
)

// Known provider names (Phase 3).
const (
	NameLLMGate    = "llmgate"
	NameOpenAI     = "openai"
	NameDashScope  = "dashscope"
	NameOpenRouter = "openrouter"
	NameAnthropic  = "anthropic"
	NameCodex      = "codex"
	NameACP        = "acp"
	NameClaudeCLI  = "claude_cli"
)

// PresetBaseURL returns the default API base for a known provider name.
func PresetBaseURL(name string) string {
	switch strings.ToLower(name) {
	case NameLLMGate:
		return "https://llmgate.app/v1"
	case NameOpenAI:
		return "https://api.openai.com/v1"
	case NameDashScope:
		return "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	case NameOpenRouter:
		return "https://openrouter.ai/api/v1"
	case NameAnthropic:
		return "https://api.anthropic.com"
	default:
		return ""
	}
}

// PresetDefaultModel returns a sensible default model id for presets.
func PresetDefaultModel(name string) string {
	switch strings.ToLower(name) {
	case NameLLMGate:
		return "gpt-oss-120b"
	case NameOpenAI:
		return "gpt-4o-mini"
	case NameDashScope:
		return "qwen-plus"
	case NameOpenRouter:
		return "openai/gpt-4o-mini"
	case NameAnthropic:
		return "claude-3-5-haiku-latest"
	default:
		return ""
	}
}

// BuildConfig is used by the composition root to construct the active Provider.
type BuildConfig struct {
	Name         string
	APIKey       string
	BaseURL      string
	DefaultModel string
	Timeout      time.Duration
	MaxRetries   int
}

// NewFromConfig builds a Provider from config (Strategy factory).
func NewFromConfig(cfg BuildConfig) (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Name))
	if name == "" {
		name = NameLLMGate
	}

	switch name {
	case NameCodex, NameACP, NameClaudeCLI:
		return nil, fmt.Errorf("provider %q is deferred (needs OAuth/subprocess); use llmgate/openai/dashscope/openrouter/anthropic for Phase 3", name)
	}

	base := cfg.BaseURL
	if base == "" {
		base = PresetBaseURL(name)
	}
	model := cfg.DefaultModel
	if model == "" {
		model = PresetDefaultModel(name)
	}
	if cfg.APIKey == "" && name != NameOpenAI { // still require key for real calls
		// All HTTP providers need a key in practice; allow empty only for local proxies.
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("AGENT_LLM_API_KEY is required for provider %q", name)
	}
	if base == "" {
		return nil, fmt.Errorf("AGENT_LLM_BASE_URL is required for provider %q", name)
	}
	if model == "" {
		return nil, fmt.Errorf("AGENT_LLM_MODEL is required for provider %q", name)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	retry := DefaultRetryConfig()
	if cfg.MaxRetries > 0 {
		retry.MaxAttempts = cfg.MaxRetries
	}

	switch name {
	case NameAnthropic:
		return NewAnthropic(AnthropicConfig{
			Name:         name,
			APIKey:       cfg.APIKey,
			BaseURL:      base,
			DefaultModel: model,
			Timeout:      timeout,
			Retry:        retry,
		}), nil
	default:
		// llmgate, openai, dashscope, openrouter, or any custom openai-compat name
		return NewOpenAICompat(OpenAICompatConfig{
			Name:         name,
			APIKey:       cfg.APIKey,
			BaseURL:      base,
			DefaultModel: model,
			Timeout:      timeout,
			Retry:        retry,
		}), nil
	}
}

// Registry maps provider name → Provider (optional multi-provider later).
type Registry struct {
	providers map[string]Provider
	default_  string
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
	if r.default_ == "" {
		r.default_ = p.Name()
	}
}

func (r *Registry) SetDefault(name string) error {
	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider %q not registered", name)
	}
	r.default_ = name
	return nil
}

func (r *Registry) Get(name string) (Provider, error) {
	if name == "" {
		name = r.default_
	}
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

func (r *Registry) Default() (Provider, error) {
	return r.Get(r.default_)
}
