package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"agent_stock/internal/progress"
	"agent_stock/pkg/protocol"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func (s *Server) checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	var allowed []string
	if s.policy != nil {
		allowed = s.policy.WebSocket.AllowedOrigins
	}
	if len(allowed) == 0 {
		// Dev-friendly: allow missing Origin (curl/websocat) and same-host/local.
		return origin == "" || originIsLocal(origin, r.Host)
	}
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	slog.Warn("security.ws_origin_denied", "origin", origin, "host", r.Host)
	return false
}

func originIsLocal(origin, host string) bool {
	return strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") || strings.Contains(origin, host)
}

type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) send(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return c.conn.WriteJSON(v)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := wsUpgrader
	upgrader.CheckOrigin = s.checkWSOrigin
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "error", err)
		return
	}
	client := &wsClient{conn: conn}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		return nil
	})

	slog.Info("ws connected", "remote", r.RemoteAddr)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			slog.Info("ws disconnected", "remote", r.RemoteAddr, "error", err)
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		var frame protocol.Frame
		if err := json.Unmarshal(data, &frame); err != nil {
			_ = client.send(protocol.ResErr("", "invalid json frame"))
			continue
		}
		if frame.Type != protocol.TypeReq {
			_ = client.send(protocol.ResErr(frame.ID, "expected type=req"))
			continue
		}
		s.handleWSRequest(r, client, frame)
	}
}

func (s *Server) handleWSRequest(r *http.Request, client *wsClient, frame protocol.Frame) {
	switch frame.Method {
	case protocol.MethodConnect:
		_ = client.send(protocol.ResOK(frame.ID, map[string]any{
			"version":  s.version,
			"protocol": 1,
			"methods":  []string{protocol.MethodConnect, protocol.MethodChatSend, protocol.MethodPing},
		}))
	case protocol.MethodPing:
		_ = client.send(protocol.ResOK(frame.ID, map[string]any{"pong": true}))
	case protocol.MethodChatSend:
		s.handleWSChatSend(r, client, frame)
	default:
		_ = client.send(protocol.ResErr(frame.ID, "unknown method: "+frame.Method))
	}
}

type wsChatParams struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
}

func (s *Server) handleWSChatSend(r *http.Request, client *wsClient, frame protocol.Frame) {
	var params wsChatParams
	if len(frame.Params) > 0 {
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			_ = client.send(protocol.ResErr(frame.ID, "invalid params"))
			return
		}
	}
	if params.Message == "" {
		_ = client.send(protocol.ResErr(frame.ID, "message is required"))
		return
	}
	if s.sessions == nil {
		_ = client.send(protocol.ResErr(frame.ID, "session service not configured"))
		return
	}

	emit := func(ev progress.Event) {
		_ = client.send(protocol.EventFrame(ev.Type, ev.Payload))
	}

	result, err := s.sessions.ChatTurnWithEvents(r.Context(), params.SessionID, params.Message, params.AgentID, emit)
	if err != nil {
		_ = client.send(protocol.ResErr(frame.ID, err.Error()))
		return
	}
	_ = client.send(protocol.ResOK(frame.ID, map[string]any{
		"session_id":    result.SessionID,
		"reply":         result.Reply,
		"message_count": result.MessageCount,
		"model":         result.Model,
		"iterations":    result.Iterations,
		"tool_calls":    result.ToolCalls,
		"usage":         result.Usage,
		"agent_id":      result.AgentID,
	}))
}
