# mtmr-lyrx

Show synced Spotify lyrics on your MTMR Touch Bar. Inspired by [sptlrx](https://github.com/raitonoberu/sptlrx), built for macOS Touch Bar via [MTMR (My TouchBar My Rules)](https://github.com/Toxblh/MTMR).

`mtmr-lyrx` is two pieces:

1. Go CLI daemon — reads Spotify playback, fetches synced lyrics from LRCLIB, writes current lyric text to a local state file.
2. Optional Swift menu bar controller — starts/stops daemon and edits common settings without terminal.

MTMR only reads a local file. Spotify/LRCLIB calls never run inside MTMR.

## Requirements

- macOS
- Go 1.21+ for building from source
- MTMR installed for Touch Bar output
- Spotify account + Spotify Developer app
- Optional: macOS 13+ and Swift 5.9+ for menu bar controller

## Quick Start

```bash
git clone https://github.com/zakyyudha/mtmr-lyrx.git
cd mtmr-lyrx
go build -o mtmr-lyrx ./cmd/mtmr-lyrx
./mtmr-lyrx config init
```

Create Spotify app:

1. Open <https://developer.spotify.com/dashboard>
2. Create app
3. Add redirect URI: `http://127.0.0.1:8888/callback`
4. Copy Client ID and Client Secret

Login:

```bash
SPOTIFY_CLIENT_ID=your-id SPOTIFY_CLIENT_SECRET=your-secret ./mtmr-lyrx login
./mtmr-lyrx status
```

Install MTMR helper and copy printed JSON into MTMR `items.json`:

```bash
./mtmr-lyrx mtmr-config --install
```

MTMR config path:

```text
~/Library/Application Support/MTMR/items.json
```

Run daemon:

```bash
./mtmr-lyrx run
```

Play Spotify. Current lyric line appears on Touch Bar.

## Installation

### Homebrew cask (app + CLI)

Recommended after publishing the `v0.1.0` GitHub Release:

```bash
brew tap zakyyudha/mtmr-lyrx
brew install --cask mtmr-lyrx
```

The cask installs both:

- `MTMRLyrx.app` into `/Applications`
- `mtmr-lyrx` CLI into Homebrew's `bin`

Cask definition:

```text
Casks/mtmr-lyrx.rb
```

Build the cask artifact locally:

```bash
make cask
# or explicit:
make cask VERSION=0.1.0
```

This creates:

```text
dist/MTMRLyrx-0.1.0-macos.zip
dist/MTMRLyrx-0.1.0-macos.zip.sha256
```

Upload the zip to GitHub Releases at:

```text
https://github.com/zakyyudha/mtmr-lyrx/releases/download/v0.1.0/MTMRLyrx-0.1.0-macos.zip
```

`Casks/mtmr-lyrx.rb` currently pins the SHA-256 for the generated `0.1.0` cask zip. If you rebuild the zip, update the cask `sha256` from `dist/MTMRLyrx-0.1.0-macos.zip.sha256`.

**Note:** The app is currently unsigned. Homebrew cask can install it, but macOS Gatekeeper may require right-click → Open. Signed/notarized app release is future work.

### Build from source

```bash
go build -o mtmr-lyrx ./cmd/mtmr-lyrx
```

Install manually:

```bash
install -m 0755 mtmr-lyrx /usr/local/bin/mtmr-lyrx
# or Apple Silicon Homebrew prefix:
install -m 0755 mtmr-lyrx /opt/homebrew/bin/mtmr-lyrx
```

Check:

```bash
mtmr-lyrx --version
mtmr-lyrx --help
```

### Release binary

Download release artifact for your Mac:

- `mtmr-lyrx_darwin_arm64.tar.gz` — Apple Silicon
- `mtmr-lyrx_darwin_amd64.tar.gz` — Intel
- `checksums.txt` — SHA-256 checksums

Verify and install:

```bash
shasum -a 256 -c checksums.txt
tar -xzf mtmr-lyrx_darwin_arm64.tar.gz
install -m 0755 mtmr-lyrx /usr/local/bin/mtmr-lyrx
```

Homebrew formula/cask is future work, not shipped in v1.

## Configuration

Create config:

```bash
mtmr-lyrx config init
mtmr-lyrx config init --force
```

Paths:

```text
config:  ~/.config/mtmr-lyrx/config.yaml
cache:   ~/.config/mtmr-lyrx/cache/
token:   ~/.config/mtmr-lyrx/spotify-auth.json
state:   ~/.config/mtmr-lyrx/cache/current.txt
helper:  ~/.config/mtmr-lyrx/read-lyrics.sh
```

Override config path:

```bash
mtmr-lyrx --config /path/to/config.yaml status
export MTMR_LYRX_CONFIG=/path/to/config.yaml
```

Common settings:

```yaml
provider:
  lrclib:
    base_url: "https://lrclib.net"
    timeout: 5s
lyrics:
  duration_tolerance_ms: 2000
  prefer_isrc: true
  require_synced: true
  offset_ms: 0
display:
  width: 30
  scroll_speed_ms: 200
  separator: " · "
  placeholder: "♪"
  state_file: ""
  mtmr_refresh_interval: 1
spotify:
  redirect_url: "http://127.0.0.1:8888/callback"
  token_file: ""
  poll_interval_ms: 2000
  seek_resync_threshold_ms: 2000
```

Set supported values without editing YAML:

```bash
mtmr-lyrx config set display.width 45
mtmr-lyrx config set display.scroll_speed_ms 150
mtmr-lyrx config set lyrics.offset_ms -500
mtmr-lyrx config set display.placeholder "♪"
mtmr-lyrx config set display.separator " · "
```

Supported keys:

- `display.width`
- `display.scroll_speed_ms`
- `display.placeholder`
- `display.separator`
- `lyrics.offset_ms`
- `spotify.poll_interval_ms`
- `spotify.seek_resync_threshold_ms`

## Command Reference

### `config`

```bash
mtmr-lyrx config path
mtmr-lyrx config init
mtmr-lyrx config init --force
mtmr-lyrx config set display.width 45
```

### `login`

Authenticate Spotify with official OAuth Authorization Code flow:

```bash
SPOTIFY_CLIENT_ID=your-id SPOTIFY_CLIENT_SECRET=your-secret mtmr-lyrx login
mtmr-lyrx login --client-id your-id --client-secret your-secret
```

`login` opens browser and waits for callback on `127.0.0.1:8888/callback`. Credentials are saved into config so token refresh works later.

### `status`

```bash
mtmr-lyrx status
mtmr-lyrx status --json
```

Shows login/token state, current track, playback progress, active device, and errors.

### `lookup`

Look up synced lyrics via LRCLIB:

```bash
mtmr-lyrx lookup \
  --artist "Artist" \
  --title "Song" \
  --album "Album" \
  --duration-ms 180000

mtmr-lyrx lookup --artist "Artist" --title "Song" --json
```

Statuses:

- `matched`
- `no_match`
- `no_synced_lyrics`
- `malformed_lyrics`
- `provider_error`
- `rate_limited`
- `invalid_metadata`

Full lyric text is not printed by default.

### `run`

Run Spotify-driven daemon:

```bash
mtmr-lyrx run
mtmr-lyrx run --offset-ms -500
```

Mock mode for testing:

```bash
mtmr-lyrx run --mock --artist "Public Domain" --title "Test Song" --duration-ms 9000
mtmr-lyrx run --mock --once --artist "Public Domain" --title "Test Song"
```

Daemon writes current Touch Bar text to `~/.config/mtmr-lyrx/cache/current.txt`.

### `offset`

```bash
mtmr-lyrx offset show
mtmr-lyrx offset +500
mtmr-lyrx offset -500
mtmr-lyrx offset set 0
```

Positive offset delays lyrics. Negative offset advances lyrics.

### `mtmr-config`

Print MTMR `shellScriptTitledButton` snippet:

```bash
mtmr-lyrx mtmr-config
```

Install helper script and print snippet:

```bash
mtmr-lyrx mtmr-config --install
```

Current MTMR snippet shape uses object `source`:

```json
{
  "type": "shellScriptTitledButton",
  "width": 380,
  "refreshInterval": 0.5,
  "bordered": false,
  "source": {
    "inline": "/bin/bash /Users/you/.config/mtmr-lyrx/read-lyrics.sh"
  }
}
```

### `cache`

`mtmr-lyrx` caches LRCLIB lookup metadata only. It does not cache lyric text.

```bash
mtmr-lyrx cache show
mtmr-lyrx cache show --provider lrclib
mtmr-lyrx cache show --json
mtmr-lyrx cache clear --provider lrclib --dry-run
mtmr-lyrx cache clear --provider lrclib
mtmr-lyrx cache clear --provider all
```

### `update`

Check GitHub Releases for newer CLI binary:

```bash
mtmr-lyrx update check
mtmr-lyrx update check --json
```

Dry-run install:

```bash
mtmr-lyrx update install --dry-run
```

Install update:

```bash
mtmr-lyrx update install
```

Updater downloads official release asset, verifies SHA-256 checksum from `checksums.txt`, backs up current binary as `mtmr-lyrx.bak`, then replaces the CLI binary. Restart daemon/menu bar after updating.

Limitations:

- Updates CLI binary only.
- Unsigned menu bar app does not self-update yet.
- No hidden `sudo`; permission errors must be fixed manually.

## MTMR Touch Bar Integration

State-file flow:

```text
Spotify → mtmr-lyrx daemon → ~/.config/mtmr-lyrx/cache/current.txt → MTMR helper → Touch Bar
```

Steps:

1. Run `mtmr-lyrx mtmr-config --install`.
2. Copy printed JSON object into MTMR `items.json`.
3. Restart MTMR.
4. Run `mtmr-lyrx run`.

Why this architecture:

- MTMR command stays cheap.
- No Spotify/LRCLIB calls from Touch Bar widget.
- State file writes are atomic, so MTMR never reads partial text.

## Spotify Setup

Spotify provides playback state only. It does not provide lyrics through public Web API.

Required scopes:

- `user-read-currently-playing`
- `user-read-playback-state`

Setup:

1. Create Spotify Developer app.
2. Add redirect URI `http://127.0.0.1:8888/callback`.
3. Login with env vars or flags.
4. Run `mtmr-lyrx status`.
5. Start daemon with `mtmr-lyrx run`.

## LRCLIB Lyrics Lookup

v1 uses [LRCLIB](https://lrclib.net) as online synced lyrics provider.

Matching uses:

- ISRC when available
- normalized artist/title/album
- duration tolerance, default ±2000ms

No local `.lrc` library support in v1. Local lyric files are deferred to future source support.

## macOS Menu Bar Controller

Build:

```bash
swift build --package-path macos/mtmr-lyrx-menu
```

Run:

```bash
./macos/mtmr-lyrx-menu/.build/debug/MTMRLyrx
```

Controls:

- Start/Stop Daemon
- Spotify Login
- Open Config
- Timing Offset presets
- Display Width presets
- Scroll Speed presets
- Check/Install update
- Quit

Binary resolution order:

1. `UserDefaults` key `binaryPath`
2. `./mtmr-lyrx`
3. `/usr/local/bin/mtmr-lyrx`
4. `/opt/homebrew/bin/mtmr-lyrx`

Limitations:

- Unsigned development build.
- macOS Gatekeeper may require right-click → Open or terminal launch.
- Signed app packaging and notarization are future work; Homebrew cask packaging is provided but unsigned.

## Release Builds

Dependency-free release flow uses `Makefile`.

Build local binary:

```bash
make build
```

Run tests:

```bash
make test
```

Build CLI-only release artifacts:

```bash
make release
# or explicit:
make release VERSION=0.1.0
```

Creates:

```text
dist/mtmr-lyrx_darwin_arm64.tar.gz
dist/mtmr-lyrx_darwin_amd64.tar.gz
dist/checksums.txt
```

Build Homebrew cask artifact containing both `MTMRLyrx.app` and `mtmr-lyrx` CLI:

```bash
make cask
# or explicit:
make cask VERSION=0.1.0
```

Creates:

```text
dist/MTMRLyrx-0.1.0-macos.zip
dist/MTMRLyrx-0.1.0-macos.zip.sha256
```

Version is embedded with:

```bash
-ldflags "-X github.com/zakyyudha/mtmr-lyrx/internal/cli.version=0.1.0"
```

Validate CLI checksum:

```bash
cd dist
shasum -a 256 -c checksums.txt
```

Validate cask zip checksum:

```bash
shasum -a 256 dist/MTMRLyrx-0.1.0-macos.zip
```

## Troubleshooting

**No lyrics appear**

- Run `mtmr-lyrx status` to confirm Spotify playback.
- Run `mtmr-lyrx lookup --artist "..." --title "..." --debug`.
- Check `~/.config/mtmr-lyrx/cache/current.txt`.
- Confirm MTMR snippet calls `~/.config/mtmr-lyrx/read-lyrics.sh`.

**Spotify login/token fails**

- Confirm redirect URI exactly equals `http://127.0.0.1:8888/callback`.
- Re-run `mtmr-lyrx login` with Client ID/Secret.
- Check `~/.config/mtmr-lyrx/config.yaml` has Spotify credentials.

**Lyrics timing is early/late**

```bash
mtmr-lyrx offset +500
mtmr-lyrx offset -500
mtmr-lyrx offset set 0
```

**MTMR widget missing**

- Ensure `source` is an object with `inline`, not a string.
- Restart MTMR after editing `items.json`.
- Run `/bin/bash ~/.config/mtmr-lyrx/read-lyrics.sh` manually.

**Update fails**

- Run `mtmr-lyrx update check --json`.
- Try `mtmr-lyrx update install --dry-run`.
- Permission errors mean target path is not writable; reinstall manually with `install -m 0755`.
- Checksum mismatch means update is aborted; do not bypass it.

**Debug logging**

```bash
mtmr-lyrx --debug status
mtmr-lyrx --debug run
mtmr-lyrx --debug lookup --artist "..." --title "..."
```

## Uninstall

Stop daemon/menu bar, then remove files:

```bash
rm -rf ~/.config/mtmr-lyrx/
rm /usr/local/bin/mtmr-lyrx        # or actual install path
rm /usr/local/bin/mtmr-lyrx.bak    # if updater created backup
```

Remove MTMR snippet from:

```text
~/Library/Application Support/MTMR/items.json
```

## Legal / Copyright

This project does not ship copyrighted lyrics. Tests use fake/public-domain lyric fixtures only.

Lyrics are fetched at runtime from LRCLIB, a community-maintained provider. Users are responsible for ensuring they have rights to display lyrics from any provider they use.

Spotify is used only for playback metadata and progress. Spotify public Web API does not provide lyrics.

No scraping of Spotify, Genius, Apple Music, YouTube, NetEase, or other lyric websites/private APIs is performed.
