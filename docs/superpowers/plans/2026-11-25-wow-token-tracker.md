# WoW Token Tracker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single Windows `.exe` tray app that polls the Blizzard WoW Token API, shows a green/red/grey icon, and posts to a Discord webhook whenever the price crosses a gold threshold.

**Architecture:** The pure decision logic lives in `internal/tracker` and does no I/O. Thin adapters wrap Blizzard, Discord, settings/secrets, the localhost settings page and the Scheduled Task. `internal/app` runs one check cycle against those adapters through interfaces, so the cycle can be tested without a network. `cmd/wowtokentracker` wires it all into a systray loop.

**Tech Stack:** Go (latest stable), `fyne.io/systray`, `github.com/danieljoos/wincred`, `golang.org/x/sys/windows`, stdlib `net/http`, `html/template`, `log/slog`, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-11-25-wow-token-tracker-design.md` (also read `CLAUDE.md`)

## Global Constraints

- Windows only. Not .NET. Single `.exe`, no installer.
- Module path: `github.com/StormChild/Wow-Token-Tracker`. Licence GPL-3.0 (`LICENSE` already in repo).
- Green iff `priceGold < thresholdGold`; `priceGold = priceCopper / 10000` (integer division).
- Discord is posted to only on a green↔red flip, or when there's no stored state for the current region + webhook. Never on errors, never on stale data.
- No buffer or hysteresis. Don't add one.
- Stale = Blizzard `last_updated_timestamp` more than **2 hours** before now → grey, no alert.
- Defaults: `region` `us` (allowed: `us`, `eu`, `kr`, `tw`), `thresholdGold` `400000`, `pollMinutes` `20` (minimum `5`).
- Paths: settings `%APPDATA%\WowTokenTracker\settings.json`, state `%APPDATA%\WowTokenTracker\state.json`, logs `%LOCALAPPDATA%\WowTokenTracker\logs\`.
- `clientSecret` and `webhookUrl` go only in Windows Credential Manager, never in JSON or logs. The state key is a hash, never the raw webhook.
- Tooltip ≤127 characters.
- Scheduled Task name `WowTokenTracker`, `/SC ONLOGON /RL LIMITED`, runs only when the user is logged on.
- Tests never hit real Blizzard or Discord endpoints.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Price exactly at the threshold** (400,000g = 4,000,000,000 copper) must be **red**, not green. Pinned in Task 1.
2. **Wrong client ID/secret** (OAuth 401) must show grey with a plain message ("Blizzard rejected the client ID or secret"), not a raw HTTP dump, and must never alert. Pinned in Tasks 3 and 5.
3. **Webhook deleted or rate limited** (Discord 404/429) must keep the stored state, so the next poll retries, and the tooltip shows the error. Pinned in Tasks 4 and 5.
4. **Saving Settings with the secret fields left blank** must keep the existing secret and webhook, not wipe them. The page never echoes secrets back. Pinned in Task 6.
5. **An exe path with spaces** (e.g. `C:\Users\Jo Smith\...`) must still produce a working Scheduled Task. If the exe was moved after enabling autostart, startup re-points the task. Pinned in Tasks 7 and 8.

---

### Task 0: Prerequisites

- [ ] **Step 1:** Install Go: `winget install --id GoLang.Go -e`, then open a new shell.
  Verify: `go version` prints `go1.2x` or newer.
- [ ] **Step 2:** `git push -u origin main` is **not** done here. Ask the owner before the first push.

### Task 1: Scaffold + tracker decision logic

**Files:**
- Create: `go.mod`, `.gitignore` (`*.exe`, `/dist/`), `internal/tracker/tracker.go`, `internal/tracker/format.go`
- Test: `internal/tracker/tracker_test.go`, `internal/tracker/format_test.go`

**Interfaces — Produces:**
```go
type Light int
const ( Unknown Light = iota; Green; Red )
func (l Light) String() string // "unknown" | "green" | "red"

type Observation struct { PriceCopper int64; UpdatedAt time.Time }

const StaleAfter = 2 * time.Hour

func Classify(priceCopper, thresholdGold int64) Light
func StateKey(region, webhookURL string) string // hex sha256 of region + "\n" + webhookURL

