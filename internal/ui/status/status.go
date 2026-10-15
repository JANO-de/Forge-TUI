package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	labelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	valueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	modelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("046")).Bold(true)
	modePlanStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("039")).Bold(true).Background(lipgloss.Color("235")).Padding(0, 1)
	modeBuildStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Background(lipgloss.Color("235")).Padding(0, 1)
	barBgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	barFgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("046"))
	costStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	spinnerFrames  = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

type Mode int

const (
	ModePlan Mode = iota
	ModeBuild
)

type StatusData struct {
	Model          string
	Mode           Mode
	SessionName    string
	ContextPercent int
	TokensIn       int
	TokensOut      int
	Cost           float64
	Elapsed        time.Duration
	CurrentTool    string
	TodoDone       int
	TodoTotal      int
	Working        bool
	SpinnerIndex   int
}

type StatusBar struct {
	width int
	data  StatusData
}

func NewStatusBar(width int) StatusBar {
	return StatusBar{
		width: width,
		data: StatusData{
			Model:          "nemotron-3.5-lightning-free",
			Mode:           ModePlan,
			SessionName:    "Session 1",
			ContextPercent: 45,
			TokensIn:       12500,
			TokensOut:      3200,
			Cost:           0.0042,
			Elapsed:        time.Minute*3 + time.Second*42,
			TodoDone:       3,
			TodoTotal:      7,
			Working:        false,
		},
	}
}

func (s StatusBar) SetWidth(w int) StatusBar {
	s.width = w
	return s
}

func (s StatusBar) Update(data StatusData) StatusBar {
	s.data = data
	return s
}

func (s StatusBar) Tick() StatusBar {
	if s.data.Working {
		s.data.SpinnerIndex = (s.data.SpinnerIndex + 1) % len(spinnerFrames)
	}
	return s
}

func (s StatusBar) View() string {
	if s.width == 0 {
		return ""
	}

	d := s.data

	// Build left section: model, mode, session
	var leftParts []string
	leftParts = append(leftParts, modelStyle.Render(" "+d.Model+" "))

	if d.Mode == ModePlan {
		leftParts = append(leftParts, modePlanStyle.Render(" PLAN "))
	} else {
		leftParts = append(leftParts, modeBuildStyle.Render(" BUILD "))
	}

	leftParts = append(leftParts, labelStyle.Render(d.SessionName))
	leftSection := strings.Join(leftParts, " ")

	// Build center section: context bar, tokens, cost
	contextBar := s.renderContextBar(d.ContextPercent)
	tokens := labelStyle.Render(fmt.Sprintf("↑%s ↓%s", formatTokens(d.TokensIn), formatTokens(d.TokensOut)))
	cost := costStyle.Render(fmt.Sprintf("$%.4f", d.Cost))
	centerSection := strings.Join([]string{contextBar, tokens, cost}, "  ")

	// Build right section: elapsed, tool, todo progress
	elapsed := labelStyle.Render(formatDuration(d.Elapsed))

	var tool string
	if d.Working {
		spinner := spinnerFrames[d.SpinnerIndex%len(spinnerFrames)]
		tool = toolStyle.Render(spinner + " " + d.CurrentTool)
	} else if d.CurrentTool != "" {
		tool = toolStyle.Render(d.CurrentTool)
	} else {
		tool = mutedStyle.Render("idle")
	}

	todoBar := s.renderTodoBar(d.TodoDone, d.TodoTotal)
	rightParts := []string{elapsed, tool, todoBar}
	rightSection := strings.Join(rightParts, "  ")

	// Calculate spacing
	leftWidth := lipgloss.Width(leftSection)
	centerWidth := lipgloss.Width(centerSection)
	rightWidth := lipgloss.Width(rightSection)

	totalContentWidth := leftWidth + centerWidth + rightWidth
	spacing := s.width - totalContentWidth - 4 // padding

	if spacing < 2 {
		spacing = 2
	}

	leftPad := strings.Repeat(" ", spacing/2)
	rightPad := strings.Repeat(" ", spacing-spacing/2)

	return leftSection + leftPad + centerSection + rightPad + rightSection
}

func (s StatusBar) renderContextBar(percent int) string {
	barWidth := 20
	filled := barWidth * percent / 100
	empty := barWidth - filled

	bar := barFgStyle.Render(strings.Repeat("█", filled)) + barBgStyle.Render(strings.Repeat("░", empty))
	return labelStyle.Render(fmt.Sprintf("ctx %3d%% ", percent)) + "[" + bar + "]"
}

func (s StatusBar) renderTodoBar(done, total int) string {
	if total == 0 {
		return mutedStyle.Render("todo: --")
	}
	barWidth := 10
	filled := barWidth * done / total
	empty := barWidth - filled

	bar := barFgStyle.Render(strings.Repeat("█", filled)) + barBgStyle.Render(strings.Repeat("░", empty))
	return labelStyle.Render(fmt.Sprintf("todo %d/%d ", done, total)) + "[" + bar + "]"
}

func formatTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatDuration(d time.Duration) string {
	if d >= time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if d >= time.Minute {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
