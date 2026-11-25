package config

import (
	"path/filepath"
	"testing"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

func TestStateRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "state.json")
	in := map[string]tracker.Light{"k1": tracker.Green, "k2": tracker.Red}
	if err := SaveState(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadState(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out["k1"] != tracker.Green || out["k2"] != tracker.Red {
		t.Fatalf("got %v", out)
	}
}

func TestStateMissingFile(t *testing.T) {
	out, err := LoadState(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || len(out) != 0 {
		t.Fatalf("want empty non-nil map, got %v", out)
	}
}
