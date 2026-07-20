package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"agent_stock/internal/agent"
	"agent_stock/internal/auth"
	"agent_stock/internal/bootstrap"
	"agent_stock/internal/progress"
	"agent_stock/internal/provider"
	"agent_stock/internal/security"
	"agent_stock/internal/skills"
	"agent_stock/internal/slash"
	"agent_stock/internal/store"
	"agent_stock/internal/tools"
	"agent_stock/internal/workspacepath"
)

// Service orchestrates session persistence + agent loop turns.
type Service struct {
	sessions      store.SessionStore
	identities    store.IdentityStore
	llm           provider.Provider
	tools         *tools.Registry
	skills        *skills.Registry
	workspaceBase string
	basePrompt    string
	maxIter       int
	policy        *security.Policy
	guard         *security.InputGuard
}

func NewService(
	sessions store.SessionStore,
	identities store.IdentityStore,
	llm provider.Provider,
	toolReg *tools.Registry,
	skillsReg *skills.Registry,
	workspaceBase string,
	basePrompt string,
	maxIter int,
	pol *security.Policy,
	guard *security.InputGuard,
) *Service {
	return &Service{
		sessions:      sessions,
		identities:    identities,
		llm:           llm,
		tools:         toolReg,
		skills:        skillsReg,
		workspaceBase: workspaceBase,
		basePrompt:    basePrompt,
		maxIter:       maxIter,
		policy:        pol,
		guard:         guard,
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
	AgentID      string `json:"agent_id,omitempty"`
}

// ChatTurn runs a non-streaming chat turn.
func (s *Service) ChatTurn(ctx context.Context, sessionID, message, agentRef string) (*TurnResult, error) {
	return s.ChatTurnWithEvents(ctx, sessionID, message, agentRef, nil)
}

// ChatTurnWithEvents runs a chat turn and emits progress events (SSE / WS).
func (s *Service) ChatTurnWithEvents(ctx context.Context, sessionID, message, agentRef string, emit progress.Emitter) (*TurnResult, error) {
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	id, ok := auth.FromContext(ctx)
	if !ok || id.UserID == "" {
		return nil, fmt.Errorf("%w: authentication required", store.ErrForbidden)
	}
	if !id.CanChat() {
		return nil, fmt.Errorf("%w: role %s cannot chat", store.ErrForbidden, id.Role)
	}

	if agentRef == "" {
		agentRef = "default"
	}
	agentSlug := agentRef
	if s.identities != nil {
		ag, err := s.identities.GetAgent(ctx, id.UserID, agentRef)
		if err != nil {
			return nil, fmt.Errorf("agent: %w", err)
		}
		agentSlug = ag.Slug
		if agentSlug == "" {
			agentSlug = ag.ID
		}
	}

	wsRoot, err := workspacepath.PrivateRoot(s.workspaceBase, id.UserID, agentSlug)
	if err != nil {
		return nil, err
	}
	if err := bootstrap.SeedIfMissing(wsRoot); err != nil {
		return nil, fmt.Errorf("seed workspace: %w", err)
	}
	if err := skills.SeedIfMissing(wsRoot); err != nil {
		return nil, fmt.Errorf("seed skills: %w", err)
	}

	owner := store.SessionOwner{TenantID: id.TenantID, UserID: id.UserID, AgentID: agentSlug}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	if err := s.sessions.EnsureSession(ctx, sessionID, owner); err != nil {
		return nil, err
	}

	ctx = tools.WithJailRoot(ctx, wsRoot)

	// Per-user skills metadata (workspace-local); tools still use shared registry skills if configured.
	turnSkills := skills.NewRegistry(wsRoot, "")
	_ = turnSkills.Reload()

	originalMessage := message

	var slashCmd string
	if slash.IsSlash(message) {
		sr := slash.Dispatch(message, turnSkills)
		if sr.Handled && sr.Action == slash.ActionDirect {
			progress.Emit(emit, progress.EventRunStarted, map[string]any{"session_id": sessionID, "slash": sr.Command})
			reply := security.Redact(s.policy, sr.Reply)
			if err := s.persistTurn(ctx, sessionID, owner, originalMessage, reply); err != nil {
				return nil, err
			}
			n, err := s.sessions.CountMessages(ctx, sessionID, owner)
			if err != nil {
				return nil, err
			}
			result := &TurnResult{SessionID: sessionID, Reply: reply, MessageCount: n, Slash: sr.Command, AgentID: agentSlug}
			progress.Emit(emit, progress.EventRunCompleted, map[string]any{
				"session_id": result.SessionID, "reply": result.Reply, "message_count": result.MessageCount, "slash": sr.Command,
			})
			return result, nil
		}
		if sr.Handled && sr.Action == slash.ActionContinue {
			message = sr.EffectiveMsg
			slashCmd = sr.Command
		}
	}

	if err := security.CheckUserMessage(s.policy, s.guard, message); err != nil {
		progress.Emit(emit, progress.EventError, map[string]any{"message": err.Error()})
		return nil, err
	}
	if s.llm == nil {
		return nil, fmt.Errorf("llm provider not configured")
	}

	history, err := s.sessions.ListMessages(ctx, sessionID, owner)
	if err != nil {
		return nil, err
	}

	files := bootstrap.Load(wsRoot)
	prompt := bootstrap.BuildSystemPromptWithSkills(s.basePrompt, files, turnSkills.FormatMetadataSection())

	msgs := make([]provider.Message, 0, len(history)+2)
	msgs = append(msgs, provider.Message{Role: provider.RoleSystem, Content: prompt})
	for _, m := range history {
		msgs = append(msgs, provider.Message{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, provider.Message{Role: provider.RoleUser, Content: message})

	progress.Emit(emit, progress.EventRunStarted, map[string]any{"session_id": sessionID, "user_id": id.UserID, "agent_id": agentSlug})

	loop := &agent.Loop{LLM: s.llm, Tools: s.tools, MaxIterations: s.maxIter, OnEvent: emit}
	run, err := loop.Run(ctx, msgs)
	if err != nil {
		return nil, err
	}

	reply := security.Redact(s.policy, run.Content)
	if err := s.persistTurn(ctx, sessionID, owner, originalMessage, reply); err != nil {
		return nil, err
	}
	n, err := s.sessions.CountMessages(ctx, sessionID, owner)
	if err != nil {
		return nil, err
	}

	result := &TurnResult{
		SessionID: sessionID, Reply: reply, MessageCount: n,
		Model: run.Model, Usage: run.Usage, Iterations: run.Iterations,
		ToolCalls: run.ToolCallsRan, Slash: slashCmd, AgentID: agentSlug,
	}
	payload := map[string]any{
		"session_id": result.SessionID, "reply": result.Reply, "message_count": result.MessageCount,
		"model": result.Model, "iterations": result.Iterations, "tool_calls": result.ToolCalls, "agent_id": agentSlug,
	}
	if result.Usage != nil {
		payload["usage"] = result.Usage
	}
	progress.Emit(emit, progress.EventRunCompleted, payload)

	slog.Info("agent turn ok",
		"provider", s.llm.Name(), "model", run.Model, "session_id", sessionID,
		"user_id", id.UserID, "agent_id", agentSlug, "message_count", n,
		"iterations", run.Iterations, "tool_calls", run.ToolCallsRan,
	)
	return result, nil
}

func (s *Service) persistTurn(ctx context.Context, sessionID string, owner store.SessionOwner, userMsg, reply string) error {
	now := time.Now().UTC()
	return s.sessions.AppendMessages(ctx, sessionID, owner,
		store.Message{Role: store.RoleUser, Content: userMsg, CreatedAt: now},
		store.Message{Role: store.RoleAssistant, Content: reply, CreatedAt: now},
	)
}
