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
}

func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) BaseURL() string { return c.base }

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
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
	resp, err := c.http.Do(req)
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

// Todos: GET /session/:id/todo.
func (c *Client) Todos(ctx context.Context, sessionID string) ([]Todo, error) {
	var t []Todo
	err := c.do(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/todo", nil, &t)
	return t, err
}
