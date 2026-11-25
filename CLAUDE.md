# WoW Token Tracker

A Windows tray app that polls the Blizzard WoW Token API and posts to a Discord webhook
when the price crosses a gold threshold. Red/green "robot": **green = under threshold
(buy)**, **red = at/above**, **grey = unknown/error/stale**.

Goal: a friend can download one `.exe`, enter their own credentials, and be running in
minutes. Keep it simple — Windows only.

Full design: `docs/superpowers/specs/2026-11-25-wow-token-tracker-design.md`.

## Stack

- Go, built to a single Windows `.exe`. No runtime, no installer. **Not .NET.**
- Tray icon via a Go systray library; secrets via Windows Credential Manager.
- Settings UI is a local browser page served on `127.0.0.1` (random port + one-time token).
- No in-game addon — WoW addons have no network access.

## Layout

```
cmd/wowtokentracker/   # main: tray wiring, startup
internal/blizzard/     # OAuth + token price client
internal/discord/      # webhook notifier
internal/tracker/      # state machine — pure logic, no I/O
internal/config/       # settings.json + Credential Manager
internal/settingsui/   # localhost settings page (embedded HTML)
internal/autostart/    # Scheduled Task create/remove
assets/                # icons (embedded)
```

Keep decision logic in `internal/tracker` with no I/O so it's fully unit-testable.

## Commands

```
go build ./...
go test ./...
go vet ./...
go build -ldflags "-H windowsgui -s -w" -o WowTokenTracker.exe ./cmd/wowtokentracker
```

## Core rules

- **Alert only on crossing**, both directions. Discord gets a message only when state
  flips green↔red. Never on every poll, never on errors.
- **No buffer / hysteresis.** Owner decision: every crossing alerts, even if the price
  bounces around the threshold. Don't add one.
- **Threshold comparison:** green if `priceGold < thresholdGold`. API price is in copper
  (÷ 10,000).
- **Grey is display-only.** Errors, missing config, and stale data (`last_updated_timestamp`
  > 2h old) show grey; they never change the stored alert state.
- **State persisted** in `%APPDATA%\WowTokenTracker\state.json`, keyed by region + webhook.
  No stored state → post current state once (first run, or region/webhook changed).
- **Threshold change** re-evaluates immediately and alerts if the state flips.
- **Failed webhook post** → don't advance stored state; next poll retries.

## Config & secrets

- `%APPDATA%\WowTokenTracker\settings.json`: `region` (us/eu/kr/tw, default us), `clientId`,
  `thresholdGold` (default 400000), `pollMinutes` (default 20, min 5).
- `clientSecret` and `webhookUrl` live in Windows Credential Manager, never in the JSON.
  Treat the webhook URL as a secret.
- Never hard-code, log, or commit credentials or webhook URLs. No real secrets in tests.
- Settings page must bind to `127.0.0.1` only and require the per-launch token.

## Blizzard API

- Token: `POST https://oauth.battle.net/token` (client credentials, basic auth). Cache until
  near expiry; refresh once on 401.
- Price: `GET https://{region}.api.blizzard.com/data/wow/token/index?namespace=dynamic-{region}&locale=en_US`
  → `price` (copper), `last_updated_timestamp` (ms).
- Price updates ~every 20 min; never poll faster than 5 min.

## Testing

- Table-driven tests for `internal/tracker`: crossings both ways, first run, region/webhook
  change, threshold change, failed post, errors/stale → grey with state unchanged.
- Use `httptest.Server` for Blizzard and Discord clients; tests never hit real APIs.

## Windows specifics

- Build with `-H windowsgui` (no console window).
- Single instance via named mutex.
- Tooltip text ≤127 chars.
- Autostart = Scheduled Task with a per-user logon trigger, registered from XML (plain
  `schtasks /SC ONLOGON` needs admin rights and fails for normal users), "only when user is logged on" (otherwise no tray
  icon). Tray menu has a **Start with Windows** toggle; README documents manual PowerShell (`Register-ScheduledTask`)
  and Task Scheduler GUI steps too.
- Exe is unsigned — README must explain the SmartScreen "More info → Run anyway" step.

## Repo & releases

- GitHub: https://github.com/StormChild/Wow-Token-Tracker, licensed GPL-3.0.
- Go module path: `github.com/StormChild/Wow-Token-Tracker`.
- Users download the `.exe` from GitHub Releases. A GitHub Actions workflow builds and
  attaches it when a `v*` tag is pushed. A CI workflow runs vet and tests on pushes and PRs.

## README must get a non-developer fully running, step by step

Someone following it top to bottom with no other help must end with the tray icon
running and starting at logon. Use numbered steps, exact button and field names, and say
what they should see after each step. Sections, in order (full detail in the design doc):

1. What it does (green/red/grey).
2. Download from Releases, where to put the exe, and getting past SmartScreen.
3. Blizzard API client at develop.battle.net (client ID + secret).
4. Discord webhook (Server Settings → Integrations → Webhooks).
5. First run, the Settings page and the **Test** button.
6. Start with Windows: the tray toggle **and** full manual Scheduled Task steps (the
   PowerShell `Register-ScheduledTask` lines and Task Scheduler GUI), how to verify it, and how to remove it.
7. Using it.
8. Troubleshooting.
9. Uninstall: task, exe, the data folders and the Credential Manager entries.
10. Building from source.

Keep the README in sync with behaviour. If a change affects setup, update the README in
the same change.

## Out of scope

Mac/Linux, in-game addon, Discord bot commands, price history/charts, multi-region at
once, China region, installer/auto-update.
