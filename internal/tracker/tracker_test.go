package tracker

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		copper int64
		want   Light
	}{
		{"just under threshold", 3_999_999_999, Green},
		{"exactly threshold is red", 4_000_000_000, Red},
		{"over by copper rounds down to threshold", 4_000_009_999, Red},
		{"well under", 3_951_200_000, Green},
		{"well over", 5_000_000_000, Red},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.copper, 400000); got != c.want {
				t.Fatalf("Classify(%d, 400000) = %v, want %v", c.copper, got, c.want)
			}
		})
	}
}

func ptr(l Light) *Light { return &l }

func TestDecide(t *testing.T) {
	now := time.Date(2026, 11, 25, 14, 5, 0, 0, time.UTC)
	fresh := now.Add(-10 * time.Minute)
	const green, red = 3_951_200_000, 4_100_000_000

	cases := []struct {
		name    string
		copper  int64
		updated time.Time
		stored  *Light
		want    Decision
	}{
		{"first run notifies", green, fresh, nil, Decision{Display: Green, Notify: true, Current: Green}},
		{"same state no notify", green, fresh, ptr(Green), Decision{Display: Green, Notify: false, Current: Green}},
		{"green to red notifies", red, fresh, ptr(Green), Decision{Display: Red, Notify: true, Current: Red}},
		{"red to green notifies", green, fresh, ptr(Red), Decision{Display: Green, Notify: true, Current: Green}},
		{"stale never notifies", red, now.Add(-2*time.Hour - time.Second), ptr(Green), Decision{Display: Unknown, Stale: true, Notify: false, Current: Red}},
		{"stale first run never notifies", red, now.Add(-3 * time.Hour), nil, Decision{Display: Unknown, Stale: true, Notify: false, Current: Red}},
		{"exactly 2h is not stale", red, now.Add(-2 * time.Hour), ptr(Red), Decision{Display: Red, Notify: false, Current: Red}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(Observation{PriceCopper: c.copper, UpdatedAt: c.updated}, 400000, c.stored, now)
			if got != c.want {
				t.Fatalf("Decide = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestStateKey(t *testing.T) {
	a := StateKey("us", "https://x")
	b := StateKey("eu", "https://x")
	c := StateKey("us", "https://y")
	if a == b || a == c || b == c {
		t.Fatalf("keys collide: %s %s %s", a, b, c)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) {
		t.Fatalf("key %q is not 64 hex chars", a)
	}
	if strings.Contains(a, "https") {
		t.Fatalf("key leaks webhook: %s", a)
	}
	if StateKey("us", "https://x") != a {
		t.Fatal("key not deterministic")
	}
}

func TestLightString(t *testing.T) {
	for l, want := range map[Light]string{Unknown: "unknown", Green: "green", Red: "red"} {
		if l.String() != want {
			t.Fatalf("%d.String() = %q, want %q", l, l.String(), want)
		}
	}
}
