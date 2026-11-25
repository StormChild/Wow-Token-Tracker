# WoW Token Tracker — Design

Date: 2026-11-25
Status: Draft, awaiting approval

## Intent

The owner buys WoW Tokens with gold when they're cheap — currently "cheap" means
**under 400,000 gold**. They want to be told when the price crosses that line and to see
the current state at a glance. A friend (and others) should be able to download it, enter
their own Blizzard API credentials and Discord webhook, and be running in minutes.

## What it is

A Windows tray app — a "red/green robot":

- **Green** icon: token price is below the threshold (buy window open).
- **Red** icon: price is at or above the threshold.
- **Grey** icon: unknown (starting up, not configured, check failed, or stale data).
- Posts to Discord **only when the state flips** between green and red, in either
  direction ("Under 400,000g — buy", "Back above 400,000g").
- Distributed as a single `.exe` — no runtime or installer needed.
- Started at logon by a Windows Scheduled Task.

## Goals

- Poll Blizzard's official WoW Token API on a schedule.
- Red/green tray icon with a tooltip showing price, threshold, and last-checked time.
- Discord webhook message on each threshold crossing, and nothing else.
- All user-specific settings (credentials, webhook, region, threshold) configurable
  without rebuilding, so anyone can use it.
- A friend can go from download to running quickly.
- README covering: Battle.net API client setup, Discord webhook setup, first run, and
  Scheduled Task setup.

## Non-goals

- An in-game WoW addon (addons have no network access; not needed).
- Mac / Linux (for now — Go keeps this possible later).
- Discord bot commands / a bot account (webhook only).
- Price history, charts, or a database.
- Tracking multiple regions at once.
- China region (separate API, not supported).
- An installer or auto-update.

## Anti-goals (would be a failure even if it "works")

- Discord messages that aren't crossings: repeats while the price stays on one side, or
  messages on errors.
- Secrets stored in plain text or committed to the repo.
- Silent failure: the icon stays green/red while checks are failing or data is stale.
- Setup so fiddly a friend gives up.

## Constraints

- Blizzard API requires an OAuth client (client ID + secret) from https://develop.battle.net.
- Token price updates roughly every 20 minutes; price is returned in copper
  (1 gold = 10,000 copper).
- A Scheduled Task must run "only when user is logged on" for the tray icon to be visible.
- The `.exe` will be unsigned, so Windows SmartScreen shows a warning on first launch.

## Approaches considered

1. **Go — chosen.** Single ~10 MB `.exe`, no runtime to install, light enough to sit in
   the tray all day. No native GUI toolkit, so Settings is a local browser page.
2. **Tauri (Rust + web UI).** Real installer, native settings window, auto-update; but the
   most work to build and maintain. Overkill for Windows-only, get-a-friend-running.
3. **Python + PyInstaller.** Quick to write, but packaged `.exe`s are frequently flagged by
   antivirus — bad for strangers installing it. Ruled out.
4. **C# / .NET (self-contained).** Considered first; owner chose not to use .NET.

## Decided by the owner

- Alerts only on crossing the threshold, both directions.
- **No buffer / hysteresis:** if the price bounces around 400k, every crossing alerts.
  Each crossing is a real change in whether they'd buy.
- Threshold: 400,000 gold, must be changeable.
- Runs as a standalone app, not in-game; tray icon showing green/red.
- Windows only for now; keep it simple.
- Scheduled Task setup documented in the README.
- Client ID and secret configurable so other people use their own credentials.
- Not .NET.

## Design (builder's calls marked *call*)

### Blizzard API

- OAuth client-credentials: `POST https://oauth.battle.net/token` with
  `grant_type=client_credentials`, basic auth with client ID/secret. Cache the access
  token until shortly before `expires_in`; on 401, refresh once and retry.
- Price: `GET https://{region}.api.blizzard.com/data/wow/token/index?namespace=dynamic-{region}&locale=en_US`
  with `Authorization: Bearer <token>`. Response has `price` (copper) and
  `last_updated_timestamp` (ms since epoch).

### Settings (*call*)

- Non-secret settings in `%APPDATA%\WowTokenTracker\settings.json`:
  `region` (us/eu/kr/tw, default us), `clientId`, `thresholdGold` (default 400000),
  `pollMinutes` (default 20, minimum 5).
- `clientSecret` and `webhookUrl` stored in **Windows Credential Manager** (current user).
  A webhook URL is itself a secret — anyone with it can post to the channel.
- **Settings page:** tray menu **Settings…** opens a small page in the default browser,
  served by the app on `127.0.0.1` on a random port, guarded by a one-time random token
  in the URL. Fields for everything above, plus a **Test** button that fetches the price
  and sends a test Discord message. Opens automatically on first run when required
  fields are missing.

### State & alerting

- State = Green if `priceGold < thresholdGold`, else Red. No buffer.
- Grey is display-only (errors, unconfigured, stale) — never changes the stored alert state.
- Last alerted state persisted to `%APPDATA%\WowTokenTracker\state.json`, keyed by
  region + webhook, so restarts don't re-alert and a crossing that happened while the app
  was off is still reported on the next check.
