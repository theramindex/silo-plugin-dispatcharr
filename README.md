# Dispatcharr Silo Plugin

Dispatcharr-specific Silo plugin that runs as a Silo-hosted IPTV app.

The plugin owns the IPTV experience directly instead of trying to create fake
Silo media items. Native Jellyfin `/LiveTv/*` export can be added later if
Silo exposes a first-class Live TV provider capability.

## Supported source modes

- **Dispatcharr Direct Connect** (default/recommended)
  - Dispatcharr URL
  - Username
  - Password
  - Uses Dispatcharr REST APIs for catalog data and Dispatcharr proxy/output routes for playback
- **Dispatcharr Direct: API Key**
  - Dispatcharr URL
  - Admin API key from `System > Users > Edit User > API & XC`
  - Uses the same Dispatcharr REST catalog client without storing a password
- **Xtream Codes**
  - Base URL
  - Username
  - Password
  - Live TV, EPG, VOD, and series metadata when the provider exposes those APIs
- **M3U/XMLTV** fallback
  - M3U URL
  - EPG XML URL
  - Live TV and guide data only

## Current behavior

- Validates admin configuration for Dispatcharr, Xtream, and M3U/XMLTV modes
- Syncs Live TV channels, groups, guide data, VOD, and series through Dispatcharr REST
- Keeps Xtream VOD and series support available when Xtream Codes mode is selected
- Resolves playback targets fresh at play time
- Tracks favorites, auto-favorites, hidden categories, recent channels, continue watching, and playback preferences in the plugin preference model
- Keeps stale metadata visible when sync fails
- Exposes a plugin status route at `/dispatcharr/status`
- Exposes a Silo-hosted IPTV app:
  - `/dispatcharr` (navigable user app shown in Silo's Apps sidebar section)
  - `/dispatcharr/player`
  - `/dispatcharr/api/app`
  - `/dispatcharr/api/channels`
  - `/dispatcharr/api/guide`
  - `/dispatcharr/api/categories`
  - `/dispatcharr/api/vod`
  - `/dispatcharr/api/series`
  - `/dispatcharr/api/recordings` (`GET` lists Dispatcharr DVR rows, `POST` schedules an EPG program on Dispatcharr)
  - `/dispatcharr/api/favorites`
  - `/dispatcharr/api/hidden-categories`
  - `/dispatcharr/api/playback`
  - `/dispatcharr/stream?channel_id=...`
  - `/dispatcharr/vod/stream?item_id=...`
- Supports a scheduled sync task with key `dispatcharr-sync`
- Shows Dispatcharr-managed DVR recordings in the plugin app when using Dispatcharr Direct Login or API Key mode. Like AerioTV's Dispatcharr server-side DVR flow, the plugin schedules recordings through Dispatcharr and opens Dispatcharr-owned playback URLs for completed or in-progress server recordings. It does not expose cancel/delete/stop controls.
  - Recommended Silo task trigger: `interval`, `interval_ms: 86400000`
  - A `startup` trigger is useful after install/restart so channels and EPG populate immediately

## v1 limitations

- Exactly one Dispatcharr-backed source
- EPG is required for setup in Xtream and M3U/XMLTV-compatible modes
- Per-user preference persistence depends on Silo exposing plugin-side user config writes; until then this repo keeps an in-memory preference store behind the route handlers
- Source-mode changes reset cached channel/guide state before rebuilding
- Dispatcharr Direct does not silently fall back to Xtream Codes; Direct failures are surfaced in plugin health/status
- Silo host integration still needs real environment validation
- Native Jellyfin `/LiveTv/*` export is not available until Silo exposes a Live TV provider SDK/host capability
- Backend proxy/remux/transcode is not enabled because the current HTTP route SDK response is buffered; playback uses direct redirect URLs

## Silo host notes

Silo shows plugin Apps entries in the normal user sidebar when an HTTP route is declared with:

```json
{
  "access": "authenticated",
  "navigable": true,
  "navigation_label": "IPTV",
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

- Runs tests on every pull request and push.
- Builds Linux binaries for `amd64` and `arm64`.
- Publishes a GitHub Release on every push:
  - `main` branch pushes publish prerelease snapshots (`snapshot-<sha>` tags).
  - `v*` tags publish normal releases.

## Test

```bash
go test ./... -v
```

## Inspect manifest

```bash
go run . manifest
```

## License

`silo-plugin-dispatcharr` is licensed under `AGPL-3.0-or-later`. See `LICENSE`.
