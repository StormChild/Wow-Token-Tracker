package config

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

// StatePath is %APPDATA%\WowTokenTracker\state.json.
func StatePath() string { return DataDir() + `\state.json` }

// LoadState reads last-alerted lights keyed by tracker.StateKey. A missing file is empty.
func LoadState(path string) (map[string]tracker.Light, error) {
	m := map[string]tracker.Light{}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]tracker.Light{}, err
	}
	return m, nil
}

// SaveState writes the state map atomically.
func SaveState(path string, m map[string]tracker.Light) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, raw)
}
