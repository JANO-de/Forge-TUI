// Package gitx wraps the few git operations Forge needs for reviewing the
// agent's edits: list unstaged hunks, accept one (stage it) or revert one
// (undo it in the working tree). It shells out to git, stdlib only.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotRepo is returned by Open when dir is not inside a git work tree.
var ErrNotRepo = errors.New("not a git repository")

const (
	maxUntrackedFiles = 200
	maxUntrackedBytes = 512 << 10
)

// Hunk is one @@ block of a unified diff.
type Hunk struct {
	Header string   // "@@ -1,3 +1,4 @@ optional context"
	Lines  []string // body lines, each with its leading ' ', '+', '-' or '\'
}

// Counts returns the number of added and removed lines.
func (h Hunk) Counts() (added, removed int) {
	for _, l := range h.Lines {
		switch {
		case strings.HasPrefix(l, "+"):
			added++
		case strings.HasPrefix(l, "-"):
			removed++
		}
	}
	return
}

// File is the diff of one file against the index.
type File struct {
	Path      string   // relative to the repo root, slash separated
	Untracked bool     // new file git does not know yet
	Header    []string // "diff --git" line through "+++" line
	Hunks     []Hunk
	Note      string // why Hunks is empty: "binary file", "mode change", ...
}

// Repo is a git work tree plus the sub-directory Forge was opened in.
type Repo struct {
	Top string // absolute repo root
	Rel string // project dir relative to Top ("." when equal)
}

// Open locates the repo containing dir.
func Open(ctx context.Context, dir string) (*Repo, error) {
	out, err := run(ctx, dir, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, ErrNotRepo
	}
	top := strings.TrimSpace(out)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	// Resolve symlinks on both sides so Rel never starts with "..".
	if r, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = r
	}
	if r, err := filepath.EvalSymlinks(top); err == nil {
		top = r
	}
	rel, err := filepath.Rel(top, absDir)
	if err != nil {
		return nil, err
	}
	return &Repo{Top: top, Rel: filepath.ToSlash(rel)}, nil
}

func (r *Repo) pathspec() []string {
	if r.Rel == "." || r.Rel == "" {
		return nil
	}
	return []string{"--", r.Rel}
}

// Diff returns every change in the working tree that is not yet staged,
// including untracked files. Staged hunks (accepted ones) do not appear.
func (r *Repo) Diff(ctx context.Context) ([]File, error) {
	args := append([]string{"diff", "--no-color", "--no-ext-diff", "--no-renames", "-U3"}, r.pathspec()...)
	out, err := run(ctx, r.Top, nil, args...)
	if err != nil {
		return nil, err
	}
	files := Parse(out)

	lsArgs := append([]string{"ls-files", "--others", "--exclude-standard", "-z"}, r.pathspec()...)
	list, err := run(ctx, r.Top, nil, lsArgs...)
	if err != nil {
		return nil, err
	}
	n := 0
	for _, p := range strings.Split(list, "\x00") {
		if p == "" {
			continue
		}
		if n++; n > maxUntrackedFiles {
			break
		}
		files = append(files, r.untracked(ctx, p))
	}
	return files, nil
}

func (r *Repo) untracked(ctx context.Context, path string) File {
	f := File{Path: path, Untracked: true}
	st, err := os.Stat(filepath.Join(r.Top, filepath.FromSlash(path)))
	if err != nil {
		f.Note = "unreadable"
		return f
	}
	if st.Size() > maxUntrackedBytes {
		f.Note = "large new file"
		return f
	}
	// --no-index exits 1 when the files differ, which is the normal case.
	out, _ := run(ctx, r.Top, nil, "diff", "--no-index", "--no-color", "--", "/dev/null", path)
	parsed := Parse(out)
	if len(parsed) == 0 {
		f.Note = "empty file"
		return f
	}
	parsed[0].Path = path
	parsed[0].Untracked = true
	return parsed[0]
}

// Accept stages one hunk (git add -p equivalent). For an untracked file the
// whole file is staged.
func (r *Repo) Accept(ctx context.Context, f File, h Hunk) error {
	if f.Untracked {
		_, err := run(ctx, r.Top, nil, "add", "--", f.Path)
		return err
	}
	_, err := run(ctx, r.Top, strings.NewReader(Patch(f, h)), "apply", "--cached", "--recount", "--whitespace=nowarn", "-")
	return err
}

