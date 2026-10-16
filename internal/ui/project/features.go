package project

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/api"
)

// ---- helpers ---------------------------------------------------------

// clip shortens s to n runes, ending with an ellipsis.
func clip(s string, n int) string {
	r := []rune(s)
	if n <= 0 {
		return ""
	}
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// tailClip keeps the last n runes of s.
func tailClip(s string, n int) string {
	r := []rune(strings.ReplaceAll(s, "\n", " "))
	if n < 4 {
		n = 4
	}
	if len(r) <= n {
		return string(r)
	}
	return "…" + string(r[len(r)-n+1:])
}

// toolDetail renders a tool part's input and output as short lines for the
// Ctrl+X view.
func toolDetail(part api.Part) string {
	var lines []string
	keys := make([]string, 0, len(part.State.Input))
	for k := range part.State.Input {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := fmt.Sprint(part.State.Input[k])
		parts := strings.Split(strings.TrimRight(v, "\n"), "\n")
		line := k + ": " + clip(parts[0], 160)
		if len(parts) > 1 {
			line += fmt.Sprintf(" (+%d lines)", len(parts)-1)
		}
		lines = append(lines, line)
	}
	out := part.State.Output
	if part.State.Status == "error" && part.State.Error != "" {
		out = part.State.Error
	}
	if out = strings.TrimRight(out, "\n"); out != "" {
		ol := strings.Split(out, "\n")
		const maxLines = 12
		for i, l := range ol {
			if i == maxLines {
				lines = append(lines, fmt.Sprintf("… %d more lines", len(ol)-maxLines))
				break
			}
			lines = append(lines, "│ "+clip(l, 160))
		}
	}
	return strings.Join(lines, "\n")
}

// lastUser returns the ID and text of the last user prompt before the
// message `before` ("" = the whole list).
func lastUser(msgs []api.MessageWithParts, before string) (id, text string) {
	end := len(msgs)
	if before != "" {
		for i, m := range msgs {
			if m.Info.ID == before {
				end = i
				break
			}
		}
	}
	for i := end - 1; i >= 0; i-- {
		if msgs[i].Info.Role != "user" {
			continue
		}
		var parts []string
		for _, pt := range msgs[i].Parts {
			if pt.Type == "text" && !pt.Synthetic {
				parts = append(parts, pt.Text)
			}
		}
		return msgs[i].Info.ID, strings.Join(parts, "\n")
	}
	return "", ""
}

func (r *runState) knowsQuestion(id string) bool {
	if r.question != nil && r.question.ID == id {
		return true
	}
	for _, q := range r.questQ {
		if q.ID == id {
			return true
		}
	}
	return false
}

func (r *runState) knowsPermission(id string) bool {
	if r.pending != nil && r.pending.ID == id {
		return true
	}
	for _, q := range r.permQ {
		if q.ID == id {
			return true
		}
	}
	return false
}

// ---- waiting indicator (window title + bell) --------------------------

func (p ProjectView) anyWaiting() bool {
	for _, s := range p.sessions {
		if s.run.pending != nil || s.run.question != nil {
			return true
		}
	}
	return false
}

// waitingCmd updates the terminal title when a prompt starts or stops
// waiting for the user, and rings the bell when one starts.
func (p *ProjectView) waitingCmd() tea.Cmd {
	a := p.agent
	w := p.anyWaiting()
	if w == a.waiting {
		return nil
	}
	a.waiting = w
	if w {
		_, _ = os.Stdout.WriteString("\a")
		return tea.SetWindowTitle("Forge — waiting for you")
	}
	return tea.SetWindowTitle("Forge")
}

// ---- reconnect resync -------------------------------------------------

type resyncMsg struct {
	questions []api.QuestionRequest
	perms     []api.Permission
	status    map[string]api.SessionState
	statusOK  bool
}

// resync asks the server what is pending and busy, to recover from events
// missed while the stream was down.
func (a *agentState) resync() tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		var m resyncMsg
		m.questions, _ = c.ListQuestions(ctx)
		m.perms, _ = c.ListPermissions(ctx)
		st, err := c.SessionStatuses(ctx)
		m.status, m.statusOK = st, err == nil
		return m
	}
}