type Decision struct {
    Display Light // what the icon shows; Unknown when stale
    Stale   bool
    Notify  bool  // true → post an alert for Current, then persist Current
    Current Light // Classify result (Green/Red), even when stale
}
func Decide(obs Observation, thresholdGold int64, stored *Light, now time.Time) Decision

func FormatGold(gold int64) string // 395120 → "395,120g"
```

- [ ] **Step 1: Init the module.** `go mod init github.com/StormChild/Wow-Token-Tracker`, and add `.gitignore`.
- [ ] **Step 2: Write the failing tests** (table-driven `TestClassify`, `TestDecide`, `TestStateKey`, `TestFormatGold`):
  - `Classify(3_999_999_999, 400000)` → `Green` (399,999g 99s 99c). `Classify(4_000_000_000, 400000)` → `Red` (exactly 400,000g). `Classify(4_000_009_999, 400000)` → `Red` (rounds down to 400,000g).
  - `Decide`, with `now` fixed and `UpdatedAt = now-10m`:
    - stored nil, price green → `Notify:true, Current:Green, Display:Green`
    - stored Green, price green → `Notify:false`
    - stored Green, price red → `Notify:true, Current:Red`
    - stored Red, price green → `Notify:true, Current:Green`
    - `UpdatedAt = now-2h-1s`, stored Green, price red → `Stale:true, Display:Unknown, Notify:false`
    - `UpdatedAt = now-2h` exactly → not stale
  - `StateKey("us","https://x")` ≠ `StateKey("eu","https://x")` ≠ `StateKey("us","https://y")`, is 64 hex characters, and doesn't contain `"https"`.
  - `FormatGold`: `0`→`"0g"`, `999`→`"999g"`, `1000`→`"1,000g"`, `395120`→`"395,120g"`, `1234567`→`"1,234,567g"`.
- [ ] **Step 3: Run** `go test ./internal/tracker/`. Expected: FAIL (undefined).
- [ ] **Step 4: Implement** the signatures above.
- [ ] **Step 5: Run** `go test ./internal/tracker/ && go vet ./...`. Expected: PASS.
- [ ] **Step 6: Commit** `feat(tracker): threshold classification and alert decision`.

### Task 2: Config, secrets and state store

**Files:**
- Create: `internal/config/settings.go`, `internal/config/secrets.go`, `internal/config/secrets_windows.go`, `internal/config/state.go`
- Test: `internal/config/settings_test.go`, `internal/config/state_test.go`

**Interfaces:**
- Consumes: `tracker.Light`
- Produces:
```go
type Settings struct {
    Region        string `json:"region"`
    ClientID      string `json:"clientId"`
    ThresholdGold int64  `json:"thresholdGold"`
    PollMinutes   int    `json:"pollMinutes"`
    ClientSecret  string `json:"-"`
    WebhookURL    string `json:"-"`
}
func Defaults() Settings
func (s Settings) Validate() error // region in set; threshold > 0; poll >= 5; webhook (if set) starts with https://discord.com/api/webhooks/ or https://discordapp.com/api/webhooks/
func (s Settings) Complete() bool  // ClientID, ClientSecret, WebhookURL all non-empty and Validate()==nil

type SecretStore interface {
    Get(name string) (string, error) // missing → "", nil
    Set(name, value string) error
    Delete(name string) error
}
const SecretClientSecret = "WowTokenTracker/clientSecret"
const SecretWebhookURL   = "WowTokenTracker/webhookUrl"
func WinCred() SecretStore           // secrets_windows.go, danieljoos/wincred generic credentials
type MemSecrets map[string]string    // test double, implements SecretStore

type Store struct { Dir string; Secrets SecretStore }
func DataDir() string // %APPDATA%\WowTokenTracker
func LogDir() string  // %LOCALAPPDATA%\WowTokenTracker\logs
func (st Store) Load() (Settings, error) // missing file → Defaults() + secrets; pollMinutes < 5 clamped to 5; unknown region → "us"
func (st Store) Save(s Settings) error   // Validate first; JSON (indented) + secrets; creates Dir

