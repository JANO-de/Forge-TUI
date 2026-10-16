package lobby

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/registry"
)

func sample() Lobby {
	l := InitialLobby(100, 40)
	m, _ := l.Update(loadDataMsg{projects: []registry.Project{
		{Name: "Forge", Path: "/home/jano/Proyectos/Forge", GitBranch: "master", GitDirty: true,
			TodoProgress: "9/10", Model: "anthropic/claude", LastOpened: time.Now().Add(-3 * time.Hour)},
		{Name: "dynisland", Path: "/home/jano/Proyectos/dynisland", GitBranch: "main"},
	}})
	return m.(Lobby)
}

func TestViewAndNavigation(t *testing.T) {
	l := sample()
	v := l.View()
	for _, want := range []string{"███████╗", "Forge", "9/10", "master", "New Project", "3h ago"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
	if l.items[l.cursor].Type != ItemProject {
		t.Fatal("cursor should start on first project, not a separator")
	}
	for i := 0; i < 10; i++ {
		m, _ := l.Update(tea.KeyMsg{Type: tea.KeyDown})
		l = m.(Lobby)
		if l.items[l.cursor].Type == ItemSeparator {
			t.Fatal("cursor landed on separator")
		}
	}
	if l.items[l.cursor].Label != "Quit" {
		t.Errorf("want Quit at the end, got %s", l.items[l.cursor].Label)
	}
	if _, cmd := l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}); cmd == nil {
		t.Error("n should open the wizard")
	}
}

func TestSmallTerminal(t *testing.T) {
	l := sample()
	l.height = 14
	if strings.Contains(l.View(), "███████╗") {
		t.Error("short terminal should use the compact banner")
	}
}

func TestViewFitsHeight(t *testing.T) {
	for _, h := range []int{12, 20, 26, 30, 40} {
		l := sample()
		l.height = h
		if n := strings.Count(l.View(), "\n") + 1; n > h {
			t.Errorf("height %d: view has %d lines", h, n)
		}
	}
}
