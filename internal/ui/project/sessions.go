package project

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/api"
)

// sessionPicker is the "open a previous OpenCode session" list (Ctrl+O). It
// replaces the chat column while open. Nil when closed.
type sessionPicker struct {
	items   []api.Session
	cursor  int
	loading bool
	err     string
}

type sessionListMsg struct {
	items []api.Session
	err   error
}

type sessionLoadedMsg struct {
	sess  api.Session
	msgs  []api.MessageWithParts
	todos []api.Todo
	err   error
}

// listSessions fetches the sessions OpenCode knows for dir, newest first.
// OpenCode groups non-git directories under one "global" project, so the
// list is filtered by directory here.
func (a *agentState) listSessions(dir string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		all, err := c.Sessions(ctx)
		if err != nil {
			return sessionListMsg{err: err}
		}
		var out []api.Session
		for _, s := range all {
			if s.Directory == "" || sameDir(s.Directory, dir) {
				out = append(out, s)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Updated > out[j].Time.Updated })
		return sessionListMsg{items: out}
	}
}

// loadSession fetches the full history (and todo list) of one session.
func (a *agentState) loadSession(s api.Session) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
		defer cancel()
		msgs, err := c.Messages(ctx, s.ID)
		if err != nil {
			return sessionLoadedMsg{sess: s, err: err}
		}
		todos, _ := c.Todos(ctx, s.ID) // optional: the progress bar falls back to HANDOFF.md
		return sessionLoadedMsg{sess: s, msgs: msgs, todos: todos}
	}
}

func sameDir(a, b string) bool {
	clean := func(p string) string {
		p = filepath.Clean(p)
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	return clean(a) == clean(b)
}

func (p *ProjectView) openPicker() tea.Cmd {
	if !p.agent.ready {
		p.note(&p.sessions[p.activeSession], MsgError, "OpenCode is not ready yet")
		return nil
	}
	p.picker = &sessionPicker{loading: true}
	return p.agent.listSessions(p.ProjectPath)
}

func (p *ProjectView) handleSessionMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case sessionListMsg:
		if p.picker == nil {
			return nil, true
		}
		p.picker.loading = false
		if msg.err != nil {
			p.picker.err = msg.err.Error()
		} else {
			p.picker.items = msg.items
		}
		return nil, true
	case sessionLoadedMsg:
		if p.picker == nil {
			return nil, true // cancelled while loading
		}
		if msg.err != nil {
			p.picker.loading, p.picker.err = false, msg.err.Error()
			return nil, true
		}
		p.picker = nil
		p.adoptSession(msg)
		return nil, true
	}
	return nil, false
}

// handlePickerKey consumes every key while the picker is open.
func (p *ProjectView) handlePickerKey(msg tea.KeyMsg) tea.Cmd {
	pk := p.picker
	rows := max(p.rows(p.layout())-1, 1)
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		p.picker = nil
	case tea.KeyUp:
		pk.cursor = max(pk.cursor-1, 0)
	case tea.KeyDown:
		pk.cursor = min(pk.cursor+1, max(len(pk.items)-1, 0))
	case tea.KeyPgUp:
		pk.cursor = max(pk.cursor-rows, 0)
	case tea.KeyPgDown:
		pk.cursor = min(pk.cursor+rows, max(len(pk.items)-1, 0))
	case tea.KeyHome:
		pk.cursor = 0
	case tea.KeyEnd:
		pk.cursor = max(len(pk.items)-1, 0)
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'k':
				pk.cursor = max(pk.cursor-1, 0)
			case 'j':
				pk.cursor = min(pk.cursor+1, max(len(pk.items)-1, 0))
			case 'q':
				p.picker = nil
			}
		}
	case tea.KeyEnter:
		if pk.loading || pk.cursor >= len(pk.items) {
			return nil
		}
		chosen := pk.items[pk.cursor]
		for i := range p.sessions {
			if p.sessions[i].RemoteID == chosen.ID { // already open: just switch
				p.activeSession, p.picker = i, nil
				return nil
			}
		}
		pk.loading, pk.err = true, ""
		return p.agent.loadSession(chosen)
	}
	return nil
}