func LoadState(path string) (map[string]tracker.Light, error) // missing file → empty map
func SaveState(path string, m map[string]tracker.Light) error // write temp file then rename
```

- [ ] **Step 1: Write the failing tests** (use `t.TempDir()` and `MemSecrets`):
  - `TestLoadMissingReturnsDefaults`: region `us`, threshold `400000`, poll `20`.
  - `TestSaveLoadRoundTrip`: all six fields survive. The written `settings.json` doesn't contain the secret or webhook strings.
  - `TestLoadClampsPoll`: a file with `pollMinutes: 1` loads as `5`.
  - `TestValidate`: rejects region `cn`, threshold `0`, poll `4`, and webhook `https://evil.example/`. Accepts `Defaults()` with a valid webhook.
  - `TestComplete`: false while any of the three credentials is empty.
  - `TestStateRoundTrip` and `TestStateMissingFile`.
- [ ] **Step 2: Run** `go test ./internal/config/`. Expected: FAIL.
- [ ] **Step 3: Implement** the code. `WinCred` uses `wincred.NewGenericCredential(name)` with `CredentialBlob = []byte(value)` and `Persist = wincred.PersistLocalMachine`. Treat `wincred.GetGenericCredential` returning an `ERROR_NOT_FOUND` error as `"", nil`.
- [ ] **Step 4: Run** `go test ./internal/config/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(config): settings, Credential Manager secrets, state file`.

### Task 3: Blizzard client

**Files:**
- Create: `internal/blizzard/client.go`
- Test: `internal/blizzard/client_test.go`

**Interfaces:**
- Consumes: `tracker.Observation`
- Produces:
```go
var ErrBadCredentials = errors.New("Blizzard rejected the client ID or secret")
type Client struct {
    HTTP         *http.Client
    OAuthURL     string // default "https://oauth.battle.net/token"
    APIBaseFmt   string // default "https://%s.api.blizzard.com"
    ClientID     string
    ClientSecret string
    // unexported: cached token, expiry, mutex
}
func New(clientID, clientSecret string) *Client // 15s timeout
func (c *Client) TokenPrice(ctx context.Context, region string) (tracker.Observation, error)
```

- [ ] **Step 1: Write the failing tests** with one `httptest.Server` serving both `/token` and `/data/wow/token/index`. Point `OAuthURL` and `APIBaseFmt` at it (`APIBaseFmt = srv.URL + "/%.0s"` or similar to swallow the region):
  - `TestTokenPrice`: OAuth gets basic auth `id:secret` and form `grant_type=client_credentials`. The price request has `Authorization: Bearer abc`, `namespace=dynamic-us` and `locale=en_US`. A response `{"price":3951200000,"last_updated_timestamp":1764072000000}` → `PriceCopper 3951200000`, `UpdatedAt` = `time.UnixMilli(1764072000000)`.
  - `TestTokenCached`: two `TokenPrice` calls → one OAuth hit, given `expires_in: 86399`.
  - `TestRefreshOn401`: price endpoint returns 401 once, then 200 → two OAuth hits, success.
  - `TestBadCredentials`: OAuth returns 401 → `errors.Is(err, ErrBadCredentials)`.
  - `TestServerError`: price returns 503 → an error whose message contains `503` and doesn't contain the secret.
- [ ] **Step 2: Run** `go test ./internal/blizzard/`. Expected: FAIL.
- [ ] **Step 3: Implement** the client. Refresh the token when fewer than 60 seconds remain.
- [ ] **Step 4: Run** `go test ./internal/blizzard/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(blizzard): OAuth + WoW Token price client`.

### Task 4: Discord notifier

**Files:**
- Create: `internal/discord/discord.go`
- Test: `internal/discord/discord_test.go`

**Interfaces:**
- Consumes: `tracker.Light`, `tracker.FormatGold`
- Produces:
```go
type Alert struct {
    Light         tracker.Light
    PriceGold     int64
    ThresholdGold int64
    Region        string
    UpdatedAt     time.Time
    Test          bool // "Test message from WoW Token Tracker" prefix
}
func Send(ctx context.Context, httpc *http.Client, webhookURL string, a Alert) error // non-2xx → error with status code; 429 error mentions "rate limited"
```

