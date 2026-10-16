package panes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	dirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("039")).Bold(true)
	fileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true).Background(lipgloss.Color("236"))
	modifiedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	addedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("046"))
	deletedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	branchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	lineNumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	stagedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
)

type FileNode struct {
	Name      string
	Path      string
	IsDir     bool
	Children  []*FileNode
	Expanded  bool
	GitStatus string // "", "M", "A", "D", "??"
	Parent    *FileNode
	Depth     int
}

type FileTree struct {
	Root       *FileNode
	Cursor     int
	Visible    []*FileNode
	Width      int
	Height     int
	ShowHidden bool
	marks      map[string]string // absolute path -> git mark, see gitx.Status
}

func NewFileTree(rootPath string) (*FileTree, error) {
	root := &FileNode{
		Name:     filepath.Base(rootPath),
		Path:     rootPath,
		IsDir:    true,
		Expanded: true,
		Depth:    0,
	}

	ft := &FileTree{
		Root:       root,
		Cursor:     0,
		ShowHidden: false,
	}

	if err := ft.loadChildren(root); err != nil {
		return nil, err
	}
	ft.updateVisible()
	return ft, nil
}

// loadChildren (re)reads the direct children of node. Sub-directories are
// loaded when they are expanded, not before.
func (ft *FileTree) loadChildren(node *FileNode) error {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		return err
	}
	node.Children = nil

	for _, entry := range entries {
		if !ft.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		childPath := filepath.Join(node.Path, entry.Name())
		isDir := entry.IsDir()

		node.Children = append(node.Children, &FileNode{
			Name:      entry.Name(),
			Path:      childPath,
			IsDir:     isDir,
			Depth:     node.Depth + 1,
			Parent:    node,
			GitStatus: ft.getGitStatus(childPath, isDir),
		})
	}

	// Sort: directories first, then files, alphabetically
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	return nil
}

// getGitStatus returns the mark for a path. A directory shows "??" when
// everything changed below it is untracked, and otherwise a modified mark
// ("M", or "m" when all of it is staged).
func (ft *FileTree) getGitStatus(path string, isDir bool) string {
	if !isDir {
		return ft.marks[path]
	}
	prefix := path + string(filepath.Separator)
	found, untrackedOnly, allStaged, conflict := false, true, true, false
	for p, m := range ft.marks {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		found = true
		if m != "??" {
			untrackedOnly = false
		}
		if m != "m" && m != "a" && m != "d" {
			allStaged = false
		}
		if m == "U" {
			conflict = true
		}
	}
	switch {
	case !found:
		return ""
	case conflict:
		return "U"
	case untrackedOnly:
		return "??"
	case allStaged:
		return "m"
	}
	return "M"
}

