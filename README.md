# WoW Token Tracker

[![Build](https://github.com/StormChild/Wow-Token-Tracker/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/StormChild/Wow-Token-Tracker/actions/workflows/ci.yml)
[![Release build](https://github.com/StormChild/Wow-Token-Tracker/actions/workflows/release.yml/badge.svg)](https://github.com/StormChild/Wow-Token-Tracker/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/StormChild/Wow-Token-Tracker?display_name=tag&sort=semver)](https://github.com/StormChild/Wow-Token-Tracker/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/StormChild/Wow-Token-Tracker/total)](https://github.com/StormChild/Wow-Token-Tracker/releases)
[![License: GPL-3.0](https://img.shields.io/github/license/StormChild/Wow-Token-Tracker)](LICENSE)

A small Windows tray app that watches the **World of Warcraft Token** price and tells you
in Discord when it's cheap enough to buy with gold.

> **Status: in development.** There's no download yet. These instructions describe how
> the app will work once the first release is published.

---

## 1. What it does

The app sits in your taskbar's system tray as a coloured dot. Every 20 minutes it checks
the official Blizzard token price and compares it with your **threshold** (400,000 gold
by default).

| Icon | Meaning |
|------|---------|
| 🟢 Green | The price is **under** your threshold. Time to buy. |
| 🔴 Red | The price is **at or above** your threshold. |
| ⚪ Grey | Unknown: starting up, not set up yet, a problem reaching Blizzard, or Blizzard's data is out of date. |

It posts a message to your Discord channel **only when the colour flips** between green
and red. You won't get a message every 20 minutes, and you won't get messages about errors.

Hover over the icon to see the latest price, for example:
`WoW Token (US): 395,120g · under 400,000g · checked 14:05`.

You'll need:
- A Windows 10 or 11 PC.
- A free Battle.net developer account. This is your normal Battle.net login.
- A Discord server where you're allowed to create webhooks.

Setup takes about 10 minutes.

---

## 2. Download and put it somewhere permanent

1. Go to the **[latest release](https://github.com/StormChild/Wow-Token-Tracker/releases/latest)**
   and download **`WowTokenTracker.exe`**.
2. Make a permanent home for it, so it doesn't stay in your Downloads folder:
   1. Open **File Explorer**.
   2. Click in the address bar at the top, type `%LOCALAPPDATA%\Programs` and press **Enter**.
      If Windows says it can't find it, type `%LOCALAPPDATA%` instead, press **Enter**,
      and create a folder called `Programs` there.
   3. Right-click an empty area, then choose **New → Folder**. Name it `WowTokenTracker`.
   4. Move `WowTokenTracker.exe` from your Downloads folder into this new folder.

   You should end up with
   `C:\Users\<you>\AppData\Local\Programs\WowTokenTracker\WowTokenTracker.exe`.

3. **About the Windows warning:** the app isn't digitally signed, so the first time you
   run it, Windows may show **"Windows protected your PC"**. Click **More info**, then
   **Run anyway**. You only have to do this once.

   If Windows shows nothing and simply doesn't run it, right-click the `.exe`, choose
   **Properties**, tick **Unblock** at the bottom of the **General** tab, and click **OK**.

---

## 3. Get your Blizzard API credentials

The app uses your own free Blizzard API key to read the token price.

1. Go to **https://develop.battle.net** and click **Log In**. Use your normal Battle.net
   account.
2. Accept the API terms if you're asked to.
3. Click **API Access** (or **Clients**), then **Create Client**.
4. Fill in the form:
   - **Client Name:** anything you like, e.g. `WoW Token Tracker`.
   - **Redirect URLs:** `http://localhost`
   - **Service URL:** tick **"I do not have a service URL for this client"** if that
     option is shown. Otherwise leave it blank.
   - **Intended Use:** `Personal WoW Token price alerts`
5. Click **Save** or **Create**.
6. You'll now see a **Client ID** and a **Client Secret**. Copy both into a temporary
   note. You'll paste them into the app in step 5.

> 🔒 **Keep the Client Secret private.** Don't post it in Discord or share screenshots
> of it. If it leaks, open the client on develop.battle.net and generate a new secret.

---

## 4. Create a Discord webhook

A webhook is a private link that lets the app post into one Discord channel.

1. In Discord, open your server and click the server name, then **Server Settings**.
   You need the **Manage Webhooks** permission. Server owners and admins have it.
2. Go to **Integrations**, then **Webhooks**, then **New Webhook**.
3. Click the new webhook to open it:
   - Give it a name, e.g. `Token Robot`.
   - Under **Channel**, pick the channel the alerts should go to.
4. Click **Copy Webhook URL**. It looks like
   `https://discord.com/api/webhooks/123456789/AbCdEf...`
5. Click **Save Changes**.

> 🔒 **Treat the webhook URL like a password.** Anyone who has it can post in that channel.

---

## 5. First run and settings

1. Double-click `WowTokenTracker.exe`. If you see the Windows warning, follow step 2.3.
2. A **grey dot** appears in the system tray, and the **Settings** page opens in your web
   browser. The page runs on your own computer only, and nobody else can reach it.
3. Fill in the page:

   | Field | What to enter |
   |-------|---------------|
   | **Region** | The region you play on: US, EU, KR or TW. |
   | **Client ID** | From step 3. |
   | **Client Secret** | From step 3. |
   | **Discord Webhook URL** | From step 4. |
   | **Alert threshold (gold)** | The price you want to buy under. The default is `400000`. |
   | **Check every (minutes)** | Leave it at `20`, because Blizzard only updates the price about that often. The minimum is 5. |

4. Click **Test**.
   ✅ **You should see** a "Test message from WoW Token Tracker" in your Discord channel,
   and the page shows the current price.
   ❌ If you see an error instead, check the troubleshooting section (8).
5. Click **Save**.
   ✅ **You should see** the tray dot turn **green** or **red** within a few seconds, and
   one message in Discord saying whether the price is under or above your threshold. That
   first message confirms everything works. After that, it only posts when the colour flips.
6. You can close the browser tab now. The app keeps running in the tray.

**Can't see the dot?** Windows may hide tray icons. Click the small **^** arrow next to the
clock to find it. To keep it always visible, open **Settings → Personalization → Taskbar
→ Other system tray icons** and switch **WowTokenTracker** on.

---

## 6. Start it automatically with Windows

Choose **one** of these options. Option A is the easiest.

### Option A: from the tray menu (easiest)

1. Right-click the tray dot.
2. Click **Start with Windows** so it shows a tick.

That's it. To turn it off, click it again.

### Option B: paste a few lines into PowerShell

1. Press **Start**, type `PowerShell` and open **Windows PowerShell**. You don't need
   "Run as administrator".
2. Paste all of these lines and press **Enter**:

   ```powershell
   $exe       = "$env:LOCALAPPDATA\Programs\WowTokenTracker\WowTokenTracker.exe"
   $action    = New-ScheduledTaskAction -Execute $exe
   $trigger   = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
   $settings  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit 0
   $principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited
   Register-ScheduledTask -TaskName "WowTokenTracker" -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force
   ```

   ✅ **You should see** a short table with `WowTokenTracker` and the state `Ready`.

If you put the `.exe` somewhere other than the folder from step 2, change the path on the
first line to match.

> Why not a one-line `schtasks /SC ONLOGON` command? Without administrator rights, Windows
> refuses it with "Access is denied". The lines above work for a normal user.

### Option C: Task Scheduler, step by step

1. Press **Start**, type `Task Scheduler` and open it.
2. In the right-hand panel, click **Create Task…**. Use **Create Task**, not "Create Basic Task".
3. **General** tab:
   - **Name:** `WowTokenTracker`
   - Select **Run only when user is logged on**. This is important: with the other option,
     the tray icon will never appear.
   - Leave **Run with highest privileges** unticked.
4. **Triggers** tab, then **New…**:
   - **Begin the task:** `At log on`
   - **Settings:** `Specific user`. Your account is already selected.
   - Make sure **Enabled** is ticked, then click **OK**.
5. **Actions** tab, then **New…**:
   - **Action:** `Start a program`
   - **Program/script:** click **Browse…** and select `WowTokenTracker.exe` from step 2.
   - Click **OK**.
6. **Conditions** tab:
   - Untick **Start the task only if the computer is on AC power**, so it also runs on a
     laptop battery.
7. **Settings** tab:
   - Untick **Stop the task if it runs longer than:**. Otherwise Windows will close the
     app after 3 days.
8. Click **OK**.

### Check that it works

1. Sign out of Windows: **Start → your profile picture → Sign out**.
2. Sign back in.
3. ✅ **You should see** the tray dot appear within a few seconds of the desktop loading.

### Removing the automatic start

Use whichever matches how you set it up:
- **Option A:** right-click the tray dot and untick **Start with Windows**.
- **Option B:** in PowerShell, run `Unregister-ScheduledTask -TaskName "WowTokenTracker" -Confirm:$false`.
- **Option C:** in Task Scheduler, click **Task Scheduler Library**, right-click
  **WowTokenTracker**, then choose **Delete**.

---

## 7. Using it

Right-click the tray dot for:

| Menu item | What it does |
|-----------|--------------|
| **Check now** | Checks the price immediately instead of waiting. |
| **Settings…** | Opens the Settings page, e.g. to change your threshold. |
| **Start with Windows** | Turns the automatic start on or off. |
| **Open log folder** | Opens the folder with the app's log file, which is useful if something goes wrong. |
| **Exit** | Closes the app. |

**Changing the threshold:** open **Settings…**, change **Alert threshold (gold)** and click
**Save**. The app re-checks right away. If the new threshold flips the colour, you'll get a
Discord message.

**Secret fields:** the Settings page never shows your saved Client Secret or webhook. Leave
those boxes blank to keep what's saved, or type a new value to replace it.

---

## 8. Troubleshooting

| Problem | What to do |
|---------|------------|
| Grey dot, and the tooltip says **"Blizzard rejected the client ID or secret"** | Open **Settings…** and paste the Client ID and Client Secret again from develop.battle.net. Watch for extra spaces. |
| Grey dot, and the tooltip says **"data stale"** | Blizzard hasn't updated the price for over 2 hours. It usually fixes itself. Just wait. |
| Grey dot, and the tooltip says **"not configured"** | Open **Settings…** and fill in every field. |
| Grey dot with a network error | Check your internet connection. The app retries at the next check. |
| **Test** works, but no alerts come later | That's normal while the price stays on the same side of your threshold. Messages only come when it crosses. |
| No Discord message at all, even on **Test** | The webhook may have been deleted. Create a new one (step 4), paste it into Settings and click **Test**. |
| Can't find the tray dot | Click the **^** arrow by the clock. See the tip at the end of step 5. |
| A second copy doesn't open | That's on purpose. Only one copy runs at a time, so look for the dot that's already there. |
| It doesn't start when I log in | If you used Option C, check that **Run only when user is logged on** is selected and both boxes in steps 6–7 are unticked. Also check that the `.exe` hasn't been moved. |
| Windows blocks the app | See step 2.3. |

**Where things are stored:**
- Settings: `%APPDATA%\WowTokenTracker\settings.json`
- Last known colour: `%APPDATA%\WowTokenTracker\state.json`
- Logs: `%LOCALAPPDATA%\WowTokenTracker\logs\`
- Client Secret and webhook: **Windows Credential Manager** (encrypted, never in a file)

---

## 9. Uninstall

1. Turn off the automatic start (see **Removing the automatic start** in step 6).
2. Right-click the tray dot, then click **Exit**.
3. Delete the folder `%LOCALAPPDATA%\Programs\WowTokenTracker`.
4. In File Explorer's address bar, go to `%APPDATA%`, then delete the `WowTokenTracker` folder.
5. Go to `%LOCALAPPDATA%`, then delete the `WowTokenTracker` folder.
6. Remove the saved secrets:
   1. Press **Start**, type `Credential Manager` and open it.
   2. Click **Windows Credentials**.
   3. Under **Generic Credentials**, expand each entry starting with `WowTokenTracker/` and
      click **Remove**. There are two of them.

---

## 10. Building from source (for developers)

You'll need [Go](https://go.dev/dl/) (latest stable) and Git.

```
git clone https://github.com/StormChild/Wow-Token-Tracker.git
cd Wow-Token-Tracker
go test ./...
go build -ldflags "-H windowsgui -s -w" -o WowTokenTracker.exe ./cmd/wowtokentracker
```

`-H windowsgui` stops a console window from appearing alongside the tray icon.

**Publishing a release:** push a version tag. GitHub Actions runs the tests, builds
`WowTokenTracker.exe` and attaches it to a new release:

```
git tag v1.0.0
git push origin v1.0.0
```

Project layout and design notes are in [`CLAUDE.md`](CLAUDE.md) and
[`docs/superpowers/`](docs/superpowers/).

## License

GPL-3.0. See [LICENSE](LICENSE).
