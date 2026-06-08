package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type SessionState struct {
	SearchQuery  string   `json:"searchQuery"`
	LevelFilter  string   `json:"levelFilter"`
	HiddenFields []string `json:"hiddenFields"`
	ShowLineNum  bool     `json:"showLineNum"`
}

func sessionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "state", "logview")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "session.json"), nil
}

func SaveSession(state SessionState) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadSession() (*SessionState, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