func (p *ProjectView) onResync(m resyncMsg) {
	for _, q := range m.questions {
		s := p.sessionByRemote(q.SessionID)
		if s == nil || q.ID == "" || len(q.Questions) == 0 || s.run.knowsQuestion(q.ID) {
			continue
		}
		req := &QuestionPrompt{ID: q.ID, Items: q.Questions}
		s.run.addQuestion(req)
		s.Messages = append(s.Messages, Message{Type: MsgQuestion, Question: req, Timestamp: time.Now()})
	}
	for _, pm := range m.perms {
		s := p.sessionByRemote(pm.SessionID)
		if s == nil || pm.ID == "" || s.run.knowsPermission(pm.ID) {
			continue
		}
		req := &PermissionRequest{ID: pm.ID, Tool: pm.Permission, Description: strings.Join(pm.Patterns, ", ")}
		s.run.addPermission(req)
		s.Messages = append(s.Messages, Message{Type: MsgPermission, Permission: req, Timestamp: time.Now()})
	}
	if !m.statusOK {
		return
	}
	for i := range p.sessions {
		s := &p.sessions[i]
		if !s.run.working || s.RemoteID == "" {
			continue
		}
		if st, ok := m.status[s.RemoteID]; !ok || st.Type == "idle" {
			p.note(s, MsgSystem, "run ended while the connection was down")
			p.finishRun(s)
		}
	}
}

// ---- permission rejection with feedback --------------------------------

type rejectSentMsg struct{ err error }

func (a *agentState) rejectPermission(requestID, message string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		return rejectSentMsg{err: c.RejectPermission(ctx, requestID, message)}
	}
}

func (p ProjectView) permTyping() bool {
	if p.picker != nil || p.activePane != PaneChat || len(p.sessions) == 0 {
		return false
	}
	req := p.sessions[p.activeSession].run.pending
	return req != nil && req.Typing
}

func (p *ProjectView) handlePermTyping(msg tea.KeyMsg) (tea.Cmd, bool) {
	s := &p.sessions[p.activeSession]
	req := s.run.pending
	if req == nil {
		return nil, false
	}
	switch msg.Type {
	case tea.KeyEnter:
		id, text := req.ID, strings.TrimSpace(req.Text)
		s.run.donePermission(id)
		p.note(s, MsgSystem, "permission: reject")
		return p.agent.rejectPermission(id, text), true
	case tea.KeyEsc:
		req.Typing, req.Text = false, ""
	case tea.KeyBackspace:
		if r := []rune(req.Text); len(r) > 0 {
			req.Text = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		req.Text += " "
	case tea.KeyRunes:
		req.Text += string(msg.Runes)
	default:
		return nil, false
	}
	return nil, true
}

// ---- prompt history ----------------------------------------------------

func (a *agentState) pushHistory(text string) {
	if n := len(a.hist); n == 0 || a.hist[n-1] != text {
		a.hist = append(a.hist, text)
		if len(a.hist) > 200 {
			a.hist = a.hist[len(a.hist)-200:]
		}
	}
	a.histIdx = len(a.hist)
	a.draft = ""
}

func (a *agentState) historyPrev(input *string) {
	if len(a.hist) == 0 || a.histIdx <= 0 {
		return
	}
	if a.histIdx > len(a.hist) {
		a.histIdx = len(a.hist)
	}
	if a.histIdx == len(a.hist) {
		a.draft = *input
	}
	a.histIdx--
	*input = a.hist[a.histIdx]
}

func (a *agentState) historyNext(input *string) {
	if a.histIdx >= len(a.hist) {
		return
	}
	a.histIdx++
	if a.histIdx == len(a.hist) {
		*input = a.draft
		return
	}
	*input = a.hist[a.histIdx]
}

// ---- @file completion --------------------------------------------------

func (a *agentState) projectFiles(root string) []string {
	if !a.filesAt.IsZero() && time.Since(a.filesAt) < 30*time.Second {
		return a.files
	}
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" ||
				name == "result" || name == "dist" || name == "target") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= 5000 {
			return filepath.SkipAll
		}
		if rel, err := filepath.Rel(root, path); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	a.files, a.filesAt = out, time.Now()
	return out
}

