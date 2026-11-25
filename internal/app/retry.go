package app

import (
	"errors"
	"time"
)

// NextWait is how long to wait before the next check after `failures` failed checks in a row:
// the normal interval when healthy, otherwise 1, 2, then 5 minutes — never longer than poll.
func NextWait(failures int, poll time.Duration) time.Duration {
	var d time.Duration
	switch {
	case failures <= 0:
		return poll
	case failures == 1:
		d = time.Minute
	case failures == 2:
		d = 2 * time.Minute
	default:
		d = 5 * time.Minute
	}
	return min(d, poll)
}

// Retryable reports whether s failed in a way worth retrying sooner (network, API or Discord errors).
func Retryable(s Status) bool {
	return s.Err != nil && !errors.Is(s.Err, ErrNotConfigured)
}