// Revert undoes one hunk in the working tree (git restore -p equivalent).
// For an untracked file the file is deleted.
func (r *Repo) Revert(ctx context.Context, f File, h Hunk) error {
	if f.Untracked {
		return os.Remove(filepath.Join(r.Top, filepath.FromSlash(f.Path)))
	}
	_, err := run(ctx, r.Top, strings.NewReader(Patch(f, h)), "apply", "-R", "--recount", "--whitespace=nowarn", "-")
	return err
}

// Patch renders a single-hunk patch for f that git apply accepts.
func Patch(f File, h Hunk) string {
	var b strings.Builder
	for _, l := range f.Header {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	b.WriteString(h.Header)
	b.WriteByte('\n')
	for _, l := range h.Lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// Parse splits `git diff` output into files and hunks.
func Parse(out string) []File {
	var files []File
	var cur *File
	inHunk := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			files = append(files, File{Header: []string{line}, Path: pathFromGitLine(line)})
			cur = &files[len(files)-1]
			inHunk = false
		case cur == nil:
		case strings.HasPrefix(line, "@@ "):
			cur.Hunks = append(cur.Hunks, Hunk{Header: line})
			inHunk = true
		case inHunk:
			if line == "" {
				continue // trailing newline of the whole output
			}
			switch line[0] {
			case ' ', '+', '-', '\\':
				h := &cur.Hunks[len(cur.Hunks)-1]
				h.Lines = append(h.Lines, line)
			}
		default:
			cur.Header = append(cur.Header, line)
			switch {
			case strings.HasPrefix(line, "+++ b/"):
				cur.Path = strings.TrimSuffix(line[len("+++ b/"):], "\t")
			case strings.HasPrefix(line, "--- a/") && cur.Path == "":
				cur.Path = strings.TrimSuffix(line[len("--- a/"):], "\t")
			case strings.HasPrefix(line, "Binary files "), strings.HasPrefix(line, "GIT binary patch"):
				cur.Note = "binary file"
			case strings.HasPrefix(line, "new mode "), strings.HasPrefix(line, "old mode "):
				if cur.Note == "" {
					cur.Note = "mode change"
				}
			}
		}
	}
	return files
}

// pathFromGitLine extracts X from "diff --git a/X b/X".
func pathFromGitLine(line string) string {
	s := strings.TrimPrefix(line, "diff --git ")
	if n := (len(s) - 5) / 2; len(s) >= 5 && len(s) == 2*n+5 && s[2+n:5+n] == " b/" && s[2:2+n] == s[5+n:] {
		return s[2 : 2+n]
	}
	if i := strings.LastIndex(s, " b/"); i >= 0 {
		return s[i+3:]
	}
	return s
}

func run(ctx context.Context, dir string, stdin *strings.Reader, args ...string) (string, error) {
	full := append([]string{"-c", "core.quotepath=false", "-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("git %s: %s", args[0], firstLine(msg))
	}
	return out.String(), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Status returns one mark per changed path, keyed by absolute path:
// "M" modified, "A" added, "D" deleted, "??" untracked, "U" conflict.
// A lower-case mark ("m", "a", "d") means everything is staged already
// (all hunks accepted); an upper-case mark means unstaged changes remain.
func (r *Repo) Status(ctx context.Context) (map[string]string, error) {
	args := append([]string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}, r.pathspec()...)
	out, err := run(ctx, r.Top, nil, args...)
	if err != nil {
		return nil, err
	}
	marks := map[string]string{}
	entries := strings.Split(out, "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		x, y, path := e[0], e[1], e[3:]
		if x == 'R' || x == 'C' {
			i++ // the next field is the origin path
		}
		marks[filepath.Join(r.Top, filepath.FromSlash(path))] = mark(x, y)
	}
	return marks, nil
}

func mark(x, y byte) string {
	switch {
	case x == '?' && y == '?':
		return "??"
	case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
		return "U"
	}
	code := func(c byte) string {
		switch c {
		case 'A':
			return "A"
		case 'D':
			return "D"
		default:
			return "M" // modified, renamed, copied, type change
		}
	}
	if y != ' ' {
		return code(y)
	}
	return strings.ToLower(code(x))
}
