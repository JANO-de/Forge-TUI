// Package settings stores Forge's user settings in ~/.config/forge/settings.json.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	// DefaultModel is "provider/model"; empty = OpenCode's default.
	DefaultModel string `json:"defaultModel,omitempty"`
	// Editor overrides $EDITOR (may carry arguments, e.g. "code -w").
	Editor string `json:"editor,omitempty"`
}

func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "forge", "settings.json"), nil
}

// Load returns the saved settings, or zero values when none exist.
func Load() (Settings, error) {
	var s Settings
	p, err := path()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

func Save(s Settings) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	s.DefaultModel = strings.TrimSpace(s.DefaultModel)
	s.Editor = strings.TrimSpace(s.Editor)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// ApplyEditor exports Settings.Editor as $EDITOR when it is set.
func ApplyEditor() {
	if s, err := Load(); err == nil && s.Editor != "" {
		_ = os.Setenv("EDITOR", s.Editor)
	}
}
