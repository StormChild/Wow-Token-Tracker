package app

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

func TestTooltip(t *testing.T) {
	long := errors.New(strings.Repeat("x", 300))
	cases := []struct {
		name string
		s    Status
		want []string
	}{
		{"green", Status{Display: tracker.Green, PriceGold: 395120, Checked: now},
			[]string{"WoW Token (US): 395,120g · under 400,000g · checked 14:05"}},
		{"red", Status{Display: tracker.Red, PriceGold: 410000, Checked: now},
			[]string{"WoW Token (US): 410,000g · at/above 400,000g · checked 14:05"}},
		{"stale", Status{Display: tracker.Unknown, Stale: true, PriceGold: 395120, Checked: now},
			[]string{"data stale"}},
		{"error", Status{Display: tracker.Unknown, Err: errors.New("can't reach Blizzard"), Checked: now},
			[]string{"WoW Token (US)", "can't reach Blizzard"}},
		{"notify failed", Status{Display: tracker.Red, PriceGold: 410000, Err: errors.New("Discord returned 500"), Checked: now},
			[]string{"410,000g", "Discord returned 500"}},
		{"long error", Status{Display: tracker.Unknown, Err: long, Checked: now},
			[]string{"…"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Tooltip("us", 400000, c.s)
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("tooltip %q missing %q", got, w)
				}
			}
			if n := utf8.RuneCountInString(got); n > 127 {
				t.Errorf("tooltip is %d chars", n)
			}
		})
	}
	if got := Tooltip("us", 400000, cases[0].s); got != cases[0].want[0] {
		t.Errorf("green exact: %q", got)
	}
}
