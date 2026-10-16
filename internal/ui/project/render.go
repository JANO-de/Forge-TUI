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
	colThink      = lipgloss.Color("245")
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
	opts := viewOpts{think: p.showThink, tools: p.showTools}
	for i, m := range s.Messages {
		if i > 0 && !(isCompact(m.Type) && isCompact(prev)) {
			lines = append(lines, "")
		}
		lines = append(lines, renderBlock(m, pendingID, p.chatWidth(), opts, p.showTimestamps)...)
		prev = m.Type
	}
	return lines
}

type viewOpts struct{ think, tools bool }

func isCompact(t MsgType) bool { return t == MsgTool || t == MsgReasoning }

func renderBlock(m Message, pendingID string, w int, o viewOpts, showTimestamps bool) []string {
	switch m.Type {
	case MsgUser:
		lines := withBar("You", colUser, wrapPlain(m.Content, w-2))
		if showTimestamps && !m.Timestamp.IsZero() {
			ts := m.Timestamp.Format("15:04:05")
			lines = append([]string{mutedStyle.Render("  " + ts)}, lines...)
		}
		return lines
	case MsgAssistant:
		return withBar("Assistant", colAssistant, renderMarkdown(m.Content, w-2))
	case MsgTool:
		lines := []string{toolLineStyled(m.Content, w)}
		if o.tools && m.Detail != "" {
			for _, d := range strings.Split(m.Detail, "\n") {
				lines = append(lines, mutedStyle.Render("      "+clip(d, w-8)))
			}
		}
		return lines
	case MsgReasoning:
		if o.think {
			return withBar("Thinking", colThink, wrapMuted(m.Content, w-2))
		}
		n := strings.Count(strings.TrimSpace(m.Content), "\n") + 1
		return []string{mutedStyle.Render(fmt.Sprintf("  ▸ thinking · %d lines · Ctrl+R to show", n))}
	case MsgPermission:
		if m.Permission == nil {
			return nil
		}
		text := m.Permission.Tool + " " + m.Permission.Description
		if m.Permission.ID == pendingID {
			text += "\n[y] allow once   [a] always   [n] reject   [f] reject + feedback"
		}
		return withBar("Permission", colPermission, wrapPlain(text, w-2))
	case MsgQuestion:
		if m.Question == nil {
			return nil
		}
		return renderQuestion(m.Question, w)
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

func (p *ProjectView) performSearch(query string) {
	p.searchQuery = query
	p.currentMatch = 0
	p.searchMatches = nil
	if query == "" {
		return
	}
	s := &p.sessions[p.activeSession]
	for i, m := range s.Messages {
		if strings.Contains(strings.ToLower(m.Content), strings.ToLower(query)) {
			p.searchMatches = append(p.searchMatches, i)
		}
	}
}

func (p *ProjectView) nextSearchMatch() {
	if len(p.searchMatches) == 0 {
		return
	}
	p.currentMatch = (p.currentMatch + 1) % len(p.searchMatches)
	idx := p.searchMatches[p.currentMatch]
	p.sessions[p.activeSession].scroll = 0
	for i := idx; i >= 0; i-- {
		p.sessions[p.activeSession].scroll++
	}
}

func (p *ProjectView) prevSearchMatch() {
	if len(p.searchMatches) == 0 {
		return
	}
	p.currentMatch = (p.currentMatch - 1 + len(p.searchMatches)) % len(p.searchMatches)
	idx := p.searchMatches[p.currentMatch]
	p.sessions[p.activeSession].scroll = 0
	for i := idx; i >= 0; i-- {
		p.sessions[p.activeSession].scroll++
	}
}

func extractCodeBlocks(text string) []string {
	var blocks []string
	var currentBlock strings.Builder
	inCodeBlock := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				if currentBlock.Len() > 0 {
					blocks = append(blocks, currentBlock.String())
					currentBlock.Reset()
				}
				inCodeBlock = false
			} else {
				inCodeBlock = true
			}
		} else if inCodeBlock {
			if currentBlock.Len() > 0 {
				currentBlock.WriteString("\n")
			}
			currentBlock.WriteString(line)
		}
	}
	if inCodeBlock && currentBlock.Len() > 0 {
		blocks = append(blocks, currentBlock.String())
	}
	return blocks
}

func (p *ProjectView) copyLastCodeBlock() string {
	s := &p.sessions[p.activeSession]
	for i := len(s.Messages) - 1; i >= 0; i-- {
		blocks := extractCodeBlocks(s.Messages[i].Content)
		if len(blocks) > 0 {
			return blocks[len(blocks)-1]
		}
	}
	return ""
}
