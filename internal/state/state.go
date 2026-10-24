package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	ManagedComponents []string `json:"managed_components"`
	StackDir          string   `json:"stack_dir"`
	InstalledAt       string   `json:"installed_at,omitempty"`
}

const (
	DefaultDir  = "/var/lib/orvyn"
	DefaultFile = "state.json"
)

func Path() string {
	return filepath.Join(DefaultDir, DefaultFile)
}

func Load() (*State, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{ManagedComponents: []string{}}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.ManagedComponents == nil {
		s.ManagedComponents = []string{}
	}
	return &s, nil
}

func Save(s *State) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
