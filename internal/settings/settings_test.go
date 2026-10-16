package settings

import (
	"os"
	"testing"
)

func TestRoundTripAndEditor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("EDITOR", "vi")
	if s, err := Load(); err != nil || s != (Settings{}) {
		t.Fatalf("empty load: %v %+v", err, s)
	}
	if err := Save(Settings{DefaultModel: " a/b ", Editor: "nvim"}); err != nil {
		t.Fatal(err)
	}
	s, err := Load()
	if err != nil || s.DefaultModel != "a/b" || s.Editor != "nvim" {
		t.Fatalf("got %+v %v", s, err)
	}
	ApplyEditor()
	if os.Getenv("EDITOR") != "nvim" {
		t.Error("ApplyEditor did not set EDITOR")
	}
}
