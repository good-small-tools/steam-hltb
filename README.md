**English** | [日本語](README.ja.md)

# steam-hltb

A tool that matches the games in your Steam library against
[HowLongToBeat](https://howlongtobeat.com) (HLTB) and compiles the average
completion times (Main / Main+Extra / Completionist) alongside your own
playtime into a single **HTML table**.

- Runs as a single file — just download and double-click. No installation.
- Output is a self-contained HTML page with sorting and filtering built in.
- HLTB match results are cached locally, so re-runs are fast.

<p><img src="sample.png" alt="Report screen listing a Steam library with HowLongToBeat average completion times" width="820"></p>

## Download

Grab the ZIP for your OS from the [**Releases**](../../releases) page:

| OS | File to download |
|---|---|
| Windows (64-bit) | `steam-hltb-windows-amd64.zip` |
| macOS (Apple Silicon, M1+) | `steam-hltb-macos-arm64.zip` |
| macOS (Intel) | `steam-hltb-macos-amd64.zip` |
| Linux (64-bit) | `steam-hltb-linux-amd64.zip` |

Each ZIP already contains everything you need — no separate setup:

- the program (`steam-hltb` / `steam-hltb.exe`)
- **`steam-hltb.ini`** — a settings file you open in Notepad/TextEdit and fill in
- a short quick-start (`README-first.txt`)

`SHA256SUMS.txt` on the same page lists checksums if you want to verify your download.

> On Windows, the first time you run it you may see a blue
> "Windows protected your PC" (SmartScreen) box. This is normal for free,
> unsigned tools — click **More info → Run anyway**.

## What you need

1. **A Steam Web API key** — free; see below.
2. **Your SteamID64** (the 17-digit numeric ID)

> **Note:** You do **not** need to install Go or run any build commands to use
> this tool — just download the file from [Releases](#download). Go is only
> needed if you want to build it yourself from source (see
> [Building from source](#building-from-source-optional) at the bottom).

### Getting a Steam Web API key

1. Open https://steamcommunity.com/dev/apikey (requires a Steam login)
2. Register with any domain name (e.g. `localhost`)
3. Copy the 32-character key shown

> The key belongs to **your own account**. As long as it is your own library,
> it works even if your profile is set to private.

### Finding your SteamID64

- If your profile URL is `https://steamcommunity.com/profiles/7656119XXXXXXXXXX/`,
  that number is your SteamID64.
- For a custom URL (`/id/yourname`), paste the URL into a site like
  [steamid.io](https://steamid.io) to look up your `steamID64`.

## Setting your key and ID (`steam-hltb.ini`)

The ZIP includes a settings file named **`steam-hltb.ini`**. Open it in any text
editor (Notepad, TextEdit, …) and paste your values after the `=` signs:

```ini
STEAM_API_KEY=your-32-character-key
STEAM_ID=7656119XXXXXXXXXX
```

Save it, keep it in the same folder as the program, and you're done. That's the
only setup step.

> Keep `steam-hltb.ini` private — your API key is a password-grade secret. Never
> share it or commit it anywhere.

(Advanced users: a `.env` file with the same `KEY=VALUE` format also works, and
you can pass `-key` / `-steamid` on the command line or set the
`STEAM_API_KEY` / `STEAM_ID` environment variables. Priority is:
command-line flags > environment variables > `steam-hltb.ini` / `.env`.)

## How to use (two modes)

### A. Generate a report once (the simple way) — recommended

**No command line needed:**

1. **Unzip** the downloaded file to any folder.
2. Open **`steam-hltb.ini`** in that folder, fill in your key and SteamID, and save.
3. **Double-click `steam-hltb.exe`** (Windows) — or the `steam-hltb` program on
   macOS/Linux — in that same folder.
4. A window opens and shows the progress. When it finishes, `report.html` is
   created in that folder and **opens in your browser automatically**.

That's all — nothing to type. (Double-clicking can't pass options, so the
`steam-hltb.ini` file is how the program finds your key and SteamID.)

The first run queries your whole library against HLTB, so it can take anywhere
from tens of seconds to a few minutes depending on how many games you own
(requests are spaced out to keep the load light). Later runs are fast thanks to
the cache. The report is a snapshot from when it ran — reloading the page does
not update it.

<details>
<summary>From the command line (optional)</summary>

```bash
./steam-hltb            # no args needed if you have a .env
open report.html        # only if it didn't open automatically
```

Use `-open=false` to stop it from opening the browser automatically.
</details>

### B. Server mode (refresh to get the latest) ★

```bash
./steam-hltb -serve
# ✔ Server started: http://localhost:8765/
```

Your browser opens automatically. Every time you **reload the page, your Steam
playtime is re-fetched** and the latest table is shown (HLTB average times come
from the cache, so this stays fast). The API key is held by the program and
never appears in the HTML. You can also refresh via the "🔄 Refresh" button at
the top of the page.

To auto-refresh at a fixed interval:

```bash
./steam-hltb -serve -refresh 60s     # auto-reload every 60 seconds
```

> Playtime updates whenever Steam's own aggregation catches up (it usually
> reflects a while after you quit a game, not while you are playing).

## Options

These apply when running from the command line (double-clicking just uses the
defaults).

| Flag | Default | Description |
|---|---|---|
| `-key` | env `STEAM_API_KEY` / `.env` | Steam Web API key |
| `-steamid` | env `STEAM_ID` / `.env` | SteamID64 (17 digits) |
| `-serve` | `false` | Start a local server and refresh on reload |
| `-addr` | `localhost:8765` | Listen address in server mode |
| `-refresh` | `0` | Auto-refresh interval in server mode (e.g. `60s`; 0 = off) |
| `-out` | `report.html` | Output HTML file (file mode) |
| `-open` | `true` | Open the report / server URL in the browser when done |
| `-cache` | `hltb_cache.json` | Cache file for HLTB match results |
| `-no-cache` | `false` | Skip the cache and query HLTB every time |
| `-concurrency` | `4` | Number of concurrent HLTB requests |
| `-min-similarity` | `0.5` | Similarity threshold to count as a match (0–1) |
| `-delay` | `300ms` | Minimum interval between HLTB requests |
| `-limit` | `0` | Cap on the number of games to process (0 = all; for testing) |

Example: try just 20 games first

```bash
./steam-hltb -limit 20
```

## About the generated HTML

- **Summary cards** at the top: owned games / unplayed games / total playtime.
- **Table**: click a column header to sort, type in the box to filter by name,
  and use the "Matched only" / "Unplayed only" checkboxes to narrow it down.
- **Playtime filter**: choose Main / Main+Extra / Completionist and enter a
  lower/upper bound (in hours) to show only games in that range (rows with no
  HLTB time data are hidden while a range is set).
- **Language toggle**: the "日本語 / English" buttons in the top-right switch the
  UI language (your choice is saved in the browser and kept next time).
- Per-row **badges**:
  - `Matched` — similarity at or above the threshold (reliable)
  - `Check` — a candidate was found but similarity is low (may be a different title)
  - `No match` — no candidate found on HLTB

## Building from source (optional)

You only need this if you want to build the program yourself instead of
downloading it from [Releases](#download). It requires **Go 1.21 or later**.

```bash
cd steam-hltb
go build -o steam-hltb .
```

### Tests

```bash
go test -short          # offline unit tests (normalization / similarity)
go test                 # full tests including live HLTB searches
```

## Notes & known limitations

- HLTB **does not publish an official API**. This tool uses the internal
  endpoint (`/api/bleed`) that the site's own frontend calls. A change on HLTB's
  side may break it, in which case `hltb.go` will need updating.
- Because matching is fuzzy by game name, it occasionally picks the wrong title
  (check the `Check` badge). Tune strictness with `-min-similarity`.
- If the original name yields no match at or above the threshold, the tool
  automatically retries with a shorter title — dropping everything after `(` or
  after ` - ` (subtitle separator). This reduces misses for edition suffixes and
  the like.
- DLC, soundtracks, benchmarks, etc. in your library may simply not exist on HLTB.
- Only **owned games** are retrieved. This works even if your profile's game
  details are private, as long as it is the **SteamID of the API key's owner**.

## Disclaimer

- This is an **unofficial**, personal-use utility. It is **not affiliated with,
  endorsed by, or associated with** Valve / Steam or HowLongToBeat in any way.
  All names and data belong to their respective owners.
- Because HowLongToBeat publishes no official API, this tool uses the internal
  endpoint (`/api/bleed`) used by the site's frontend, which may change or stop
  working without notice. **Use it for personal purposes and respect each
  service's terms of use.**
- To avoid putting undue load on the servers, the request interval (`-delay`)
  and concurrency (`-concurrency`) default to conservative values. Please avoid
  bulk scraping or running it repeatedly at short intervals.
- Your Steam Web API key is a secret tied to **your own account**. Do not share
  it or include it in the repository, screenshots, etc. If it leaks, revoke /
  regenerate it promptly at https://steamcommunity.com/dev/apikey.
- The software is provided "AS IS", without warranty of any kind (see LICENSE).

## License

Released under the [MIT License](LICENSE).
