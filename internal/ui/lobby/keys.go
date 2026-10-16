package lobby

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/registry"
)

type openSettingsMsg struct{}

// visible returns the projects matching the search filter.
func (l Lobby) visible() []registry.Project {
	f := strings.ToLower(strings.TrimSpace(l.filter))
	if f == "" {
		return l.projects
	}
	var out []registry.Project
	for _, p := range l.projects {
		if strings.Contains(strings.ToLower(p.Name), f) || strings.Contains(strings.ToLower(p.Path), f) {
			out = append(out, p)
		}
	}
	return out
}

func (l *Lobby) rebuild() {
	l.buildItems()
	l.cursor = -1
	l.move(1)
}

func (l Lobby) selectedProject() *registry.Project {
	if l.cursor >= 0 && l.cursor < len(l.items) {
		return l.items[l.cursor].Project
	}
	return nil
}

func (l Lobby) footer() string {
	switch {
	case l.confirm != "":
		return errStyle.Render("Remove " + l.confirmName + " from the list? Files are kept.  y / N")
	case l.filtering:
		return mutedStyle.Render("search: ") + itemStyle.Render(l.filter+"█") + mutedStyle.Render("   Enter keep · Esc clear")
	case l.filter != "":
		return mutedStyle.Render("filter: " + l.filter + " · / edit · Esc clear")
	}
	return mutedStyle.Render("↑↓ move · Enter open · / search · p pin · d remove · n new · o open folder · ? help · q quit")
}

func (l Lobby) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case l.fb != nil:
		return l.folderKey(msg)
	case l.set != nil:
		return l.settingsKey(msg)
	case l.pal != nil:
		return l.paletteKey(msg)
	case l.help:
		l.help = false
		return l, nil
	case l.confirm != "":
		path := l.confirm
		l.confirm, l.confirmName = "", ""
		if msg.Type == tea.KeyRunes && string(msg.Runes) == "y" {
			if r, err := registry.Load(); err == nil {
				_ = r.Remove(path)
			}
			return l, l.loadData
		}
		return l, nil
	case l.filtering:
		return l.filterKey(msg)
	}

	switch msg.Type {
	case tea.KeyCtrlK:
		l.pal = &palette{}
	case tea.KeyUp, tea.KeyShiftTab:
		l.move(-1)
	case tea.KeyDown, tea.KeyTab:
		l.move(1)
	case tea.KeyEnter:
		return l.handleEnter()
	case tea.KeyEsc:
		if l.filter != "" {
			l.filter = ""
			l.rebuild()
			return l, nil
		}
		return l.handleQuit()
	case tea.KeyCtrlC:
		return l.handleQuit()
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "j":
			l.move(1)
		case "k":
			l.move(-1)
		case "q":
			return l.handleQuit()
		case "n":
			return l, func() tea.Msg { return SwitchToWizardMsg{} }
		case "o":
			return l, func() tea.Msg { return openFolderMsg{} }
		case "/":
			l.filtering = true
		case "?":
			l.help = true
		case "p":
			if p := l.selectedProject(); p != nil {
				path, snapshot := p.Path, l
				return l, func() tea.Msg {
					if r, err := registry.Load(); err == nil {
						_ = r.Update(path, func(pr *registry.Project) { pr.Pinned = !pr.Pinned })
					}
					return snapshot.loadData()
				}
			}
		case "d":
			if p := l.selectedProject(); p != nil {
				l.confirm, l.confirmName = p.Path, p.Name
			}
		}
	}
	return l, nil
}

func (l Lobby) filterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		l.filtering = false
		return l, nil
	case tea.KeyEsc:
		l.filtering, l.filter = false, ""
	case tea.KeyUp:
		l.move(-1)
		return l, nil
	case tea.KeyDown:
		l.move(1)
		return l, nil
	case tea.KeyBackspace:
		if r := []rune(l.filter); len(r) > 0 {
			l.filter = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		l.filter += " "
	case tea.KeyRunes:
		l.filter += string(msg.Runes)
	default:
		return l, nil
	}
	l.rebuild()
	return l, nil
}
