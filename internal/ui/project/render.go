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
		lines = append(lines, renderBlock(m, pendingID, p.chatWidth(), opts)...)
		prev = m.Type
	}
	return lines
}

type viewOpts struct{ think, tools bool }

func isCompact(t MsgType) bool { return t == MsgTool || t == MsgReasoning }

func renderBlock(m Message, pendingID string, w int, o viewOpts) []string {
	switch m.Type {
	case MsgUser:
		return withBar("You", colUser, wrapPlain(m.Content, w-2))
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

// performSearch searches for query in chat messages
func (p *ProjectView) performSearch(query string) {
	p.searchQuery = query
	p.searchMatches = []int{}
	p.searchIndex = 0

	if query == "" {
		p.showSearch = false
		return
	}

	p.showSearch = true
	if len(p.sessions) == 0 {
		return
	}

	lines := p.chatLines()
	queryLower := strings.ToLower(query)

	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), queryLower) {
			p.searchMatches = append(p.searchMatches, i)
		}
	}
}

// nextSearchMatch navigates to the next search match
func (p *ProjectView) nextSearchMatch() {
	if len(p.searchMatches) == 0 {
		return
	}
	p.searchIndex = (p.searchIndex + 1) % len(p.searchMatches)
	matchPos := p.searchMatches[p.searchIndex]
	s := &p.sessions[p.activeSession]
	h := p.chatHeight()
	if matchPos >= s.scroll && matchPos < s.scroll+h {
		return
	}
	s.scroll = max(0, matchPos-h/2)
}

// prevSearchMatch navigates to the previous search match
func (p *ProjectView) prevSearchMatch() {
	if len(p.searchMatches) == 0 {
		return
	}
	p.searchIndex = (p.searchIndex - 1 + len(p.searchMatches)) % len(p.searchMatches)
	matchPos := p.searchMatches[p.searchIndex]
	s := &p.sessions[p.activeSession]
	h := p.chatHeight()
	if matchPos >= s.scroll && matchPos < s.scroll+h {
		return
	}
	s.scroll = max(0, matchPos-h/2)
}

// copyLastCodeBlock extracts and returns the last code block from chat
func (p *ProjectView) copyLastCodeBlock() string {
	if len(p.sessions) == 0 {
		return ""
	}
	s := p.sessions[p.activeSession]
	var lastCodeBlock string

	for _, m := range s.Messages {
		if m.Type == MsgUser || m.Type == MsgAssistant {
			blocks := extractCodeBlocks(m.Content)
			if len(blocks) > 0 {
				lastCodeBlock = blocks[len(blocks)-1]
			}
		}
	}
	return lastCodeBlock
}

// extractCodeBlocks extracts code blocks from markdown text
func extractCodeBlocks(text string) []string {
	var blocks []string
	lines := strings.Split(text, "\n")
	inBlock := false
	var currentBlock strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inBlock {
				if currentBlock.Len() > 0 {
					blocks = append(blocks, strings.TrimSpace(currentBlock.String()))
					currentBlock.Reset()
				}
				inBlock = false
			} else {
				inBlock = true
			}
		} else if inBlock {
			currentBlock.WriteString(line + "\n")
		}
	}

	if inBlock && currentBlock.Len() > 0 {
		blocks = append(blocks, strings.TrimSpace(currentBlock.String()))
	}

	return blocks
}
