package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"forge/internal/gitx"
)

var (
	diffFileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	diffHunkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	diffCurHunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	diffHintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	diffWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

// DiffPane shows the unstaged changes of the project, one hunk at a time
// selectable. It holds state only; git runs elsewhere (see project/diff.go).
type DiffPane struct {
	Files         []gitx.File
	Cursor        int // index into the flat list of hunks
	ScrollY       int
	Loaded        bool
	NotRepo       bool
	Err           string
	Status        string // result of the last action
	ConfirmRevert bool
}

type hunkRef struct{ file, hunk int }

func NewDiffPane() *DiffPane { return &DiffPane{} }

func (d *DiffPane) refs() []hunkRef {
	var out []hunkRef
	for fi, f := range d.Files {
		if f.Untracked && len(f.Hunks) == 0 {
			continue
		}
		for hi := range f.Hunks {
			out = append(out, hunkRef{fi, hi})
		}
	}
	return out
}

// HunkCount is the number of actionable hunks.
func (d *DiffPane) HunkCount() int { return len(d.refs()) }

// SetFiles replaces the diff, keeping the cursor on the same hunk when it
// still exists.
func (d *DiffPane) SetFiles(files []gitx.File) {
	var path, header string
	if f, h, ok := d.Current(); ok {
		path, header = f.Path, h.Header
	}
	d.Files, d.Loaded, d.NotRepo, d.Err = files, true, false, ""
	refs := d.refs()
	for i, r := range refs {
		if d.Files[r.file].Path == path && d.Files[r.file].Hunks[r.hunk].Header == header {
			d.Cursor = i
			return
		}
	}
	if d.Cursor >= len(refs) {
		d.Cursor = max(len(refs)-1, 0)
	}
	if len(refs) == 0 {
		d.ScrollY = 0
	}
}

// Current returns the hunk under the cursor.
func (d *DiffPane) Current() (gitx.File, gitx.Hunk, bool) {
	refs := d.refs()
	if d.Cursor < 0 || d.Cursor >= len(refs) {
		return gitx.File{}, gitx.Hunk{}, false
	}
	r := refs[d.Cursor]
	return d.Files[r.file], d.Files[r.file].Hunks[r.hunk], true
}

// Move changes the selected hunk by delta and scrolls it into view.
func (d *DiffPane) Move(delta, rows int) {
	n := d.HunkCount()
	if n == 0 {
		return
	}
	d.Cursor = min(max(d.Cursor+delta, 0), n-1)
	d.ConfirmRevert = false
	d.EnsureVisible(rows)
}

// Scroll moves the viewport by delta lines.
func (d *DiffPane) Scroll(delta, rows int) {
	lines, _ := d.render()
	d.ScrollY = min(max(d.ScrollY+delta, 0), max(len(lines)-rows, 0))
}

// EnsureVisible scrolls so the selected hunk is on screen (its top first
// when it is taller than the viewport).
func (d *DiffPane) EnsureVisible(rows int) {
	lines, spans := d.render()
	if d.Cursor < 0 || d.Cursor >= len(spans) {
		return
	}
	s := spans[d.Cursor]
	switch {
	case s[0] < d.ScrollY:
		d.ScrollY = s[0]
	case s[1] > d.ScrollY+rows:
		d.ScrollY = min(s[0], s[1]-rows)
	}
	d.ScrollY = min(max(d.ScrollY, 0), max(len(lines)-rows, 0))
}

// Title is the pane title with a summary of what is pending.
func (d *DiffPane) Title() string {
	if !d.Loaded || d.NotRepo || d.Err != "" {
		return "Diff"
	}
	n := d.HunkCount()
	if n == 0 {
		return "Diff · clean"
	}
	return fmt.Sprintf("Diff · %d/%d", d.Cursor+1, n)
}

// Lines renders the whole pane content.
func (d *DiffPane) Lines() []string {
	lines, _ := d.render()
	return lines
}

// render returns all lines plus, per actionable hunk, its [start,end) span.
func (d *DiffPane) render() ([]string, [][2]int) {
	switch {
	case d.NotRepo:
		return []string{mutedStyle.Render("Not a git repository: the diff view needs git."), mutedStyle.Render("Run `git init` in the project to use it.")}, nil
	case d.Err != "":
		return []string{deletedStyle.Render(d.Err), mutedStyle.Render("Press u to retry.")}, nil
	case !d.Loaded:
		return []string{mutedStyle.Render("Loading…")}, nil
	}
	var lines []string
	var spans [][2]int
	flat := 0
	if d.Status != "" {
		lines = append(lines, diffHintStyle.Render(d.Status), "")
	}
	if len(d.Files) == 0 {
		return append(lines, mutedStyle.Render("No uncommitted changes."), mutedStyle.Render("Accepted hunks are staged; use `git diff --cached` to see them.")), nil
	}
	for _, f := range d.Files {
		tag := ""
		switch {
		case f.Untracked:
			tag = " (new)"
		case f.Note != "":
			tag = " (" + f.Note + ")"
		}
		lines = append(lines, diffFileStyle.Render(f.Path+tag))
		if f.Untracked && len(f.Hunks) == 0 {
			lines = append(lines, mutedStyle.Render("  "+f.Note+", nothing to review"), "")
			continue
		}
		if len(f.Hunks) == 0 {
			lines = append(lines, mutedStyle.Render("  no text changes to review"), "")
			continue
		}
		for _, h := range f.Hunks {
			cur := flat == d.Cursor
			start := len(lines)
			add, del := h.Counts()
			head := fmt.Sprintf("%s  +%d -%d", h.Header, add, del)
			if cur {
				lines = append(lines, diffCurHunkStyle.Render("▌ "+head))
			} else {
				lines = append(lines, diffHunkStyle.Render("  "+head))
			}
			for _, l := range h.Lines {
				var body string
				switch {
				case strings.HasPrefix(l, "+"):
					body = addedStyle.Render(l)
				case strings.HasPrefix(l, "-"):
					body = deletedStyle.Render(l)
				default:
					body = mutedStyle.Render(l)
				}
				lines = append(lines, marker(cur)+body)
			}
			if cur {
				if d.ConfirmRevert {
					what := "this hunk"
					if f.Untracked {
						what = "this file (it will be deleted)"
					}
					lines = append(lines, diffWarnStyle.Render("▌ Revert "+what+"? r again to confirm, any key cancels"))
				} else {
					lines = append(lines, diffHintStyle.Render("▌ y accept · r revert · u refresh"))
				}
			}
			spans = append(spans, [2]int{start, len(lines)})
			lines = append(lines, "")
			flat++
		}
	}
	return lines, spans
}

func marker(cur bool) string {
	if cur {
		return diffCurHunkStyle.Render("▌") + " "
	}
	return "  "
}

// plain is used by tests.
func plain(lines []string) string {
	return strings.Join(lines, "\n")
}
