package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/registry"
	"forge/internal/ui/lobby"
	"forge/internal/ui/project"
	"forge/internal/ui/wizard"
)

type screen int

const (
	screenLobby screen = iota
	screenWizard
	screenProject
)

type model struct {
	screen        screen
	width, height int
	lobby         lobby.Lobby
	wizard        wizard.Wizard
	project       project.ProjectView
}

// initialModel opens the lobby, or the project at path when one is given
// (`forge .` / `forge ~/Proyectos/Test`).
func initialModel(path string) model {
	m := model{screen: screenLobby, lobby: lobby.InitialLobby(0, 0)}
	if path != "" {
		_ = registry.Touch(path)
		m.screen = screenProject
		m.project = project.InitialProjectView(path, 0, 0)
	}
	return m
}

func (m model) Init() tea.Cmd {
	if m.screen == screenProject {
		return m.project.Init()
	}
	return m.lobby.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case lobby.SwitchToWizardMsg:
		m.screen = screenWizard
		m.wizard = wizard.InitialWizard(m.width, m.height)
		return m, m.wizard.Init()
	case wizard.DoneMsg:
		if msg.Path != "" {
			_ = registry.Touch(msg.Path)
		}
		m.screen = screenLobby
		m.lobby = lobby.InitialLobby(m.width, m.height)
		return m, m.lobby.Init()
	case lobby.SwitchToProjectMsg:
		_ = registry.Touch(msg.Path)
		m.screen = screenProject
		m.project = project.InitialProjectView(msg.Path, m.width, m.height)
		return m, m.project.Init()
	case tea.WindowSizeMsg:
		// Remember the size; each screen resizes itself in its own Update.
		// Never re-create a screen here: it would reset state (e.g. loading).
		m.width, m.height = msg.Width, msg.Height
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenLobby:
		var l tea.Model
		l, cmd = m.lobby.Update(msg)
		m.lobby = l.(lobby.Lobby)
	case screenWizard:
		var w tea.Model
		w, cmd = m.wizard.Update(msg)
		m.wizard = w.(wizard.Wizard)
	case screenProject:
		var p tea.Model
		p, cmd = m.project.Update(msg)
		m.project = p.(project.ProjectView)
	}
	return m, cmd
}

func (m model) View() string {
	switch m.screen {
	case screenLobby:
		return m.lobby.View()
	case screenWizard:
		return m.wizard.View()
	case screenProject:
		return m.project.View()
	}
	return ""
}

func main() {
	path := ""
	if len(os.Args) > 1 {
		abs, err := filepath.Abs(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			fmt.Fprintf(os.Stderr, "forge: %s is not a directory\n", abs)
			os.Exit(1)
		}
		path = abs
	}
	p := tea.NewProgram(initialModel(path), tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()
	if m, ok := final.(model); ok {
		m.project.Shutdown() // stop the opencode child process
	}
	if err != nil {
		panic(err)
	}
}
