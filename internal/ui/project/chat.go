package project

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"forge/internal/ui/status"
)

func (p *ProjectView) refreshStatus() {
	p.statusBar = p.statusBar.Update(p.statusData())
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
	if s.run.todoTotal > 0 {
		d.TodoDone, d.TodoTotal = s.run.todoDone, s.run.todoTotal
	}
	for _, m := range s.run.msgs {
		d.TokensIn += m.Tokens.Input + m.Tokens.Cache.Read + m.Tokens.Cache.Write
		d.TokensOut += m.Tokens.Output
		d.Cost += m.Cost
	}
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
		if i == p.activeSession {
			tabs = append(tabs, activeTabStyle.Render("["+sess.Name+"]"))
		} else {
			tabs = append(tabs, tabStyle.Render(" "+sess.Name+" "))
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
	case p.picker != nil:
		return box.BorderForeground(lipgloss.Color("205")).Render(mutedStyle.Render("↑↓ select · Enter open session · Esc cancel"))
	case p.inputMode:
		r := []rune(p.input)
		if limit := w - 8; len(r) > limit {
			r = append([]rune("…"), r[len(r)-limit:]...)
		}
		return box.BorderForeground(lipgloss.Color("229")).Render("› " + string(r) + "▌")
	case s.run.pending != nil:
		return box.BorderForeground(colPermission).Render("Permission requested:  [y] once   [a] always   [n] reject")
	case s.run.working:
		return box.BorderForeground(lipgloss.Color("226")).Render(mutedStyle.Render("working…  Esc aborts · PgUp/PgDn scroll"))
	}
	if p.activePane != PaneChat {
		hint := "Esc back to chat · ↑↓ PgUp/PgDn move · Enter open · Ctrl+E editor · Ctrl+F/V/D/G toggle panes"
		if p.activePane == PaneDiff {
			hint = "y/Enter accept hunk (stage) · r r revert hunk · ↑↓ hunk · PgUp/PgDn scroll · u refresh · Esc chat"
		}
		return box.BorderForeground(lipgloss.Color("240")).Render(mutedStyle.Render(lipgloss.NewStyle().MaxWidth(w - 4).Render(hint)))
	}
	hint := "Enter type · PgUp/PgDn scroll · Esc quit · Ctrl+P/B plan/build · Ctrl+L model · Ctrl+F/V/D/G panes · Ctrl+N/W session · Ctrl+O history"
	hint = lipgloss.NewStyle().MaxWidth(w - 4).Render(hint)
	return box.BorderForeground(lipgloss.Color("240")).Render(mutedStyle.Render(hint))
}
