# Dispatcharr plugin for Silo

Dispatcharr-specific Silo plugin that runs as a Silo-hosted Live TV app.

> **Important:** This app does **not** manage IPTV streams. Dispatcharr Direct
> with an **Admin API Key** is the recommended full-feature source. Xtream Codes
> and M3U/XMLTV are supported as reduced-capability source modes.

The plugin owns Silo's Live TV presentation and playback sessions. The configured upstream remains responsible for channels, guide data, stream outputs, and recording behavior in Direct mode.

## Requirements and ownership boundaries

- **Dispatcharr Direct is preferred.** It is required for DVR scheduling and recordings. Xtream and M3U/XMLTV expose playback and guide capabilities only.
- **Dispatcharr manages sources.** Stream health, output profiles, and recording behavior remain upstream responsibilities.
- **Silo presents and plays.** Live channels use an authenticated continuous MPEG-TS/HLS relay. Provider URLs and credentials remain server-side, including nested HLS playlists and keys. This requires the Ramindex Silo fork's loopback relay support. Legacy credentialed non-HLS file routes still return `stream_requires_hls` (422).
- **Compatible playback** uses host FFmpeg and a VAAPI GPU when source codecs cannot play in the browser. There is no software encoder fallback.
- **ECM is recommended.** Use Enhanced Channel Manager (ECM) with Dispatcharr for
  channel/group curation, lineup cleanup, and operational channel management.

Native Jellyfin `/LiveTv/*` export can be added later if Silo exposes a
first-class Live TV provider capability.

## Supported source modes

- **Dispatcharr Direct: API Key** (required/recommended)
  - Dispatcharr URL
  - Admin API key from `System > Users > Edit User > API & XC`
  - Uses Dispatcharr REST APIs for catalog data and Dispatcharr proxy/output routes for playback
- **Dispatcharr Direct Connect** (legacy/manual login)
  - Dispatcharr URL
  - Username
  - Password
  - Uses Dispatcharr REST APIs for catalog data and Dispatcharr proxy/output routes for playback
- **Xtream Codes**
  - Base URL
  - Username
  - Password
  - Live TV, EPG, VOD, and series metadata when the provider exposes those APIs
  - Browser live channels relay MPEG-TS and HLS without exposing account credentials. Credentialed non-HLS VOD/file routes remain unsupported.
  - Without an EPG XML URL, guide data comes from per-channel `get_short_epg`
    calls run by a bounded worker pool (8 concurrent). Channels whose guide call
    fails are skipped and counted in the log; the lineup is still saved
- **M3U/XMLTV** fallback
  - M3U URL
  - EPG XML URL
  - Live TV and guide data only

## Current behavior

- Validates admin configuration for Dispatcharr, Xtream, and M3U/XMLTV modes
- Syncs Live TV channels, groups, guide data, VOD, and series through Dispatcharr REST
- Keeps Xtream VOD and series support available when Xtream Codes mode is selected
- Resolves playback targets fresh at play time
- Stores favorites, hidden categories, recent channels, custom groups, and
  playback preferences in Silo's per-user plugin configuration, read and
  written by the browser (see "User identity" below)
- Coordinates manual, scheduled, channel-only, and guide-only refreshes through
  one serialized job state and preserves the last known good guide on failure.
  A refresh whose guide drops below half of the previous still-current program
  count or channel coverage is treated as an upstream glitch: the previous
  guide is kept and `status.epgWarning` explains why. Source-mode or
  connection changes always accept the new guide
- Streams XMLTV guides (custom EPG XML URL, Xtream `xmltv.php`, M3U/XMLTV)
  through an incremental decoder capped at 256 MiB instead of buffering the
  whole document
- Loads guide data separately from the lightweight app bootstrap; refresh
  polling uses only the status endpoint
