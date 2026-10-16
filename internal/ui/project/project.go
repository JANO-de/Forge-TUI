package project

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"forge/internal/ui/panes"
	"forge/internal/ui/status"
)

var (
	titleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	assistantStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("046")).Bold(true)
	toolStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	permissionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	mutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	inputStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Border(lipgloss.RoundedBorder()).Padding(0, 1)
	tabStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	activeTabStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true).Underline(true)
	paneHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Background(lipgloss.Color("235")).Padding(0, 1)
)

type MsgType int

const (
	MsgUser MsgType = iota
	MsgAssistant
	MsgTool
	MsgPermission
	MsgQuestion
	MsgReasoning
	MsgSystem
	MsgError
)

type Message struct {
	Type       MsgType
	Content    string
	ToolName   string
	Permission *PermissionRequest
	Question   *QuestionPrompt
	Detail     string // tool input/output, shown with Ctrl+X
	Timestamp  time.Time
}

type PermissionRequest struct {
	Tool        string
	Description string
	ID          string
	Typing      bool // typing a rejection reason
	Text        string
}

// Session is one chat tab. RemoteID is the OpenCode session ID ("" until the
// session exists on the server).
type Session struct {
	ID        string
	RemoteID  string
	Name      string
	Messages  []Message
	CreatedAt time.Time
	scroll    int
	run       runState
}

type Mode = status.Mode

const (
	ModePlan  Mode = status.ModePlan
	ModeBuild Mode = status.ModeBuild
)

type Pane int

const (
	PaneChat Pane = iota
	PaneFiles
	PaneViewer
	PaneDiff
	PaneGuidelines
)

type ProjectView struct {
	ProjectPath   string
	sessions      []Session
	activeSession int
	nextID        int
	mode          Mode
	input         string
	inputMode     bool
	height        int
	width         int
	err           error
	statusBar     status.StatusBar
	activePane    Pane
	lastPane      Pane // last side pane that had focus, for Ctrl+T
	fileTree      *panes.FileTree
	fileViewer    *panes.FileViewer
	diff          *panes.DiffPane
	dctl          *diffCtl
	tctl          *diffCtl
	agent         *agentState
	picker        *sessionPicker
	gd            *guideState
	showFiles     bool
	right         Pane // PaneChat = no right column
	guideScroll   int
	spin          int
	handoffDone   int
	handoffTotal  int
	showThink     bool // Ctrl+R: expand reasoning
	showTools     bool // Ctrl+X: expand tool input/output
}

func InitialProjectView(projectPath string, width, height int) ProjectView {
	pv := ProjectView{
		ProjectPath: projectPath,
		sessions: []Session{
			{ID: "session-1", Name: "Session 1", CreatedAt: time.Now()},
		},
		nextID:     2,
		mode:       ModePlan,
		height:     height,
		width:      width,
		statusBar:  status.NewStatusBar(width),
		activePane: PaneChat,
		agent:      newAgentState(),
		diff:       panes.NewDiffPane(),
		dctl:       &diffCtl{},
		gd:         &guideState{},
		tctl:       &diffCtl{},
	}
	if ft, err := panes.NewFileTree(projectPath); err == nil {
		pv.fileTree = ft
	}
	pv.handoffDone, pv.handoffTotal = handoffProgress(projectPath)
	pv.gd.reload(projectPath)
	pv.statusBar = pv.statusBar.Update(pv.statusData())
	return pv
}

type tickMsg time.Time

func (p ProjectView) Init() tea.Cmd {
	return tea.Batch(tea.WindowSize(), p.tickCmd(), p.agent.start(p.ProjectPath), p.refreshTree())
}