- [ ] **Step 1: Write the failing tests** against an `httptest.Server`:
  - `TestSendGreen`: POST JSON with one embed whose title contains `Under 400,000g — buy`. The description contains `395,120g` and `US`. Colour `0x2ECC71`. Uses the `?wait=true` query.
  - `TestSendRed`: the title contains `Back above 400,000g`. Colour `0xE74C3C`.
  - `TestSendTest`: when `Test:true`, the content contains `Test message`.
  - `TestSend404` and `TestSend429`: return errors, and the 429 error mentions `rate limited`.
- [ ] **Step 2: Run** `go test ./internal/discord/`. Expected: FAIL.
- [ ] **Step 3: Implement** `Send`. Embed `timestamp` = `UpdatedAt` in RFC 3339. Footer `WoW Token Tracker`.
- [ ] **Step 4: Run** `go test ./internal/discord/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(discord): webhook alerts`.

### Task 5: App check cycle + tooltip

**Files:**
- Create: `internal/app/app.go`, `internal/app/tooltip.go`
- Test: `internal/app/app_test.go`, `internal/app/tooltip_test.go`

**Interfaces:**
- Consumes: Tasks 1–4.
- Produces:
```go
type PriceSource interface { TokenPrice(ctx context.Context, region string) (tracker.Observation, error) }
type Notifier func(ctx context.Context, webhookURL string, a discord.Alert) error

type Status struct {
    Display   tracker.Light
    PriceGold int64     // 0 if never fetched
    Checked   time.Time
    Stale     bool
    Err       error     // fetch or notify error, nil on success
}

type App struct {
    Settings  func() config.Settings
    Prices    PriceSource
    Notify    Notifier
    StatePath string
    Now       func() time.Time
    // unexported: mutex, last Status
}
func (a *App) Check(ctx context.Context) Status // one cycle; safe to call concurrently (serialised)
func (a *App) Last() Status

func Tooltip(region string, thresholdGold int64, s Status) string
```
`Check` algorithm: if `!Settings().Complete()`, return grey with `Err = "not configured — open Settings"`. Otherwise fetch, then `Decide` with the stored state for `StateKey(region, webhook)`. If `Notify`, send. Only on send success, store `Current` and `SaveState`. On send failure, return `Display` as normal and set `Err`.

- [ ] **Step 1: Write the failing tests** (fake `PriceSource`, recording `Notifier`, `t.TempDir()` state path, fixed `Now`):
  - `TestFirstRunNotifiesOnce`: two checks with the same green price → one notification.
  - `TestCrossingBothWays`: green, red, green → three notifications (first run, flip, flip).
  - `TestFetchErrorNoAlertStateKept`: stored Green, the fetch returns `blizzard.ErrBadCredentials` → `Display Unknown`, no notify, state file unchanged, and `Tooltip` contains `Blizzard rejected`.
  - `TestNotifyFailureRetries`: a red flip whose notifier fails → state stays Green. On the next check (notifier OK) it notifies Red.
  - `TestStaleNoAlert`: `UpdatedAt = now-3h` → `Display Unknown`, `Stale true`, no notify.
  - `TestThresholdChangeFlips`: price 395,000g, stored Green; change the threshold to 390,000 → notify Red.
  - `TestWebhookChangeIsFirstRun`: stored Green under webhook A; switch to webhook B → one notify.
  - `TestNotConfigured`: an empty ClientID → Unknown, no fetch call.
  - `TestTooltip`: green → `WoW Token (US): 395,120g · under 400,000g · checked 14:05`. Red uses `at/above`. Stale includes `data stale`. Errors include the error text. Every case is ≤127 characters, including a 300-character error, which gets truncated with `…`.
- [ ] **Step 2: Run** `go test ./internal/app/`. Expected: FAIL.
- [ ] **Step 3: Implement** the code above.
- [ ] **Step 4: Run** `go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(app): check cycle and tooltip`.

### Task 6: Settings page

**Files:**
- Create: `internal/settingsui/server.go`, `internal/settingsui/page.html` (via `//go:embed`)
- Test: `internal/settingsui/server_test.go`

