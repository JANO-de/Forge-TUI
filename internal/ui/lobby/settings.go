package lobby

import (
	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/settings"
)

// settingsForm edits the two fields of settings.Settings.
type settingsForm struct {
	vals [2]string // default model, editor
	idx  int
	msg  string
}

var settingsLabels = [2]string{"Default model (provider/model)", "Editor (overrides $EDITOR)"}

func newSettingsForm() *settingsForm {
	f := &settingsForm{}
	if s, err := settings.Load(); err == nil {
		f.vals = [2]string{s.DefaultModel, s.Editor}
	} else {
		f.msg = "could not read settings: " + err.Error()
	}
	return f
}

func (l Lobby) settingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := l.set
	switch msg.Type {
	case tea.KeyEsc:
		l.set = nil
	case tea.KeyTab, tea.KeyDown:
		f.idx = (f.idx + 1) % len(f.vals)
	case tea.KeyShiftTab, tea.KeyUp:
		f.idx = (f.idx + len(f.vals) - 1) % len(f.vals)
	case tea.KeyEnter:
		if err := settings.Save(settings.Settings{DefaultModel: f.vals[0], Editor: f.vals[1]}); err != nil {
			f.msg = "save failed: " + err.Error()
			return l, nil
		}
		settings.ApplyEditor()
		l.set = nil
	case tea.KeyBackspace:
		if r := []rune(f.vals[f.idx]); len(r) > 0 {
			f.vals[f.idx] = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		f.vals[f.idx] += " "
	case tea.KeyRunes:
		f.vals[f.idx] += string(msg.Runes)
	}
	return l, nil
}
