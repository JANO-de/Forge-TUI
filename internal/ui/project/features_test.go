package project

import (
	"testing"

	"forge/internal/api"
)

func TestMatchFiles(t *testing.T) {
	files := []string{"internal/api/client.go", "main.go", "internal/ui/main_view.go"}
	got := matchFiles(files, "main")
	if len(got) != 2 || got[0] != "main.go" {
		t.Fatalf("got %v", got)
	}
	if len(matchFiles(files, "zzz")) != 0 {
		t.Fatal("expected no match")
	}
}

func TestLastUser(t *testing.T) {
	mk := func(id, role, text string) api.MessageWithParts {
		return api.MessageWithParts{
			Info:  api.Message{ID: id, Role: role},
			Parts: []api.Part{{Type: "text", Text: text}},
		}
	}
	msgs := []api.MessageWithParts{mk("1", "user", "a"), mk("2", "assistant", "x"), mk("3", "user", "b"), mk("4", "assistant", "y")}
	if id, text := lastUser(msgs, ""); id != "3" || text != "b" {
		t.Fatalf("got %s %q", id, text)
	}
	if id, text := lastUser(msgs, "3"); id != "1" || text != "a" {
		t.Fatalf("got %s %q", id, text)
	}
}

func TestPromptQueues(t *testing.T) {
	var r runState
	a, b := &PermissionRequest{ID: "a"}, &PermissionRequest{ID: "b"}
	r.addPermission(a)
	r.addPermission(b)
	r.donePermission("a")
	if r.pending == nil || r.pending.ID != "b" {
		t.Fatal("second permission should become current")
	}
	q1, q2 := &QuestionPrompt{ID: "q1"}, &QuestionPrompt{ID: "q2"}
	r.addQuestion(q1)
	r.addQuestion(q2)
	r.doneQuestion("q1", false)
	if r.question != q2 || !q2.Active || !q1.Done {
		t.Fatal("second question should become active")
	}
	r.clearPrompts()
	if r.pending != nil || r.question != nil {
		t.Fatal("prompts should be cleared")
	}
}

func TestClip(t *testing.T) {
	if clip("héllo wörld", 5) != "héll…" {
		t.Fatalf("got %q", clip("héllo wörld", 5))
	}
	if clip("abc", 5) != "abc" {
		t.Fatal("short strings are unchanged")
	}
}
