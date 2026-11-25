package main

import (
	"os"
	"path/filepath"
)

const maxLogBytes = 1 << 20

// openLog opens dir\tracker.log for appending, first rotating it to tracker.log.1 if it is over 1 MiB.
func openLog(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "tracker.log")
	if fi, err := os.Stat(p); err == nil && fi.Size() > maxLogBytes {
		_ = os.Remove(p + ".1")
		_ = os.Rename(p, p+".1")
	}
	return os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
