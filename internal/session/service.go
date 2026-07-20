package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/agent"
	"agent_stock/internal/bootstrap"
	"agent_stock/internal/progress"
	"agent_stock/internal/provider"
	"agent_stock/internal/security"
	"agent_stock/internal/skills"
	"agent_stock/internal/slash"
	"agent_stock/internal/store"
	"agent_stock/internal/tools"
)

// Service orchestrates session persistence + agent loop turns.
type Service struct {
	sessions     store.SessionStore
	llm          provider.Provider
	tools        *tools.Registry
	skills       *skills.Registry
	workspaceDir string
	basePrompt   string
	maxIter      int
	policy       *security.Policy
	guard        *security.InputGuard
}

func NewService(
	sessions store.SessionStore,
	llm provider.Provider,
	toolReg *tools.Registry,
	skillsReg *skills.Registry,
	workspaceDir string,
	basePrompt string,
	maxIter int,
	pol *security.Policy,
	guard *security.InputGuard,
) *Service {
	return &Service{
		sessions:     sessions,
		llm:          llm,
		tools:        toolReg,
		skills:       skillsReg,
		workspaceDir: workspaceDir,
		basePrompt:   basePrompt,
		maxIter:      maxIter,
		policy:       pol,
		guard:        guard,
	}
}

// TurnResult is the outcome of one chat turn.
type TurnResult struct {
	SessionID    string
	Reply        string
	MessageCount int
	Model        string
	Usage        *provider.Usage
	Iterations   int
	ToolCalls    int
	Slash        string `json:"slash,omitempty"`
}

// ChatTurn runs a non-streaming chat turn.
func (s *Service) ChatTurn(ctx context.Context, sessionID, message string) (*TurnResult, error) {
	return s.ChatTurnWithEvents(ctx, sessionID, message, nil)
}

// ChatTurnWithEvents runs a chat turn and emits progress events (SSE / WS).
func (s *Service) ChatTurnWithEvents(ctx context.Context, sessionID, message string, emit progress.Emitter) (*TurnResult, error) {
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	if err := s.sessions.EnsureSession(ctx, sessionID); err != nil {
		return nil, err
	}

	originalMessage := message

	// Slash short-circuit before injection guard / LLM (Chain of Responsibility).
	var slashCmd string
	if slash.IsSlash(message) {
		sr := slash.Dispatch(message, s.skills)
		if sr.Handled && sr.Action == slash.ActionDirect {
			progress.Emit(emit, progress.EventRunStarted, map[string]any{"session_id": sessionID, "slash": sr.Command})
			reply := security.Redact(s.policy, sr.Reply)
			if err := s.persistTurn(ctx, sessionID, originalMessage, reply); err != nil {
				return nil, err
			}
			n, err := s.sessions.CountMessages(ctx, sessionID)
			if err != nil {
				return nil, err
			}
			result := &TurnResult{
				SessionID:    sessionID,
				Reply:        reply,
				MessageCount: n,
				Slash:        sr.Command,
			}
			progress.Emit(emit, progress.EventRunCompleted, map[string]any{
				"session_id":    result.SessionID,
				"reply":         result.Reply,
				"message_count": result.MessageCount,
				"slash":         sr.Command,
			})
			slog.Info("slash direct", "session_id", sessionID, "slash", sr.Command)
			return result, nil
		}
		if sr.Handled && sr.Action == slash.ActionContinue {
			message = sr.EffectiveMsg
			slashCmd = sr.Command
			slog.Info("slash continue", "session_id", sessionID, "slash", sr.Command)
		}
	}

	if err := security.CheckUserMessage(s.policy, s.guard, message); err != nil {
		progress.Emit(emit, progress.EventError, map[string]any{"message": err.Error()})
		return nil, err
	}
	if s.llm == nil {
		return nil, fmt.Errorf("llm provider not configured")
	}

	history, err := s.sessions.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	files := bootstrap.Load(s.workspaceDir)
	skillsSection := ""
	if s.skills != nil {
		skillsSection = s.skills.FormatMetadataSection()
	}
	prompt := bootstrap.BuildSystemPromptWithSkills(s.basePrompt, files, skillsSection)

	msgs := make([]provider.Message, 0, len(history)+2)
	msgs = append(msgs, provider.Message{Role: provider.RoleSystem, Content: prompt})
	for _, m := range history {
		msgs = append(msgs, provider.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, provider.Message{Role: provider.RoleUser, Content: message})

	progress.Emit(emit, progress.EventRunStarted, map[string]any{
		"session_id": sessionID,
	})

	loop := &agent.Loop{
		LLM:           s.llm,
		Tools:         s.tools,
		MaxIterations: s.maxIter,
		OnEvent:       emit,
	}
	run, err := loop.Run(ctx, msgs)
	if err != nil {
		return nil, err
	}

	reply := security.Redact(s.policy, run.Content)

	if err := s.persistTurn(ctx, sessionID, originalMessage, reply); err != nil {
		return nil, err
	}

	n, err := s.sessions.CountMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	result := &TurnResult{
		SessionID:    sessionID,
		Reply:        reply,
		MessageCount: n,
		Model:        run.Model,
		Usage:        run.Usage,
		Iterations:   run.Iterations,
		ToolCalls:    run.ToolCallsRan,
		Slash:        slashCmd,
	}

	payload := map[string]any{
		"session_id":    result.SessionID,
		"reply":         result.Reply,
		"message_count": result.MessageCount,
		"model":         result.Model,
		"iterations":    result.Iterations,
		"tool_calls":    result.ToolCalls,
	}
	if result.Usage != nil {
		payload["usage"] = result.Usage
	}
	progress.Emit(emit, progress.EventRunCompleted, payload)

	slog.Info("agent turn ok",
		"provider", s.llm.Name(),
		"model", run.Model,
		"session_id", sessionID,
		"message_count", n,
		"iterations", run.Iterations,
		"tool_calls", run.ToolCallsRan,
		"bootstrap", bootstrap.PresentNames(files),
	)

	return result, nil
}

func (s *Service) persistTurn(ctx context.Context, sessionID, userMsg, reply string) error {
	now := time.Now().UTC()
	return s.sessions.AppendMessages(ctx, sessionID,
		store.Message{Role: store.RoleUser, Content: userMsg, CreatedAt: now},
		store.Message{Role: store.RoleAssistant, Content: reply, CreatedAt: now},
	)
}
