package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/StormChild/Wow-Token-Tracker/internal/config"
	"github.com/StormChild/Wow-Token-Tracker/internal/discord"
	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

// PriceSource returns the current token price for a region.
type PriceSource interface {
	TokenPrice(ctx context.Context, region string) (tracker.Observation, error)
}

// Notifier posts an alert to a webhook.
type Notifier func(ctx context.Context, webhookURL string, a discord.Alert) error

// ErrNotConfigured is reported while required settings are missing.
var ErrNotConfigured = errors.New("not configured — open Settings")

// Status is the outcome of one check, for the tray.
type Status struct {
	Display   tracker.Light
	PriceGold int64 // 0 if never fetched
	Checked   time.Time
	Stale     bool
	Err       error // fetch or notify error, nil on success
}

// App runs check cycles. Check calls are serialised.
type App struct {
	Settings  func() config.Settings
	Prices    PriceSource
	Notify    Notifier
	StatePath string
	Now       func() time.Time

	mu   sync.Mutex
	last Status
}

// Check fetches the price, decides, notifies on a crossing and persists the alerted state.
func (a *App) Check(ctx context.Context) Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.last = a.check(ctx)
	return a.last
}

// Last returns the most recent Check result.
func (a *App) Last() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

func (a *App) check(ctx context.Context) Status {
	now := a.Now()
	s := a.Settings()
	if !s.Complete() {
		return Status{Display: tracker.Unknown, Checked: now, Err: ErrNotConfigured}
	}
	obs, err := a.Prices.TokenPrice(ctx, s.Region)
	if err != nil {
		return Status{Display: tracker.Unknown, Checked: now, Err: err}
	}
	st := Status{PriceGold: obs.PriceCopper / 10000, Checked: now}

	state, err := config.LoadState(a.StatePath)
	if err != nil {
		// A corrupt state file is treated as first run rather than blocking alerts.
		state = map[string]tracker.Light{}
	}
	key := tracker.StateKey(s.Region, s.WebhookURL)
	var stored *tracker.Light
	if l, ok := state[key]; ok {
		stored = &l
	}
	d := tracker.Decide(obs, s.ThresholdGold, stored, now)
	st.Display, st.Stale = d.Display, d.Stale
	if !d.Notify {
		return st
	}
	alert := discord.Alert{
		Light: d.Current, PriceGold: st.PriceGold, ThresholdGold: s.ThresholdGold,
		Region: s.Region, UpdatedAt: obs.UpdatedAt,
	}
	if err := a.Notify(ctx, s.WebhookURL, alert); err != nil {
		st.Err = err
		return st
	}
	state[key] = d.Current
	if err := config.SaveState(a.StatePath, state); err != nil {
		st.Err = fmt.Errorf("couldn't save state: %w", err)
	}
	return st
}
