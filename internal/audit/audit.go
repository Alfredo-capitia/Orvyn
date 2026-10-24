package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Event struct {
	Timestamp string `json:"ts"`
	Actor     string `json:"actor"`
	Operation string `json:"op"`
	Target    string `json:"target,omitempty"`
	Approved  bool   `json:"approved,omitempty"`
	Result    string `json:"result"`
}

type Logger struct {
	path string
}

func New(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Log(e Event) error {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if e.Actor == "" {
		e.Actor = os.Getenv("USER")
		if e.Actor == "" {
			e.Actor = "unknown"
		}
	}
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