func (p ProjectView) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (p ProjectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd, ok := p.handleAgentMsg(msg); ok {
		p.refreshStatus()
		return p, cmd
	}
	if cmd, ok := p.handleDiffMsg(msg); ok {
		return p, cmd
	}
	if cmd, ok := p.handleTreeMsg(msg); ok {
		return p, cmd
	}
	if cmd, ok := p.handleSessionMsg(msg); ok {
		p.refreshStatus()
		return p, cmd
	}
	switch msg := msg.(type) {
	case tickMsg:
		p.spin++
		p.refreshStatus()
		return p, p.tickCmd()
	case panes.EditorFinishedMsg:
		if p.fileViewer != nil && msg.Path == p.fileViewer.Path && msg.Err == nil {
			p.fileViewer.Reload()
		}
		p.gd.reload(p.ProjectPath)
		p.handoffDone, p.handoffTotal = handoffProgress(p.ProjectPath)
		return p, tea.Batch(p.refreshTree(), p.refreshDiffIfOpen())
	case tea.KeyMsg:
		if p.permTyping() {
			if cmd, ok := p.handlePermTyping(msg); ok {
				p.refreshStatus()
				return p, cmd
			}
		}
		if p.hasQuestion() {
			if cmd, ok := p.handleQuestionKey(msg); ok {
				p.refreshStatus()
				return p, cmd
			}
		}
		var cmd tea.Cmd
		var m tea.Model
		if p.inputMode {
			m, cmd = p.handleInput(msg)
		} else {
			m, cmd = p.handleKeys(msg)
		}
		pv := m.(ProjectView)
		pv.refreshStatus()
		return pv, cmd
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
			delta := 3
			if msg.Button == tea.MouseButtonWheelDown {
				delta = -3
			}
			d := p.layout()
			switch {
			case msg.X < d.fw:
			case msg.X < d.fw+d.cw:
				p.scrollBy(delta)
			default:
				p.scrollRight(-delta)
			}
		}
		return p, nil
	case tea.WindowSizeMsg:
		p.height, p.width = msg.Height, msg.Width
		p.statusBar = p.statusBar.SetWidth(msg.Width)
		if p.fileTree != nil {
			p.fileTree.Width, p.fileTree.Height = msg.Width, msg.Height
		}
	}
	return p, nil
}

func (p ProjectView) handleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type != tea.KeyTab {
		p.agent.acOn = false
	}
	switch msg.Type {
	case tea.KeyEnter:
		if msg.Alt { // Alt+Enter: newline
			p.input += "\n"
			return p, nil
		}
		text := strings.TrimSpace(p.input)
		if text == "" {
			return p, nil
		}
		p.input, p.inputMode = "", false
		p.agent.pushHistory(text)
		if text == "/handoff" {
			return p, p.runHandoff()
		}
		if strings.HasPrefix(text, "/") {
			if cmd, ok := p.slash(text); ok {
				return p, cmd
			}
		}
		return p, p.submit(text)
	case tea.KeyCtrlJ: // newline
		p.input += "\n"
	case tea.KeyTab:
		p.input = p.complete(p.input)
	case tea.KeyUp:
		p.agent.historyPrev(&p.input)
	case tea.KeyDown:
		p.agent.historyNext(&p.input)
	case tea.KeyEsc:
		p.input, p.inputMode = "", false
		p.agent.histIdx = len(p.agent.hist)
	case tea.KeyBackspace:
		if r := []rune(p.input); len(r) > 0 {
			p.input = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		p.input += " "
	case tea.KeyPgUp:
		p.scrollBy(p.chatHeight() - 2)
	case tea.KeyPgDown:
		p.scrollBy(-(p.chatHeight() - 2))
	case tea.KeyRunes:
		p.input += string(msg.Runes)
	}
	return p, nil
}

