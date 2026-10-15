package panes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func touch(t *testing.T, root, rel string) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func names(ft *FileTree) []string {
	var out []string
	for _, n := range ft.Visible {
		out = append(out, n.Name)
	}
	return out
}

func TestExpandTwiceDoesNotDuplicate(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "pkg/a.go")
	touch(t, root, "pkg/b.go")
	ft, err := NewFileTree(root)
	if err != nil {
		t.Fatal(err)
	}
	ft.Cursor = 1 // pkg
	ft.ToggleExpand()
	ft.ToggleExpand() // collapse
	ft.ToggleExpand() // expand again
	got := strings.Join(names(ft), ",")
	if want := filepath.Base(root) + ",pkg,a.go,b.go"; got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRefreshKeepsStateAndFindsNewFiles(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "pkg/a.go")
	touch(t, root, "z.txt")
	ft, _ := NewFileTree(root)
	ft.Cursor = 1
	ft.ToggleExpand() // pkg open
	ft.Cursor = len(ft.Visible) - 1
	touch(t, root, "pkg/new.go")
	touch(t, root, "aaa.txt")
	ft.Refresh()
	got := strings.Join(names(ft), ",")
	if !strings.Contains(got, "pkg,a.go,new.go,aaa.txt") && !strings.Contains(got, "a.go,new.go") {
		t.Fatalf("new files missing: %s", got)
	}
	if sel := ft.GetSelected(); sel == nil || sel.Name != "z.txt" {
		t.Fatalf("selection lost: %+v", sel)
	}
}

func TestMarksAndDirectoryRollup(t *testing.T) {
	root := t.TempDir()
	mod := touch(t, root, "pkg/a.go")
	touch(t, root, "pkg/b.go")
	untr := touch(t, root, "tmp/n.txt")
	ft, _ := NewFileTree(root)
	ft.SetStatus(map[string]string{mod: "M", untr: "??"})
	var pkg, tmp *FileNode
	for _, n := range ft.Visible {
		switch n.Name {
		case "pkg":
			pkg = n
		case "tmp":
			tmp = n
		}
	}
	if pkg.GitStatus != "M" || tmp.GitStatus != "??" {
		t.Fatalf("dir marks: pkg=%q tmp=%q", pkg.GitStatus, tmp.GitStatus)
	}
	ft.Cursor = 1
	ft.ToggleExpand() // expanding must apply marks to freshly loaded children
	for _, n := range pkg.Children {
		want := ""
		if n.Name == "a.go" {
			want = "M"
		}
		if n.GitStatus != want {
			t.Errorf("%s: %q want %q", n.Name, n.GitStatus, want)
		}
	}
	ft.SetStatus(map[string]string{mod: "m"})
	if pkg.GitStatus != "m" {
		t.Fatalf("all-staged dir should be m, got %q", pkg.GitStatus)
	}
	if !strings.Contains(ft.View(), "●") {
		t.Fatal("mark not rendered")
	}
}
