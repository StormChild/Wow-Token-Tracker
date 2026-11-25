package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"

	"github.com/StormChild/Wow-Token-Tracker/assets"
	"github.com/StormChild/Wow-Token-Tracker/internal/app"
	"github.com/StormChild/Wow-Token-Tracker/internal/autostart"
	"github.com/StormChild/Wow-Token-Tracker/internal/blizzard"
	"github.com/StormChild/Wow-Token-Tracker/internal/config"
	"github.com/StormChild/Wow-Token-Tracker/internal/discord"
	"github.com/StormChild/Wow-Token-Tracker/internal/settingsui"
	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

type tray struct {
	store config.Store
	auto  autostart.Autostart
	exe   string
	httpc *http.Client
	app   *app.App

	mu       sync.Mutex
	settings config.Settings
	ui       *settingsui.Server

	recheck chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
}

func newTray(store config.Store, auto autostart.Autostart, exe string) *tray {
	t := &tray{store: store, auto: auto, exe: exe, httpc: &http.Client{Timeout: 15 * time.Second},
		recheck: make(chan struct{}, 1)}
	s, err := store.Load()
	if err != nil {
		slog.Error("loading settings", "err", err)
	}
	t.settings = s
	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.app = &app.App{
		Settings: t.current,
		Prices:   &prices{get: t.current},
		Notify: func(ctx context.Context, url string, a discord.Alert) error {
			return discord.Send(ctx, t.httpc, url, a)
		},
		StatePath: filepath.Join(store.Dir, "state.json"),
		Now:       time.Now,
	}
	return t
}

func (t *tray) current() config.Settings {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.settings
}

// prices keeps one Blizzard client per credential pair so its access token is reused.
type prices struct {
	get func() config.Settings
	mu  sync.Mutex
	key string
	c   *blizzard.Client
}

func (p *prices) TokenPrice(ctx context.Context, region string) (tracker.Observation, error) {
	s := p.get()
	p.mu.Lock()
	if k := s.ClientID + "\x00" + s.ClientSecret; p.c == nil || p.key != k {
		p.c, p.key = blizzard.New(s.ClientID, s.ClientSecret), k
	}
	c := p.c
	p.mu.Unlock()
	return c.TokenPrice(ctx, region)
}

func (t *tray) onReady() {
	systray.SetIcon(assets.Icon(tracker.Unknown))
	systray.SetTitle("WoW Token Tracker")
	systray.SetTooltip("WoW Token Tracker: starting…")

	mCheck := systray.AddMenuItem("Check now", "Check the token price now")
	mSettings := systray.AddMenuItem("Settings…", "Open the settings page in your browser")
	enabled, _ := t.auto.Enabled()
	mAuto := systray.AddMenuItemCheckbox("Start with Windows", "Start automatically when you log in", enabled)
	mLogs := systray.AddMenuItem("Open log folder", "Show the log file")
	systray.AddSeparator()
	mExit := systray.AddMenuItem("Exit", "Close WoW Token Tracker")

	if enabled {
		t.repointAutostart()
	}
	if !t.current().Complete() {
		t.openSettings()
	}
	go t.loop()

	go func() {
		for {
			select {
			case <-mCheck.ClickedCh:
				t.triggerCheck()
			case <-mSettings.ClickedCh:
				t.openSettings()
			case <-mAuto.ClickedCh:
				t.toggleAutostart(mAuto)
			case <-mLogs.ClickedCh:
				start("explorer", config.LogDir())
			case <-mExit.ClickedCh:
				systray.Quit()
				return
			case <-t.ctx.Done():
				return
			}
		}
	}()
}

func (t *tray) onExit() {
	t.cancel()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ui != nil {
		t.ui.Close()
	}
}

func (t *tray) triggerCheck() {
	select {
	case t.recheck <- struct{}{}:
	default:
	}
}

// loop checks now, then whenever PollMinutes of wall-clock time have passed (robust to sleep/resume)
// or a recheck is requested.
func (t *tray) loop() {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	var last time.Time
	var lastErr string
	for {
		st := t.app.Check(t.ctx)
		last = time.Now()
		s := t.current()
		systray.SetIcon(assets.Icon(st.Display))
		systray.SetTooltip(app.Tooltip(s.Region, s.ThresholdGold, st))
		if e := errString(st.Err); e != lastErr {
			if e != "" {
				slog.Warn("check failed", "err", e)
			} else {
				slog.Info("check ok again")
			}
			lastErr = e
		}
		slog.Debug("checked", "light", st.Display, "gold", st.PriceGold, "stale", st.Stale)
	wait:
		for {
			select {
			case <-t.ctx.Done():
				return
			case <-t.recheck:
				break wait
			case <-tick.C:
				if time.Since(last.Round(0)) >= time.Duration(t.current().PollMinutes)*time.Minute {
					break wait
				}
			}
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (t *tray) openSettings() {
	t.mu.Lock()
	if t.ui == nil {
		ui, err := settingsui.Start(settingsui.Callbacks{Load: t.current, Save: t.save, Test: t.test})
		if err != nil {
			t.mu.Unlock()
			slog.Error("settings page", "err", err)
			return
		}
		t.ui = ui
	}
	url := t.ui.URL()
	t.mu.Unlock()
	start("rundll32", "url.dll,FileProtocolHandler", url)
}

func (t *tray) save(s config.Settings) error {
	if err := t.store.Save(s); err != nil {
		return err
	}
	t.mu.Lock()
	t.settings = s
	t.mu.Unlock()
	slog.Info("settings saved", "region", s.Region, "threshold", s.ThresholdGold, "poll", s.PollMinutes)
	t.triggerCheck()
	return nil
}

func (t *tray) test(ctx context.Context, s config.Settings) (int64, error) {
	obs, err := blizzard.New(s.ClientID, s.ClientSecret).TokenPrice(ctx, s.Region)
	if err != nil {
		return 0, err
	}
	gold := obs.PriceCopper / 10000
	a := discord.Alert{Light: tracker.Classify(obs.PriceCopper, s.ThresholdGold), PriceGold: gold,
		ThresholdGold: s.ThresholdGold, Region: s.Region, UpdatedAt: obs.UpdatedAt, Test: true}
	return gold, discord.Send(ctx, t.httpc, s.WebhookURL, a)
}

func (t *tray) toggleAutostart(m *systray.MenuItem) {
	if m.Checked() {
		if err := t.auto.Disable(); err != nil {
			slog.Error("disable autostart", "err", err)
			systray.SetTooltip("Couldn't turn off Start with Windows: " + err.Error())
			return
		}
		m.Uncheck()
		slog.Info("autostart disabled")
		return
	}
	if err := t.auto.Enable(t.exe); err != nil {
		slog.Error("enable autostart", "err", err)
		systray.SetTooltip("Couldn't turn on Start with Windows: " + err.Error())
		return
	}
	m.Check()
	slog.Info("autostart enabled", "exe", t.exe)
}

// repointAutostart fixes the task if the exe was moved since autostart was turned on.
func (t *tray) repointAutostart() {
	p, err := t.auto.TaskPath()
	if err != nil || t.exe == "" || strings.EqualFold(filepath.Clean(p), filepath.Clean(t.exe)) {
		return
	}
	if err := t.auto.Enable(t.exe); err != nil {
		slog.Error("repoint autostart", "err", err)
		return
	}
	slog.Info("autostart repointed", "from", p, "to", t.exe)
}

// start launches a helper program without a console window and without waiting.
func start(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		slog.Error("start "+name, "err", err)
		return
	}
	go cmd.Wait()
}
