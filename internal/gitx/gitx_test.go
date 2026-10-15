package gitx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func numbered(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func setup(t *testing.T) (string, *Repo) {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, dir, "a.txt", numbered(30))
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "init")
	r, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, r
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNotRepo(t *testing.T) {
	if _, err := Open(context.Background(), t.TempDir()); err != ErrNotRepo {
		t.Fatalf("got %v, want ErrNotRepo", err)
	}
}

func TestAcceptAndRevertHunks(t *testing.T) {
	ctx := context.Background()
	dir, r := setup(t)
	edited := strings.Replace(numbered(30), "line 3\n", "THREE\n", 1)
	edited = strings.Replace(edited, "line 27\n", "TWENTY-SEVEN\nextra\n", 1)
	write(t, dir, "a.txt", edited)

	files, err := r.Diff(ctx)
	if err != nil || len(files) != 1 || len(files[0].Hunks) != 2 {
		t.Fatalf("diff: %v %+v", err, files)
	}
	f := files[0]
	if f.Path != "a.txt" {
		t.Fatalf("path %q", f.Path)
	}

	// Accept hunk 1: it leaves the unstaged diff and lands in the index.
	if err := r.Accept(ctx, f, f.Hunks[0]); err != nil {
		t.Fatal(err)
	}
	files, _ = r.Diff(ctx)
	if len(files) != 1 || len(files[0].Hunks) != 1 || !strings.Contains(strings.Join(files[0].Hunks[0].Lines, "\n"), "TWENTY-SEVEN") {
		t.Fatalf("after accept: %+v", files)
	}
	if staged := git(t, dir, "diff", "--cached"); !strings.Contains(staged, "+THREE") || strings.Contains(staged, "TWENTY") {
		t.Fatalf("staged:\n%s", staged)
	}

	// Revert the remaining hunk: file keeps the accepted edit only.
	if err := r.Revert(ctx, files[0], files[0].Hunks[0]); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(numbered(30), "line 3\n", "THREE\n", 1)
	if got := read(t, dir, "a.txt"); got != want {
		t.Fatalf("file after revert:\n%s", got)
	}
	if files, _ = r.Diff(ctx); len(files) != 0 {
		t.Fatalf("expected clean, got %+v", files)
	}
}

func TestRevertFirstHunkWithShiftedSecond(t *testing.T) {
	ctx := context.Background()
	dir, r := setup(t)
	edited := strings.Replace(numbered(30), "line 3\n", "a\nb\nc\nd\n", 1)
	edited = strings.Replace(edited, "line 27\n", "changed\n", 1)
	write(t, dir, "a.txt", edited)
	files, _ := r.Diff(ctx)
	f := files[0]
	if len(f.Hunks) != 2 {
		t.Fatalf("hunks %d", len(f.Hunks))
	}
	// Reverting the first (stale offsets for the second) must still work.
	if err := r.Revert(ctx, f, f.Hunks[0]); err != nil {
		t.Fatal(err)
	}
	files, _ = r.Diff(ctx)
	if len(files) != 1 || len(files[0].Hunks) != 1 {
		t.Fatalf("%+v", files)
	}
	if err := r.Revert(ctx, files[0], files[0].Hunks[0]); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "a.txt"); got != numbered(30) {
		t.Fatalf("not restored:\n%s", got)
	}
}

func TestUntrackedAndDeleted(t *testing.T) {
	ctx := context.Background()
	dir, r := setup(t)
	write(t, dir, "sub/new.go", "package sub\n")
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	files, err := r.Diff(ctx)
	if err != nil || len(files) != 2 {
		t.Fatalf("%v %+v", err, files)
	}
	var del, add File
	for _, f := range files {
		if f.Untracked {
			add = f
		} else {
			del = f
		}
	}
	if add.Path != "sub/new.go" || len(add.Hunks) != 1 {
		t.Fatalf("untracked: %+v", add)
	}
	if del.Path != "a.txt" || len(del.Hunks) != 1 {
		t.Fatalf("deleted: %+v", del)
	}

	if err := r.Revert(ctx, del, del.Hunks[0]); err != nil {
		t.Fatal(err)
	}
	if read(t, dir, "a.txt") != numbered(30) {
		t.Fatal("deleted file not restored")
	}
	if err := r.Accept(ctx, add, add.Hunks[0]); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(git(t, dir, "status", "--porcelain"), "A  sub/new.go") {
		t.Fatal("new file not staged")
	}
	files, _ = r.Diff(ctx)
	if len(files) != 0 {
		t.Fatalf("expected clean, got %+v", files)
	}

	write(t, dir, "tmp.txt", "x\n")
	files, _ = r.Diff(ctx)
	if len(files) != 1 {
		t.Fatalf("%+v", files)
	}
	if err := r.Revert(ctx, files[0], Hunk{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tmp.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file not removed")
	}
}

func TestNoNewlineAtEOFAndSpaces(t *testing.T) {
	ctx := context.Background()
	dir, r := setup(t)
	write(t, dir, "my file.txt", "one\ntwo")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "two")
	write(t, dir, "my file.txt", "one\nTWO")
	files, err := r.Diff(ctx)
	if err != nil || len(files) != 1 || files[0].Path != "my file.txt" {
		t.Fatalf("%v %+v", err, files)
	}
	if err := r.Revert(ctx, files[0], files[0].Hunks[0]); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "my file.txt"); got != "one\ntwo" {
		t.Fatalf("got %q", got)
	}
}

func TestSubdirScopeAndBinary(t *testing.T) {
	ctx := context.Background()
	dir, _ := setup(t)
	write(t, dir, "proj/x.txt", "x\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "proj")
	write(t, dir, "proj/x.txt", "y\n")
	write(t, dir, "a.txt", "outside\n")
	if err := os.WriteFile(filepath.Join(dir, "proj", "b.bin"), []byte{0, 1, 2, 0}, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Open(ctx, filepath.Join(dir, "proj"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := r.Diff(ctx)
	if err != nil || len(files) != 2 {
		t.Fatalf("%v %+v", err, files)
	}
	if files[0].Path != "proj/x.txt" {
		t.Fatalf("path %q", files[0].Path)
	}
	if err := r.Revert(ctx, files[0], files[0].Hunks[0]); err != nil {
		t.Fatal(err)
	}
	if read(t, dir, "proj/x.txt") != "x\n" {
		t.Fatal("revert from subdir failed")
	}
	if b := files[1]; b.Path != "proj/b.bin" || len(b.Hunks) != 0 || b.Note != "binary file" {
		t.Fatalf("binary: %+v", b)
	}
}

func TestStatusMarks(t *testing.T) {
	ctx := context.Background()
	dir, r := setup(t)
	write(t, dir, "b.txt", "b\n")
	write(t, dir, "c.txt", "c\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "more")

	write(t, dir, "a.txt", "changed\n")     // unstaged modify
	write(t, dir, "b.txt", "staged only\n") // staged modify
	git(t, dir, "add", "b.txt")
	os.Remove(filepath.Join(dir, "c.txt")) // unstaged delete
	write(t, dir, "d/new.txt", "n\n")      // untracked in a dir
	write(t, dir, "e.txt", "e\n")          // staged add
	git(t, dir, "add", "e.txt")

	marks, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a.txt": "M", "b.txt": "m", "c.txt": "D", "d/new.txt": "??", "e.txt": "a"}
	if len(marks) != len(want) {
		t.Fatalf("marks %v", marks)
	}
	top := r.Top
	for p, m := range want {
		if got := marks[filepath.Join(top, p)]; got != m {
			t.Errorf("%s: got %q want %q", p, got, m)
		}
	}
}
