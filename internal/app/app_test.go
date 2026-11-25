package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StormChild/Wow-Token-Tracker/internal/blizzard"
	"github.com/StormChild/Wow-Token-Tracker/internal/config"
	"github.com/StormChild/Wow-Token-Tracker/internal/discord"
	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

var now = time.Date(2026, 11, 25, 14, 5, 0, 0, time.Local)

const (
	hookA = "https://discord.com/api/webhooks/1/a"
	hookB = "https://discord.com/api/webhooks/2/b"
)

type fakePrices struct {
	gold  int64
	age   time.Duration
	err   error
	calls int
}

func (f *fakePrices) TokenPrice(context.Context, string) (tracker.Observation, error) {
	f.calls++
	if f.err != nil {
		return tracker.Observation{}, f.err
	}
	return tracker.Observation{PriceCopper: f.gold * 10000, UpdatedAt: now.Add(-f.age)}, nil
}

type harness struct {
	app      *App
	prices   *fakePrices
	settings config.Settings
	sent     []discord.Alert
	failNext bool
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{prices: &fakePrices{gold: 395120, age: 10 * time.Minute}}
	h.settings = config.Defaults()
	h.settings.ClientID, h.settings.ClientSecret, h.settings.WebhookURL = "id", "secret", hookA
	h.app = &App{
		Settings: func() config.Settings { return h.settings },
		Prices:   h.prices,
		Notify: func(_ context.Context, url string, a discord.Alert) error {
			if h.failNext {
				h.failNext = false
				return errors.New("Discord returned 500")
			}
			h.sent = append(h.sent, a)
			return nil
		},
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Now:       func() time.Time { return now },
	}
	return h
}

func (h *harness) check(t *testing.T) Status {
	t.Helper()
	return h.app.Check(context.Background())
}

func TestFirstRunNotifiesOnce(t *testing.T) {
	h := newHarness(t)
	h.check(t)
	s := h.check(t)
	if len(h.sent) != 1 || h.sent[0].Light != tracker.Green {
		t.Fatalf("sent = %+v", h.sent)
	}
	if s.Display != tracker.Green || s.PriceGold != 395120 || s.Err != nil {
		t.Fatalf("status = %+v", s)
	}
	a := h.sent[0]
	if a.PriceGold != 395120 || a.ThresholdGold != 400000 || a.Region != "eu" || a.Test {
		t.Fatalf("alert = %+v", a)
	}
}

func TestCrossingBothWays(t *testing.T) {
	h := newHarness(t)
	for _, g := range []int64{395000, 410000, 399999} {
		h.prices.gold = g
		h.check(t)
	}
	if len(h.sent) != 3 || h.sent[1].Light != tracker.Red || h.sent[2].Light != tracker.Green {
		t.Fatalf("sent = %+v", h.sent)
	}
}

func TestFetchErrorNoAlertStateKept(t *testing.T) {
	h := newHarness(t)
	h.check(t) // stores Green
	before, _ := os.ReadFile(h.app.StatePath)
	h.prices.err = blizzard.ErrBadCredentials
	s := h.check(t)
	if s.Display != tracker.Unknown || !errors.Is(s.Err, blizzard.ErrBadCredentials) {
		t.Fatalf("status = %+v", s)
	}
	if len(h.sent) != 1 {
		t.Fatalf("alerted on error: %+v", h.sent)
	}
	after, _ := os.ReadFile(h.app.StatePath)
	if string(before) != string(after) {
		t.Fatal("state changed on error")
	}
	if tip := Tooltip("us", 400000, s); !strings.Contains(tip, "Blizzard rejected") {
		t.Fatalf("tooltip = %q", tip)
	}
}

func TestNotifyFailureRetries(t *testing.T) {
	h := newHarness(t)
	h.check(t) // Green stored
	h.prices.gold = 410000
	h.failNext = true
	s := h.check(t)
	if s.Display != tracker.Red || s.Err == nil {
		t.Fatalf("status = %+v", s)
	}
	st, _ := config.LoadState(h.app.StatePath)
	if st[tracker.StateKey("eu", hookA)] != tracker.Green {
		t.Fatal("state advanced despite failed post")
	}
	h.check(t)
	if len(h.sent) != 2 || h.sent[1].Light != tracker.Red {
		t.Fatalf("no retry: %+v", h.sent)
	}
	if h.app.Last().Err != nil {
		t.Fatalf("error not cleared: %v", h.app.Last().Err)
	}
}

func TestStaleNoAlert(t *testing.T) {
	h := newHarness(t)
	h.prices.age = 3 * time.Hour
	s := h.check(t)
	if s.Display != tracker.Unknown || !s.Stale || len(h.sent) != 0 {
		t.Fatalf("status = %+v sent = %d", s, len(h.sent))
	}
}

func TestThresholdChangeFlips(t *testing.T) {
	h := newHarness(t)
	h.prices.gold = 395000
	h.check(t) // Green
	h.settings.ThresholdGold = 390000
	h.check(t)
	if len(h.sent) != 2 || h.sent[1].Light != tracker.Red || h.sent[1].ThresholdGold != 390000 {
		t.Fatalf("sent = %+v", h.sent)
	}
}

func TestWebhookChangeIsFirstRun(t *testing.T) {
	h := newHarness(t)
	h.check(t)
	h.settings.WebhookURL = hookB
	h.check(t)
	h.check(t)
	if len(h.sent) != 2 {
		t.Fatalf("sent = %d, want 2", len(h.sent))
	}
}

func TestNotConfigured(t *testing.T) {
	h := newHarness(t)
	h.settings.ClientID = ""
	s := h.check(t)
	if s.Display != tracker.Unknown || s.Err == nil || h.prices.calls != 0 {
		t.Fatalf("status = %+v calls = %d", s, h.prices.calls)
	}
}
