package wizard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	itemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	inputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Border(lipgloss.RoundedBorder()).Padding(0, 1)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type Step int

const (
	StepName Step = iota
	StepFolder
	StepTemplate
	StepModel
	StepGuidelines
	StepConfirm
	StepDone
)

// DoneMsg is sent when the project was created; Path is its directory.
type DoneMsg struct{ Path string }

type Template struct {
	Name        string
	Description string
	Files       map[string]string
}

type GuidelinesTemplate struct {
	Name        string
	Description string
	Content     string
}

var Templates = []Template{
	{
		Name:        "empty",
		Description: "Empty directory",
		Files:       map[string]string{},
	},
	{
		Name:        "go",
		Description: "Go project with modules",
		Files: map[string]string{
			"go.mod":     "module {{.Name}}\n\ngo 1.24\n",
			"main.go":    "package main\n\nfunc main() {\n\tprintln(\"Hello, {{.Name}}!\")\n}\n",
			".gitignore": "# Go\nvendor/\n*.exe\n*.test\n",
		},
	},
	{
		Name:        "nix",
		Description: "Nix flake project",
		Files: map[string]string{
			"flake.nix":  "{ description = \"{{.Name}}\"; inputs.nixpkgs.url = \"github:NixOS/nixpkgs/nixos-unstable\"; outputs = { self, nixpkgs }: { packages.x86_64-linux.default = nixpkgs.legacyPackages.x86_64-linux.stdenv.mkDerivation { name = \"{{.Name}}\"; src = .; }; }; }",
			".gitignore": "# Nix\nresult*\n.envrc\n",
		},
	},
	{
		Name:        "web",
		Description: "Simple web project (HTML/CSS/JS)",
		Files: map[string]string{
			"index.html": "<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n\t<meta charset=\"UTF-8\">\n\t<title>{{.Name}}</title>\n\t<link rel=\"stylesheet\" href=\"style.css\">\n</head>\n<body>\n\t<h1>{{.Name}}</h1>\n\t<script src=\"app.js\"></script>\n</body>\n</html>",
			"style.css":  "body { font-family: sans-serif; max-width: 800px; margin: 2rem auto; padding: 0 1rem; }\n",
			"app.js":     "console.log('{{.Name}} loaded');\n",
			".gitignore": "# Web\nnode_modules/\ndist/\n.env\n",
		},
	},
	{
		Name:        "qml",
		Description: "Qt QML project",
		Files: map[string]string{
			"main.cpp":      "#include <QGuiApplication>\n#include <QQmlApplicationEngine>\n\nint main(int argc, char *argv[]) {\n\tQGuiApplication app(argc, argv);\n\tQQmlApplicationEngine engine;\n\tengine.load(QUrl(QStringLiteral(\"qrc:/main.qml\")));\n\treturn app.exec();\n}\n",
			"main.qml":      "import QtQuick\nimport QtQuick.Controls\n\nApplicationWindow {\n\twidth: 800\n\theight: 600\n\tvisible: true\n\ttitle: \"{{.Name}}\"\n\tText { text: \"Hello {{.Name}}\"; anchors.centerIn: parent }\n}",
			"{{.Name}}.pro": "QT += quick\nCONFIG += c++17\nSOURCES += main.cpp\nRESOURCES += qml.qrc\n",
			"qml.qrc":       "<RCC>\n\t<qresource prefix=\"/\">\n\t\t<file>main.qml</file>\n\t</qresource>\n</RCC>",
			".gitignore":    "# Qt\nbuild-*\n*.o\n*.moc\n",
		},
	},
}

