package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Settings are the user's configuration. ClientSecret and WebhookURL never go to JSON.
type Settings struct {
	Region        string `json:"region"`
	ClientID      string `json:"clientId"`
	ThresholdGold int64  `json:"thresholdGold"`
	PollMinutes   int    `json:"pollMinutes"`
	ClientSecret  string `json:"-"`
	WebhookURL    string `json:"-"`
}

// Regions lists the supported Blizzard regions, in display order.
var Regions = []string{"us", "eu", "kr", "tw"}

const (
	DefaultRegion    = "us"
	DefaultThreshold = 400000
	DefaultPoll      = 20
	MinPoll          = 5
)

var webhookPrefixes = []string{
	"https://discord.com/api/webhooks/",
	"https://discordapp.com/api/webhooks/",
}

func Defaults() Settings {
	return Settings{Region: DefaultRegion, ThresholdGold: DefaultThreshold, PollMinutes: DefaultPoll}
}

func validRegion(r string) bool {
	for _, v := range Regions {
		if v == r {
			return true
		}
	}
	return false
}

// Validate checks field values. Empty credentials are allowed here; see Complete.
func (s Settings) Validate() error {
	var errs []error
	if !validRegion(s.Region) {
		errs = append(errs, fmt.Errorf("region must be one of %s", strings.Join(Regions, ", ")))
	}
	if s.ThresholdGold <= 0 {
		errs = append(errs, errors.New("threshold must be more than 0 gold"))
	}
	if s.PollMinutes < MinPoll {
		errs = append(errs, fmt.Errorf("check interval must be at least %d minutes", MinPoll))
	}
	if s.WebhookURL != "" {
		ok := false
		for _, p := range webhookPrefixes {
			if strings.HasPrefix(s.WebhookURL, p) {
				ok = true
			}
		}
		if !ok {
			errs = append(errs, errors.New("webhook URL must start with https://discord.com/api/webhooks/"))
		}
	}
	return errors.Join(errs...)
}

// Complete reports whether the settings are enough to run.
func (s Settings) Complete() bool {
	return s.ClientID != "" && s.ClientSecret != "" && s.WebhookURL != "" && s.Validate() == nil
}

// Store reads and writes settings.json in Dir and secrets in Secrets.
type Store struct {
	Dir     string
	Secrets SecretStore
}

// DataDir is %APPDATA%\WowTokenTracker.
func DataDir() string {
	return filepath.Join(os.Getenv("APPDATA"), "WowTokenTracker")
}

// LogDir is %LOCALAPPDATA%\WowTokenTracker\logs.
func LogDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "WowTokenTracker", "logs")
}

func (st Store) path() string { return filepath.Join(st.Dir, "settings.json") }

// Load returns saved settings, falling back to defaults for a missing file or bad values.
func (st Store) Load() (Settings, error) {
	s := Defaults()
	raw, err := os.ReadFile(st.path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return s, err
	default:
		if err := json.Unmarshal(raw, &s); err != nil {
			return Defaults(), fmt.Errorf("settings.json is not valid: %w", err)
		}
	}
	if !validRegion(s.Region) {
		s.Region = DefaultRegion
	}
	if s.PollMinutes < MinPoll {
		s.PollMinutes = MinPoll
	}
	if s.ThresholdGold <= 0 {
		s.ThresholdGold = DefaultThreshold
	}
	if s.ClientSecret, err = st.Secrets.Get(SecretClientSecret); err != nil {
		return s, err
	}
	if s.WebhookURL, err = st.Secrets.Get(SecretWebhookURL); err != nil {
		return s, err
	}
	return s, nil
}

// Save validates, then writes settings.json and the secrets.
func (st Store) Save(s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(st.path(), raw); err != nil {
		return err
	}
	if err := putSecret(st.Secrets, SecretClientSecret, s.ClientSecret); err != nil {
		return err
	}
	return putSecret(st.Secrets, SecretWebhookURL, s.WebhookURL)
}

func putSecret(ss SecretStore, name, value string) error {
	if value == "" {
		return ss.Delete(name)
	}
	return ss.Set(name, value)
}

// writeAtomic writes data to a temp file in the same directory, then renames it over path.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
