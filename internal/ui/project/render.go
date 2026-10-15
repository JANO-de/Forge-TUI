package project

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	colUser       = lipgloss.Color("229")
	colAssistant  = lipgloss.Color("046")
	colPermission = lipgloss.Color("214")
	colError      = lipgloss.Color("196")
)

var toolIconColor = map[string]lipgloss.Color{
	"✓": "046", "✗": "196", "⟳": "226", "…": "245",
}

// chatWidth is the usable width inside the chat column.
func (p ProjectView) chatWidth() int {
	d := p.layout()
	return max(d.cw-2, 20)
}

// chatHeight is the number of chat rows inside the chat column.
func (p ProjectView) chatHeight() int {
	return max(p.rows(p.layout()), 3)
}

// chatLines renders the active session's messages into display lines.
func (p ProjectView) chatLines() []string {
	if len(p.sessions) == 0 {
		return nil
	}
	s := p.sessions[p.activeSession]
	pendingID := ""
	if s.run.pending != nil {
		pendingID = s.run.pending.ID
	}
	var lines []string
	prev := MsgType(-1)
	for i, m := range s.Messages {
		if i > 0 && !(m.Type == MsgTool && prev == MsgTool) {
			lines = append(lines, "")
		}
		lines = append(lines, renderBlock(m, pendingID, p.chatWidth())...)
		prev = m.Type
	}
	return lines
}

func renderBlock(m Message, pendingID string, w int) []string {
	switch m.Type {
	case MsgUser:
		return withBar("You", colUser, wrapPlain(m.Content, w-2))
	case MsgAssistant:
		return withBar("Assistant", colAssistant, renderMarkdown(m.Content, w-2))
	case MsgTool:
		return []string{toolLineStyled(m.Content, w)}
	case MsgPermission:
		if m.Permission == nil {
			return nil
		}
		text := m.Permission.Tool + " " + m.Permission.Description
		if m.Permission.ID == pendingID {
			text += "\n[y] allow once   [a] always   [n] reject"
		}
		return withBar("Permission", colPermission, wrapPlain(text, w-2))
	case MsgError:
		return withBar("Error", colError, wrapPlain(m.Content, w-2))
	}
	return wrapMuted(m.Content, w)
}

func wrapPlain(text string, w int) []string {
	return strings.Split(lipgloss.NewStyle().Width(w).Render(text), "\n")
}

func wrapMuted(text string, w int) []string {
	return strings.Split(mutedStyle.Width(w).Render(text), "\n")
}

func withBar(label string, color lipgloss.Color, body []string) []string {
	bar := lipgloss.NewStyle().Foreground(color).Render("▎") + " "
	out := make([]string, 0, len(body)+1)
	out = append(out, bar+lipgloss.NewStyle().Foreground(color).Bold(true).Render(label))
	for _, l := range body {
		out = append(out, bar+l)
	}
	return out
}

func toolLineStyled(content string, w int) string {
	r := []rune(content)
	if len(r) == 0 {
		return ""
	}
	icon := string(r[0])
	col, ok := toolIconColor[icon]
	if !ok {
		col = "245"
	}
	line := "  " + lipgloss.NewStyle().Foreground(col).Render(icon) + mutedStyle.Render(string(r[1:]))
	return lipgloss.NewStyle().MaxWidth(w).Render(line)
}

func (p ProjectView) renderChat() string {
	h := p.chatHeight()
	if len(p.sessions) == 0 {
		return mutedStyle.Render("No active session. Press Ctrl+N to create one.")
	}
	s := p.sessions[p.activeSession]
	var lines []string
	if len(s.Messages) == 0 {
		switch {
		case p.agent.err != nil:
			lines = wrapPlain(errorStyle.Render("OpenCode failed to start: "+p.agent.err.Error()), p.chatWidth())
		case !p.agent.ready:
			lines = []string{mutedStyle.Render("Starting OpenCode...")}
		default:
			lines = []string{mutedStyle.Render("No messages yet. Press Enter, type a message and press Enter to send.")}
		}
	} else {
		lines = p.chatLines()
	}

	maxOff := len(lines) - h
	if maxOff < 0 {
		maxOff = 0
	}
	off := s.scroll
	if off > maxOff {
		off = maxOff
	}
	end := len(lines) - off
	start := end - h
	if start < 0 {
		start = 0
	}
	view := append([]string(nil), lines[start:end]...)
	if off > 0 && len(view) > 0 {
		view[len(view)-1] = mutedStyle.Render(fmt.Sprintf("── ↓ %d more lines · End to follow ──", off))
	}
	for len(view) < h {
		view = append(view, "")
	}
	return strings.Join(view, "\n")
}

// scrollBy moves the chat view up (delta > 0) or down; 0 follows the tail.
func (p *ProjectView) scrollBy(delta int) {
	if len(p.sessions) == 0 {
		return
	}
	s := &p.sessions[p.activeSession]
	maxOff := len(p.chatLines()) - p.chatHeight()
	if maxOff < 0 {
		maxOff = 0
	}
	s.scroll = min(max(s.scroll+delta, 0), maxOff)
}
