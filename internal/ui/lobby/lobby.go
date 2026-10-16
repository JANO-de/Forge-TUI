package lobby

import (
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

	filter      string
	filtering   bool
	confirm     string // path pending removal
	confirmName string
	help        bool
	pal         *palette
	set         *settingsForm
	fb          *folderBrowser
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

	projects := registry.Sorted(reg.List())

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
		l.cursor = -1
		l.move(1)
		return l, nil

	case tea.KeyMsg:
		return l.handleKey(msg)

	case openFolderMsg:
		l.fb = newFolderBrowser(l.startDir())
		return l, nil

	case openSettingsMsg:
		l.set = newSettingsForm()
		return l, nil

	case tea.WindowSizeMsg:
		l.height = msg.Height
		l.width = msg.Width
	}

	return l, nil
}

// move shifts the cursor to the next selectable (non-separator) item.
func (l *Lobby) move(d int) {
	for i := l.cursor + d; i >= 0 && i < len(l.items); i += d {
		if l.items[i].Type != ItemSeparator {
			l.cursor = i
			return
		}
	}
}

func (l *Lobby) buildItems() {
	l.items = nil

	visible := l.visible()
	if len(visible) > 0 {
		l.items = append(l.items, LobbyItem{Type: ItemSeparator, Label: "Recent Projects"})
		for _, p := range visible {
			l.items = append(l.items, LobbyItem{
				Type:        ItemProject,
				Label:       p.Name,
				Description: p.Path,
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
		Label:       "Open Folder",
		Description: "Open an existing folder as a project",
		Action:      func() tea.Msg { return openFolderMsg{} },
	})
	l.items = append(l.items, LobbyItem{
		Type:        ItemAction,
		Label:       "Settings",
		Description: "Configure Forge settings",
		Action:      func() tea.Msg { return openSettingsMsg{} },
	})
	l.items = append(l.items, LobbyItem{
		Type:        ItemAction,
		Label:       "Quit",
		Description: "Exit Forge",
		Action:      tea.Quit,
	})
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
