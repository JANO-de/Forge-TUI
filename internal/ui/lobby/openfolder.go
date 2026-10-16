package lobby

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/registry"
)

type openFolderMsg struct{}

type folderRowKind int

const (
	rowOpen folderRowKind = iota // open the current folder as a project
	rowUp                        // parent folder
	rowDir                       // sub-folder
)

type folderRow struct {
	kind folderRowKind
	name string
	path string
}

// folderBrowser is the "open an existing folder as a project" picker.
type folderBrowser struct {
	dir    string
	rows   []folderRow
	cursor int
	hidden bool // show dot-folders
	err    string
}

func newFolderBrowser(start string) *folderBrowser {
	fb := &folderBrowser{}
	fb.load(start, "")
	if fb.dir == "" {
		fb.load("/", "")
	}
	return fb
}

// load lists dir. On failure the previous folder stays and err is set.
// select names the entry to put the cursor on (the folder we came from).
func (fb *folderBrowser) load(dir, selectPath string) {
	dir = filepath.Clean(dir)
	ents, err := os.ReadDir(dir)
	if err != nil {
		fb.err = err.Error()
		return
	}
	fb.err, fb.dir = "", dir
	fb.rows = []folderRow{{kind: rowOpen, name: filepath.Base(dir), path: dir}}
	if parent := filepath.Dir(dir); parent != dir {
		fb.rows = append(fb.rows, folderRow{kind: rowUp, name: "..", path: parent})
	}
	for _, e := range ents {
		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, e.Name())); err == nil && st.IsDir() {
				isDir = true
			}
		}
		if !isDir || (!fb.hidden && strings.HasPrefix(e.Name(), ".")) {
			continue
		}
		fb.rows = append(fb.rows, folderRow{kind: rowDir, name: e.Name(), path: filepath.Join(dir, e.Name())})
	}
	fb.cursor = 0
	for i, r := range fb.rows {
		if selectPath != "" && r.path == selectPath {
			fb.cursor = i
		}
	}
}

func (fb *folderBrowser) up() {
	if parent := filepath.Dir(fb.dir); parent != fb.dir {
		fb.load(parent, fb.dir)
	}
}

func (l Lobby) startDir() string {
	if p, ok := registry.MostRecent(); ok {
		return filepath.Dir(p.Path)
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "/"
}

func openProject(path string) tea.Cmd {
	return func() tea.Msg { return SwitchToProjectMsg{Path: path} }
}

func (l Lobby) folderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fb := l.fb
	last := max(len(fb.rows)-1, 0)
	page := 10
	switch msg.Type {
	case tea.KeyEsc:
		l.fb = nil
	case tea.KeyUp:
		fb.cursor = max(fb.cursor-1, 0)
	case tea.KeyDown:
		fb.cursor = min(fb.cursor+1, last)
	case tea.KeyPgUp:
		fb.cursor = max(fb.cursor-page, 0)
	case tea.KeyPgDown:
		fb.cursor = min(fb.cursor+page, last)
	case tea.KeyHome:
		fb.cursor = 0
	case tea.KeyEnd:
		fb.cursor = last
	case tea.KeyLeft, tea.KeyBackspace:
		fb.up()
	case tea.KeyRight:
		fb.descend()
	case tea.KeyEnter:
		if fb.cursor < len(fb.rows) && fb.rows[fb.cursor].kind == rowOpen {
			return l, openProject(fb.dir)
		}
		fb.descend()
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			break
		}
		switch msg.Runes[0] {
		case 'k':
			fb.cursor = max(fb.cursor-1, 0)
		case 'j':
			fb.cursor = min(fb.cursor+1, last)
		case 'h':
			fb.up()
		case 'l':
			fb.descend()
		case 'o':
			return l, openProject(fb.dir)
		case '.':
			fb.hidden = !fb.hidden
			fb.load(fb.dir, "")
		case '~':
			if h, err := os.UserHomeDir(); err == nil {
				fb.load(h, "")
			}
		case 'q':
			l.fb = nil
		}
	}
	return l, nil
}

// descend enters the folder under the cursor (the ".." row goes up).
func (fb *folderBrowser) descend() {
	if fb.cursor >= len(fb.rows) {
		return
	}
	switch r := fb.rows[fb.cursor]; r.kind {
	case rowUp:
		fb.up()
	case rowDir:
		fb.load(r.path, "")
	}
}

func tailPath(s string, n int) string {
	r := []rune(s)
	if n < 8 || len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n+1:])
}

func (l Lobby) folderLines() []string {
	fb := l.fb
	out := []string{headStyle.Render("▌ Open folder as project"), mutedStyle.Render(tailPath(fb.dir, cardWidth-6)), ""}
	if fb.err != "" {
		out = append(out, errStyle.Render(tailPath(fb.err, cardWidth-6)), "")
	}
	visible := 12
	if l.height > 0 {
		visible = min(max(l.height-22, 5), 20)
	}
	start := min(max(fb.cursor-visible/2, 0), max(len(fb.rows)-visible, 0))
	for i := start; i < len(fb.rows) && i < start+visible; i++ {
		r := fb.rows[i]
		var label string
		switch r.kind {
		case rowOpen:
			label = "● Open this folder  (" + r.name + ")"
		case rowUp:
			label = "↑ .."
		default:
			label = "  " + r.name + "/"
		}
		if i == fb.cursor {
			out = append(out, selectedStyle.Render("▸ "+label))
		} else {
			out = append(out, itemStyle.Render("  "+label))
		}
	}
	if len(fb.rows) <= 2 {
		out = append(out, mutedStyle.Render("  (no sub-folders)"))
	}
	return append(out, "", mutedStyle.Render("↑↓ move · Enter open/enter · ←/h up · o open this folder · . hidden · ~ home · Esc cancel"))
}
