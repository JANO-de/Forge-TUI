package project

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/api"
)

// agentState is shared by every copy of ProjectView (pointer). It is only
// touched from Update, never from a goroutine; goroutines talk via tea.Msg.
type agentState struct {
	ctx      context.Context
	cancel   context.CancelFunc
	srv      *api.Server
	client   *api.Client
	events   <-chan api.Event
	errs     <-chan error
	asm      *api.PartAssembler
	roles    map[string]string // messageID -> user|assistant
	models   []modelChoice
	modelIdx int
	ready    bool
	err      error
}

type modelChoice struct {
	Provider string
	Model    string
	Context  int
}

// runState is the live state of one Forge session talking to OpenCode.
type runState struct {
	working       bool
	startedAt     time.Time
	lastRun       time.Duration
	tool          string
	todoDone      int
	todoTotal     int
	pending       *PermissionRequest
	partIdx       map[string]int
	msgs          map[string]api.Message
	lastAssistant string
}

type agentReadyMsg struct {
	srv       *api.Server
	providers api.ProvidersResponse
	events    <-chan api.Event
	errs      <-chan error
}

type agentErrMsg struct{ err error }

type agentEventMsg struct{ ev api.Event }

type agentStreamEndMsg struct{ err error }

type permissionSentMsg struct{ err error }

type abortSentMsg struct{ err error }

type sessionCreatedMsg struct {
	localID string
	text    string
	session api.Session
	err     error
}

type promptSentMsg struct {
	localID string
	err     error
}

func newAgentState() *agentState {
	ctx, cancel := context.WithCancel(context.Background())
	return &agentState{
		ctx:    ctx,
		cancel: cancel,
		asm:    api.NewPartAssembler(),
		roles:  map[string]string{},
	}
}

// start launches `opencode serve` for dir, lists providers and opens the
// event stream. Sessions are created lazily on the first prompt, so Forge
// does not leave empty sessions behind.
func (a *agentState) start(dir string) tea.Cmd {
	return func() tea.Msg {
		srv, err := api.Start(a.ctx, dir)
		if err != nil {
			return agentErrMsg{err}
		}
		c := srv.Client
		prov, err := c.Providers(a.ctx)
		if err != nil {
			srv.Stop()
			return agentErrMsg{err}
		}
		if a.ctx.Err() != nil {
			srv.Stop()
			return agentErrMsg{a.ctx.Err()}
		}
		events, errs := c.Events(a.ctx)
		return agentReadyMsg{srv: srv, providers: prov, events: events, errs: errs}
	}
}

// wait blocks for the next SSE event; Update re-arms it after each one.
func (a *agentState) wait() tea.Cmd {
	events, errs := a.events, a.errs
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return agentStreamEndMsg{err: <-errs}
		}
		return agentEventMsg{ev}
	}
}

func (a *agentState) createSession(localID, text string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		s, err := c.CreateSession(ctx, "")
		return sessionCreatedMsg{localID: localID, text: text, session: s, err: err}
	}
}

func (a *agentState) prompt(localID, remoteID, text string, mode Mode) tea.Cmd {
	c, ref, agent := a.client, a.modelRef(), agentName(mode)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		return promptSentMsg{localID: localID, err: c.PromptAsync(ctx, remoteID, text, ref, agent)}
	}
}

func (a *agentState) reply(requestID, reply string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		return permissionSentMsg{err: c.RespondPermission(ctx, requestID, reply)}
	}
}

func (a *agentState) abort(remoteID string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		return abortSentMsg{err: c.Abort(ctx, remoteID)}
	}
}

func agentName(m Mode) string {
	if m == ModeBuild {
		return "build"
	}
	return "plan"
}

func (a *agentState) setProviders(r api.ProvidersResponse) {
	provs := append([]api.Provider(nil), r.Providers...)
	sort.Slice(provs, func(i, j int) bool { return provs[i].ID < provs[j].ID })
	a.models = nil
	for _, pr := range provs {
		ids := make([]string, 0, len(pr.Models))
		for id := range pr.Models {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			a.models = append(a.models, modelChoice{pr.ID, id, pr.Models[id].Limit.Context})
		}
	}
	a.modelIdx = 0
	for _, pr := range provs {
		d := r.Default[pr.ID]
		for i, m := range a.models {
			if d != "" && m.Provider == pr.ID && m.Model == d {
				a.modelIdx = i
				return
			}
		}
	}
}

func (a *agentState) modelRef() *api.ModelRef {
	if len(a.models) == 0 {
		return nil
	}
	m := a.models[a.modelIdx]
	return &api.ModelRef{ProviderID: m.Provider, ModelID: m.Model}
}