- Keeps stale metadata visible when sync fails
- Exposes a plugin status route at `/dispatcharr/status`
- Exposes a Silo-hosted IPTV app:
  - `/dispatcharr` (navigable user app shown in Silo's Apps sidebar section)
  - `/dispatcharr/player`
  - `/dispatcharr/api/app`
  - `/dispatcharr/api/channels`
  - `/dispatcharr/api/guide` (see "Guide API" below)
  - `/dispatcharr/api/categories`
  - `/dispatcharr/api/recordings` (`GET` lists Dispatcharr DVR rows, `POST` schedules an EPG program on Dispatcharr; available to all signed-in users)
  - `/dispatcharr/api/recordings/stop`, `/dispatcharr/api/recordings/delete` (`POST`, **admin only**)
  - `/dispatcharr/api/refresh` (`POST` force refresh, **admin only**)
  - `/dispatcharr/api/timeshift/*` (shared Live Rewind leases, status, and administration)
  - `/dispatcharr/api/sports`
  - `/dispatcharr/api/events`
  - `/dispatcharr/api/status`
  - `/dispatcharr/stream?channel_id=...`
  - `/dispatcharr/vod/stream?item_id=...`
- Exposes host-scheduled task keys `dispatcharr-sync`,
  `dispatcharr-refresh-channels`, and `dispatcharr-refresh-epg`
- Shows Dispatcharr-managed DVR recordings in the plugin app when using Dispatcharr Direct Login or API Key mode. Like AerioTV's Dispatcharr server-side DVR flow, the plugin schedules recordings through Dispatcharr and opens Dispatcharr-owned playback URLs for completed or in-progress server recordings. Any signed-in user can schedule; stopping and deleting recordings are admin-only.
  - Silo owns task cadence; the plugin cannot declare or change intervals
  - A `startup` trigger is useful after install/restart so channels and EPG populate immediately

## Shared playback and Live Rewind

- Dispatcharr, XC, and HDHomeRun use the same guarded playback, viewer ownership, startup fallback, and cleanup library (SDK v0.30.0).
- Admin **Station alternatives** explicitly map equivalent channel IDs to one station key. Lower priority numbers win; another mapped source is tried before any stream bytes are sent. Names never imply equivalence. Interrupted streams need restarting.
- Live Rewind is opt-in and limited to Dispatcharr Direct. H.264 MPEG-TS can be copied into keyframe-aligned HLS; unsupported rewind codecs can use GPU conversion with a bounded history.
- Administrators control cache budget, rewind window, free-space reserve, and copy-buffer count. Defaults are 5 GiB, 30 minutes, 2 GiB free, and 20 buffers. GPU histories also respect the window and budget. Cache status and clear actions include both paths.
- Shared playback leases belong to the authenticated user/profile. Leaving playback releases resources; abandoned leases expire after 75 seconds. Legacy copy buffers may share one channel ingest; managed browser sessions reserve their own ingest.

## Compatible playback

- Main, preview, and multiview players try source codecs first. A codec failure triggers one GPU conversion attempt; the player menu also offers **Compatible playback**.
- Conversion requires host FFmpeg and `/dev/dri/renderD128`. The bundled Jellyfin FFmpeg location is detected automatically. Output is H.264/AAC HLS capped at 1080p, 30 fps, and 6 Mbps video. Decoding and scaling use the CPU; encoding uses the GPU, with no software fallback.
- Two GPU encoding slots are shared across Dispatcharr, XC, and HDHomeRun processes on one Silo server. A viewer/profile can have one converted channel per plugin; additional direct streams remain subject to source limits. Startup is limited to 30 seconds.
- Inputs stay behind a restricted loopback gateway and generated media passes through authenticated, owner-checked routes. Normal conversion has a short rolling HLS window; Live Rewind conversion retains bounded history.

## Profile and group organization

- Leave **Channel Profile** blank to ingest all Dispatcharr profiles and attach
  every profile membership to each channel. Enter one profile ID or exact name
  to scope both the lineup and browse tree to that profile.
- Profiles answer “which lineup?” and channel groups answer “what kind of
  channel?” In `Profile pipe + group pipe` mode the browser combines them as
  `Profile / Group`, for example `US TV / NY / Locals / Buffalo`.
- Each Silo user can choose which imported Dispatcharr profiles appear under
  **Live TV Settings**. Existing users default to all profiles; selecting a
  subset filters browse folders, guide programs, search, On Later, sports,
  events, favorites, custom-group pickers, and multiview suggestions. This is
  presentation filtering, not a server-side channel entitlement boundary.
- **Collapse duplicate virtual groups** removes only repeated adjacent path
  labels such as `International TV / International TV`; it never drops a
  channel. Sports/event channel matches also collapse repeated channel IDs.

## Sports-first player

- Sports schedules, identities, live status, scores, league metadata, team
  artwork, venues, and broadcast timezones come from the
  [Sportarr public partner API](https://sportarr.net/docs/api). Sportarr is the
  canonical sports data source; the plugin does not maintain a second sports
  schedule model.
- When Sportarr does not provide league or team marks, recognized competition
  and team names can use public [Game Thumbs](https://github.com/sethwv/game-thumbs)
  logo endpoints as visual fallbacks. Team lookups fail independently and
  unknown sports buckets retain local glyph or abbreviation fallbacks.
- Dispatcharr channels and guide programs are matched against Sportarr events
  to determine what is actually playable. Events without a confident channel
  match are omitted from live/upcoming browsing instead of presenting dead
  playback actions.
- Administrators can opt into a score and channel drawer in the video player.
- The drawer reuses the existing sports feed, prioritizes the current channel,
  live games, and favorite teams, and refreshes only while it is open.
- Event and channel actions require a medium-or-better guide match.
  [Teamarr](https://github.com/Pharaoh-Labs/teamarr) and the
  [Dispatcharr Sports Event Autocreator](https://github.com/titooo7/dispatcharr-sports-event-autocreator)
  remain upstream organizers; the plugin does not create, rename, or delete
  Dispatcharr channels.

Sports navigation separates discovery from personal follows:

- **Teams** browses the full team directory by league, with search and follow actions.
- **My Sports** shows existing team, fighter, and league follows, with their live and upcoming events.
- Following and unfollowing use the same profile preferences as Live TV; no migration or re-selection is needed.

Sports guide preparation runs in the background, reusing normalized event identities during matching. Parsed guide events and the channel word index are cached across score refreshes, invalidating when the guide changes or a broadcast crosses a time boundary. The browser keeps checking long enough for a cold build to finish. Slow Sportarr or ESPN requests keep the last available data visible; individual Sports data requests stop waiting before Silo's plugin timeout while shared ESPN cache fills can finish for later requests.

## Sports replays from Silo

- Administrators can enable the Sports experience and select one or more
  dedicated Silo Sports libraries as replay sources.
- Replay catalog queries run in the signed-in user's browser through Silo's
  authenticated catalog API. The configured library list is intersected with
  that user's accessible libraries before any query is made.
- Only high-confidence event matches are shown on Sports cards. The matcher
  requires both teams and scores league, date proximity, and title overlap;
  unmatched library items are never exposed as a general VOD browser.
- Replay actions use Silo's normal item/watch routes. The plugin does not store
  Silo tokens, library paths, or media file locations.

## Event-series guide detection

- The Events view recognizes selected guide-driven series such as golf,
  Formula 1, combat sports, and tennis without adding another schedule service.
- Event rules support inclusion keywords, exclusion keywords for replays and
  studio programming, and a configurable 15-360 minute coverage window.
- Same-day airings inside a coverage window share one event window and
  deduplicate channels. Later coverage remains a separate ordered window on the
  same event card while the legacy flat event fields remain available.
- This adapts the EPG matching approach used by
  [Alertle V2](https://github.com/Deekerman/Alertle-V2); alerts, notification
  endpoints, and its scheduler remain outside the plugin.

## User identity and admin-only actions

The Silo host's plugin HTTP proxy forwards only an allowlist of client headers
(`accept`, `accept-language`, `content-type`, `if-none-match`,
`if-modified-since`, `range`, `user-agent`, `origin`, `referer`) and stamps its
own identity headers on authenticated requests: `X-Silo-User-Id`,
`X-Silo-User-Role` (`admin` or `user`), `X-Silo-User-Name`, and
`X-Silo-Profile-Name`. A browser cannot inject or override them, so the plugin
trusts `X-Silo-User-Role` for authorization.

- `POST /dispatcharr/api/refresh`, `POST /dispatcharr/api/recordings/stop`, and
  `POST /dispatcharr/api/recordings/delete` are declared `"access": "admin"`
  in the manifest and are also enforced in the handlers, which return
  `403 {"ok":false,"error":"admin_required",...}` for non-admins.
- `GET /dispatcharr/api/app` includes `"isAdmin": true|false` so the UI can
  hide those controls.
- The plugin does not yet key server-side state by `X-Silo-User-Id`; per-user
  preferences remain browser-managed.

## Guide API

`GET /dispatcharr/api/guide` (also `/dispatcharr/guide`) returns
`{"programs":[...]}` ordered by start time, then program ID. Schedules are
persisted in an indexed local SQLite database; channel/time filters read only
the requested window.
Optional query parameters:

| Parameter | Meaning |
| --- | --- |
| `channel_id` | Single channel ID (legacy). |
| `channel_ids` | Comma-separated channel IDs; merged with `channel_id`. Max 2000. |
| `start` | Unix seconds. With `end`, returns programs overlapping `[start, end)`. |
| `end` | Unix seconds. Either bound may be omitted to leave that side open. |

With no parameters the full guide is returned (backward compatible). A program
without an end time matches when its start lies inside the window. Invalid
numbers or `start >= end` return `400`.

### Local guide storage

The private catalog directory contains `catalog-snapshot.sqlite`. The database
uses WAL mode and must be on a local filesystem. Each refresh commits the
lineup metadata and programme replacement together. Readers continue using
the last committed guide while the import runs. Health-only updates do not
rewrite programme rows.

Existing `catalog-snapshot.json` files are served while a background import
migrates them. The JSON file is retained as a rollback copy; after migration,
SQLite takes precedence and the JSON copy is no longer updated. Startup reads
catalog metadata without deserializing the complete schedule. Normal guide
requests fetch channel/time windows; the unfiltered full-guide export remains
available. A cache read failure returns `503`, rather than an empty successful
guide. Sports preparation keeps its derived matching index in memory.

## v1 limitations

- Exactly one Dispatcharr-backed source
- EPG is required for setup in Xtream and M3U/XMLTV-compatible modes
- Per-user state (favorites, hidden categories, playback preferences) is read
  and written by the browser through Silo's user config API, never through
  process-global plugin state.
- Source-mode changes reset cached channel/guide state before rebuilding
- Dispatcharr Direct does not silently fall back to Xtream Codes; Direct failures are surfaced in plugin health/status
- Native Jellyfin `/LiveTv/*` export is not available until Silo exposes a Live TV provider SDK/host capability
- The HTTP route SDK buffers each response. Live Rewind and hardware playback
  conversion serve bounded HLS segments through that interface; they never put
  an endless transport stream inside an RPC response. Continuous live streams use the host's loopback relay instead.
- HTTP routes and navigation are static manifest declarations. Settings cannot
  dynamically add/remove navigation entries.
- Scheduled-task cadence and retry policy are host-owned. The plugin only
  implements task execution.

## Silo host notes

Silo shows plugin Apps entries in the normal user sidebar when an HTTP route is declared with:

```json
{
  "access": "authenticated",
  "navigable": true,
    "navigation_label": "Live TV (Dispatcharr)",
  "navigation_kind": "user"
}
```

This plugin declares `/dispatcharr` that way. Admin-only plugin pages can use `navigation_kind: "admin"` with `access: "admin"`.

Scheduled task cadence is not read from the plugin manifest by current Silo host builds. The plugin exposes `scheduled_task.v1` capabilities, but Silo stores active schedules in its task trigger table. Configure `plugin:<installation_id>:dispatcharr-sync` or `plugin:<installation_id>:dispatcharr-refresh-channels` with a slower catalog interval such as 24 hours, and configure `plugin:<installation_id>:dispatcharr-refresh-epg` with a shorter guide interval such as 6 hours. A startup trigger is useful for immediate cache hydration after restarts.

## Build

```bash
go build ./...
```

## Build Upload ZIP (Silo Admin Upload)

Build a Linux binary and package a Silo-compatible upload ZIP containing
`plugin` + `manifest.json`:

```bash
VERSION="0.0.0-local-$(git rev-parse --short HEAD)"
BIN="dispatcharr-${VERSION}-linux-amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -X main.buildVersion=${VERSION}" -o "dist/${BIN}" .
go run ./cmd/package-upload \
  -binary "dist/${BIN}" \
  -version "${VERSION}" \
  -goos linux \
  -goarch amd64 \
  -plugin-id silo.ramindex.dispatcharr
```

Upload the generated `dist/<binary>.silo-plugin.zip` file in Silo.

## GitLab CI builds

The repository includes `.gitlab-ci.yml` to run tests and produce versioned plugin binaries.

- Tagged builds (`vX.Y.Z`) use `X.Y.Z` as the plugin manifest version.
- Branch builds use a snapshot version `0.0.0-<shortsha>`.
- Artifacts include:
  - Linux binaries (`amd64`, `arm64`)
  - generated manifest JSON from each binary (`<binary>.manifest.json`)
  - SHA256 files (`<binary>.sha256`)

## GitHub Actions builds and releases

The repository also includes `.github/workflows/ci.yml` for GitHub-hosted runners.

- Runs vet, unit tests, the race detector, JavaScript syntax checks, and manifest
  validation on every pull request and push.
- Builds Linux binaries for `amd64` and `arm64`.
- Publishes a GitHub Release only for a `v*` tag after confirming the tag and
  manifest versions match. Branch builds remain downloadable workflow artifacts.
- When `CATALOG_PUSH_TOKEN` is configured, the tagged workflow updates and
  validates `theramindex/silo-plugins` from the published binary checksums, then
  commits the catalog change as `ramindex-ci <github@ramindex.org>`.

## Test

```bash
./scripts/verify-release.sh
```

For a tagged release candidate, also verify the intended version:

```bash
./scripts/verify-release.sh 0.3.7 --version-only
```

## Inspect manifest

```bash
go run . manifest
```

## License

`silo-plugin-dispatcharr` is licensed under `AGPL-3.0-or-later`. See `LICENSE`.

## Generated sports guide

In Admin settings, Sports guide can fill missing EPG coverage from the existing
Sportarr/ESPN schedule providers. Generation runs in the background and creates
one hour of pregame, an estimated game window, and one hour of postgame coverage.
Provider listings always take precedence; generated blocks are clipped around
them. The guide endpoints include a `generated` object with schedule source,
event identity, phase, match evidence, and estimated timing. No upstream channel
names or guide records are rewritten.

Automatic matchup matching requires a league context, a matchup separator, and
both named competitors. Multiple matching fixtures are left unresolved unless
an ISO date in the channel name identifies one game. Group hints can select a
league and choose two-competitor or conservative full-title tournament/session
matching. Manual event links resolve ambiguous channels or disable automatic
filling for a channel; an expired or missing linked event never rolls forward
to a different game. Links and hints are saved with the other admin settings.

The private `sports-guide.sqlite` cache sits beside `catalog-snapshot.sqlite`.
It survives restarts independently of provider EPG, preserves its last committed
data when schedule refresh fails, and scopes entries to source configuration
and policy. Renamed channels and entries older than 24 hours are excluded until
regenerated. Generation never reads this overlay as EPG evidence. Admin controls use the installation's lineup.
