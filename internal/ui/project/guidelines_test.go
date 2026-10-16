package project

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/api"
)

func guideProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Project: Demo\nUse gofmt.\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "HANDOFF.md"), []byte("# Handoff\n\n## TODO\n1. [x] Scaffold\n2. [ ] Write core\n3. [ ] Add tests\n"), 0o644)
	return dir
}

func TestGuidelinesTabsAndChecklist(t *testing.T) {
	pv := InitialProjectView(guideProject(t), 140, 40)
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlG})
	v := pv.View()
	if !strings.Contains(v, "[AGENTS.md]") || !strings.Contains(v, "Use gofmt") {
		t.Fatalf("AGENTS tab:\n%s", v)
	}
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyRight})
	v = pv.View()
	for _, want := range []string{"[HANDOFF.md]", "1/3", "✓", "☐", "Write core"} {
		if !strings.Contains(v, want) {
			t.Errorf("HANDOFF tab missing %q:\n%s", want, v)
		}
	}
	// The file changed on disk (e.g. by the agent): u reloads it.
	os.WriteFile(filepath.Join(pv.ProjectPath, "HANDOFF.md"), []byte("1. [x] a\n2. [x] b\n"), 0o644)
	pv = send(t, pv, key('u'))
	if !strings.Contains(pv.View(), "2/2") {
		t.Fatalf("not reloaded:\n%s", pv.View())
	}
}

func TestHandoffSendsBuildPromptAndFocusesChat(t *testing.T) {
	var mu sync.Mutex
	var promptBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/session":
			io.WriteString(w, `{"id":"s1"}`)
		case r.URL.Path == "/session/s1/prompt_async":
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			json.Unmarshal(b, &promptBody)
			mu.Unlock()
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	pv := InitialProjectView(guideProject(t), 140, 40)
	pv.agent.client, pv.agent.ready = api.New(srv.URL), true
	if pv.mode != ModePlan {
		t.Fatal("test expects Plan mode by default")
	}
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlG})
	pv = send(t, pv, key('h'))

	mu.Lock()
	defer mu.Unlock()
	if promptBody == nil {
		t.Fatal("no prompt sent")
	}
	if promptBody["agent"] != "build" {
		t.Fatalf("handoff must run in build mode, got %v", promptBody["agent"])
	}
	parts, _ := promptBody["parts"].([]any)
	text, _ := parts[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "HANDOFF.md") {
		t.Fatalf("prompt: %q", text)
	}
	if pv.activePane != PaneChat || pv.lastPane != PaneGuidelines || pv.right != PaneGuidelines {
		t.Fatalf("focus: active=%v last=%v right=%v", pv.activePane, pv.lastPane, pv.right)
	}
	if got := pv.sessions[0].Messages[0]; got.Type != MsgUser || got.Content != "/handoff" {
		t.Fatalf("chat shows %+v", got)
	}
	if pv.mode != ModePlan {
		t.Fatal("handoff must not switch the UI mode")
	}
}

func TestTypedHandoffCommand(t *testing.T) {
	pv := InitialProjectView(guideProject(t), 140, 40) // agent not ready
	pv.inputMode, pv.input = true, "/handoff"
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyEnter})
	msgs := pv.sessions[0].Messages
	if len(msgs) == 0 || msgs[len(msgs)-1].Type != MsgError || !strings.Contains(msgs[len(msgs)-1].Content, "not ready") {
		t.Fatalf("expected a not-ready note, got %+v", msgs)
	}
}
