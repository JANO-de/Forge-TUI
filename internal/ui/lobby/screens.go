package lobby

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (l Lobby) View() string {
	switch {
	case l.fb != nil:
		return l.frame(l.folderLines())
	case l.set != nil:
		return l.frame(l.settingsLines())
	case l.pal != nil:
		return l.frame(l.paletteLines())
	case l.help:
		return l.frame(helpLines())
	}
	return l.listView()
}

// frame draws the banner with a block of lines centred under it.
func (l Lobby) frame(lines []string) string {
	w := l.width
	if w <= 0 {
		w = 80
	}
	cw := min(cardWidth, max(w-4, 30))
	var out []string
	for _, r := range lines {
		out = append(out, center(w, lipgloss.NewStyle().Width(cw).Render(r)))
	}
	return "\n" + banner(w, l.height) + "\n\n" + strings.Join(out, "\n")
}

func helpLines() []string {
	rows := [][2]string{
		{"↑↓ / j k / Tab", "move"},
		{"Enter", "open project / run action"},
		{"/", "search projects (Esc clears)"},
		{"p", "pin / unpin project"},
		{"d", "remove project from list (files kept)"},
		{"n", "new project"},
		{"o", "open an existing folder as a project"},
		{"Ctrl+K", "command palette"},
		{"?", "this help"},
		{"q / Esc", "quit"},
	}
	out := []string{headStyle.Render("▌ Keys"), ""}
	for _, r := range rows {
		out = append(out, itemStyle.Render(padTo(r[0], 18))+mutedStyle.Render(r[1]))
	}
	return append(out, "", mutedStyle.Render("any key to close · forge --continue reopens the last project"))
}

func (l Lobby) paletteLines() []string {
	p := l.pal
	out := []string{headStyle.Render("▌ Commands"), mutedStyle.Render("> ") + itemStyle.Render(p.query+"█"), ""}
	ms := p.matches()
	if len(ms) == 0 {
		out = append(out, mutedStyle.Render("no match"))
	}
	for i, c := range ms {
		if i == p.cursor {
			out = append(out, selectedStyle.Render("▸ "+c.label))
		} else {
			out = append(out, itemStyle.Render("  "+c.label))
		}
	}
	return append(out, "", mutedStyle.Render("↑↓ choose · Enter run · Esc close"))
}

func (l Lobby) settingsLines() []string {
	f := l.set
	out := []string{headStyle.Render("▌ Settings"), ""}
	for i, lab := range settingsLabels {
		val := f.vals[i]
		if i == f.idx {
			out = append(out, selectedStyle.Render("▸ "+lab), "    "+itemStyle.Render(val+"█"))
		} else {
			out = append(out, itemStyle.Render("  "+lab), "    "+mutedStyle.Render(val))
		}
	}
	if f.msg != "" {
		out = append(out, "", errStyle.Render(f.msg))
	}
	return append(out, "", mutedStyle.Render("Tab next field · Enter save · Esc cancel · empty = default"))
}

func padTo(s string, n int) string {
	if d := n - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}
