package lobby

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var bannerLines = []string{
	`███████╗ ██████╗ ██████╗  ██████╗ ███████╗`,
	`██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝`,
	`█████╗  ██║   ██║██████╔╝██║  ███╗█████╗  `,
	`██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝  `,
	`██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗`,
	`╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝`,
}

// ember gradient, top (hot yellow) to bottom (deep red)
var emberColors = []string{"226", "220", "214", "208", "202", "196"}

const tagline = "terminal workshop for OpenCode"

// banner renders the logo centered in width. Short terminals get one line.
func banner(width, height int) string {
	if height < 26 {
		t := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Render("▟█▙ FORGE")
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, t+" "+mutedStyle.Render("· "+tagline))
	}
	var out []string
	for i, l := range bannerLines {
		l += strings.Repeat(" ", max(0, 43-lipgloss.Width(l)))
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(emberColors[i])).Bold(true)
		out = append(out, lipgloss.PlaceHorizontal(width, lipgloss.Center, st.Render(l)))
	}
	out = append(out, lipgloss.PlaceHorizontal(width, lipgloss.Center, mutedStyle.Render(tagline)))
	return strings.Join(out, "\n")
}
