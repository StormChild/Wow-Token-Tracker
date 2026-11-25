// Command wowtokentracker is the WoW Token Tracker tray app.
package main

import (
	"log/slog"
	"os"

	"fyne.io/systray"

	"github.com/StormChild/Wow-Token-Tracker/internal/autostart"
	"github.com/StormChild/Wow-Token-Tracker/internal/config"
)

// version is set at release build time with -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	release, ok := acquireSingleInstance()
	if !ok {
		return // already running; its icon is in the tray
	}
	defer release()

	if f, err := openLog(config.LogDir()); err == nil {
		defer f.Close()
		slog.SetDefault(slog.New(slog.NewTextHandler(f, nil)))
	}
	slog.Info("starting", "version", version)

	exe, err := os.Executable()
	if err != nil {
		slog.Error("can't find own path", "err", err)
	}
	t := newTray(config.Store{Dir: config.DataDir(), Secrets: config.WinCred()}, autostart.Default(), exe)
	systray.Run(t.onReady, t.onExit)
	slog.Info("exited")
}