// adoptSession opens the loaded session as a new tab (reusing the initial
// tab when it is still empty) and fills it with the stored history.
func (p *ProjectView) adoptSession(m sessionLoadedMsg) {
	idx := -1
	if len(p.sessions) == 1 && p.sessions[0].RemoteID == "" && len(p.sessions[0].Messages) == 0 {
		idx = 0
	}
	if idx < 0 {
		p.newSession()
		idx = len(p.sessions) - 1
	}
	s := &p.sessions[idx]
	s.RemoteID = m.sess.ID
	s.Name = tabName(m.sess.Title)
	if m.sess.Time.Created > 0 {
		s.CreatedAt = time.UnixMilli(m.sess.Time.Created)
	}
	s.Messages, s.scroll = nil, 0
	s.run = runState{}
	p.loadHistory(s, m.msgs)
	done := 0
	for _, t := range m.todos {
		if t.Status == "completed" {
			done++
		}
	}
	s.run.todoDone, s.run.todoTotal = done, len(m.todos)
	p.activeSession = idx
}

// loadHistory converts stored messages into chat lines the same way live
// events are converted, so later events for the same parts update in place.
func (p *ProjectView) loadHistory(s *Session, msgs []api.MessageWithParts) {
	a := p.agent
	for _, mw := range msgs {
		info := mw.Info
		a.roles[info.ID] = info.Role
		if info.Role == "user" {
			for _, part := range mw.Parts {
				if part.Type == "text" && !part.Synthetic && strings.TrimSpace(part.Text) != "" {
					s.Messages = append(s.Messages, Message{Type: MsgUser, Content: part.Text, Timestamp: time.Now()})
				}
			}
			continue
		}
		if s.run.msgs == nil {
			s.run.msgs = map[string]api.Message{}
		}
		s.run.msgs[info.ID] = info
		s.run.lastAssistant = info.ID
		for _, part := range mw.Parts {
			p.upsertPart(s, part)
		}
	}
	s.run.tool = "" // nothing is running in a stored session
}

func tabName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Session"
	}
	if r := []rune(title); len(r) > 22 {
		title = string(r[:21]) + "…"
	}
	return title
}

func ago(ms int64) string {
	d := time.Since(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return time.UnixMilli(ms).Format("2006-01-02")
}

func (p ProjectView) pickerLines() []string {
	pk := p.picker
	switch {
	case pk.err != "":
		return []string{errorStyle.Render(pk.err), mutedStyle.Render("Esc closes.")}
	case pk.loading && len(pk.items) == 0:
		return []string{mutedStyle.Render("Loading sessions…")}
	case len(pk.items) == 0:
		return []string{mutedStyle.Render("No previous sessions for this project."), mutedStyle.Render("Esc closes.")}
	}
	open := map[string]bool{}
	for _, s := range p.sessions {
		if s.RemoteID != "" {
			open[s.RemoteID] = true
		}
	}
	rows := max(p.rows(p.layout())-1, 1)
	start := min(max(pk.cursor-rows/2, 0), max(len(pk.items)-rows, 0))
	var out []string
	if pk.loading {
		out = append(out, mutedStyle.Render("Loading session…"))
	} else {
		out = append(out, mutedStyle.Render(fmt.Sprintf("%d sessions · Enter opens · Esc closes", len(pk.items))))
	}
	for i := start; i < len(pk.items) && i < start+rows-1; i++ {
		it := pk.items[i]
		title := it.Title
		if strings.TrimSpace(title) == "" {
			title = "(untitled)"
		}
		line := fmt.Sprintf("%s  %s", title, mutedStyle.Render(ago(it.Time.Updated)))
		if open[it.ID] {
			line += mutedStyle.Render("  · open")
		}
		if i == pk.cursor {
			out = append(out, titleStyle.Render("▌ ")+userStyle.Render(title)+"  "+mutedStyle.Render(ago(it.Time.Updated))+openTag(open[it.ID]))
		} else {
			out = append(out, "  "+line)
		}
	}
	return out
}

func openTag(open bool) string {
	if open {
		return mutedStyle.Render("  · open")
	}
	return ""
}
