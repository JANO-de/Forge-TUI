package registry

import (
	"testing"
	"time"
)

func TestSortedAndMostRecent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, b, c := t.TempDir(), t.TempDir(), t.TempDir()
	now := time.Now()
	r, _ := Load()
	_ = r.Add(Project{Path: a, Name: "a", LastOpened: now.Add(-3 * time.Hour)})
	_ = r.Add(Project{Path: b, Name: "b", LastOpened: now.Add(-1 * time.Hour)})
	_ = r.Add(Project{Path: c, Name: "c", LastOpened: now.Add(-2 * time.Hour), Pinned: true})
	got := Sorted(r.Projects)
	if got[0].Name != "c" || got[1].Name != "b" || got[2].Name != "a" {
		t.Errorf("order: %s %s %s", got[0].Name, got[1].Name, got[2].Name)
	}
	if m, ok := MostRecent(); !ok || m.Name != "b" {
		t.Errorf("MostRecent = %+v %v", m, ok)
	}
}
