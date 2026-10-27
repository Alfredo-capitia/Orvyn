package apply

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
)

type Step struct {
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Path   string    `json:"path,omitempty"`
	At     time.Time `json:"at,omitempty"`
	Err    string    `json:"err,omitempty"`
}

type Session struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	Steps     []Step    `json:"steps"`
	Created   []string  `json:"created"`
	Compose   string    `json:"compose"`
	Status    string    `json:"status"`
}

func sessionPath() string {
	cfg := config.Default()
	return filepath.Join(cfg.StateDir, "orvyn-apply.json")
}

func NewSession() *Session {
	return &Session{
		ID:        time.Now().Format("20060102-150405"),
		StartedAt: time.Now(),
		Steps:     []Step{},
		Created:   []string{},
		Status:    "in_progress",
	}
}

func (s *Session) AddStep(name, status, path string) {
	s.Steps = append(s.Steps, Step{Name: name, Status: status, Path: path, At: time.Now()})
}

func (s *Session) AddCreated(p string) {
	s.Created = append(s.Created, p)
}

func (s *Session) Save() error {
	p := sessionPath()
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

func Load() (*Session, error) {
	p := sessionPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func Clear() error {
	p := sessionPath()
	return os.Remove(p)
}