**Interfaces:**
- Consumes: `config.Settings`
- Produces:
```go
type Callbacks struct {
    Load func() config.Settings
    Save func(config.Settings) error                     // validated by caller's Store.Save
    Test func(ctx context.Context, s config.Settings) error // fetch price + send Test alert
}
type Server struct { /* listener, token */ }
func Start(cb Callbacks) (*Server, error) // listens on 127.0.0.1:0, crypto/rand 32-byte hex token
func (s *Server) URL() string             // "http://127.0.0.1:<port>/?t=<token>"
func (s *Server) Close() error
func NewHandler(token string, cb Callbacks) http.Handler // used by Start and tests
```
Routes: `GET /` renders the form. `POST /save` and `POST /test` take form posts. Every request needs `t=<token>` (query or hidden field) → otherwise `403`. The `Host` header must be `127.0.0.1:<port>` or `localhost:<port>` → otherwise `403`. Secret inputs render empty with the placeholder `saved — leave blank to keep`. A blank secret on submit means keep the current value. Fields: region select, client ID, client secret, webhook URL, threshold (gold), poll minutes. After a save, show `Saved ✓` or the validation error inline. After a test, show `Test sent — check Discord (price 395,120g)` or the error.

- [ ] **Step 1: Write the failing tests** with `httptest.NewRecorder` on `NewHandler("tok", cb)`:
  - `TestRejectsMissingToken` → 403. `TestRejectsForeignHost` (`Host: evil.com`) → 403.
  - `TestGetNeverEchoesSecrets`: Load returns secret `S3CRET` and webhook `https://discord.com/api/webhooks/1/abc` → the body contains neither.
  - `TestSaveBlankSecretsKeepsExisting`: posting a blank secret and webhook with a new threshold → Save receives the old secret and webhook plus the new threshold.
  - `TestSaveShowsValidationError`: Save returns an error → the body contains its message, status 200.
  - `TestTestButton`: the Test callback is called with the merged settings.
- [ ] **Step 2: Run** `go test ./internal/settingsui/`. Expected: FAIL.
- [ ] **Step 3: Implement** with `html/template`. Plain, readable single-page styling. No JS framework.
- [ ] **Step 4: Run** `go test ./internal/settingsui/`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(settingsui): localhost settings page`.

### Task 7: Autostart (Scheduled Task)

**Files:**
- Create: `internal/autostart/autostart.go`
- Test: `internal/autostart/autostart_test.go`

**Interfaces — Produces:**
```go
const TaskName = "WowTokenTracker"
type Runner func(name string, args ...string) (output []byte, err error) // default: exec.Command(...).CombinedOutput with HideWindow
type Autostart struct { Run Runner }
func Default() Autostart
func CreateArgs(exePath string) []string // {"/Create","/TN","WowTokenTracker","/TR",`"<exe>"`,"/SC","ONLOGON","/RL","LIMITED","/F"}
func (a Autostart) Enable(exePath string) error
func (a Autostart) Disable() error            // /Delete /TN WowTokenTracker /F
func (a Autostart) Enabled() (bool, error)    // /Query /TN WowTokenTracker; non-zero exit → false, nil
func (a Autostart) TaskPath() (string, error) // /Query /TN WowTokenTracker /XML, parse <Command>
```

- [ ] **Step 1: Write the failing tests** with a recording `Runner`:
  - `TestCreateArgsQuotesPathWithSpaces`: `C:\Users\Jo Smith\AppData\Local\Programs\WowTokenTracker\WowTokenTracker.exe` → the `/TR` value is that path wrapped in `"`.
  - `TestEnableCallsSchtasks`: the runner gets `schtasks` + `CreateArgs`.
  - `TestEnabledFalseOnError`.
  - `TestTaskPathParsesXML`: sample XML (UTF-16 output decoded, or the test feeds UTF-8) with `<Command>"C:\a b\x.exe"</Command>` → `C:\a b\x.exe`.
- [ ] **Step 2: Run** `go test ./internal/autostart/`. Expected: FAIL.
- [ ] **Step 3: Implement** the code. `schtasks /XML` output is UTF-16LE with a BOM, so decode it with `golang.org/x/text/encoding/unicode` when a BOM is present.
- [ ] **Step 4: Run** `go test ./internal/autostart/`. Expected: PASS.
- [ ] **Step 5: Manual check** on Windows: build a throwaway exe, call Enable, and confirm `schtasks /Query /TN WowTokenTracker` lists it. Then Disable.
- [ ] **Step 6: Commit** `feat(autostart): Scheduled Task enable/disable`.

