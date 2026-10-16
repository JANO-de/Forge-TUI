package project

import "forge/internal/settings"

// applyDefaultModel selects Settings.DefaultModel ("provider/model") when the
// server offers it; otherwise the server default chosen by setProviders stays.
func (a *agentState) applyDefaultModel() {
	s, err := settings.Load()
	if err != nil || s.DefaultModel == "" {
		return
	}
	for i, m := range a.models {
		if m.Provider+"/"+m.Model == s.DefaultModel {
			a.modelIdx = i
			return
		}
	}
}
