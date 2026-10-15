package project

import (
	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/gitx"
)

type treeMarksMsg struct {
	marks map[string]string // nil when the project is not a git repo
}

// refreshTree rereads git status in the background; when it arrives the tree
// is rebuilt from disk and marked. Overlapping requests are coalesced.
func (p *ProjectView) refreshTree() tea.Cmd {
	if p.fileTree == nil {
		return nil
	}
	if p.tctl.busy {
		p.tctl.again = true
		return nil
	}
	p.tctl.busy = true
	dir := p.ProjectPath
	return func() tea.Msg {
		ctx, cancel := gitCtx()
		defer cancel()
		r, err := gitx.Open(ctx, dir)
		if err != nil {
			return treeMarksMsg{}
		}
		marks, err := r.Status(ctx)
		if err != nil {
			return treeMarksMsg{}
		}
		return treeMarksMsg{marks: marks}
	}
}

func (p *ProjectView) handleTreeMsg(msg tea.Msg) (tea.Cmd, bool) {
	m, ok := msg.(treeMarksMsg)
	if !ok {
		return nil, false
	}
	p.tctl.busy = false
	if p.fileTree != nil {
		p.fileTree.SetStatus(m.marks)
		p.fileTree.Refresh()
	}
	if p.tctl.again {
		p.tctl.again = false
		return p.refreshTree(), true
	}
	return nil, true
}
