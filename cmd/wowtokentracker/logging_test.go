package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogRotates(t *testing.T) {
	dir := t.TempDir()
	big := make([]byte, 2<<20)
	if err := os.WriteFile(filepath.Join(dir, "tracker.log"), big, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if fi, err := os.Stat(filepath.Join(dir, "tracker.log.1")); err != nil || fi.Size() != int64(len(big)) {
		t.Fatalf("rotated file: %v", err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "tracker.log")); fi.Size() != 0 {
		t.Fatalf("fresh log size %d", fi.Size())
	}
}

func TestOpenLogAppendsWhenSmall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs") // also checks the folder is created
	f, err := openLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("one\n")
	f.Close()
	f, _ = openLog(dir)
	f.WriteString("two\n")
	f.Close()
	b, _ := os.ReadFile(filepath.Join(dir, "tracker.log"))
	if string(b) != "one\ntwo\n" {
		t.Fatalf("log = %q", b)
	}
}
