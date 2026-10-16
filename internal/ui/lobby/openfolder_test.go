package lobby

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFolderBrowserOpen(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	l := sample()
	l.fb = newFolderBrowser(root)
	// rows: open, "..", alpha
	for i := 0; i < 2; i++ {
		m, _ := l.Update(tea.KeyMsg{Type: tea.KeyDown})
		l = m.(Lobby)
	}
	m, _ := l.Update(tea.KeyMsg{Type: tea.KeyEnter})
	l = m.(Lobby)
	want := filepath.Join(filepath.Clean(root), "alpha")
	if l.fb.dir != want {
		t.Fatalf("dir = %q, want %q", l.fb.dir, want)
	}
	_, cmd := l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg, ok := cmd().(SwitchToProjectMsg)
	if !ok || msg.Path != want {
		t.Fatalf("got %#v, want project %q", msg, want)
	}
	if l.View() == "" {
		t.Fatal("empty view")
	}
}