var GuidelinesTemplates = []GuidelinesTemplate{
	{
		Name:        "default",
		Description: "Standard AGENTS.md template",
		Content: `# Project: {{.Name}}
A brief description of the project.

# Stack
- Language: {{.Language}}
- Framework: {{.Framework}}
- Build: {{.BuildTool}}

# Commands
- Build: {{.BuildCmd}}
- Test: {{.TestCmd}}
- Lint: {{.LintCmd}}

# Conventions
- Code style: {{.StyleGuide}}
- Commit format: {{.CommitFormat}}

# Testing
- Unit tests: {{.UnitTestDir}}
- Integration tests: {{.IntegrationTestDir}}

# Deployment
- Target: {{.DeployTarget}}
- Config: {{.DeployConfig}}

# Rules
- Keep changes minimal
- Write tests for new features
- Update documentation with changes`,
	},
	{
		Name:        "minimal",
		Description: "Minimal AGENTS.md template",
		Content: `# {{.Name}}
{{.Description}}

## Commands
- Build: {{.BuildCmd}}
- Test: {{.TestCmd}}
- Lint: {{.LintCmd}}

## Rules
- Keep changes minimal
- Write tests for new features`,
	},
}

type Wizard struct {
	step          Step
	name          string
	folder        string
	templateIdx   int
	modelIdx      int
	guidelinesIdx int
	cursor        int
	height        int
	width         int
	err           error
	inputMode     bool
	inputValue    string
	inputLabel    string
}

func InitialWizard(width, height int) Wizard {
	home, _ := os.UserHomeDir()
	defaultFolder := filepath.Join(home, "Proyectos")
	return Wizard{
		step:          StepName,
		folder:        defaultFolder,
		templateIdx:   0,
		modelIdx:      0,
		guidelinesIdx: 0,
		cursor:        0,
		height:        height,
		width:         width,
	}
}

func (w Wizard) Init() tea.Cmd {
	return tea.WindowSize()
}

func (w Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if w.inputMode {
			return w.handleInput(msg)
		}
		return w.handleKeys(msg)
	case tea.WindowSizeMsg:
		w.height = msg.Height
		w.width = msg.Width
	}
	return w, nil
}

func (w Wizard) handleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		w.inputMode = false
		w.applyInput()
		return w, nil
	case tea.KeyEsc:
		w.inputMode = false
		w.inputValue = ""
		return w, nil
	case tea.KeyBackspace:
		if len(w.inputValue) > 0 {
			w.inputValue = w.inputValue[:len(w.inputValue)-1]
		}
		return w, nil
	case tea.KeyRunes:
		w.inputValue += string(msg.Runes)
		return w, nil
	}
	return w, nil
}

func (w *Wizard) applyInput() {
	switch w.step {
	case StepName:
		w.name = w.inputValue
		w.step = StepFolder
		w.cursor = 0
	case StepFolder:
		w.folder = w.inputValue
		w.step = StepTemplate
		w.cursor = 0
	}
	w.inputValue = ""
}

func (w Wizard) handleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown, tea.KeyTab:
		max := w.maxCursor()
		if w.cursor < max {
			w.cursor++
		}
	case tea.KeyEnter:
		return w.handleEnter()
	case tea.KeyCtrlC, tea.KeyEsc:
		return w, tea.Quit
	}
	return w, nil
}

func (w Wizard) maxCursor() int {
	switch w.step {
	case StepName, StepFolder:
		return 0 // input mode
	case StepTemplate:
		return len(Templates) - 1
	case StepModel:
		return 5 // placeholder for models
	case StepGuidelines:
		return len(GuidelinesTemplates) - 1
	case StepConfirm:
		return 1
	default:
		return 0
	}
}

func (w Wizard) handleEnter() (tea.Model, tea.Cmd) {
	switch w.step {
	case StepName:
		w.inputMode = true
		w.inputLabel = "Project name"
		w.inputValue = w.name
		return w, nil
	case StepFolder:
		w.inputMode = true
		w.inputLabel = "Parent folder"
		w.inputValue = w.folder
		return w, nil
	case StepTemplate:
		w.templateIdx = w.cursor
		w.step = StepModel
		w.cursor = 0
	case StepModel:
		w.modelIdx = w.cursor
		w.step = StepGuidelines
		w.cursor = 0
	case StepGuidelines:
		w.guidelinesIdx = w.cursor
		w.step = StepConfirm
		w.cursor = 0
	case StepConfirm:
		if w.cursor == 0 {
			return w, w.createProject
		}
		w.step = StepGuidelines
	case StepDone:
		return w, tea.Quit
	}
	return w, nil
}