### Task 8: Tray app, icons, logging, single instance

**Files:**
- Create: `assets/icons.go`, `cmd/wowtokentracker/main.go`, `cmd/wowtokentracker/tray.go`, `cmd/wowtokentracker/logging.go`, `cmd/wowtokentracker/instance_windows.go`
- Test: `assets/icons_test.go`, `cmd/wowtokentracker/logging_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces:
```go
// assets
func Icon(l tracker.Light) []byte // ICO bytes: 32×32 PNG-in-ICO filled circle; Green #2ECC71, Red #E74C3C, Unknown #95A5A6
// cmd
func openLog(dir string) (*os.File, error) // logs\tracker.log; if > 1 MiB at open, rename to tracker.log.1 (replacing), then create fresh
func acquireSingleInstance() (release func(), ok bool) // CreateMutex "Local\\WowTokenTracker"; ERROR_ALREADY_EXISTS → ok=false
```
`main` flow: `acquireSingleInstance` (if already running, exit 0) → open the log (`slog` text handler) → `config.Store{Dir: DataDir(), Secrets: WinCred()}` → `systray.Run(onReady, onExit)`. `onReady` does the following:
- Sets the grey icon and builds the menu: **Check now**, **Settings…**, **Start with Windows** (checkbox, from `Enabled()`), **Open log folder** (`explorer LogDir()`), **Exit**.
- If autostart is enabled and `TaskPath()` ≠ `os.Executable()`, it calls `Enable(current exe)`, logging any error.
- If `!Complete()`, it opens Settings.
- It starts a goroutine: `Check` now, then every `PollMinutes`. After each `Check`, it sets the icon and tooltip. Saving settings triggers an immediate `Check`, and the next `Check` reads the new `PollMinutes`.
- **Settings…** lazily starts `settingsui` once and opens the URL with `rundll32 url.dll,FileProtocolHandler <url>`.
- The settings Test callback uses a fresh `blizzard.New` from the submitted settings plus `discord.Send` with `Test:true`.

- [ ] **Step 1: Write the failing tests:**
  - `TestIconsAreICO`: for each Light, bytes 0–3 are `00 00 01 00`, the image count is 1, and the embedded data starts with the PNG signature. All three icons differ.
  - `TestOpenLogRotates`: a pre-seeded 2 MiB `tracker.log` → after `openLog`, `tracker.log.1` exists and `tracker.log` is empty.
- [ ] **Step 2: Run** `go test ./assets/ ./cmd/...`. Expected: FAIL.
- [ ] **Step 3: Implement** the code above. `go get fyne.io/systray golang.org/x/sys/windows`.
- [ ] **Step 4: Run** `go test ./... && go vet ./...`. Expected: PASS.
- [ ] **Step 5: Build** `go build -ldflags "-H windowsgui -s -w" -o WowTokenTracker.exe ./cmd/wowtokentracker`. Expected: the exe exists and is under 15 MB.
- [ ] **Step 6: Manual check** (the owner's real credentials, entered by the owner):
  - Launch → grey icon, Settings opens in the browser.
  - Fill in the fields, press Test → a Discord test message arrives.
  - Save → the icon turns green or red within seconds, and Discord gets one state message.
  - Launch the exe again → no second icon.
  - Toggle **Start with Windows** → `schtasks /Query /TN WowTokenTracker` lists it. Sign out and back in → the icon appears.
  - Exit → the icon disappears.
- [ ] **Step 7: Commit** `feat: tray app wiring, icons, logging, single instance`.

### Task 9: GitHub Actions — CI and releases

**Files:**
- Create: `.github/workflows/ci.yml`, `.github/workflows/release.yml`

- [ ] **Step 1: `ci.yml`:** on push and pull_request, `windows-latest`, `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, then `go vet ./...` and `go test ./...`.
- [ ] **Step 2: `release.yml`:** on push of tags `v*`, `permissions: contents: write`, `windows-latest`. Run the same setup and tests, then `go build -ldflags "-H windowsgui -s -w -X main.version=${{ github.ref_name }}" -o WowTokenTracker.exe ./cmd/wowtokentracker`, then `gh release create ${{ github.ref_name }} WowTokenTracker.exe --generate-notes` with `GH_TOKEN: ${{ github.token }}`. Add `var version = "dev"` in `main.go` and log it at startup.
- [ ] **Step 3: Validate** that the YAML parses (`go run` isn't needed. Check it by eye or with `actionlint` if available). The real verification happens after the owner approves pushing.
- [ ] **Step 4: Commit** `ci: test on push, build exe on version tags`.

### Task 10: README — complete step-by-step setup

**Files:**
- Create: `README.md`

Audience: a non-developer friend. Someone following it top to bottom with no other help ends with the tray icon running and starting at logon. Use numbered steps, exact button and field names, and an "You should see…" line after key steps. Release download link: `https://github.com/StormChild/Wow-Token-Tracker/releases/latest`.

- [ ] **Step 1: Write the sections, in this order** (the content requirements are spec § README, items 1–10):
  1. What it does, with a green/red/grey table.
  2. Download. Put the exe in `%LOCALAPPDATA%\Programs\WowTokenTracker\` (say how to create the folder via File Explorer's address bar). For SmartScreen: **More info → Run anyway**.
  3. Blizzard credentials: develop.battle.net → sign in → API Access → **Create Client**. Client name: anything. Redirect URL: `http://localhost`. Service URL: leave blank. Intended use: "Personal WoW Token price alerts". Copy the Client ID and Client Secret. Keep the secret private.
  4. Discord webhook: Server Settings → Integrations → Webhooks → **New Webhook**, choose the channel → **Copy Webhook URL**. They need the Manage Webhooks permission. Treat the URL like a password.
  5. First run: double-click the exe → the browser opens Settings → fill in each field → **Test** → you should see a test message in Discord → **Save** → you should see the tray icon turn green or red (with a pointer to the `^` hidden-icons arrow and how to pin the icon).
  6. Start with Windows:
     - **Option A:** right-click the tray icon → **Start with Windows**.
     - **Option B, Command Prompt:** the exact `schtasks /Create /TN "WowTokenTracker" /TR "\"%LOCALAPPDATA%\Programs\WowTokenTracker\WowTokenTracker.exe\"" /SC ONLOGON /RL LIMITED /F` command.
     - **Option C, Task Scheduler GUI:** Start → Task Scheduler → **Create Task…** (not "Basic").
       - General: name `WowTokenTracker`, **Run only when user is logged on**, leave **Run with highest privileges** unticked.
       - Triggers: **New… → At log on**, your user.
       - Actions: **New… → Start a program**, browse to the exe.
       - Conditions: untick **Start the task only if the computer is on AC power**.
       - Settings: untick **Stop the task if it runs longer than**.
       - OK.
     - Verify: sign out and back in, and the icon appears. Remove: untick the tray option, or `schtasks /Delete /TN WowTokenTracker /F`, or delete it in Task Scheduler.
  7. Using it: what the tooltip shows, the menu items, how to change the threshold.
  8. Troubleshooting:
     - Grey icon with "Blizzard rejected…" → re-copy the secret.
     - Grey with "data stale" → Blizzard hasn't updated, so wait.
     - No Discord message → re-copy the webhook and press Test.
     - Icon missing → hidden-icons arrow.
     - The task doesn't start at logon → check Option C's settings.
     - Where the logs and settings live.
  9. Uninstall: disable autostart, Exit, delete the exe and the folder, delete `%APPDATA%\WowTokenTracker` and `%LOCALAPPDATA%\WowTokenTracker`. In Credential Manager → Windows Credentials, remove the two `WowTokenTracker/…` entries.
  10. Building from source: install Go, `go test ./...`, the build command, and how to tag a release (`git tag v1.0.0 && git push origin v1.0.0`).
- [ ] **Step 2: Walk-through check.** Follow the README literally on this machine from step 2 onward, using the Task 8 exe. Fix any step that doesn't match what's on screen.
- [ ] **Step 3: Commit** `docs: step-by-step README`.
