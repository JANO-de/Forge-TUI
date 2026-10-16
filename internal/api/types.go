package api

import "encoding/json"

// Types follow the OpenCode OpenAPI spec (GET /doc) and the SDK's
// types.gen.ts. Fields not listed are ignored on decode.

type Health struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
}

// Model is one model of a provider.
type Model struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
}

// Provider is an entry of GET /config/providers (models is keyed by model ID).
type Provider struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Models map[string]Model `json:"models"`
}

// ProvidersResponse is the body of GET /config/providers.
type ProvidersResponse struct {
	Providers []Provider        `json:"providers"`
	Default   map[string]string `json:"default"`
}

// ModelRef selects a model for a prompt.
type ModelRef struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

type SessionTime struct {
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
}

type Session struct {
	ID        string      `json:"id"`
	ProjectID string      `json:"projectID"`
	Directory string      `json:"directory"`
	Title     string      `json:"title"`
	Time      SessionTime `json:"time"`
	Share     struct {
		URL string `json:"url"`
	} `json:"share"`
}

// SessionState is one entry of GET /session/status (idle sessions are absent).
type SessionState struct {
	Type string `json:"type"` // busy | retry | idle
}

type Todo struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"` // pending, in_progress, completed, cancelled
	Priority string `json:"priority"`
}

type Tokens struct {
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

// Message is the "info" of a message (user or assistant).
type Message struct {
	ID         string  `json:"id"`
	SessionID  string  `json:"sessionID"`
	Role       string  `json:"role"` // user | assistant
	ProviderID string  `json:"providerID"`
	ModelID    string  `json:"modelID"`
	Cost       float64 `json:"cost"`
	Tokens     Tokens  `json:"tokens"`
	Finish     string  `json:"finish"`
}

// ToolState is the state of a tool part.
type ToolState struct {
	Status string `json:"status"` // pending, running, completed, error
	Title  string `json:"title"`
	Output string `json:"output"`
	Error  string `json:"error"`
	Input  map[string]any `json:"input"`
}

// Part is a message part. Type selects which fields are set:
// text/reasoning -> Text; tool -> Tool, State.
type Part struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionID"`
	MessageID string    `json:"messageID"`
	Type      string    `json:"type"`
	Text      string    `json:"text"`
	Tool      string    `json:"tool"`
	State     ToolState `json:"state"`
	Synthetic bool      `json:"synthetic"` // text injected by OpenCode, not typed by the user
}

type MessageWithParts struct {
	Info  Message `json:"info"`
	Parts []Part  `json:"parts"`
}

// Permission is a pending permission request (event permission.asked).
// Permission is the kind (edit, bash, ...); Patterns are what it applies to.
type Permission struct {
	ID         string   `json:"id"`
	SessionID  string   `json:"sessionID"`
	Permission string   `json:"permission"`
	Patterns   []string `json:"patterns"`
}

// PermissionReplied is the payload of permission.replied.
type PermissionReplied struct {
	SessionID string `json:"sessionID"`
	RequestID string `json:"requestID"`
	Reply     string `json:"reply"`
}

func (e Event) PermissionReplied() (PermissionReplied, error) {
	var v PermissionReplied
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}

// Event is one SSE event: {"type": "...", "properties": {...}}.
// Use the helper methods to decode Properties.
type Event struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

// Known event types.
const (
	EventServerConnected   = "server.connected"
	EventMessageUpdated    = "message.updated"
	EventPartUpdated       = "message.part.updated"
	EventPermissionAsked   = "permission.asked"
	EventPermissionReplied = "permission.replied"
	EventSessionStatus     = "session.status"
	EventSessionIdle       = "session.idle"
	EventSessionError      = "session.error"
	EventTodoUpdated       = "todo.updated"
	EventFileEdited        = "file.edited"
	EventQuestionAsked     = "question.asked"
	EventQuestionReplied   = "question.replied"
	EventQuestionRejected  = "question.rejected"
)

// QuestionOption is one choice of a question.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// QuestionInfo is one question asked by the agent's question tool.
// Custom nil means a typed answer is allowed (OpenCode's default).
type QuestionInfo struct {
	Question string           `json:"question"`
	Header   string           `json:"header"`
	Options  []QuestionOption `json:"options"`
	Multiple bool             `json:"multiple"`
	Custom   *bool            `json:"custom"`
}

// AllowCustom reports whether the user may type their own answer.
func (q QuestionInfo) AllowCustom() bool { return q.Custom == nil || *q.Custom }

// QuestionRequest is the payload of question.asked.
type QuestionRequest struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionID"`
	Questions []QuestionInfo `json:"questions"`
}

// QuestionDone is the payload of question.replied and question.rejected.
type QuestionDone struct {
	SessionID string `json:"sessionID"`
	RequestID string `json:"requestID"`
}

func (e Event) Question() (QuestionRequest, error) {
	var v QuestionRequest
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}

func (e Event) QuestionDone() (QuestionDone, error) {
	var v QuestionDone
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}

// SessionStatusProps is the payload of session.status.
type SessionStatusProps struct {
	SessionID string `json:"sessionID"`
	Status    struct {
		Type    string `json:"type"` // idle | busy | retry
		Attempt int    `json:"attempt"`
		Message string `json:"message"`
	} `json:"status"`
}

func (e Event) SessionStatus() (SessionStatusProps, error) {
	var v SessionStatusProps
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}

// PartUpdate is the payload of message.part.updated.
type PartUpdate struct {
	Part  Part   `json:"part"`
	Delta string `json:"delta"`
}

func (e Event) PartUpdate() (PartUpdate, error) {
	var v PartUpdate
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}

func (e Event) MessageInfo() (Message, error) {
	var v struct {
		Info Message `json:"info"`
	}
	err := json.Unmarshal(e.Properties, &v)
	return v.Info, err
}

func (e Event) Permission() (Permission, error) {
	var v Permission
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}

// TodoUpdate is the payload of todo.updated.
type TodoUpdate struct {
	SessionID string `json:"sessionID"`
	Todos     []Todo `json:"todos"`
}

func (e Event) TodoUpdate() (TodoUpdate, error) {
	var v TodoUpdate
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}

// SessionID returns the sessionID of the event, if its properties carry one.
func (e Event) SessionID() string {
	var v struct {
		SessionID string `json:"sessionID"`
		Info      struct {
			SessionID string `json:"sessionID"`
		} `json:"info"`
		Part struct {
			SessionID string `json:"sessionID"`
		} `json:"part"`
	}
	if json.Unmarshal(e.Properties, &v) != nil {
		return ""
	}
	switch {
	case v.SessionID != "":
		return v.SessionID
	case v.Info.SessionID != "":
		return v.Info.SessionID
	}
	return v.Part.SessionID
}

// EventPartDelta carries streamed text. message.part.updated holds the whole
// part without a delta, so streaming text arrives only through this event.
const EventPartDelta = "message.part.delta"

// PartDelta is the payload of message.part.delta. Field is "text" for both
// text and reasoning parts; the part type comes from message.part.updated.
type PartDelta struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
	Field     string `json:"field"`
	Delta     string `json:"delta"`
}

func (e Event) PartDelta() (PartDelta, error) {
	var v PartDelta
	err := json.Unmarshal(e.Properties, &v)
	return v, err
}
