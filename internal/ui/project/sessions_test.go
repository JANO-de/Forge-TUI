package project

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/api"
)

func TestLoadHistoryConvertsMessages(t *testing.T) {
	pv := InitialProjectView(t.TempDir(), 120, 40)
	s := &pv.sessions[0]
	pv.loadHistory(s, []api.MessageWithParts{
		{Info: api.Message{ID: "m1", Role: "user"}, Parts: []api.Part{
			{ID: "p1", Type: "text", Text: "add a flag"},
			{ID: "p1b", Type: "text", Text: "<injected file contents>", Synthetic: true},
		}},
		{Info: api.Message{ID: "m2", Role: "assistant", Tokens: api.Tokens{Input: 10, Output: 5}}, Parts: []api.Part{
			{ID: "p2", Type: "reasoning", Text: "thinking"},
			{ID: "p3", Type: "tool", Tool: "edit", State: api.ToolState{Status: "completed", Title: "main.go"}},
			{ID: "p4", Type: "text", Text: "Done."},
		}},
	})
	var kinds []string
	for _, m := range s.Messages {
		kinds = append(kinds, fmt.Sprintf("%d:%s", m.Type, m.Content))
	}
	got := strings.Join(kinds, "|")
	want := fmt.Sprintf("%d:add a flag|%d:thinking|%d:✓ edit: main.go|%d:Done.", MsgUser, MsgReasoning, MsgTool, MsgAssistant)
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if s.run.lastAssistant != "m2" || s.run.msgs["m2"].Tokens.Output != 5 {
		t.Fatal("assistant info not recorded for the status bar")
	}
	// A live update for a stored part replaces the line instead of appending.
	pv.upsertPart(s, api.Part{ID: "p4", Type: "text", Text: "Done. More."})
	if len(s.Messages) != 4 || s.Messages[3].Content != "Done. More." {
		t.Fatalf("part not updated in place: %+v", s.Messages)
	}
}

func TestPickerAgainstRealOpenCode(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not on PATH")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, k := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(k, "")
	}
	dir := t.TempDir()
	sh(t, dir, "init", "-q")

	pv := InitialProjectView(dir, 140, 40)
	defer pv.Shutdown()
	msg := pv.agent.start(dir)()
	if e, bad := msg.(agentErrMsg); bad {
		t.Fatal(e.err)
	}
	m, _ := pv.Update(msg) // ignore the blocking event-wait command
	pv = m.(ProjectView)

	// Two stored sessions; one has a message (noReply: no model is called).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := pv.agent.client
	old, err := c.CreateSession(ctx, "Older work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSession(ctx, "Empty one"); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"noReply":true,"parts":[{"type":"text","text":"hello from history"}]}`)
	resp, err := http.Post(c.BaseURL()+"/session/"+old.ID+"/message", "application/json", body)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("seed message: %v %v", err, resp)
	}
	resp.Body.Close()
	time.Sleep(200 * time.Millisecond)

	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlO})
	if pv.picker == nil || len(pv.picker.items) != 2 {
		t.Fatalf("picker: %+v", pv.picker)
	}
	if v := pv.View(); !strings.Contains(v, "Older work") || !strings.Contains(v, "Empty one") {
		t.Fatalf("picker not rendered:\n%s", v)
	}
	// Newest first: "Older work" was updated last (it got a message).
	if pv.picker.items[0].ID != old.ID {
		t.Fatalf("order: %+v", pv.picker.items)
	}

	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyEnter})
	if pv.picker != nil {
		t.Fatal("picker should close after loading")
	}
	if len(pv.sessions) != 1 || pv.sessions[0].RemoteID != old.ID || pv.sessions[0].Name != "Older work" {
		t.Fatalf("session not adopted into the empty first tab: %+v", pv.sessions)
	}
	if v := pv.View(); !strings.Contains(v, "hello from history") {
		t.Fatalf("history not shown:\n%s", v)
	}

	// Opening the same session again switches to it instead of duplicating.
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlO})
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyEnter})
	if len(pv.sessions) != 1 {
		t.Fatalf("duplicate tab: %d", len(pv.sessions))
	}

	// Another stored session opens in a new tab.
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyCtrlO})
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyDown})
	pv = send(t, pv, tea.KeyMsg{Type: tea.KeyEnter})
	if len(pv.sessions) != 2 || pv.activeSession != 1 || pv.sessions[1].Name != "Empty one" {
		t.Fatalf("second tab: %+v", pv.sessions)
	}
	_ = os.Remove
}
