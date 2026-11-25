package app

import (
	"testing"
	"time"
)

func TestNextWait(t *testing.T) {
	poll := 20 * time.Minute
	cases := []struct {
		failures int
		poll     time.Duration
		want     time.Duration
	}{
		{0, poll, poll},
		{1, poll, time.Minute},
		{2, poll, 2 * time.Minute},
		{3, poll, 5 * time.Minute},
		{9, poll, 5 * time.Minute},
		{3, 3 * time.Minute, 3 * time.Minute}, // never longer than the normal interval
	}
	for _, c := range cases {
		if got := NextWait(c.failures, c.poll); got != c.want {
			t.Errorf("NextWait(%d, %v) = %v, want %v", c.failures, c.poll, got, c.want)
		}
	}
}

func TestRetryable(t *testing.T) {
	if Retryable(Status{Err: ErrNotConfigured}) {
		t.Error("not-configured should wait for Settings, not retry")
	}
	if !Retryable(Status{Err: errTest}) {
		t.Error("fetch/notify errors should retry sooner")
	}
	if Retryable(Status{}) {
		t.Error("success is not retryable")
	}
}
