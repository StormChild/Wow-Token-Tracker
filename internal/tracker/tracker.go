package tracker

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Light is the tray state.
type Light int

const (
	Unknown Light = iota
	Green
	Red
)

func (l Light) String() string {
	switch l {
	case Green:
		return "green"
	case Red:
		return "red"
	default:
		return "unknown"
	}
}

// Observation is one token price reading from Blizzard.
type Observation struct {
	PriceCopper int64
	UpdatedAt   time.Time
}

// StaleAfter is how old Blizzard's last update may be before the data counts as stale.
const StaleAfter = 2 * time.Hour

const copperPerGold = 10000

// Classify returns Green when the price in whole gold is strictly below thresholdGold.
func Classify(priceCopper, thresholdGold int64) Light {
	if priceCopper/copperPerGold < thresholdGold {
		return Green
	}
	return Red
}

// StateKey identifies stored alert state for a region + webhook without storing the webhook.
func StateKey(region, webhookURL string) string {
	sum := sha256.Sum256([]byte(region + "\n" + webhookURL))
	return hex.EncodeToString(sum[:])
}

// Decision is the outcome of evaluating one observation.
type Decision struct {
	Display Light // what the icon shows; Unknown when stale
	Stale   bool
	Notify  bool  // true → post an alert for Current, then persist Current
	Current Light // Classify result (Green/Red), even when stale
}

// Decide evaluates obs against the threshold and the last alerted state (nil if none).
func Decide(obs Observation, thresholdGold int64, stored *Light, now time.Time) Decision {
	current := Classify(obs.PriceCopper, thresholdGold)
	if now.Sub(obs.UpdatedAt) > StaleAfter {
		return Decision{Display: Unknown, Stale: true, Current: current}
	}
	return Decision{
		Display: current,
		Notify:  stored == nil || *stored != current,
		Current: current,
	}
}
