package lobby

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"forge/internal/registry"
)

const cardWidth = 78

var (
	cardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	cardSelStyle = cardStyle.BorderForeground(lipgloss.Color("208"))
	headStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func (l Lobby) listView() string {
	w := l.width
	if w <= 0 {
		w = 80
	}
	head := banner(w, l.height)
	if l.loading {
		return head + "\n\n" + center(w, mutedStyle.Render("Loading…"))
	}
	if l.err != nil {
		return head + "\n\n" + center(w, errStyle.Render("Error: "+l.err.Error()))
	}

	cw := min(cardWidth, max(w-4, 30))
	body := l.renderBody(cw)
	// scroll body so the selected row stays visible
	rows := strings.Split(body, "\n")
	avail := max(l.height-lipgloss.Height(head)-5, 3) // 1 top margin + 2 blanks + help + 1 spare
	if l.height > 0 && len(rows) > avail {
		sel := l.selectedRow(cw)
		start := min(max(sel-avail/2, 0), len(rows)-avail)
		rows = rows[start : start+avail]
	}
	var out []string
	for _, r := range rows {
		out = append(out, center(w, lipgloss.NewStyle().Width(cw).Render(r)))
	}
	help := l.footer()
	return "\n" + head + "\n\n" + strings.Join(out, "\n") + "\n\n" + center(w, help)
}

func center(w int, s string) string { return lipgloss.PlaceHorizontal(w, lipgloss.Center, s) }

// renderBody draws sections; selectedRow depends on the same layout.
func (l Lobby) renderBody(cw int) string {
	var b []string
	for i, it := range l.items {
		switch it.Type {
		case ItemSeparator:
			b = append(b, "", headStyle.Render("▌ "+it.Label))
		case ItemProject:
			b = append(b, l.projectCard(it, i == l.cursor, cw))
		case ItemAction:
			b = append(b, l.actionRow(it, i == l.cursor, cw))
		}
	}
	return strings.TrimLeft(strings.Join(b, "\n"), "\n")
}

func (l Lobby) selectedRow(cw int) int {
	row := 0
	for i, it := range l.items {
		h := 1
		switch it.Type {
		case ItemSeparator:
			h = 2
		case ItemProject:
			h = 5
		}
		if i == l.cursor {
			return row + h/2
		}
		row += h
	}
	return row
}

func (l Lobby) projectCard(it LobbyItem, sel bool, cw int) string {
	p := it.Project
	title := p.Name
	if p.Pinned {
		title = "★ " + title
	}
	name := itemStyle.Render(title)
	if sel {
		name = selectedStyle.Render("▸ " + title)
	}
	line1 := name
	if p.GitBranch != "" {
		br := branchStyle.Render(" " + p.GitBranch)
		if p.GitDirty {
			br += dirtyStyle.Render(" ●")
		}
		line1 += "  " + br
	}
	if ago := ago(p); ago != "" {
		pad := cw - 4 - lipgloss.Width(line1) - lipgloss.Width(ago)
		line1 += strings.Repeat(" ", max(pad, 2)) + mutedStyle.Render(ago)
	}
	line2 := mutedStyle.Render(shorten(p.Path, cw-6))
	meta := []string{}
	if bar := todoBar(p.TodoProgress); bar != "" {
		meta = append(meta, bar)
	}
	if p.Model != "" {
		meta = append(meta, mutedStyle.Render(p.Model))
	}
	content := line1 + "\n" + line2
	if len(meta) > 0 {
		content += "\n" + strings.Join(meta, mutedStyle.Render("  ·  "))
	}
	st := cardStyle
	if sel {
		st = cardSelStyle
	}
	return st.Width(cw - 2).Render(content)
}

func (l Lobby) actionRow(it LobbyItem, sel bool, cw int) string {
	icons := map[string]string{"New Project": "＋", "Settings": "⚙", "Quit": "⏻"}
	label := fmt.Sprintf("  %s  %s", icons[it.Label], it.Label)
	if sel {
		label = selectedStyle.Render("▸ "+strings.TrimLeft(label, " ")) + "  " + mutedStyle.Render(it.Description)
	} else {
		label = itemStyle.Render(label)
	}
	return " " + label
}

// todoBar turns "3/10" into a small progress bar.
func todoBar(s string) string {
	a, b, ok := strings.Cut(s, "/")
	if !ok {
		if s == "" {
			return ""
		}
		return progressStyle.Render("todo " + s)
	}
	d, e1 := strconv.Atoi(strings.TrimSpace(a))
	t, e2 := strconv.Atoi(strings.TrimSpace(b))
	if e1 != nil || e2 != nil || t <= 0 {
		return progressStyle.Render("todo " + s)
	}
	const w = 10
	fill := min(d*w/t, w)
	return progressStyle.Render(strings.Repeat("█", fill)) + mutedStyle.Render(strings.Repeat("░", w-fill)) +
		" " + progressStyle.Render(fmt.Sprintf("%d/%d", d, t))
}

func ago(p *registry.Project) string {
	if p.LastOpened.IsZero() {
		return ""
	}
	d := time.Since(p.LastOpened)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return p.LastOpened.Format("Jan 2 2006")
}

func shorten(s string, n int) string {
	r := []rune(s)
	if n < 4 || len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n+1:])
}