// SetStatus installs new git marks (absolute path -> mark) on every node.
func (ft *FileTree) SetStatus(marks map[string]string) {
	ft.marks = marks
	var walk func(n *FileNode)
	walk = func(n *FileNode) {
		n.GitStatus = ft.getGitStatus(n.Path, n.IsDir)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(ft.Root)
}

// Refresh rereads every expanded directory from disk, keeping expansion and
// the selected entry, so files created or deleted by the agent show up.
func (ft *FileTree) Refresh() {
	sel := ""
	if n := ft.GetSelected(); n != nil {
		sel = n.Path
	}
	open := map[string]bool{}
	var collect func(n *FileNode)
	collect = func(n *FileNode) {
		for _, c := range n.Children {
			if c.IsDir && c.Expanded {
				open[c.Path] = true
				collect(c)
			}
		}
	}
	collect(ft.Root)

	var reload func(n *FileNode)
	reload = func(n *FileNode) {
		if ft.loadChildren(n) != nil {
			return
		}
		for _, c := range n.Children {
			if c.IsDir && open[c.Path] {
				c.Expanded = true
				reload(c)
			}
		}
	}
	reload(ft.Root)
	ft.updateVisible()

	for i, n := range ft.Visible {
		if n.Path == sel {
			ft.Cursor = i
			return
		}
	}
	ft.Cursor = min(ft.Cursor, max(len(ft.Visible)-1, 0))
}

func (ft *FileTree) updateVisible() {
	ft.Visible = nil
	ft.collectVisible(ft.Root)
}

func (ft *FileTree) collectVisible(node *FileNode) {
	ft.Visible = append(ft.Visible, node)
	if node.IsDir && node.Expanded {
		for _, child := range node.Children {
			ft.collectVisible(child)
		}
	}
}

func (ft *FileTree) GetSelected() *FileNode {
	if ft.Cursor >= 0 && ft.Cursor < len(ft.Visible) {
		return ft.Visible[ft.Cursor]
	}
	return nil
}

func (ft *FileTree) ToggleExpand() {
	node := ft.GetSelected()
	if node != nil && node.IsDir {
		node.Expanded = !node.Expanded
		if node.Expanded {
			ft.loadChildren(node)
		}
		ft.updateVisible()
	}
}

func (ft *FileTree) MoveUp() {
	if ft.Cursor > 0 {
		ft.Cursor--
	}
}

func (ft *FileTree) MoveDown() {
	if ft.Cursor < len(ft.Visible)-1 {
		ft.Cursor++
	}
}

func (ft *FileTree) MoveToParent() {
	node := ft.GetSelected()
	if node != nil && node.Parent != nil {
		for i, n := range ft.Visible {
			if n == node.Parent {
				ft.Cursor = i
				break
			}
		}
	}
}

func (ft *FileTree) View() string {
	var s strings.Builder

	for i, node := range ft.Visible {
		selected := i == ft.Cursor
		prefix := strings.Repeat("  ", node.Depth)

		if node.IsDir {
			icon := "▼"
			if !node.Expanded {
				icon = "▶"
			}

			line := fmt.Sprintf("%s%s %s", prefix, icon, node.Name)
			if selected {
				line = selectedStyle.Render(line)
			} else {
				line = dirStyle.Render(line)
			}

			if node.GitStatus != "" {
				status := ft.formatGitStatus(node.GitStatus)
				line += " " + status
			}

			s.WriteString(line + "\n")
		} else {
			line := fmt.Sprintf("%s  %s", prefix, node.Name)
			if selected {
				line = selectedStyle.Render(line)
			} else {
				line = fileStyle.Render(line)
			}

			if node.GitStatus != "" {
				status := ft.formatGitStatus(node.GitStatus)
				line += " " + status
			}

			s.WriteString(line + "\n")
		}
	}

	return s.String()
}

func (ft *FileTree) formatGitStatus(status string) string {
	switch status {
	case "M":
		return modifiedStyle.Render("●")
	case "A":
		return addedStyle.Render("+")
	case "D":
		return deletedStyle.Render("✕")
	case "??":
		return branchStyle.Render("?")
	case "U":
		return deletedStyle.Render("!")
	case "m":
		return stagedStyle.Render("●")
	case "a":
		return stagedStyle.Render("+")
	case "d":
		return stagedStyle.Render("✕")
	default:
		return mutedStyle.Render(status)
	}
}

type FileViewer struct {
	Content      string
	Path         string
	Width        int
	Height       int
	ScrollY      int
	ShowLineNums bool
	FoldedRanges []foldRange // ranges of lines that are folded
}

type foldRange struct {
	Start, End int
}

func NewFileViewer(path string) (*FileViewer, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return &FileViewer{
		Content:      string(content),
		Path:         path,
		ShowLineNums: true,
	}, nil
}

func (fv *FileViewer) View() string {
	if fv.Content == "" {
		return mutedStyle.Render("(empty file)")
	}

	lines := strings.Split(fv.Content, "\n")
	var s strings.Builder

	for i, line := range lines {
		lineNum := fmt.Sprintf("%4d │ ", i+1)
		if fv.ShowLineNums {
			s.WriteString(lineNumStyle.Render(lineNum))
		}
		s.WriteString(line)
		s.WriteString("\n")
	}

	return s.String()
}

func (fv *FileViewer) OpenInEditor() tea.Cmd { return EditFile(fv.Path) }

// EditFile opens path in $EDITOR (default vi) and reports back with an
// EditorFinishedMsg. $EDITOR may carry arguments, e.g. "code -w".
func EditFile(path string) tea.Cmd {
	fields := strings.Fields(os.Getenv("EDITOR"))
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	cmd := exec.Command(fields[0], append(fields[1:], path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorFinishedMsg{Path: path, Err: err}
	})
}

func (fv *FileViewer) Reload() error {
	content, err := os.ReadFile(fv.Path)
	if err != nil {
		return err
	}
	fv.Content = string(content)
	return nil
}

type EditorFinishedMsg struct {
	Path string
	Err  error
}