- *Call:* first run (no stored state for this region + webhook) posts the current state
  once, so the user knows it's working. Changing region or webhook therefore also posts
  once.
- *Call:* changing the threshold re-evaluates immediately; if that flips the state, it alerts.
- *Call:* stale data — if `last_updated_timestamp` is more than 2 hours old, show grey with
  "data stale" in the tooltip. No alert.
- Discord message: embed with state, price (e.g. `395,120g`), threshold, region, and
  Blizzard's last-updated time. Green/red embed colour.
- Failed webhook post: log it, don't advance stored state, so the next poll retries.

### Tray

- Icons: green, red, grey `.ico` files embedded in the binary.
- Tooltip (keep ≤127 chars): `WoW Token (US): 395,120g · under 400,000g · checked 14:05`.
- Menu: **Check now**, **Settings…**, **Start with Windows** (toggle), **Open log folder**,
  **Exit**.
- *Call:* **Start with Windows** creates/removes the Scheduled Task for the user (via
  `schtasks /XML` with a per-user logon trigger — plain `/SC ONLOGON` needs admin), so a friend doesn't need the command line. The README still documents the
  manual steps.
- Single instance via a named mutex.

### Errors & logging

- Network/API errors: grey icon, tooltip shows the error, retry next poll. No Discord
  message for errors.
- Log file in `%LOCALAPPDATA%\WowTokenTracker\logs\`, size-capped. Never log secrets.

### Project layout (*call*)

```
go.mod
cmd/wowtokentracker/      # main: tray wiring, startup
internal/blizzard/        # OAuth + token price client
internal/discord/         # webhook notifier
internal/tracker/         # state machine (pure logic, heavily tested)
internal/config/          # settings.json + Credential Manager
internal/settingsui/      # localhost settings page (embedded HTML)
internal/autostart/       # Scheduled Task create/remove
assets/                   # icons
README.md
```

### Build & distribution

- Repo: https://github.com/StormChild/Wow-Token-Tracker (GPL-3.0, already in the repo).
  Go module path: `github.com/StormChild/Wow-Token-Tracker`.
- `go build -ldflags "-H windowsgui -s -w" -o WowTokenTracker.exe ./cmd/wowtokentracker`
  (`-H windowsgui` so no console window appears).
- Shared as a single `.exe` attached to a **GitHub Release**.
- *Call:* a GitHub Actions workflow builds the `.exe` and attaches it to a Release when a
  version tag (`v*`) is pushed, so the owner doesn't have to build by hand. A second
  workflow runs `go vet` + `go test` on pushes and PRs.

### README (owner requirement: complete, step by step)

Written for a non-developer friend. Someone following it top to bottom, with no other
help, must end with the tray icon running and starting at logon. Numbered steps, exact
button/field names, and what they should see after each step.

1. **What it does** — one paragraph plus what green/red/grey mean.
2. **Download** — link to the latest Release, where to put the `.exe`
   (suggest `%LOCALAPPDATA%\Programs\WowTokenTracker\`), and the SmartScreen
   "More info → Run anyway" step.
3. **Get Blizzard API credentials** — sign in at https://develop.battle.net, create a
   client (name, redirect URL can be `http://localhost`, intended use), copy client ID and
   secret. Note the secret is only shown once and must be kept private.
4. **Create a Discord webhook** — Server Settings → Integrations → Webhooks → New Webhook,
   pick the channel, Copy Webhook URL. Note it's a secret.
5. **First run & Settings** — run the `.exe`, the Settings page opens; fill in region,
   client ID, secret, webhook, threshold; press **Test**; expect a test message in
   Discord and the icon turning green/red.
6. **Start with Windows** — option A: tray menu → **Start with Windows**. Option B
   (manual): PowerShell `Register-ScheduledTask` lines to paste (no admin needed), and full Task
   Scheduler GUI steps (Create Task → General: "Run only when user is logged on" → Triggers:
   At log on → Actions: start the `.exe` → Conditions: untick "Start only if on AC power"
   → Settings: untick "Stop the task if it runs longer than…"). How to check it works
   (sign out and back in) and how to remove it.
7. **Using it** — tooltip, tray menu items, changing the threshold.
8. **Troubleshooting** — grey icon (check credentials / log folder), no Discord message,
   icon missing (hidden tray icons overflow), SmartScreen, where logs and settings live.
9. **Uninstall** — turn off Start with Windows / delete the task, delete the `.exe`,
   `%APPDATA%\WowTokenTracker`, `%LOCALAPPDATA%\WowTokenTracker`, and the Credential
   Manager entries.
10. **Building from source** — short section for developers.

## Left to the builder

- Tray and Credential Manager library choice (e.g. `fyne.io/systray`, `danieljoos/wincred`).
- Exact Discord message wording and embed layout.
- Icon artwork and settings page styling.
- Retry/backoff details within a poll.
