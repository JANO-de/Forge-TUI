package lobby

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/registry"
)

type palette struct {
	query  string
	cursor int
}

type command struct {
	label string
	run   func(l Lobby) (Lobby, tea.Cmd)
}

func commands() []command {
	return []command{
		{"New project", func(l Lobby) (Lobby, tea.Cmd) {
			return l, func() tea.Msg { return SwitchToWizardMsg{} }
		}},
		{"Open folder as project", func(l Lobby) (Lobby, tea.Cmd) {
			l.fb = newFolderBrowser(l.startDir())
			return l, nil
		}},
		{"Continue last project", func(l Lobby) (Lobby, tea.Cmd) {
			if p, ok := registry.MostRecent(); ok {
				return l, func() tea.Msg { return SwitchToProjectMsg{Path: p.Path} }
			}
			return l, nil
		}},
		{"Search projects", func(l Lobby) (Lobby, tea.Cmd) {
			l.filtering = true
			return l, nil
		}},
		{"Settings", func(l Lobby) (Lobby, tea.Cmd) {
			l.set = newSettingsForm()
			return l, nil
		}},
		{"Help", func(l Lobby) (Lobby, tea.Cmd) {
			l.help = true
			return l, nil
		}},
		{"Quit", func(l Lobby) (Lobby, tea.Cmd) { return l, tea.Quit }},
	}
}

func (p *palette) matches() []command {
	q := strings.ToLower(strings.TrimSpace(p.query))
	var out []command
	for _, c := range commands() {
		if q == "" || strings.Contains(strings.ToLower(c.label), q) {
			out = append(out, c)
		}
	}
	return out
}

func (l Lobby) paletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := l.pal
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlK:
		l.pal = nil
	case tea.KeyUp:
		p.cursor = max(p.cursor-1, 0)
	case tea.KeyDown:
		p.cursor = min(p.cursor+1, max(len(p.matches())-1, 0))
	case tea.KeyEnter:
		ms := p.matches()
		l.pal = nil
		if p.cursor < len(ms) {
			return ms[p.cursor].run(l)
		}
	case tea.KeyBackspace:
		if r := []rune(p.query); len(r) > 0 {
			p.query = string(r[:len(r)-1])
		}
		p.cursor = 0
	case tea.KeySpace:
		p.query += " "
		p.cursor = 0
	case tea.KeyRunes:
		p.query += string(msg.Runes)
		p.cursor = 0
	}
	return l, nil
}
