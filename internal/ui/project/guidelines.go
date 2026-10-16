package project

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/ui/panes"
)

var guideFiles = [2]string{"AGENTS.md", "HANDOFF.md"}

// handoffPrompt is what /handoff sends. It is a Forge command, not an
// OpenCode one: OpenCode has no built-in /handoff.
const handoffPrompt = `Write the handoff for the next session. Edit only HANDOFF.md in this project:
- tick finished TODO items as [x], keep the numbered checklist format, add new TODOs if work was discovered;
- refresh "Current state", "Where the conversation stopped" and "Open questions" so a fresh session can continue from them;
- keep it terse: facts and next steps, no recap of the conversation.`

// guideState caches the two guideline files so View does not hit the disk.
type guideState struct {
	tab  int // index into guideFiles
	text [2]string
	ok   [2]bool
}

func (g *guideState) reload(dir string) {
	for i, name := range guideFiles {
		b, err := os.ReadFile(filepath.Join(dir, name))
		g.text[i], g.ok[i] = string(b), err == nil
	}
}

func (p ProjectView) guidelineLines() []string {
	g := p.gd
	var tabs []string
	for i, name := range guideFiles {
		if i == g.tab {
			tabs = append(tabs, activeTabStyle.Render("["+name+"]"))
		} else {
			tabs = append(tabs, tabStyle.Render(" "+name+" "))
		}
	}
	lines := []string{strings.Join(tabs, " "), mutedStyle.Render("←/→ file · e edit · h /handoff")}

	if !g.ok[g.tab] {
		return append(lines, "", mutedStyle.Render(guideFiles[g.tab]+" not found. Press e to create it."))
	}
	src := g.text[g.tab]
	if g.tab == 1 {
		done, total := checklistCounts(src)
		lines = append(lines, progressLine(done, total))
		src = checkboxGlyphs(src)
	}
	width := max(p.layout().rw-4, 20)
	return append(lines, "", strings.Join(renderMarkdown(src, width), "\n"))
}

func progressLine(done, total int) string {
	if total == 0 {
		return mutedStyle.Render("TODO  no checklist in HANDOFF.md")
	}
	const w = 20
	fill := done * w / total
	return toolStyle.Render("TODO ") + assistantStyle.Render(strings.Repeat("█", fill)) +
		mutedStyle.Render(strings.Repeat("░", w-fill)) + " " + strconv.Itoa(done) + "/" + strconv.Itoa(total)
}

func checklistCounts(src string) (done, total int) {
	for _, m := range checkboxRe.FindAllStringSubmatch(src, -1) {
		total++
		if m[1] != " " {
			done++
		}
	}
	return
}

// checkboxGlyphs swaps "[x]"/"[ ]" in checklist lines for ✓/☐.
func checkboxGlyphs(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if !checkboxRe.MatchString(l) {
			continue
		}
		l = strings.Replace(l, "[ ]", "☐", 1)
		l = strings.Replace(l, "[x]", "✓", 1)
		lines[i] = strings.Replace(l, "[X]", "✓", 1)
	}
	return strings.Join(lines, "\n")
}

// handleGuideKey handles keys while the Guidelines pane has focus.
func (p *ProjectView) handleGuideKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	g := p.gd
	switch msg.Type {
	case tea.KeyLeft:
		g.tab, p.guideScroll = 0, 0
		return nil, true
	case tea.KeyRight:
		g.tab, p.guideScroll = 1, 0
		return nil, true
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return nil, false
		}
		switch msg.Runes[0] {
		case '1', '2':
			g.tab, p.guideScroll = int(msg.Runes[0]-'1'), 0
		case 'j':
			p.scrollRight(1)
		case 'k':
			p.scrollRight(-1)
		case 'u':
			g.reload(p.ProjectPath)
		case 'e':
			return panes.EditFile(filepath.Join(p.ProjectPath, guideFiles[g.tab])), true
		case 'h':
			return p.runHandoff(), true
		default:
			return nil, false
		}
		return nil, true
	}
	return nil, false
}

// runHandoff asks the agent to update HANDOFF.md (always in Build mode) and
// moves focus to the chat so permission prompts can be answered.
func (p *ProjectView) runHandoff() tea.Cmd {
	cmd := p.submitAs("/handoff", handoffPrompt, ModeBuild)
	if cmd != nil {
		if p.activePane != PaneChat {
			p.lastPane = p.activePane
		}
		p.activePane = PaneChat
	}
	return cmd
}
