package lobby

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/registry"
	"forge/internal/settings"
)

func press(l Lobby, k tea.KeyMsg) Lobby {
	m, _ := l.Update(k)
	return m.(Lobby)
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestSearchFilter(t *testing.T) {
	l := press(sample(), runes("/"))
	l = press(l, runes("dyn"))
	v := l.View()
	if !strings.Contains(v, "dynisland") || strings.Contains(v, "9/10") {
		t.Errorf("filter 'dyn' should keep only dynisland:\n%s", v)
	}
	l = press(l, tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(l.View(), "9/10") {
		t.Error("Esc should clear the filter")
	}
}

func TestHelpAndPalette(t *testing.T) {
	l := press(sample(), runes("?"))
	if !strings.Contains(l.View(), "command palette") {
		t.Error("help not shown")
	}
	l = press(l, runes("x"))
	if !strings.Contains(l.View(), "Recent Projects") {
		t.Error("any key should close help")
	}
	l = press(l, tea.KeyMsg{Type: tea.KeyCtrlK})
	if !strings.Contains(l.View(), "Commands") {
		t.Fatal("palette not shown")
	}
	l = press(l, runes("set"))
	l = press(l, tea.KeyMsg{Type: tea.KeyEnter})
	if l.set == nil || !strings.Contains(l.View(), "Default model") {
		t.Error("palette should open settings")
	}
}

func TestSettingsSave(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("EDITOR", "vi")
	l := Lobby{set: newSettingsForm(), width: 100, height: 40}
	l = press(l, runes("a/b"))
	l = press(l, tea.KeyMsg{Type: tea.KeyTab})
	l = press(l, runes("nvim"))
	l = press(l, tea.KeyMsg{Type: tea.KeyEnter})
	if l.set != nil {
		t.Fatal("form should close after save")
	}
	s, err := settings.Load()
	if err != nil || s.DefaultModel != "a/b" || s.Editor != "nvim" {
		t.Errorf("saved %+v %v", s, err)
	}
}

func TestPinAndRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r, _ := registry.Load()
	now := time.Now()
	_ = r.Add(Project("a", t.TempDir(), now.Add(-2*time.Hour)))
	_ = r.Add(Project("b", t.TempDir(), now.Add(-1*time.Hour)))

	l := InitialLobby(100, 40)
	mm, _ := l.Update(l.loadData())
	l = mm.(Lobby)
	if l.projects[0].Name != "b" {
		t.Fatalf("want b first, got %s", l.projects[0].Name)
	}
	l = press(l, runes("j")) // select a
	m, cmd := l.Update(runes("p"))
	if cmd == nil {
		t.Fatal("p should return a command")
	}
	m, _ = m.(Lobby).Update(cmd())
	l = m.(Lobby)
	if l.projects[0].Name != "a" || !l.projects[0].Pinned {
		t.Errorf("a should be pinned and first: %+v", l.projects[0])
	}

	l = press(l, runes("d"))
	if l.confirm == "" || !strings.Contains(l.View(), "Remove a") {
		t.Fatal("d should ask for confirmation")
	}
	m, cmd = l.Update(runes("y"))
	m, _ = m.(Lobby).Update(cmd())
	if got := len(m.(Lobby).projects); got != 1 {
		t.Errorf("want 1 project left, got %d", got)
	}
}

// Project builds a registry entry for tests.
func Project(name, path string, opened time.Time) registry.Project {
	return registry.Project{Name: name, Path: path, LastOpened: opened}
}
