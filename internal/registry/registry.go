package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Project struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	LastOpened   time.Time `json:"lastOpened"`
	Model        string    `json:"model"`
	GitBranch    string    `json:"gitBranch,omitempty"`
	GitDirty     bool      `json:"gitDirty,omitempty"`
	TodoProgress string    `json:"todoProgress,omitempty"`
}

type Registry struct {
	Projects []Project `json:"projects"`
}

const (
	ConfigDir    = ".config/forge"
	ProjectsFile = "projects.json"
)

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ConfigDir), nil
}

func GetProjectsPath() (string, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(configPath, ProjectsFile), nil
}

func Load() (*Registry, error) {
	projectsPath, err := GetProjectsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(projectsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{Projects: []Project{}}, nil
		}
		return nil, err
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}
	return &registry, nil
}

func (r *Registry) Save() error {
	projectsPath, err := GetProjectsPath()
	if err != nil {
		return err
	}

	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(projectsPath, data, 0644)
}

func (r *Registry) Add(project Project) error {
	r.Projects = append(r.Projects, project)
	return r.Save()
}

func (r *Registry) Remove(path string) error {
	for i, p := range r.Projects {
		if p.Path == path {
			r.Projects = append(r.Projects[:i], r.Projects[i+1:]...)
			return r.Save()
		}
	}
	return nil
}

func (r *Registry) Update(path string, updates func(*Project)) error {
	for i := range r.Projects {
		if r.Projects[i].Path == path {
			updates(&r.Projects[i])
			return r.Save()
		}
	}
	return nil
}

func (r *Registry) Get(path string) *Project {
	for i := range r.Projects {
		if r.Projects[i].Path == path {
			return &r.Projects[i]
		}
	}
	return nil
}

func (r *Registry) List() []Project {
	return r.Projects
}

// Touch registers path (adding it when unknown) and sets LastOpened to now.
func Touch(path string) error {
	r, err := Load()
	if err != nil {
		return err
	}
	if r.Get(path) == nil {
		return r.Add(Project{Path: path, Name: filepath.Base(path), LastOpened: time.Now()})
	}
	return r.Update(path, func(p *Project) { p.LastOpened = time.Now() })
}
