package app

import (
	"strings"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

// MaxTooltip is the Windows tray tooltip limit (128 UTF-16 units including the terminator).
const MaxTooltip = 127

// Tooltip renders the tray tooltip for s, at most MaxTooltip characters.
func Tooltip(region string, thresholdGold int64, s Status) string {
	parts := []string{"WoW Token (" + strings.ToUpper(region) + ")"}
	switch {
	case s.Stale:
		parts = append(parts, "data stale", "last "+tracker.FormatGold(s.PriceGold))
	case s.Display == tracker.Green:
		parts = append(parts, tracker.FormatGold(s.PriceGold), "under "+tracker.FormatGold(thresholdGold))
	case s.Display == tracker.Red:
		parts = append(parts, tracker.FormatGold(s.PriceGold), "at/above "+tracker.FormatGold(thresholdGold))
	}
	if s.Err != nil {
		parts = append(parts, s.Err.Error())
	}
	if !s.Checked.IsZero() {
		parts = append(parts, "checked "+s.Checked.Format("15:04"))
	}
	out := parts[0] + ": " + strings.Join(parts[1:], " · ")
	if r := []rune(out); len(r) > MaxTooltip {
		out = string(r[:MaxTooltip-1]) + "…"
	}
	return out
}
