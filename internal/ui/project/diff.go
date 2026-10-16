package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/api"
	"forge/internal/gitx"
)

// diffCtl is shared by every copy of ProjectView. It only coalesces refreshes
// so a burst of file.edited events does not spawn a burst of git processes.
type diffCtl struct {
	busy, again bool
}

type diffLoadedMsg struct {
	files   []gitx.File
	notRepo bool
	err     error
}

type diffActionMsg struct {
	accept bool
	abs    string // absolute path of the touched file
	rel    string
	err    error
}

func gitCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// refreshDiff reloads the diff, or marks one more reload when one is running.
func (p *ProjectView) refreshDiff() tea.Cmd {
	if p.dctl.busy {
		p.dctl.again = true
		return nil
	}
	p.dctl.busy = true
	dir := p.ProjectPath
	return func() tea.Msg {
		ctx, cancel := gitCtx()
		defer cancel()
		r, err := gitx.Open(ctx, dir)
		if errors.Is(err, gitx.ErrNotRepo) {
			return diffLoadedMsg{notRepo: true}
		}
		if err != nil {
			return diffLoadedMsg{err: err}
		}
		files, err := r.Diff(ctx)
		return diffLoadedMsg{files: files, err: err}
	}
}

// gitAfterEvent refreshes the open diff and the file tree marks when the
// agent touched files or finished a run.
func (p *ProjectView) gitAfterEvent(ev api.Event) tea.Cmd {
	if ev.Type != api.EventFileEdited && ev.Type != api.EventSessionIdle {
		return nil
	}
	return tea.Batch(p.refreshTree(), p.refreshDiffIfOpen())
}

func (p *ProjectView) refreshDiffIfOpen() tea.Cmd {
	if p.right != PaneDiff {
		return nil
	}
	return p.refreshDiff()
}

func (p *ProjectView) handleDiffMsg(msg tea.Msg) (tea.Cmd, bool) {
	rows := p.rows(p.layout())
	switch msg := msg.(type) {
	case diffLoadedMsg:
		p.dctl.busy = false
		switch {
		case msg.notRepo:
			p.diff.NotRepo, p.diff.Loaded = true, true
		case msg.err != nil:
			p.diff.Err, p.diff.Loaded = "git: "+msg.err.Error(), true
		default:
			p.diff.SetFiles(msg.files)
			p.diff.EnsureVisible(rows)
		}
		if p.dctl.again {
			p.dctl.again = false
			return p.refreshDiff(), true
		}
		return nil, true
	case diffActionMsg:
		verb := "reverted"
		if msg.accept {
			verb = "accepted"
		}
		if msg.err != nil {
			p.diff.Status = verb + " failed: " + msg.err.Error()
		} else {
			p.diff.Status = verb + " a hunk in " + msg.rel
			if p.fileViewer != nil && sameFile(p.fileViewer.Path, msg.abs) {
				_ = p.fileViewer.Reload()
			}
		}
		return tea.Batch(p.refreshDiff(), p.refreshTree()), true
	}
	return nil, false
}

// handleDiffKey handles keys while the diff pane has focus. ok=false lets the
// generic handler deal with the key (Esc, Ctrl+D, ...).
func (p *ProjectView) handleDiffKey(msg tea.KeyMsg) (cmd tea.Cmd, ok bool) {
	d := p.diff
	rows := p.rows(p.layout())
	confirming := d.ConfirmRevert
	d.ConfirmRevert = false

	switch msg.Type {
	case tea.KeyUp:
		d.Move(-1, rows)
		return nil, true
	case tea.KeyDown:
		d.Move(1, rows)
		return nil, true
	case tea.KeyPgUp:
		d.Scroll(-(rows - 1), rows)
		return nil, true
	case tea.KeyPgDown:
		d.Scroll(rows-1, rows)
		return nil, true
	case tea.KeyHome:
		d.Move(-(1 << 20), rows)
		return nil, true
	case tea.KeyEnd:
		d.Move(1<<20, rows)
		return nil, true
	case tea.KeyEnter:
		return p.diffAct(true), true
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return nil, false
		}
		switch msg.Runes[0] {
		case 'k':
			d.Move(-1, rows)
		case 'j':
			d.Move(1, rows)
		case 'y':
			return p.diffAct(true), true
		case 'A':
			return p.diffActAll(true), true
		case 'R':
			return p.diffActAll(false), true
		case 'u':
			d.Status = ""
			return p.refreshDiff(), true
		case 'r':
			if _, _, has := d.Current(); !has {
				return nil, true
			}
			if confirming {
				return p.diffAct(false), true
			}
			d.ConfirmRevert = true
			d.EnsureVisible(rows)
		default:
			return nil, true // swallow other letters while the diff has focus
		}
		return nil, true
	}
	return nil, false
}

// diffAct accepts (stages) or reverts the selected hunk.
func (p *ProjectView) diffAct(accept bool) tea.Cmd {
	f, h, ok := p.diff.Current()
	if !ok {
		return nil
	}
	dir := p.ProjectPath
	return func() tea.Msg {
		ctx, cancel := gitCtx()
		defer cancel()
		r, err := gitx.Open(ctx, dir)
		if err != nil {
			return diffActionMsg{accept: accept, rel: f.Path, err: err}
		}
		if accept {
			err = r.Accept(ctx, f, h)
		} else {
			err = r.Revert(ctx, f, h)
		}
		return diffActionMsg{accept: accept, abs: filepath.Join(r.Top, filepath.FromSlash(f.Path)), rel: f.Path, err: err}
	}
}

func sameFile(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	return err == nil && os.SameFile(sa, sb)
}

// diffActAll accepts or reverts all hunks in all files
func (p *ProjectView) diffActAll(accept bool) tea.Cmd {
	if p.diff == nil || !p.diff.Loaded || p.diff.NotRepo {
		return nil
	}
	files := p.diff.Files
	dir := p.ProjectPath
	return func() tea.Msg {
		ctx, cancel := gitCtx()
		defer cancel()
		r, err := gitx.Open(ctx, dir)
		if err != nil {
			return diffActionMsg{accept: accept, rel: "all files", err: err}
		}
		for _, f := range files {
			for _, h := range f.Hunks {
				if accept {
					r.Accept(ctx, f, h)
				} else {
					r.Revert(ctx, f, h)
				}
			}
		}
		return diffActionMsg{accept: accept, rel: "all files", err: nil}
	}
}