func (a *agentState) modelLabel() string {
	if len(a.models) == 0 {
		if a.err != nil {
			return "offline"
		}
		return "starting..."
	}
	return a.models[a.modelIdx].Model
}

func (a *agentState) cycleModel() {
	if len(a.models) > 0 {
		a.modelIdx = (a.modelIdx + 1) % len(a.models)
	}
}

// Shutdown stops the opencode child process. Safe on a zero ProjectView.
func (p ProjectView) Shutdown() {
	if p.agent == nil {
		return
	}
	p.agent.cancel()
	p.agent.srv.Stop()
}

func (p *ProjectView) sessionByRemote(id string) *Session {
	if id == "" {
		return nil
	}
	for i := range p.sessions {
		if p.sessions[i].RemoteID == id {
			return &p.sessions[i]
		}
	}
	return nil
}

func (p *ProjectView) sessionByLocal(id string) *Session {
	for i := range p.sessions {
		if p.sessions[i].ID == id {
			return &p.sessions[i]
		}
	}
	return nil
}

func (p *ProjectView) note(s *Session, t MsgType, text string) {
	s.Messages = append(s.Messages, Message{Type: t, Content: text, Timestamp: time.Now()})
}

func (p *ProjectView) finishRun(s *Session) {
	if s.run.working {
		s.run.lastRun = time.Since(s.run.startedAt)
	}
	s.run.working = false
	s.run.tool = ""
	p.handoffDone, p.handoffTotal = handoffProgress(p.ProjectPath)
}

// submit sends text to the active session. Returns nil when it cannot.
func (p *ProjectView) submit(text string) tea.Cmd {
	a := p.agent
	s := &p.sessions[p.activeSession]
	if !a.ready {
		p.note(s, MsgError, "OpenCode is not ready yet")
		return nil
	}
	if s.run.working {
		p.note(s, MsgSystem, "the agent is busy; press Esc to abort it")
		return nil
	}
	s.Messages = append(s.Messages, Message{Type: MsgUser, Content: text, Timestamp: time.Now()})
	s.run.working = true
	s.run.startedAt = time.Now()
	s.scroll = 0
	if s.RemoteID == "" {
		return a.createSession(s.ID, text)
	}
	return a.prompt(s.ID, s.RemoteID, text, p.mode)
}

// handleAgentMsg processes every agent-related tea.Msg. ok reports whether
// msg was one of them.
func (p *ProjectView) handleAgentMsg(msg tea.Msg) (cmd tea.Cmd, ok bool) {
	a := p.agent
	switch msg := msg.(type) {
	case agentReadyMsg:
		a.srv, a.client = msg.srv, msg.srv.Client
		a.events, a.errs = msg.events, msg.errs
		a.ready = true
		a.setProviders(msg.providers)
		return a.wait(), true
	case agentErrMsg:
		a.err = msg.err
		p.note(&p.sessions[p.activeSession], MsgError, "OpenCode: "+msg.err.Error())
		return nil, true
	case agentStreamEndMsg:
		a.ready = false
		text := "OpenCode event stream closed"
		if msg.err != nil {
			text += ": " + msg.err.Error()
		}
		p.note(&p.sessions[p.activeSession], MsgError, text)
		return nil, true
	case agentEventMsg:
		p.onAgentEvent(msg.ev)
		return tea.Batch(a.wait(), p.gitAfterEvent(msg.ev)), true
	case sessionCreatedMsg:
		s := p.sessionByLocal(msg.localID)
		if s == nil {
			return nil, true
		}
		if msg.err != nil {
			p.note(s, MsgError, "create session: "+msg.err.Error())
			p.finishRun(s)
			return nil, true
		}
		s.RemoteID = msg.session.ID
		return a.prompt(s.ID, s.RemoteID, msg.text, p.mode), true
	case promptSentMsg:
		if s := p.sessionByLocal(msg.localID); s != nil && msg.err != nil {
			p.note(s, MsgError, "send: "+msg.err.Error())
			p.finishRun(s)
		}
		return nil, true
	case permissionSentMsg:
		if msg.err != nil {
			p.note(&p.sessions[p.activeSession], MsgError, "permission reply: "+msg.err.Error())
		}
		return nil, true
	case abortSentMsg:
		if msg.err != nil {
			p.note(&p.sessions[p.activeSession], MsgError, "abort: "+msg.err.Error())
		}
		return nil, true
	}
	return nil, false
}

