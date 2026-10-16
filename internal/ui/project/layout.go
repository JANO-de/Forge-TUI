package project

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Layout: [ files | chat | right pane ] over the input box and status bar.
// The chat is always visible; files and the right pane (viewer, diff or
// guidelines) are optional columns. On a narrow terminal the right pane
// replaces the chat instead of squeezing it.

type dims struct {
	fw, cw, rw int // column widths including borders; 0 = hidden
	body       int // column height including borders
}

func (p ProjectView) layout() dims {
	w := p.width
	if w < 40 {
		w = 80
	}
	d := dims{body: p.height - 5}
	if d.body < 6 {
		d.body = 6
	}
	rest := w
	if p.showFiles {
		d.fw = min(max(w/4, 24), 36)
		rest -= d.fw
	}
	if p.right == PaneChat {
		d.cw = rest
		return d
	}
	d.rw = rest * 45 / 100
	if rest-d.rw < 44 {
		d.rw = rest - 44
	}
	if d.rw < 24 {
		d.rw, d.cw = rest, 0 // too narrow: right pane replaces the chat
		return d
	}
	d.cw = rest - d.rw
	return d
}

func (p ProjectView) View() string {
	if p.err != nil {
		return titleStyle.Render("Project View") + "\n\nError: " + p.err.Error()
	}
	d := p.layout()
	var cols []string
	if d.fw > 0 {
		cols = append(cols, p.filesColumn(d))
	}
	if d.cw > 0 {
		cols = append(cols, p.chatColumn(d))
	}
	if d.rw > 0 {
		cols = append(cols, p.rightColumn(d))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	return p.renderTabs() + "\n" + body + "\n" + p.renderInput() + "\n" + p.statusBar.View()
}

// column draws a bordered column of exactly w x h cells with a title row.
func column(title string, lines []string, w, h int, focused bool) string {
	col := lipgloss.Color("240")
	titleCol := lipgloss.Color("245")
	if focused {
		col, titleCol = lipgloss.Color("205"), lipgloss.Color("205")
	}
	inner := w - 2
	rows := h - 2
	clip := lipgloss.NewStyle().MaxWidth(inner)
	out := make([]string, 0, rows)
	out = append(out, clip.Render(lipgloss.NewStyle().Foreground(titleCol).Bold(true).Render(title)))
	for _, l := range lines {
		if len(out) == rows {
			break
		}
		out = append(out, clip.Render(strings.ReplaceAll(l, "\t", "    ")))
	}
	for len(out) < rows {
		out = append(out, "")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(col).
		Width(inner).
		Render(strings.Join(out, "\n"))
}

func (p ProjectView) rows(d dims) int { return d.body - 3 }

func (p ProjectView) chatColumn(d dims) string {
	if p.picker != nil {
		return column("Sessions", p.pickerLines(), d.cw, d.body, p.activePane == PaneChat)
	}
	title := "Chat"
	if len(p.sessions) > 0 {
		title = "Chat · " + p.sessions[p.activeSession].Name
	}
	return column(title, strings.Split(p.renderChat(), "\n"), d.cw, d.body, p.activePane == PaneChat)
}

func (p ProjectView) filesColumn(d dims) string {
	var lines []string
	if p.fileTree == nil {
		lines = []string{mutedStyle.Render("File tree not available")}
	} else {
		all := strings.Split(strings.TrimRight(p.fileTree.View(), "\n"), "\n")
		rows := p.rows(d)
		start := 0
		if len(all) > rows {
			start = min(max(p.fileTree.Cursor-rows/2, 0), len(all)-rows)
		}
		lines = all[start:]
	}
	return column("Files", lines, d.fw, d.body, p.activePane == PaneFiles)
}

func (p ProjectView) rightColumn(d dims) string {
	title, lines := p.rightContent()
	rows := p.rows(d)
	off := p.rightScroll(len(lines), rows)
	if off > 0 {
		lines = lines[off:]
	}
	return column(title, lines, d.rw, d.body, p.activePane == p.right)
}

// rightContent returns the title and all lines of the right pane.
func (p ProjectView) rightContent() (string, []string) {
	switch p.right {
	case PaneViewer:
		if p.fileViewer == nil {
			return "Viewer", []string{mutedStyle.Render("No file selected. Open one from Files (Ctrl+F).")}
		}
		name, err := filepath.Rel(p.ProjectPath, p.fileViewer.Path)
		if err != nil {
			name = p.fileViewer.Path
		}
		return "Viewer · " + name, strings.Split(p.fileViewer.View(), "\n")
	case PaneDiff:
		return p.diff.Title(), p.diff.Lines()
	case PaneGuidelines:
		return "Guidelines · " + guideFiles[p.gd.tab], p.guidelineLines()
	}
	return "", nil
}

// rightScroll returns the clamped scroll offset of the right pane.
func (p ProjectView) rightScroll(total, rows int) int {
	off := 0
	switch p.right {
	case PaneViewer:
		if p.fileViewer != nil {
			off = p.fileViewer.ScrollY
		}
	case PaneGuidelines:
		off = p.guideScroll
	case PaneDiff:
		off = p.diff.ScrollY
	}
	return min(max(off, 0), max(total-rows, 0))
}

func (p *ProjectView) scrollRight(delta int) {
	_, lines := p.rightContent()
	cur := p.rightScroll(len(lines), p.rows(p.layout()))
	maxOff := max(len(lines)-p.rows(p.layout()), 0)
	next := min(max(cur+delta, 0), maxOff)
	switch p.right {
	case PaneViewer:
		if p.fileViewer != nil {
			p.fileViewer.ScrollY = next
		}
	case PaneGuidelines:
		p.guideScroll = next
	case PaneDiff:
		p.diff.ScrollY = next
	}
}

// navigate handles Up (-1) and Down (+1) for the focused pane.
func (p *ProjectView) navigate(dir int) {
	switch p.activePane {
	case PaneChat:
		p.scrollBy(-dir)
	case PaneFiles:
		if p.fileTree != nil {
			if dir < 0 {
				p.fileTree.MoveUp()
			} else {
				p.fileTree.MoveDown()
			}
		}
	case PaneViewer, PaneGuidelines:
		p.scrollRight(dir)
	}
}

func (p *ProjectView) page(dir int) {
	n := p.chatHeight() - 2
	switch p.activePane {
	case PaneChat:
		p.scrollBy(-dir * n)
	case PaneFiles:
		for i := 0; i < n; i++ {
			p.navigate(dir)
		}
	case PaneViewer, PaneGuidelines:
		p.scrollRight(dir * n)
	}
}

func (p *ProjectView) edge(top bool) {
	switch p.activePane {
	case PaneChat:
		if top {
			p.scrollBy(1 << 30)
		} else if len(p.sessions) > 0 {
			p.sessions[p.activeSession].scroll = 0
		}
	case PaneViewer, PaneGuidelines:
		if top {
			p.scrollRight(-(1 << 30))
		} else {
			p.scrollRight(1 << 30)
		}
	}
}
