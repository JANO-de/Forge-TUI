package lobby

import (
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"forge/internal/api"
	"forge/internal/registry"
)

var (
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	itemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	branchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("039"))
	progressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("046"))
)

type LobbyItemType int

const (
	ItemProject LobbyItemType = iota
	ItemSeparator
	ItemAction
)

type LobbyItem struct {
	Type        LobbyItemType
	Label       string
	Description string
	Project     *registry.Project
	Action      tea.Cmd
}

type SwitchToWizardMsg struct{}

type SwitchToProjectMsg struct {
	Path string
}

type Lobby struct {
	items     []LobbyItem
	cursor    int
	height    int
	width     int
	projects  []registry.Project
	models    []api.Model
	providers []api.Provider
	loading   bool
	err       error
}

func InitialLobby(width, height int) Lobby {
	return Lobby{
		height:  height,
		width:   width,
		cursor:  0,
		loading: true,
	}
}

func (l Lobby) Init() tea.Cmd {
	return l.loadData
}

type loadDataMsg struct {
	projects  []registry.Project
	models    []api.Model
	providers []api.Provider
	err       error
}

func (l Lobby) loadData() tea.Msg {
	reg, err := registry.Load()
	if err != nil {
		return loadDataMsg{err: err}
	}

	projects := reg.List()

	models := []api.Model{}
	providers := []api.Provider{}

	return loadDataMsg{
		projects:  projects,
		models:    models,
		providers: providers,
		err:       nil,
	}
}

func (l Lobby) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadDataMsg:
		l.loading = false
		l.err = msg.err
		l.projects = msg.projects
		l.models = msg.models
		l.providers = msg.providers
		l.buildItems()
		return l, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyShiftTab:
			if l.cursor > 0 {
				l.cursor--
			}
		case tea.KeyDown, tea.KeyTab:
			if l.cursor < len(l.items)-1 {
				l.cursor++
			}
		case tea.KeyEnter:
			return l.handleEnter()
		case tea.KeyCtrlC, tea.KeyEsc:
			return l.handleQuit()
		}

	case tea.WindowSizeMsg:
		l.height = msg.Height
		l.width = msg.Width
	}

	return l, nil
}

func (l *Lobby) buildItems() {
	l.items = nil

	if len(l.projects) > 0 {
		l.items = append(l.items, LobbyItem{Type: ItemSeparator, Label: "Recent Projects"})
		for _, p := range l.projects {
			l.items = append(l.items, LobbyItem{
				Type:        ItemProject,
				Label:       p.Name,
				Description: l.formatProjectDesc(p),
				Project:     &p,
				Action:      func(path string) tea.Cmd { return func() tea.Msg { return SwitchToProjectMsg{Path: path} } }(p.Path),
			})
		}
	}

	l.items = append(l.items, LobbyItem{Type: ItemSeparator, Label: "Actions"})
	l.items = append(l.items, LobbyItem{
		Type:        ItemAction,
		Label:       "New Project",
		Description: "Create a new Forge project",
		Action:      func() tea.Msg { return SwitchToWizardMsg{} },
	})
	l.items = append(l.items, LobbyItem{
		Type:        ItemAction,
		Label:       "Settings",
		Description: "Configure Forge settings",
		Action:      nil,
	})
	l.items = append(l.items, LobbyItem{
		Type:        ItemAction,
		Label:       "Quit",
		Description: "Exit Forge",
		Action:      tea.Quit,
	})
}

func (l Lobby) formatProjectDesc(p registry.Project) string {
	var parts []string

	if p.GitBranch != "" {
		branch := branchStyle.Render("[" + p.GitBranch + "]")
		if p.GitDirty {
			branch = dirtyStyle.Render("*") + branch
		}
		parts = append(parts, branch)
	}

	if p.Model != "" {
		parts = append(parts, mutedStyle.Render("model: "+p.Model))
	}

	if p.TodoProgress != "" {
		parts = append(parts, progressStyle.Render("todo: "+p.TodoProgress))
	}

	if !p.LastOpened.IsZero() {
		parts = append(parts, mutedStyle.Render("opened: "+p.LastOpened.Format("Jan 2 15:04")))
	}

	if len(parts) == 0 {
		return p.Path
	}

	return p.Path + "  " + strings.Join(parts, " · ")
}

func (l Lobby) handleEnter() (tea.Model, tea.Cmd) {
	if l.cursor >= 0 && l.cursor < len(l.items) {
		item := l.items[l.cursor]
		if item.Action != nil {
			return l, item.Action
		}
	}
	return l, nil
}

func (l Lobby) handleQuit() (tea.Model, tea.Cmd) {
	return l, tea.Quit
}

func (l Lobby) View() string {
	if l.loading {
		return titleStyle.Render("Forge Lobby") + "\n\nLoading..."
	}

	if l.err != nil {
		return titleStyle.Render("Forge Lobby") + "\n\nError: " + l.err.Error()
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render("Forge Lobby\n\n"))

	for i, item := range l.items {
		cursor := l.cursor == i

		switch item.Type {
		case ItemSeparator:
			s.WriteString(mutedStyle.Render("\n" + item.Label + "\n"))
		case ItemProject, ItemAction:
			prefix := "  "
			if cursor {
				prefix = "> "
			}

			line := prefix + item.Label
			if cursor {
				line = selectedStyle.Render(line)
			} else {
				line = itemStyle.Render(line)
			}
			s.WriteString(line)

			if item.Description != "" && cursor {
				s.WriteString("  " + mutedStyle.Render(item.Description))
			}
			s.WriteString("\n")
		}
	}

	s.WriteString("\n" + mutedStyle.Render("Press q to quit, ↑↓ to navigate, Enter to select"))
	return s.String()
}