func (p *ProjectView) onAgentEvent(ev api.Event) {
	a := p.agent
	switch ev.Type {
	case api.EventMessageUpdated:
		m, err := ev.MessageInfo()
		if err != nil || m.ID == "" {
			return
		}
		a.roles[m.ID] = m.Role
		if s := p.sessionByRemote(m.SessionID); s != nil && m.Role == "assistant" {
			if s.run.msgs == nil {
				s.run.msgs = map[string]api.Message{}
			}
			s.run.msgs[m.ID] = m
			s.run.lastAssistant = m.ID
		}
	case api.EventPartUpdated, api.EventPartDelta:
		part, _, ok := a.asm.Apply(ev)
		if !ok {
			return
		}
		if s := p.sessionByRemote(part.SessionID); s != nil && a.roles[part.MessageID] != "user" {
			p.upsertPart(s, part)
		}
	case api.EventTodoUpdated:
		u, err := ev.TodoUpdate()
		if err != nil {
			return
		}
		if s := p.sessionByRemote(u.SessionID); s != nil {
			done := 0
			for _, t := range u.Todos {
				if t.Status == "completed" {
					done++
				}
			}
			s.run.todoDone, s.run.todoTotal = done, len(u.Todos)
		}
	case api.EventPermissionAsked:
		pm, err := ev.Permission()
		if err != nil {
			return
		}
		if s := p.sessionByRemote(pm.SessionID); s != nil {
			req := &PermissionRequest{ID: pm.ID, Tool: pm.Permission, Description: strings.Join(pm.Patterns, ", ")}
			s.run.pending = req
			s.Messages = append(s.Messages, Message{Type: MsgPermission, Permission: req, Timestamp: time.Now()})
		}
	case api.EventPermissionReplied:
		r, err := ev.PermissionReplied()
		if err != nil {
			return
		}
		if s := p.sessionByRemote(r.SessionID); s != nil && s.run.pending != nil && s.run.pending.ID == r.RequestID {
			s.run.pending = nil
		}
	case api.EventSessionIdle:
		if s := p.sessionByRemote(ev.SessionID()); s != nil {
			p.finishRun(s)
		}
	case api.EventSessionError:
		var e struct {
			SessionID string `json:"sessionID"`
			Error     struct {
				Name string `json:"name"`
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			} `json:"error"`
		}
		_ = json.Unmarshal(ev.Properties, &e)
		s := p.sessionByRemote(e.SessionID)
		if s == nil {
			s = &p.sessions[p.activeSession]
		}
		text := e.Error.Data.Message
		if text == "" {
			text = e.Error.Name
		}
		if text == "" {
			text = string(ev.Properties)
		}
		p.note(s, MsgError, text)
		p.finishRun(s)
	}
}

func (p *ProjectView) upsertPart(s *Session, part api.Part) {
	m := Message{Timestamp: time.Now()}
	switch part.Type {
	case "text":
		if strings.TrimSpace(part.Text) == "" {
			return
		}
		m.Type, m.Content = MsgAssistant, part.Text
	case "tool":
		m.Type, m.ToolName, m.Content = MsgTool, part.Tool, toolLine(part)
		if part.State.Status == "pending" || part.State.Status == "running" {
			s.run.tool = part.Tool
		} else if s.run.tool == part.Tool {
			s.run.tool = ""
		}
	default:
		return
	}
	if s.run.partIdx == nil {
		s.run.partIdx = map[string]int{}
	}
	if i, ok := s.run.partIdx[part.ID]; ok {
		m.Timestamp = s.Messages[i].Timestamp
		s.Messages[i] = m
		return
	}
	s.run.partIdx[part.ID] = len(s.Messages)
	s.Messages = append(s.Messages, m)
}

func toolLine(part api.Part) string {
	icon := map[string]string{"pending": "…", "running": "⟳", "completed": "✓", "error": "✗"}[part.State.Status]
	if icon == "" {
		icon = "•"
	}
	line := icon + " " + part.Tool
	if part.State.Title != "" {
		line += ": " + part.State.Title
	}
	if part.State.Status == "error" && part.State.Error != "" {
		line += " — " + strings.SplitN(part.State.Error, "\n", 2)[0]
	}
	return line
}

var checkboxRe = regexp.MustCompile(`(?m)^\s*(?:\d+\.|[-*])\s*\[([ xX])\]`)

// handoffProgress counts [x]/[ ] items in HANDOFF.md (fallback for the
// progress bar when the agent has no todo list).
func handoffProgress(projectPath string) (done, total int) {
	b, err := os.ReadFile(filepath.Join(projectPath, "HANDOFF.md"))
	if err != nil {
		return 0, 0
	}
	for _, m := range checkboxRe.FindAllStringSubmatch(string(b), -1) {
		total++
		if m[1] != " " {
			done++
		}
	}
	return done, total
}
