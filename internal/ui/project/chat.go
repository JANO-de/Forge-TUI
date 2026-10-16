package project

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"forge/internal/ui/status"
)

func (p *ProjectView) refreshStatus() {
	p.updateProjectTotals()
	p.statusBar = p.statusBar.Update(p.statusData())
}

// updateProjectTotals accumulates token and cost totals across all sessions
func (p *ProjectView) updateProjectTotals() {
	p.projectTokensIn = 0
	p.projectTokensOut = 0
	p.projectCost = 0
	for _, sess := range p.sessions {
		for _, m := range sess.run.msgs {
			p.projectTokensIn += m.Tokens.Input + m.Tokens.Cache.Read + m.Tokens.Cache.Write
			p.projectTokensOut += m.Tokens.Output
			p.projectCost += m.Cost
		}
	}
}

func (p ProjectView) statusData() status.StatusData {
	d := status.StatusData{
		Mode:         p.mode,
		Model:        p.agent.modelLabel(),
		SpinnerIndex: p.spin,
		TodoDone:     p.handoffDone,
		TodoTotal:    p.handoffTotal,
	}
	if len(p.sessions) == 0 {
		return d
	}
	s := p.sessions[p.activeSession]
	d.SessionName = s.Name
	d.Working = s.run.working
	d.CurrentTool = s.run.tool
	if s.run.working {
		d.Elapsed = time.Since(s.run.startedAt)
		if d.CurrentTool == "" {
			d.CurrentTool = "thinking"
		}
	} else {
		d.Elapsed = s.run.lastRun
	}
	if s.run.pending != nil || s.run.question != nil {
		d.Working, d.CurrentTool = true, "waiting for your answer"
	}
	if s.run.todoTotal > 0 {
		d.TodoDone, d.TodoTotal = s.run.todoDone, s.run.todoTotal
	}
	for _, m := range s.run.msgs {
		d.TokensIn += m.Tokens.Input + m.Tokens.Cache.Read + m.Tokens.Cache.Write
		d.TokensOut += m.Tokens.Output
		d.Cost += m.Cost
	}
	// Add project-level totals
	d.TokensIn += p.projectTokensIn
	d.TokensOut += p.projectTokensOut
	d.Cost += p.projectCost
	if last, ok := s.run.msgs[s.run.lastAssistant]; ok && p.agent != nil {
		used := last.Tokens.Input + last.Tokens.Output + last.Tokens.Cache.Read + last.Tokens.Cache.Write
		for _, m := range p.agent.models {
			if m.Provider == last.ProviderID && m.Model == last.ModelID && m.Context > 0 {
				d.ContextPercent = min(100, used*100/m.Context)
			}
		}
	}
	return d
}

func (p ProjectView) renderTabs() string {
	var tabs []string
	for i, sess := range p.sessions {
		name := sess.Name
		if sess.run.working {
			name = "\u280b " + name // spinner for busy
		} else if sess.run.pending != nil || sess.run.question != nil {
			name = "! " + name // waiting for user
		} else if sess.run.todoTotal > 0 && sess.run.todoDone == sess.run.todoTotal {
			name = "\u2713 " + name // checkmark for done
		}
		if i == p.activeSession {
			tabs = append(tabs, activeTabStyle.Render("["+name+"]"))
		} else {
			tabs = append(tabs, tabStyle.Render(" "+name+" "))
		}
	}
	return strings.Join(tabs, " ")
}

func (p ProjectView) renderInput() string {
	w := p.width - 2
	if w < 20 {
		w = 78
	}
	box := inputStyle.Width(w)
	s := p.sessions[p.activeSession]
	switch {
	case s.run.question != nil && p.picker == nil:
		return box.BorderForeground(colPermission).Render(questionHint(s.run.question, w-4))
	case p.picker != nil:
		return box.BorderForeground(lipgloss.Color("205")).Render(mutedStyle.Render("\u2191\u2193 select \u00b7 Enter open session \u00b7 Esc cancel"))
	case p.inputMode:
		r := []rune(strings.ReplaceAll(p.input, "\n", "\u21b5"))
		if limit := w - 8; len(r) > limit {
			r = append([]rune("\u2026"), r[len(r)-limit:]...)
		}
		hint := p.acHint()
		if room := w - 10 - len(r); room > 6 {
			hint = clip(hint, room)
		} else {
			hint = ""
		}
		return box.BorderForeground(lipgloss.Color("229")).Render("\u203a " + string(r) + "\u258c" + mutedStyle.Render(hint))
	case s.run.pending != nil && s.run.pending.Typing:
		return box.BorderForeground(colPermission).Render("Reject reason \u203a " + tailClip(s.run.pending.Text, w-40) + "\u258c  (Enter send \u00b7 Esc back)")
	case s.run.pending != nil:
		return box.BorderForeground(colPermission).Render("Permission:  [y] once  [a] always  [n] reject  [f] reject + feedback")
	case s.run.working:
		return box.BorderForeground(lipgloss.Color("226")).Render(mutedStyle.Render("working\u2026  Esc aborts \u00b7 PgUp/PgDn scroll"))
	}
	if p.activePane != PaneChat {
		hint := "Esc/Ctrl+T back to chat \u00b7 \u2191\u2193 PgUp/PgDn move \u00b7 Enter open \u00b7 Ctrl+E editor \u00b7 Ctrl+F/V/D/G toggle panes"
		if p.activePane == PaneDiff {
			hint = "y/Enter accept hunk (stage) \u00b7 r r revert hunk \u00b7 \u2191\u2193 hunk \u00b7 PgUp/PgDn scroll \u00b7 u refresh \u00b7 Ctrl+T chat"
		}
		return box.BorderForeground(lipgloss.Color("240")).Render(mutedStyle.Render(lipgloss.NewStyle().MaxWidth(w - 4).Render(hint)))
	}
	hint := "Enter type (/help) \u00b7 Ctrl+R thinking \u00b7 Ctrl+X tool details \u00b7 PgUp/PgDn scroll \u00b7 Esc quit \u00b7 Ctrl+P/B plan/build \u00b7 Ctrl+L model \u00b7 Ctrl+F/V/D/G panes \u00b7 Ctrl+N/W session \u00b7 Ctrl+O history \u00b7 Ctrl+T panes"
	hint = lipgloss.NewStyle().MaxWidth(w - 4).Render(hint)
	return box.BorderForeground(lipgloss.Color("240")).Render(mutedStyle.Render(hint))
}