// matchFiles returns up to 20 paths containing q, best matches first.
func matchFiles(files []string, q string) []string {
	q = strings.ToLower(q)
	type scored struct {
		path  string
		score int
	}
	var res []scored
	for _, f := range files {
		l := strings.ToLower(f)
		i := strings.Index(l, q)
		if i < 0 {
			continue
		}
		score := len(f)
		base := strings.LastIndex(l, "/") + 1
		if strings.HasPrefix(l[base:], q) {
			score -= 1000
		} else if i == 0 {
			score -= 500
		}
		res = append(res, scored{f, score})
	}
	sort.SliceStable(res, func(i, j int) bool { return res[i].score < res[j].score })
	out := make([]string, 0, min(len(res), 20))
	for i := 0; i < len(res) && i < 20; i++ {
		out = append(out, res[i].path)
	}
	return out
}

// complete finishes the @token at the end of input with the best matching
// file; repeated Tab presses cycle through the matches.
func (p ProjectView) complete(input string) string {
	a := p.agent
	i := strings.LastIndexAny(input, " \n")
	tok := input[i+1:]
	if !a.acOn && !strings.HasPrefix(tok, "@") {
		return input
	}
	idx := 0
	query := strings.TrimPrefix(tok, "@")
	if a.acOn {
		query, idx = a.acQuery, a.acIdx+1
	}
	matches := matchFiles(a.projectFiles(p.ProjectPath), query)
	if len(matches) == 0 {
		return input
	}
	idx %= len(matches)
	a.acOn, a.acQuery, a.acIdx = true, query, idx
	return input[:i+1] + "@" + matches[idx]
}

// acHint is the muted suggestion shown after the cursor.
func (p ProjectView) acHint() string {
	if p.agent.acOn {
		return ""
	}
	i := strings.LastIndexAny(p.input, " \n")
	tok := p.input[i+1:]
	if !strings.HasPrefix(tok, "@") {
		return ""
	}
	m := matchFiles(p.agent.projectFiles(p.ProjectPath), tok[1:])
	if len(m) == 0 {
		return ""
	}
	return "  ⇥ " + m[0]
}

// ---- slash commands ----------------------------------------------------

type slashDoneMsg struct {
	localID  string
	kind     string // undo | redo | compact | share | fork
	note     string
	restore  string // prompt text to put back in the input
	revertID string
	err      error
	sess     api.Session
	msgs     []api.MessageWithParts
}

const helpText = "commands: /undo /redo /compact /share /fork [prev] /handoff /help\n" +
	"keys: Ctrl+R thinking · Ctrl+X tool details · @file then Tab · Alt+Enter or Ctrl+J newline · ↑/↓ history"

func (p *ProjectView) slash(text string) (tea.Cmd, bool) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil, false
	}
	s := &p.sessions[p.activeSession]
	a := p.agent
	switch fields[0] {
	case "/help":
		p.note(s, MsgSystem, helpText)
		return nil, true
	case "/undo", "/redo", "/compact", "/share", "/fork":
	default:
		return nil, false
	}
	if !a.ready {
		p.note(s, MsgError, "OpenCode is not ready yet")
		return nil, true
	}
	if s.RemoteID == "" {
		p.note(s, MsgSystem, fields[0]+": this tab has no session yet")
		return nil, true
	}
	switch fields[0] {
	case "/undo":
		return a.undo(s.ID, s.RemoteID, s.run.revertID), true
	case "/redo":
		return a.redo(s.ID, s.RemoteID), true
	case "/compact":
		p.note(s, MsgSystem, "compacting…")
		return a.compact(s.ID, s.RemoteID), true
	case "/share":
		return a.share(s.ID, s.RemoteID), true
	case "/fork":
		return a.fork(s.ID, s.RemoteID, len(fields) > 1 && fields[1] == "prev"), true
	}
	return nil, false
}