func (w Wizard) createProject() tea.Msg {
	fullPath := filepath.Join(w.folder, w.name)

	// Create directory
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		w.err = err
		return nil
	}

	// Get selected template
	template := Templates[w.templateIdx]

	// Write template files
	for filename, content := range template.Files {
		// Replace placeholders
		content = strings.ReplaceAll(content, "{{.Name}}", w.name)
		filePath := filepath.Join(fullPath, filename)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			w.err = err
			return nil
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			w.err = err
			return nil
		}
	}

	// Create AGENTS.md
	guidelines := GuidelinesTemplates[w.guidelinesIdx]
	agentsContent := strings.ReplaceAll(guidelines.Content, "{{.Name}}", w.name)
	agentsContent = strings.ReplaceAll(agentsContent, "{{.Language}}", "Go")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.Framework}}", "Standard Library")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.BuildTool}}", "go build")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.BuildCmd}}", "go build")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.TestCmd}}", "go test ./...")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.LintCmd}}", "go vet ./...")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.StyleGuide}}", "gofmt")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.CommitFormat}}", "Conventional Commits")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.UnitTestDir}}", "_test.go files")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.IntegrationTestDir}}", "N/A")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.DeployTarget}}", "Binary")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.DeployConfig}}", "N/A")
	agentsContent = strings.ReplaceAll(agentsContent, "{{.Description}}", "A {{.Name}} project")
	if err := os.WriteFile(filepath.Join(fullPath, "AGENTS.md"), []byte(agentsContent), 0644); err != nil {
		w.err = err
		return nil
	}

	// Create HANDOFF.md
	handoffContent := `# Handoff

## Projects and facts
{{.Name}}: {{.Description}}. Repo {{.Path}}. Spec in AGENTS.md.

## TODO
1. [ ] Set up development environment
2. [ ] Write core functionality
3. [ ] Add tests
4. [ ] Document API
5. [ ] Deploy to production

## Current state
Initial commit.

## Where the conversation started
Project created via Forge wizard.

## Open questions
- None yet.
`
	handoffContent = strings.ReplaceAll(handoffContent, "{{.Name}}", w.name)
	handoffContent = strings.ReplaceAll(handoffContent, "{{.Description}}", "A {{.Name}} project")
	handoffContent = strings.ReplaceAll(handoffContent, "{{.Path}}", fullPath)
	if err := os.WriteFile(filepath.Join(fullPath, "HANDOFF.md"), []byte(handoffContent), 0644); err != nil {
		w.err = err
		return nil
	}

	// Create opencode.json
	opencodeContent := `{
  "$schema": "https://opencode.ai/config.json",
  "instructions": ["HANDOFF.md"],
  "permission": {
    "bash": {
      "go build*": "allow",
      "go vet*": "allow",
      "go test*": "allow",
      "gofmt*": "allow"
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(fullPath, "opencode.json"), []byte(opencodeContent), 0644); err != nil {
		w.err = err
		return nil
	}

	// Initialize git
	if err := w.initGit(fullPath); err != nil {
		w.err = err
		return nil
	}

	return DoneMsg{Path: fullPath}
}

func (w Wizard) initGit(path string) error {
	cmd := []string{"git", "init"}
	if err := runCmd(path, cmd...); err != nil {
		return err
	}
	cmd = []string{"git", "add", "."}
	if err := runCmd(path, cmd...); err != nil {
		return err
	}
	cmd = []string{"git", "commit", "-m", "Initial commit"}
	return runCmd(path, cmd...)
}

func runCmd(dir string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	return cmd.Run()
}

func (w Wizard) View() string {
	if w.err != nil {
		return titleStyle.Render("New Project Wizard") + "\n\nError: " + w.err.Error() + "\n\nPress q to quit"
	}

	if w.step == StepDone {
		return titleStyle.Render("New Project Wizard") + "\n\n✓ Project created successfully!\n\nPress q to return to lobby"
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render("New Project Wizard\n\n"))

	// Progress indicator
	steps := []string{"Name", "Folder", "Template", "Model", "Guidelines", "Confirm"}
	for i, step := range steps {
		if i < int(w.step) {
			s.WriteString(mutedStyle.Render("✓ "))
		} else if i == int(w.step) {
			s.WriteString(selectedStyle.Render("● "))
		} else {
			s.WriteString(mutedStyle.Render("○ "))
		}
		s.WriteString(step + " ")
	}
	s.WriteString("\n\n")

	switch w.step {
	case StepName:
		s.WriteString("Project name:\n")
		if w.inputMode {
			s.WriteString(inputStyle.Render(w.inputLabel+": "+w.inputValue+"▌") + "\n")
		} else {
			s.WriteString(itemStyle.Render(w.name) + "\n")
			s.WriteString(helpStyle.Render("Press Enter to edit") + "\n")
		}
	case StepFolder:
		s.WriteString("Parent folder:\n")
		if w.inputMode {
			s.WriteString(inputStyle.Render(w.inputLabel+": "+w.inputValue+"▌") + "\n")
		} else {
			s.WriteString(itemStyle.Render(w.folder) + "\n")
			s.WriteString(helpStyle.Render("Press Enter to edit (default: ~/Proyectos)") + "\n")
		}
	case StepTemplate:
		s.WriteString("Select template:\n")
		for i, t := range Templates {
			prefix := "  "
			if i == w.cursor {
				prefix = "> "
			}
			line := prefix + t.Name
			if i == w.cursor {
				line = selectedStyle.Render(line)
				s.WriteString(line + " - " + t.Description + "\n")
			} else {
				line = itemStyle.Render(line)
				s.WriteString(line + "\n")
			}
		}
	case StepModel:
		s.WriteString("Select model (placeholder):\n")
		models := []string{"nemotron-3.5-lightning-free", "gpt-4", "claude-3", "custom"}
		for i, m := range models {
			prefix := "  "
			if i == w.cursor {
				prefix = "> "
			}
			line := prefix + m
			if i == w.cursor {
				line = selectedStyle.Render(line)
			} else {
				line = itemStyle.Render(line)
			}
			s.WriteString(line + "\n")
		}
	case StepGuidelines:
		s.WriteString("Select guidelines template:\n")
		for i, g := range GuidelinesTemplates {
			prefix := "  "
			if i == w.cursor {
				prefix = "> "
			}
			line := prefix + g.Name
			if i == w.cursor {
				line = selectedStyle.Render(line)
				s.WriteString(line + " - " + g.Description + "\n")
			} else {
				line = itemStyle.Render(line)
				s.WriteString(line + "\n")
			}
		}
	case StepConfirm:
		s.WriteString("Confirm creation:\n")
		s.WriteString(fmt.Sprintf("  Name: %s\n", w.name))
		s.WriteString(fmt.Sprintf("  Path: %s\n", filepath.Join(w.folder, w.name)))
		s.WriteString(fmt.Sprintf("  Template: %s\n", Templates[w.templateIdx].Name))
		s.WriteString(fmt.Sprintf("  Guidelines: %s\n", GuidelinesTemplates[w.guidelinesIdx].Name))
		s.WriteString("\n")
		options := []string{"Create Project", "Back"}
		for i, opt := range options {
			prefix := "  "
			if i == w.cursor {
				prefix = "> "
			}
			line := prefix + opt
			if i == w.cursor {
				line = selectedStyle.Render(line)
			} else {
				line = itemStyle.Render(line)
			}
			s.WriteString(line + "\n")
		}
	}

	s.WriteString("\n" + helpStyle.Render("Press q/Ctrl+C to quit, Esc to cancel input"))
	return s.String()
}
