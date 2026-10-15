package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sh(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// send feeds msg to the view and runs the resulting git commands to
// completion (batches are flattened). Commands that would block (tea.Tick,
// the SSE wait) are never produced because Init is not called.
func send(t *testing.T, pv ProjectView, msg tea.Msg) ProjectView {
	t.Helper()
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		if b, ok := m.(tea.BatchMsg); ok {
			for _, c := range b {
				if c != nil {
					queue = append(queue, c())
				}
			}
			continue
		}
		switch m.(type) {
		case nil, diffLoadedMsg, diffActionMsg, treeMarksMsg, sessionListMsg, sessionLoadedMsg:
		case tea.KeyMsg, tea.WindowSizeMsg:
		default:
			continue
		}
		nm, cmd := pv.Update(m)
		pv = nm.(ProjectView)
		if cmd != nil {
			queue = append(queue, cmd())
		}
	}
	return pv
}

func key(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestDiffPaneAcceptRevertFlow(t *testing.T) {
	dir := t.TempDir()
	sh(t, dir, "init", "-q")
	var base strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&base, "line %d\n", i)
	}
	file := filepath.Join(dir, "a.txt")
	os.WriteFile(file, []byte(base.String()), 0o644)
	sh(t, dir, "add", ".")
	sh(t, dir, "commit", "-qm", "init")
	edited := strings.Replace(base.String(), "line 3\n", "THREE\n", 1)
	edited = strings.Replace(edited, "line 27\n", "TWENTY-SEVEN\n", 1)
	os.WriteFile(file, []byte(edited), 0o644)

	pv := InitialProjectView(dir, 140, 40)
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlD})
	if pv.right != PaneDiff || pv.activePane != PaneDiff {
		t.Fatalf("diff pane not focused: right=%v active=%v", pv.right, pv.activePane)
	}
	view := pv.View()
	for _, want := range []string{"Diff · 1/2", "a.txt", "+THREE", "-line 3", "accept"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}

	// y accepts the first hunk: it is staged and disappears from the list.
	pv = send(t, pv, key('y'))
	if got := pv.diff.HunkCount(); got != 1 {
		t.Fatalf("hunks after accept = %d", got)
	}
	if staged := sh(t, dir, "diff", "--cached"); !strings.Contains(staged, "+THREE") {
		t.Fatalf("not staged:\n%s", staged)
	}

	// r needs a second r; another key cancels.
	pv = send(t, pv, key('r'))
	if !pv.diff.ConfirmRevert || !strings.Contains(pv.View(), "r again to confirm") {
		t.Fatal("no confirmation prompt")
	}
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyDown})
	if pv.diff.ConfirmRevert {
		t.Fatal("confirmation not cancelled")
	}
	if b, _ := os.ReadFile(file); !strings.Contains(string(b), "TWENTY-SEVEN") {
		t.Fatal("file changed without confirmation")
	}
	pv = send(t, pv, key('r'))
	pv = send(t, pv, key('r'))
	b, _ := os.ReadFile(file)
	if strings.Contains(string(b), "TWENTY-SEVEN") || !strings.Contains(string(b), "THREE") {
		t.Fatalf("revert wrong:\n%s", b)
	}
	if pv.diff.HunkCount() != 0 || !strings.Contains(pv.View(), "No uncommitted changes") {
		t.Fatalf("expected clean diff:\n%s", pv.View())
	}

	// Esc leaves the pane without closing it.
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyEsc})
	if pv.activePane != PaneChat || pv.right != PaneDiff {
		t.Fatal("Esc should refocus chat and keep the column")
	}
}

func TestDiffPaneNotARepo(t *testing.T) {
	pv := InitialProjectView(t.TempDir(), 120, 40)
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlD})
	if !strings.Contains(pv.View(), "Not a git repository") {
		t.Fatalf("missing message:\n%s", pv.View())
	}
}

func TestDiffScrollKeepsHunkVisible(t *testing.T) {
	dir := t.TempDir()
	sh(t, dir, "init", "-q")
	var base strings.Builder
	for i := 1; i <= 400; i++ {
		fmt.Fprintf(&base, "line %d\n", i)
	}
	file := filepath.Join(dir, "big.txt")
	os.WriteFile(file, []byte(base.String()), 0o644)
	sh(t, dir, "add", ".")
	sh(t, dir, "commit", "-qm", "init")
	edited := base.String()
	for i := 10; i <= 390; i += 40 {
		edited = strings.Replace(edited, fmt.Sprintf("line %d\n", i), fmt.Sprintf("EDIT %d\n", i), 1)
	}
	os.WriteFile(file, []byte(edited), 0o644)

	pv := InitialProjectView(dir, 120, 30)
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlD})
	n := pv.diff.HunkCount()
	if n < 5 {
		t.Fatalf("expected many hunks, got %d", n)
	}
	for i := 0; i < n-1; i++ {
		pv = send(t, pv, tea.KeyMsg{Type: tea.KeyDown})
	}
	if v := pv.View(); !strings.Contains(v, "EDIT 370") || !strings.Contains(v, fmt.Sprintf("Diff · %d/%d", n, n)) {
		t.Fatalf("last hunk not scrolled into view:\n%s", v)
	}
}

func TestFileTreeShowsGitMarks(t *testing.T) {
	dir := t.TempDir()
	sh(t, dir, "init", "-q")
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("a\n"), 0o644)
	sh(t, dir, "add", ".")
	sh(t, dir, "commit", "-qm", "init")
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("changed\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("n\n"), 0o644)

	pv := InitialProjectView(dir, 120, 30)
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlF})
	v := pv.View()
	if !strings.Contains(v, "a.go ●") || !strings.Contains(v, "new.go ?") {
		t.Fatalf("marks missing:\n%s", v)
	}
	// A file the "agent" creates appears after the next refresh.
	os.WriteFile(filepath.Join(dir, "agent.go"), []byte("x\n"), 0o644)
	pv = send(t, pv, treeMarksMsg{marks: map[string]string{}})
	if !strings.Contains(pv.View(), "agent.go") {
		t.Fatal("new file not listed after refresh")
	}
}
