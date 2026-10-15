package project

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// A small markdown renderer for chat output (no new dependency): headings,
// bullets, quotes, rules, fenced code, and inline `code`, **bold**, *italic*.

var (
	mdHeading = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	mdBold    = lipgloss.NewStyle().Bold(true)
	mdItalic  = lipgloss.NewStyle().Italic(true)
	mdCode    = lipgloss.NewStyle().Foreground(lipgloss.Color("215")).Background(lipgloss.Color("236"))
	mdBlock   = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236"))
	mdQuote   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	mdBullet  = lipgloss.NewStyle().Foreground(lipgloss.Color("039"))
)

// renderMarkdown returns src as styled lines, each at most width cells wide.
func renderMarkdown(src string, width int) []string {
	if width < 10 {
		width = 10
	}
	para := lipgloss.NewStyle().Width(width)
	var out []string
	inCode := false
	for _, raw := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		line := strings.ReplaceAll(raw, "\t", "    ")
		trim := strings.TrimSpace(line)

		if strings.HasPrefix(trim, "```") {
			inCode = !inCode
			if lang := strings.TrimSpace(strings.TrimPrefix(trim, "```")); inCode && lang != "" {
				out = append(out, mutedStyle.Render(lang))
			}
			continue
		}
		if inCode {
			out = append(out, strings.Split(mdBlock.Width(width).Render(" "+line), "\n")...)
			continue
		}

		switch {
		case trim == "":
			out = append(out, "")
		case isRule(trim):
			n := width
			if n > 40 {
				n = 40
			}
			out = append(out, mutedStyle.Render(strings.Repeat("─", n)))
		case strings.HasPrefix(trim, "#"):
			text := strings.TrimSpace(strings.TrimLeft(trim, "#"))
			out = append(out, strings.Split(mdHeading.Width(width).Render(text), "\n")...)
		case strings.HasPrefix(trim, "> "):
			out = append(out, strings.Split(mdQuote.Width(width).Render("│ "+trim[2:]), "\n")...)
		default:
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			if rest, ok := bulletText(trim); ok {
				line = indent + mdBullet.Render("•") + " " + inline(rest)
			} else {
				line = indent + inline(trim)
			}
			out = append(out, strings.Split(para.Render(line), "\n")...)
		}
	}
	return out
}

func isRule(s string) bool {
	if len(s) < 3 {
		return false
	}
	return strings.Trim(s, "-") == "" || strings.Trim(s, "*") == "" || strings.Trim(s, "_") == ""
}

func bulletText(s string) (string, bool) {
	if len(s) > 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return s[2:], true
	}
	return "", false
}

func inline(s string) string {
	var b strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); {
		switch {
		case r[i] == '`':
			if j := indexRune(r, '`', i+1); j > i+1 {
				b.WriteString(mdCode.Render(string(r[i+1 : j])))
				i = j + 1
				continue
			}
		case r[i] == '*' && i+1 < len(r) && r[i+1] == '*':
			if j := indexDouble(r, i+2); j > i+2 {
				b.WriteString(mdBold.Render(string(r[i+2 : j])))
				i = j + 2
				continue
			}
		case r[i] == '*' && i+1 < len(r) && r[i+1] != ' ' && r[i+1] != '*':
			if j := indexRune(r, '*', i+1); j > i+1 && r[j-1] != ' ' {
				b.WriteString(mdItalic.Render(string(r[i+1 : j])))
				i = j + 1
				continue
			}
		}
		b.WriteRune(r[i])
		i++
	}
	return b.String()
}

func indexRune(r []rune, c rune, from int) int {
	for j := from; j < len(r); j++ {
		if r[j] == c {
			return j
		}
	}
	return -1
}

func indexDouble(r []rune, from int) int {
	for j := from; j+1 < len(r); j++ {
		if r[j] == '*' && r[j+1] == '*' {
			return j
		}
	}
	return -1
}
