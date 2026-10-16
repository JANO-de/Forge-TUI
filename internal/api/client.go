package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to `opencode serve` over HTTP. Safe for concurrent use.
type Client struct {
	base string
	http *http.Client
	long *http.Client // for slow calls such as summarize
}

func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 30 * time.Second},
		long: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (c *Client) BaseURL() string { return c.base }

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	return c.doWith(ctx, c.http, method, path, in, out)
}

func (c *Client) doWith(ctx context.Context, hc *http.Client, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Health: GET /global/health.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var h Health
	err := c.do(ctx, http.MethodGet, "/global/health", nil, &h)
	return h, err
}

// Providers: GET /config/providers (providers + default model per provider).
func (c *Client) Providers(ctx context.Context) (ProvidersResponse, error) {
	var r ProvidersResponse
	err := c.do(ctx, http.MethodGet, "/config/providers", nil, &r)
	return r, err
}

// Sessions: GET /session.
func (c *Client) Sessions(ctx context.Context) ([]Session, error) {
	var s []Session
	err := c.do(ctx, http.MethodGet, "/session", nil, &s)
	return s, err
}

// CreateSession: POST /session.
func (c *Client) CreateSession(ctx context.Context, title string) (Session, error) {
	body := map[string]any{}
	if title != "" {
		body["title"] = title
	}
	var s Session
	err := c.do(ctx, http.MethodPost, "/session", body, &s)
	return s, err
}

// Messages: GET /session/:id/message.
func (c *Client) Messages(ctx context.Context, sessionID string) ([]MessageWithParts, error) {
	var m []MessageWithParts
	err := c.do(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/message", nil, &m)
	return m, err
}

// PromptAsync: POST /session/:id/prompt_async (204, no wait). Progress
// arrives through Events. model and agent are optional (zero value = default).
func (c *Client) PromptAsync(ctx context.Context, sessionID, text string, model *ModelRef, agent string) error {
	body := map[string]any{
		"parts": []map[string]any{{"type": "text", "text": text}},
	}
	if model != nil && model.ProviderID != "" {
		body["model"] = model
	}
	if agent != "" {
		body["agent"] = agent
	}
	return c.do(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/prompt_async", body, nil)
}

// Abort: POST /session/:id/abort.
func (c *Client) Abort(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/abort", nil, nil)
}

// RespondPermission: POST /permission/:requestID/reply.
// reply is "once", "always" or "reject".
func (c *Client) RespondPermission(ctx context.Context, requestID, reply string) error {
	body := map[string]any{"reply": reply}
	return c.do(ctx, http.MethodPost, "/permission/"+url.PathEscape(requestID)+"/reply", body, nil)
}

// ReplyQuestion: POST /question/:requestID/reply. answers has one entry
// (the chosen labels or typed text) per question, in order.
func (c *Client) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	body := map[string]any{"answers": answers}
	return c.do(ctx, http.MethodPost, "/question/"+url.PathEscape(requestID)+"/reply", body, nil)
}

// RejectQuestion: POST /question/:requestID/reject.
func (c *Client) RejectQuestion(ctx context.Context, requestID string) error {
	return c.do(ctx, http.MethodPost, "/question/"+url.PathEscape(requestID)+"/reject", nil, nil)
}

// RejectPermission rejects a permission request, optionally telling the
// agent why.
func (c *Client) RejectPermission(ctx context.Context, requestID, message string) error {
	body := map[string]any{"reply": "reject"}
	if message != "" {
		body["message"] = message
	}
	return c.do(ctx, http.MethodPost, "/permission/"+url.PathEscape(requestID)+"/reply", body, nil)
}

// ListQuestions: GET /question (pending question requests).
func (c *Client) ListQuestions(ctx context.Context) ([]QuestionRequest, error) {
	var q []QuestionRequest
	err := c.do(ctx, http.MethodGet, "/question", nil, &q)
	return q, err
}

// ListPermissions: GET /permission (pending permission requests).
func (c *Client) ListPermissions(ctx context.Context) ([]Permission, error) {
	var p []Permission
	err := c.do(ctx, http.MethodGet, "/permission", nil, &p)
	return p, err
}

// SessionStatuses: GET /session/status, keyed by session ID.
func (c *Client) SessionStatuses(ctx context.Context) (map[string]SessionState, error) {
	var m map[string]SessionState
	err := c.do(ctx, http.MethodGet, "/session/status", nil, &m)
	return m, err
}

// Revert: POST /session/:id/revert (undo a message and its file changes).
func (c *Client) Revert(ctx context.Context, sessionID, messageID string) error {
	body := map[string]any{"messageID": messageID}
	return c.do(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/revert", body, nil)
}

// Unrevert: POST /session/:id/unrevert.
func (c *Client) Unrevert(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/unrevert", nil, nil)
}

// Summarize: POST /session/:id/summarize (compact the context). Slow.
func (c *Client) Summarize(ctx context.Context, sessionID string, m *ModelRef) error {
	if m == nil || m.ProviderID == "" {
		return fmt.Errorf("no model selected")
	}
	body := map[string]any{"providerID": m.ProviderID, "modelID": m.ModelID}
	return c.doWith(ctx, c.long, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/summarize", body, nil)
}

// Share: POST /session/:id/share.
func (c *Client) Share(ctx context.Context, sessionID string) (Session, error) {
	var s Session
	err := c.do(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/share", nil, &s)
	return s, err
}

// Fork: POST /session/:id/fork. With messageID the copy stops before it.
func (c *Client) Fork(ctx context.Context, sessionID, messageID string) (Session, error) {
	body := map[string]any{}
	if messageID != "" {
		body["messageID"] = messageID
	}
	var s Session
	err := c.do(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/fork", body, &s)
	return s, err
}

// Todos: GET /session/:id/todo.
func (c *Client) Todos(ctx context.Context, sessionID string) ([]Todo, error) {
	var t []Todo
	err := c.do(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/todo", nil, &t)
	return t, err
}