func (a *agentState) undo(localID, remoteID, before string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
		defer cancel()
		msgs, err := c.Messages(ctx, remoteID)
		if err != nil {
			return slashDoneMsg{localID: localID, kind: "undo", err: err}
		}
		id, text := lastUser(msgs, before)
		if id == "" {
			return slashDoneMsg{localID: localID, kind: "undo", err: fmt.Errorf("nothing to undo")}
		}
		if err := c.Revert(ctx, remoteID, id); err != nil {
			return slashDoneMsg{localID: localID, kind: "undo", err: err}
		}
		return slashDoneMsg{localID: localID, kind: "undo", revertID: id, restore: text,
			note: "last prompt undone and its file changes reverted (/redo restores them)"}
	}
}

func (a *agentState) redo(localID, remoteID string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
		defer cancel()
		if err := c.Unrevert(ctx, remoteID); err != nil {
			return slashDoneMsg{localID: localID, kind: "redo", err: err}
		}
		msgs, err := c.Messages(ctx, remoteID)
		return slashDoneMsg{localID: localID, kind: "redo", msgs: msgs, err: err, note: "changes restored"}
	}
}

func (a *agentState) compact(localID, remoteID string) tea.Cmd {
	c, ref := a.client, a.modelRef()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
		defer cancel()
		err := c.Summarize(ctx, remoteID, ref)
		return slashDoneMsg{localID: localID, kind: "compact", err: err, note: "context compacted"}
	}
}

func (a *agentState) share(localID, remoteID string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
		defer cancel()
		sess, err := c.Share(ctx, remoteID)
		note := "shared: " + sess.Share.URL
		if err == nil && sess.Share.URL == "" {
			err = fmt.Errorf("the server returned no share link (is sharing disabled?)")
		}
		return slashDoneMsg{localID: localID, kind: "share", err: err, note: note}
	}
}

func (a *agentState) fork(localID, remoteID string, prev bool) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
		defer cancel()
		var before, text string
		if prev {
			msgs, err := c.Messages(ctx, remoteID)
			if err != nil {
				return slashDoneMsg{localID: localID, kind: "fork", err: err}
			}
			before, text = lastUser(msgs, "")
		}
		sess, err := c.Fork(ctx, remoteID, before)
		if err != nil {
			return slashDoneMsg{localID: localID, kind: "fork", err: err}
		}
		msgs, err := c.Messages(ctx, sess.ID)
		return slashDoneMsg{localID: localID, kind: "fork", sess: sess, msgs: msgs, err: err,
			restore: text, note: "forked from the previous session"}
	}
}

func (p *ProjectView) onSlashDone(m slashDoneMsg) tea.Cmd {
	s := p.sessionByLocal(m.localID)
	if s == nil {
		s = &p.sessions[p.activeSession]
	}
	if m.err != nil {
		p.note(s, MsgError, "/"+m.kind+": "+m.err.Error())
		return nil
	}
	active := s == &p.sessions[p.activeSession]
	switch m.kind {
	case "undo":
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Type == MsgUser {
				s.Messages = s.Messages[:i]
				break
			}
		}
		s.run.partIdx = nil
		s.run.revertID = m.revertID
		s.scroll = 0
		p.note(s, MsgSystem, m.note)
		if active && m.restore != "" {
			p.input, p.inputMode = m.restore, true
		}
	case "redo":
		s.Messages, s.run.partIdx, s.run.revertID, s.scroll = nil, nil, "", 0
		p.loadHistory(s, m.msgs)
		p.note(s, MsgSystem, m.note)
	case "fork":
		p.adoptSession(sessionLoadedMsg{sess: m.sess, msgs: m.msgs})
		ns := &p.sessions[p.activeSession]
		p.note(ns, MsgSystem, m.note)
		if m.restore != "" {
			p.input, p.inputMode = m.restore, true
		}
	default:
		p.note(s, MsgSystem, m.note)
	}
	return nil
}
