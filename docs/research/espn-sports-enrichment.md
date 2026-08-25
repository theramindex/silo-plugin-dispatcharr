# ESPN as a sports enrichment source

Research date: 2026-08-25

Source revisions inspected:

- Dispatcharr plugin at [`e7a028a`](https://github.com/theramindex/silo-plugin-dispatcharr/tree/e7a028a51ece0c203e14c2adb258bc228bc16943)
- Game Thumbs at [`433d62d`](https://github.com/sethwv/game-thumbs/tree/433d62d716f254073734c7abf52404c042d9c572)

## Recommendation

Do **not** add a direct ESPN production provider under the current public terms. ESPN's live JSON endpoints are rich and currently work without an API key, but ESPN does not publish a current public contract, schema, quota, SLA, or deprecation policy for them. More importantly, ESPN directs users to the Disney Terms of Use, which cover ESPN-branded products and restrict automated extraction, database compilation, and commercial or business use without express written permission.

Keep the existing architecture:

1. Sportarr owns event identity, schedule, status, scores, and provider IDs.
2. Sportarr detail responses fill league/team identity, colors, logos, and event art.
3. Game Thumbs supplies missing visual identity for recognized leagues/teams.
4. Local glyphs/abbreviations remain the honest final fallback.

Game Thumbs already uses ESPN internally for many league/team resolutions. Therefore the plugin already benefits indirectly when a Game Thumbs image URL resolves, without acquiring a second schedule model or embedding ESPN's internal endpoint contract.

If ESPN/Disney later grants written API and asset-use permission, add ESPN only as an optional, server-side **identity gap filler after Sportarr and before Game Thumbs**. It should never replace Sportarr's event schedule or score authority.

## What the endpoints expose today

These are observations of live ESPN-owned endpoints, not a documented API guarantee.

| Need | Observed ESPN surface | Useful fields |
| --- | --- | --- |
| Sport and league discovery | Core collections such as `/v2/sports` and `/v2/sports/{sport}/leagues` | Sport/league slugs and `$ref` links |
| League identity | [`sports.core.api.espn.com/v2/sports/football/leagues/nfl`](https://sports.core.api.espn.com/v2/sports/football/leagues/nfl) | ESPN league ID/UID, name, abbreviation, slug, color, season/calendar links, light/dark logos |
| Teams | [`site.api.espn.com/.../football/nfl/teams`](https://site.api.espn.com/apis/site/v2/sports/football/nfl/teams) and [`.../soccer/eng.1/teams`](https://site.api.espn.com/apis/site/v2/sports/soccer/eng.1/teams) | Team ID/UID, slug, location, nickname/display names, abbreviation, colors, light/dark logos, group/venue references |
| Schedule and scores | [`.../football/nfl/scoreboard`](https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard), [`.../hockey/nhl/scoreboard`](https://site.api.espn.com/apis/site/v2/sports/hockey/nhl/scoreboard), [`.../baseball/mlb/scoreboard`](https://site.api.espn.com/apis/site/v2/sports/baseball/mlb/scoreboard) | Event ID/UID, date, names, season/week, competitors, scores/linescores, status/clock, venue, broadcasts, records, leaders and ESPN links |
| Team schedule | [`.../basketball/nba/teams/13/schedule`](https://site.api.espn.com/apis/site/v2/sports/basketball/nba/teams/13/schedule) | Team-specific events and status |
| Event detail | Site API `summary?event={eventId}` and linked Core resources | Box score, plays and other game detail; this surface is even more coupled to ESPN's internal product model |

The strongest incremental value over Sportarr is team identity: durable provider-local IDs, aliases/display names, abbreviations, logos, and colors. The schedule/status/score fields mostly duplicate Sportarr and would introduce reconciliation problems rather than a clear product gain.

## Stability, authentication, rate limits, and terms

- The observed Site/Core requests currently return JSON without an API key or signed-in ESPN session. That is an implementation observation, not an authorization grant.
- No current ESPN-owned public developer documentation was found for these endpoints. There is no published schema/version promise, quota, rate-limit behavior, SLA, changelog, or deprecation policy.
- ESPN payloads link deeply through `$ref` URLs and vary by sport. Individual/team, pseudo-team, tournament, and combat-sport shapes cannot be assumed to be uniform.
- Disney states that its products evolve, may change, and may be discontinued ([Terms section 3A](https://disneytermsofuse.com/english/)).
- ESPN Support points users to the [Disney Terms of Use](https://support.espn.com/hc/en-us/articles/360035445091-Terms-of-Use). Those terms expressly include ESPN-branded products, grant a personal noncommercial consumer license, and—absent written permission—restrict commercial/business use and automated access, monitoring, copying, extraction, scraping, and compilation of datasets/databases ([sections 2A–2B](https://disneytermsofuse.com/english/)).
- The terms identify images, artwork, code, and other product elements as Disney or licensor intellectual property. ESPN/team/league marks also carry independent trademark and league/team licensing concerns.

Accordingly, unauthenticated access does not make this a supported public API. A production plugin that polls and caches these responses should require written ESPN/Disney permission and legal review, including explicit rights for scores/data and CDN-hosted marks.

## What Game Thumbs demonstrates

Game Thumbs' commit-pinned [`ESPNProvider.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/providers/ESPNProvider.js) is a useful implementation study, not evidence that ESPN authorizes downstream use.

It:

- maps each configured league to an explicit ESPN `sport` and `league` slug;
- fetches bulk team lists from the Site API and league metadata from the Core API;
- normalizes a team into ESPN ID/slug, full and short names, city, nickname, abbreviation, conference/division, light/dark logo, and primary/alternate colors;
- uses curated aliases and weighted normalized-name matching;
- extracts colors from the logo only when ESPN omits them;
- stores successful team/league data and extracted colors for 24 hours;
- versions the cached team shape so deployments can invalidate incompatible records;
- uses a 15-minute negative cache for failed All-Star discovery so transient timeouts/rate limits self-heal;
- skips ambiguous All-Star collections instead of guessing;
- treats individual team failures independently and wraps requests in timeouts.

Its [`ProviderManager.js`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/src/helpers/ProviderManager.js) also provides the right structural lesson: explicit per-league provider ordering, provider-local normalization, fallback to the next provider, then controlled feeder-league lookup. Game Thumbs' [`leagues.json`](https://github.com/sethwv/game-thumbs/blob/433d62d716f254073734c7abf52404c042d9c572/leagues.json) is the source of its ESPN sport/league mappings.

Important failure modes visible in that implementation are rate limits/timeouts during fan-out discovery, missing/inconsistent colors, pseudo-teams omitted from bulk lists, historical teams with no reliable "current" marker, different group shapes, and false positive name matches. These reinforce that ESPN must be isolated behind a fallible provider boundary.

## Exact placement if ESPN is licensed later

The current plugin first normalizes events and applies Game Thumbs URLs in [`sports.go`](https://github.com/theramindex/silo-plugin-dispatcharr/blob/e7a028a51ece0c203e14c2adb258bc228bc16943/internal/plugin/sports.go), then overlays cached Sportarr detail data asynchronously through [`sports_sportarr.go`](https://github.com/theramindex/silo-plugin-dispatcharr/blob/e7a028a51ece0c203e14c2adb258bc228bc16943/internal/plugin/sports_sportarr.go). League routing and final artwork URLs live in [`sports_identity.go`](https://github.com/theramindex/silo-plugin-dispatcharr/blob/e7a028a51ece0c203e14c2adb258bc228bc16943/internal/plugin/sports_identity.go).

A licensed provider should make the precedence explicit per field:

```text
Sportarr event + Sportarr cached details
  -> licensed ESPN identity fill (only empty team/league identity fields)
  -> Game Thumbs artwork fill (only empty logo/art fields)
  -> local glyph/abbreviation fallback
```

Implementation constraints:

- Maintain an allowlisted `Sportarr league ID -> ESPN sport/league slug` mapping. Do not probe arbitrary league paths from browser input.
- Match a team within the mapped league using, in order: an already persisted provider crosswalk, exact ESPN ID if Sportarr exposes one, exact normalized alternate name/abbreviation, then a high-confidence unique name match. Reject ties.
- Keep ESPN IDs namespaced as `espn:{sport}:{league}:{teamID}` and never replace the `sportarr:` event ID.
- Fill only missing `LogoURL`, `PrimaryColor`, `SecondaryColor`, abbreviation, and carefully vetted alternate identity. Do not overwrite nonempty Sportarr names, scores, status, start time, venue, or channel matching.
- Re-run visual fallback after cached Sportarr/ESPN detail application so a placeholder URL cannot outrank newly available authoritative metadata.
- If an ESPN event is ever used solely to corroborate identity, require both teams, mapped league, and a narrow start-time window. Never create a browseable event from ESPN alone.

## Caching and failure policy if licensed

- Fetch server-side with a short deadline, bounded response size, HTTPS-only URLs, strict JSON/content-type checks, and per-host concurrency limits.
- Coalesce identical in-flight league/team requests.
- Cache stable league/team identity for 24 hours; cache event corroboration only for minutes, not as a second schedule database.
- Cache unresolved/ambiguous matches and 404s for 10–30 minutes; do not poison the 24-hour positive cache.
- Honor `Retry-After` when present and back off on 429/5xx. Because no quota is published, operate conservatively and disable probes after repeated failures.
- Serve stale known-good identity while revalidating. ESPN failure must never remove Sportarr data, hide a playable Sportarr event, or fail `/dispatcharr/api/sports`.
- Bound caches and version their stored shape. Keep provenance and `fetched_at` on every resolved field so ESPN can be disabled or purged cleanly.
- Proxy/cache permitted artwork rather than hotlinking from the browser only if the license expressly allows it; otherwise omit it.

## Decision

ESPN is technically valuable but not currently suitable as a direct production dependency. The immediate path is to retain Sportarr-first data plus Game Thumbs visual fallbacks. Revisit a direct identity adapter only after written ESPN/Disney permission establishes an API contract, quotas, permitted caching, attribution, and logo/art rights.
