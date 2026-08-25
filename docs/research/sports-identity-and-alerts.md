# Sports identity, matchup artwork, and alert research

Research date: 2026-08-25

Upstream revisions inspected:

- `sethwv/game-thumbs` at [`433d62d`](https://github.com/sethwv/game-thumbs/tree/433d62d716f254073734c7abf52404c042d9c572)
- `Pharaoh-Labs/teamarr` at [`a9be542`](https://github.com/Pharaoh-Labs/teamarr/tree/a9be5428f3795c8082674b54c57b0b63b56f9325)
- `Deekerman/Alertle-V2` at [`2ddc939`](https://github.com/Deekerman/Alertle-V2/tree/2ddc9399befd42d383b47c0d971af6db948a960e)

## Executive recommendation

Dispatcharr should own a small, provider-neutral sports identity layer and use Game Thumbs as an optional artwork provider, not as the source of truth for events. Teamarr can optionally enrich that identity layer through its read-only HTTP cache API when a user already runs Teamarr, but the plugin should continue to work with Sportarr/EPG data alone. Alerts are feasible, but they should be a separate, staged capability built around Silo's persisted plugin state and authenticated routes rather than copying Alertle's scheduler.

Recommended delivery order:

1. Normalize league and team identity in the plugin backend.
2. Resolve logos with a deterministic fallback chain and server-side cache.
3. Use the same canonical identity for matchup cards and Game Thumbs thumbnail URLs.
4. Add optional Teamarr enrichment behind an admin setting and short timeout.
5. Design alerts as a new backend subsystem after the identity work is stable.

This avoids making the browser guess leagues, prevents repeated third-party requests, and gives cards and future alerts one shared event identity.

## Licensing and reuse boundary

| Project | Repository license state | Safe use |
| --- | --- | --- |
| Game Thumbs | An MIT `LICENSE` file is present. | Code can be reused or adapted if the copyright and MIT permission notice are retained in copies or substantial portions. Concepts and API use are also fine. |
| Teamarr | The README says “MIT,” but the checked revision contains no `LICENSE` file and GitHub reports no detected license. | Treat the implementation as all-rights-reserved until Pharaoh Labs publishes a license file or gives written permission. Reimplement patterns from behavior/API contracts; do not copy source. |
| Alertle-V2 | No `LICENSE` file and GitHub reports no detected license. | Treat as all-rights-reserved. Use it to inform requirements and architecture only; do not copy code, templates, mascot/art, or text. |

Sources: Game Thumbs [`LICENSE`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/LICENSE), Teamarr [`README.md`](https://github.com/Pharaoh-Labs/teamarr/blob/a9be5428f3795c8082674b54c57b0b63b56f9325/README.md), and the Alertle-V2 [repository root](https://github.com/Deekerman/Alertle-V2/tree/2ddc9399befd42d383b47c0d971af6db948a960e).

The upstream sports logos and team marks are third-party trademarks and may not be covered by the software license. Dispatcharr should fetch them on demand from the configured provider and cache responses; it should not import either repository's asset directory into the plugin release.

## Game Thumbs

### What it provides

Game Thumbs exposes image endpoints with human-readable league and team identifiers:

- `/:league/:team/teamlogo[.png]`
- `/:league/leaguelogo[.png]`
- `/:league/thumb[.png]`
- `/:league/:team/thumb[.png]`
- `/:league/:team1/:team2/thumb[.png]`
- `/:league/:team/raw` for resolved metadata

The matchup thumbnail supports 4:3, 16:9, and square output, visual styles, a centered league logo, badges, winner emphasis, and `fallback=true`. The public documentation describes fallback as a league thumbnail for a missing single team and a grayscale league mark for a missing side of a matchup. See [`docs/api-reference/thumb.md`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/docs/api-reference/thumb.md) and the thin route adapters in [`src/routes/thumb.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/routes/thumb.js) and [`src/routes/teamlogo.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/routes/teamlogo.js).

### Identity and alias resolution

The strongest reusable pattern is staged identity resolution:

1. Normalize Unicode to decomposed form, remove diacritics, lowercase, replace punctuation, and collapse spaces.
2. Also produce a compact alphanumeric form for `KansasCityCurrent`, `kansas-city-current`, and `Kansas City Current` equivalence.
3. Check curated aliases first.
4. Score exact abbreviation, nickname, short display name, full display name, city, concatenated city-plus-nickname, and controlled partial matches.
5. Search configured feeder/fallback leagues when the requested competition is not the team's domestic league.
6. Return a structured unresolved result rather than silently selecting a low-confidence team.

The implementation is in [`src/helpers/teamUtils.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/helpers/teamUtils.js), with the intended priority and feeder-league behavior documented in [`docs/team-matching.md`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/docs/team-matching.md). Provider ordering and league routing live in [`src/helpers/ProviderManager.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/helpers/ProviderManager.js).

For Dispatcharr, this should become a small Go resolver with explicit confidence levels:

```text
exact provider ID > curated alias > exact normalized field > exact compact field
> league-aware partial match > unresolved
```

Do not reproduce Game Thumbs' current `findBestTeamMatch` behavior of accepting any score greater than zero. A Silo card showing the wrong crest is worse than an honest abbreviation fallback. Require a threshold and reject ambiguous ties.

Cross-league matchups need per-side league resolution. For example, Kansas City Current can resolve under `usa.nwsl`, while Palmeiras resolves under `bra.1`, even though the event is branded “NWSL Soccer: Teal Rising Cup.” The event competition and each team's identity league must therefore be separate fields.

### Provider and caching patterns

Game Thumbs uses a provider registry and league-to-provider mapping, allowing ESPN, TheSportsDB, MLB Stats, HockeyTech, NCAA, and other adapters to return a normalized team record. Its ESPN provider caches team lists for 24 hours, caches extracted colors, and keeps negative discovery results on a shorter TTL. See [`src/providers/ESPNProvider.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/providers/ESPNProvider.js).

It also has two filesystem cache concepts:

- hashed binary/JSON cache keys with optional mtime TTL in [`src/helpers/fsCache.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/helpers/fsCache.js);
- generated-image caching by request URL with an environment-controlled TTL in [`src/helpers/imageCache.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/helpers/imageCache.js).

Dispatcharr should use the ideas, but not the exact implementation:

- Cache resolved identity separately from image bytes.
- Key identity by provider, provider team ID where known, team name, and identity league.
- Cache successful team/league identity for 24 hours and unresolved results for 10–30 minutes.
- Cache image responses by the fully normalized Game Thumbs URL plus theme/variant.
- Use stale-while-revalidate for logos already shown in the UI.
- Coalesce concurrent misses for the same key so a sports grid does not fan out identical requests.
- Bound response size, validate `Content-Type`, impose a short timeout, and never let clients choose an arbitrary upstream URL.
- Do not clear the entire image cache at process startup; that defeats restart resilience.

### Concrete integration contract

Add an optional admin configuration resembling:

```json
{
  "sports_artwork": {
    "enabled": true,
    "base_url": "https://game-thumbs.swvn.io",
    "style": 4,
    "aspect": "16-9",
    "show_league_logo": true,
    "fallback": true
  }
}
```

The backend, not browser JavaScript, should build and proxy/cache the final URL. Team names must be path-escaped after slugging, the league must come from a server-maintained allowlist or normalized provider mapping, and only `http`/`https` base URLs configured by an administrator should be accepted.

Preferred artwork chain:

```text
team.logo_url supplied by Sportarr/event payload
→ canonical Teamarr/ESPN logo URL in local identity cache
→ Game Thumbs teamlogo proxy
→ league logo
→ deterministic team abbreviation glyph
```

For a matchup hero, prefer a single Game Thumbs 16:9 image when both sides resolve. If it fails, render the existing native split-team CSS card using each independently resolved logo. This means one missing team does not erase the known team.

## Teamarr

### Data model worth mirroring

Teamarr's provider-neutral `Team` carries provider ID, provider name, full/short names, abbreviation, league, sport, logo URL, and color. `Event` carries provider identity, teams, league, sport, start time, status, scores, broadcasts, and extensive optional context. See [`teamarr/core/types.py`](https://github.com/Pharaoh-Labs/teamarr/blob/a9be5428f3795c8082674b54c57b0b63b56f9325/teamarr/core/types.py).

Its persistent identity catalogue is especially relevant:

- `team_cache`: one row per provider/team/league, including names, abbreviation, sport, and logo URL;
- `league_cache`: provider-scoped league name, sport, logo, and team count;
- `team_aliases`: user-defined alias plus league mapped to a provider/team ID;
- cache metadata and refresh state.

See [`teamarr/database/schema.sql`](https://github.com/Pharaoh-Labs/teamarr/blob/a9be5428f3795c8082674b54c57b0b63b56f9325/teamarr/database/schema.sql).

Another good pattern is backfilling degraded event payloads from the canonical team cache. Teamarr memoizes identity lookups, including misses, and fills missing name, short name, abbreviation, and logo without overwriting provider data that is already present. See [`teamarr/services/sports_data.py`](https://github.com/Pharaoh-Labs/teamarr/blob/a9be5428f3795c8082674b54c57b0b63b56f9325/teamarr/services/sports_data.py) and [`teamarr/database/team_cache.py`](https://github.com/Pharaoh-Labs/teamarr/blob/a9be5428f3795c8082674b54c57b0b63b56f9325/teamarr/database/team_cache.py).

### HTTP enrichment without a runtime dependency

Teamarr exposes useful read APIs under `/api/v1`:

- `GET /api/v1/cache/status`
- `GET /api/v1/cache/leagues? sport=&provider=`
- `GET /api/v1/cache/teams/search?q=&league=&sport=`
- `GET /api/v1/cache/leagues/{league_slug}/teams`
- `GET /api/v1/cache/team-picker-leagues`

The responses include canonical names, abbreviations, provider IDs, league/sport, and logo URLs. See [`teamarr/api/routes/cache.py`](https://github.com/Pharaoh-Labs/teamarr/blob/a9be5428f3795c8082674b54c57b0b63b56f9325/teamarr/api/routes/cache.py); router mounting is shown in [`teamarr/api/app.py`](https://github.com/Pharaoh-Labs/teamarr/blob/a9be5428f3795c8082674b54c57b0b63b56f9325/teamarr/api/app.py).

An optional adapter is therefore practical:

1. Add `teamarr_url` and `teamarr_enabled` admin settings.
2. Health-check `/api/v1/cache/status` asynchronously.
3. When local identity is incomplete, query `/cache/teams/search` with known league and sport.
4. Accept only a unique, high-confidence normalized match.
5. Store the result in Dispatcharr's own cache so cards do not call Teamarr on every render.
6. Time out quickly and fall through to Game Thumbs/local initials when Teamarr is unavailable.

Teamarr must remain optional. The plugin should not import its SQLite database, Python package, scheduler, provider clients, or schema. Direct DB sharing couples the plugin to migrations and file permissions; importing its provider layer couples the Go plugin to a Python service. Its HTTP API is the clean boundary.

The inspected app does not add an authentication middleware around these routes. A Teamarr URL should therefore default to private-network use, never be exposed through a browser-supplied arbitrary proxy target, and be protected by Silo's own authenticated backend endpoint. Also allow installations where a reverse proxy adds authentication.

## Alertle-V2

### Current alert model

Alertle's core domain is understandable and useful as a requirements reference:

- A `Subscription` selects a league, team, whole league, or event series and routes it to endpoint IDs.
- An `Endpoint` selects a provider and modes, lead time, jitter/precision, digest schedule, bundling window, content options, and Game Thumbs style.
- A `ScheduledAlert` has a deterministic ID `{game_id}:{endpoint_id}:{mode}`, UTC fire time, serialized event snapshot, sent flag, and retry count.

See [`models.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/models.py) and the YAML mapping in [`config.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/config.py).

Its scanner runs daily and on demand, looks seven days ahead, fetches ESPN schedules, fetches Dispatcharr/XMLTV EPG once per scan, matches games to channels, and hands matches to the scheduler. It treats event-series sports separately and maintains include/exclude terms for EPG matching. See [`scanner.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/scanner.py).

Alert modes include:

- lead-time reminder;
- game-start;
- final-score/game summary;
- daily digest;
- weekly digest;
- standings for event-series sports.

The scheduler stores rows in `alertle_state.db`, reloads unsent rows at startup, creates one asyncio task per alert, calculates summary time from sport-specific estimated durations, and polls ESPN every ten minutes for up to two hours before sending a final summary. It prunes orphaned future alerts after rescans. See [`scheduler.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/scheduler.py).

Notification adapters are deliberately thin:

- Discord webhook with embeds: [`notifiers/discord.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/notifiers/discord.py)
- Telegram Bot API: [`notifiers/telegram.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/notifiers/telegram.py)
- Pushover: [`notifiers/pushover.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/notifiers/pushover.py)
- ntfy: [`notifiers/ntfy.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/notifiers/ntfy.py)

It already combines alerts and artwork by building Game Thumbs URLs from league and team names, with per-endpoint style, aspect, badge, league-logo, fallback, and winner options. See [`game_thumbs/builder.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/game_thumbs/builder.py).

### Patterns to reuse independently

- Deterministic idempotency key per event, destination, and alert mode.
- Persist before arming a timer so restarts do not lose alerts.
- Store fire times in UTC and compute user schedules with an IANA timezone.
- Separate subscription, destination, content template, and scheduled-delivery records.
- Scan schedules in batches and fetch EPG once per scan.
- Reconcile scheduled rows after every scan rather than only appending.
- Let each destination choose supported modes and content density.
- Bundle games beginning within a configured window.
- Poll actual event state for final alerts instead of assuming the game ended on time.
- Keep provider adapters behind one notifier interface.
- Attach the same canonical matchup artwork used by the Silo card.

### Patterns not to copy

Alertle is explicitly described by its README as “vibe coded,” and its current implementation has production gaps that matter inside Silo:

- No repository license permits copying the code.
- FastAPI settings, scan, backup, endpoint, and test-alert routes have no visible authentication middleware in [`main.py`](https://github.com/Deekerman/Alertle-V2/blob/2ddc9399befd42d383b47c0d971af6db948a960e/main.py).
- Secrets are stored in plaintext YAML; the backup response includes Dispatcharr credentials and endpoint secrets.
- Most scheduled sends are marked sent even when a notifier returns `false`; durable retry/backoff and delivery-error state are missing.
- A single SQLite connection is shared by async tasks without an explicit serialization boundary.
- One sleeping asyncio task is created per pending alert; this does not scale as cleanly as polling a due-row queue.
- Event payloads are stored as opaque serialized snapshots, complicating migrations and deduplication.
- Jitter is called “precision,” which obscures its behavior and may send reminders early or late unexpectedly.
- Summary timing relies on estimated sport duration before polling; postponements and unusual event lengths need explicit state handling.
- Endpoint-specific notifier logic duplicates formatting and size limits.
- Public Game Thumbs URLs are embedded directly; there is no Silo-side cache/privacy boundary.

### Recommended Silo alert architecture

Alerts should be a backend feature gated by an admin setting and initially limited to events already present in the Dispatcharr sports feed.

Suggested records:

```text
sports_alert_subscriptions
  id, user_id, scope, canonical_team_id, canonical_league_id,
  modes, lead_minutes, enabled, created_at, updated_at

sports_alert_destinations
  id, owner/admin scope, provider, encrypted_secret_ref,
  provider_config, enabled, last_test_at

sports_alert_jobs
  idempotency_key, subscription_id, destination_id, event_id,
  mode, fire_at_utc, status, attempts, next_attempt_at,
  last_error, payload_version, created_at, sent_at
```

Worker behavior:

1. Reconcile future jobs whenever sports data refreshes.
2. Poll a bounded set of due jobs instead of holding one long-lived timer per alert.
3. Claim jobs atomically, send with provider-specific timeouts, and record success/failure.
4. Retry transient failures with bounded exponential backoff; never mark failed delivery as sent.
5. Re-fetch the canonical event before game-start/final notifications.
6. Audit destination creation, tests, and delivery results without logging credentials.

Initial providers should be ntfy and generic webhooks because they have small contracts and work well in self-hosted environments. Discord can follow. Telegram and Pushover require more credential UX and provider-specific constraints. Browser push should be considered separately because it requires service-worker support and a Silo-host capability contract.

The Silo plugin must not expose notification secrets to frontend state. Admin routes should store secrets using the host's supported encrypted-secret facility or references, redact them on reads, enforce authenticated admin access, validate outbound destinations against SSRF rules, and rate-limit test sends.

## Concrete next implementation slice

The next code change should stay focused on identity and artwork:

1. Add canonical `SportsLeagueIdentity` and `SportsTeamIdentity` backend structures.
2. Move league aliases and Game Thumbs league mapping out of `app.js` into backend-owned data.
3. Parse event title only as a last resort; prefer explicit Sportarr/provider IDs and home/away fields.
4. Resolve each team independently and attach identity league, logo URL, colors, and confidence to the sports API payload.
5. Add a server endpoint that returns a cached/proxied team logo or matchup image.
6. Add optional Teamarr HTTP enrichment with a health state and short failure TTL.
7. Render abbreviation fallback only after all trusted sources fail.

Alerts should begin as a separate design/implementation ticket after those identities are stable. The shared prerequisite is a durable canonical event ID plus canonical home/away identities; without that, rescan reconciliation and idempotency will be unreliable.