func (p ProjectView) handleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &p.sessions[p.activeSession]
	if p.picker != nil {
		return p, p.handlePickerKey(msg)
	}
	if p.activePane == PaneDiff {
		if cmd, ok := p.handleDiffKey(msg); ok {
			return p, cmd
		}
	}
	if p.activePane == PaneGuidelines {
		if cmd, ok := p.handleGuideKey(msg); ok {
			return p, cmd
		}
	}
	if p.activePane == PaneChat && s.run.pending != nil && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		if msg.Runes[0] == 'f' {
			s.run.pending.Typing, s.run.pending.Text = true, ""
			return p, nil
		}
		reply := map[rune]string{'y': "once", 'a': "always", 'n': "reject"}[msg.Runes[0]]
		if reply != "" {
			id := s.run.pending.ID
			s.run.donePermission(id)
			p.note(s, MsgSystem, "permission: "+reply)
			return p, p.agent.reply(id, reply)
		}
	}
	switch msg.Type {
	case tea.KeyCtrlO:
		if p.activePane == PaneChat {
			return p, p.openPicker()
		}
	case tea.KeyCtrlN:
		p.newSession()
	case tea.KeyCtrlW:
		p.closeSession()
	case tea.KeyTab:
		p.activeSession = (p.activeSession + 1) % len(p.sessions)
	case tea.KeyShiftTab:
		p.activeSession = (p.activeSession - 1 + len(p.sessions)) % len(p.sessions)
	case tea.KeyCtrlP:
		p.mode = ModePlan
	case tea.KeyCtrlB:
		p.mode = ModeBuild
	case tea.KeyCtrlL:
		p.agent.cycleModel()
	case tea.KeyCtrlF:
		p.togglePane(PaneFiles)
		if p.showFiles {
			return p, p.refreshTree()
		}
	case tea.KeyCtrlV:
		p.togglePane(PaneViewer)
	case tea.KeyCtrlD:
		p.togglePane(PaneDiff)
		if p.right == PaneDiff {
			return p, p.refreshDiff()
		}
	case tea.KeyCtrlG:
		p.togglePane(PaneGuidelines)
		p.gd.reload(p.ProjectPath)
	case tea.KeyPgUp:
		p.page(-1)
	case tea.KeyPgDown:
		p.page(1)
	case tea.KeyHome:
		p.edge(true)
	case tea.KeyEnd:
		p.edge(false)
	case tea.KeyUp:
		p.navigate(-1)
	case tea.KeyDown:
		p.navigate(1)
	case tea.KeyLeft:
		if p.activePane == PaneFiles && p.fileTree != nil {
			p.fileTree.MoveToParent()
		}
	case tea.KeyRight:
		if p.activePane == PaneFiles && p.fileTree != nil {
			p.fileTree.ToggleExpand()
		}
	case tea.KeyEnter, tea.KeySpace:
		if p.activePane == PaneChat {
			p.inputMode = true
		} else if p.activePane == PaneFiles && p.fileTree != nil {
			if node := p.fileTree.GetSelected(); node != nil && !node.IsDir {
				p.openFileViewer(node.Path)
			} else if node != nil {
				p.fileTree.ToggleExpand()
			}
		}
	case tea.KeyCtrlE:
		if p.activePane == PaneFiles && p.fileTree != nil {
			if node := p.fileTree.GetSelected(); node != nil && !node.IsDir {
				p.openFileViewer(node.Path)
			}
		}
		if p.activePane == PaneViewer && p.fileViewer != nil {
			return p, p.fileViewer.OpenInEditor()
		}
	case tea.KeyCtrlR:
		p.showThink = !p.showThink
	case tea.KeyCtrlX:
		p.showTools = !p.showTools
	case tea.KeyCtrlT:
		p.swapFocus()
	case tea.KeyCtrlC, tea.KeyEsc:
		if p.activePane != PaneChat {
			p.lastPane, p.activePane = p.activePane, PaneChat
		} else if s.run.working && s.RemoteID != "" {
			return p, p.agent.abort(s.RemoteID)
		} else {
			return p, tea.Quit
		}
	}
	return p, nil
}

func (p *ProjectView) togglePane(pane Pane) {
	if pane == PaneFiles {
		if p.showFiles && p.activePane == PaneFiles {
			p.showFiles, p.activePane = false, PaneChat
		} else {
			p.showFiles, p.activePane = true, PaneFiles
		}
		return
	}
	if p.right == pane && p.activePane == pane {
		p.right, p.activePane = PaneChat, PaneChat
	} else {
		p.right, p.activePane = pane, pane
	}
}

func (p *ProjectView) openFileViewer(path string) {
	if v, err := panes.NewFileViewer(path); err == nil {
		p.fileViewer = v
		p.right, p.activePane = PaneViewer, PaneViewer
	}
}

func (p *ProjectView) newSession() {
	id := fmt.Sprintf("session-%d", p.nextID)
	p.sessions = append(p.sessions, Session{ID: id, Name: fmt.Sprintf("Session %d", p.nextID), CreatedAt: time.Now()})
	p.nextID++
	p.activeSession = len(p.sessions) - 1
}

func (p *ProjectView) closeSession() {
	if len(p.sessions) <= 1 {
		return
	}
	p.sessions = append(p.sessions[:p.activeSession], p.sessions[p.activeSession+1:]...)
	if p.activeSession >= len(p.sessions) {
		p.activeSession = len(p.sessions) - 1
	}
}

// swapFocus (Ctrl+T) jumps from a side pane back to the chat without closing
// anything, and from the chat back to the pane that had focus before.
func (p *ProjectView) swapFocus() {
	if p.activePane != PaneChat {
		p.lastPane, p.activePane = p.activePane, PaneChat
		return
	}
	switch {
	case p.lastPane == PaneFiles && p.showFiles:
		p.activePane = PaneFiles
	case p.lastPane != PaneChat && p.lastPane != PaneFiles && p.lastPane == p.right:
		p.activePane = p.lastPane
	case p.showFiles:
		p.activePane = PaneFiles
	case p.right != PaneChat:
		p.activePane = p.right
	}
}
