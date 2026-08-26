# ScoreBox and Flicka patterns for Dispatcharr sports detail

Research date: 2026-08-25

## Research question and evidence standard

Which current ScoreBox and Flicka features could improve Dispatcharr's sports detail experience, and which ideas can be built from the data Dispatcharr already receives?

This note uses only first-party sources:

- the official [ScoreBox Google Play listing](https://play.google.com/store/apps/details?id=net.darkforeststudios.scorebox);
- the listing's linked [ScoreBox product and FAQ site](https://scoreboxiptv.com/);
- the official [Flicka site](https://www.flickatv.com/);
- Dispatcharr's own current sports contracts at revision [`d53a215`](https://github.com/theramindex/silo-plugin-dispatcharr/tree/d53a2154d04651c4ebc97fa880ca5fef11a99f20).

Product claims are not treated equally. ScoreBox is a released paid Android TV product. Flicka says its beta is still in preparation and invites early testers, so its catalogue is evidence of a stated design, not proof of shipped behavior. Flicka items labeled “Futuro inteligente” are roadmap concepts and are separated from its main feature claims.

## Executive recommendation

Redesign the Dispatcharr page as an **event hub**, not a large scoreboard. Its first job should remain “watch this game”; its second job should be “understand what is happening.” The strongest transferable pattern is:

1. a compact matchup hero with score, state, clock/status, venue/round when known, and one primary watch action;
2. a ranked, grouped feed selector that remembers a preferred channel and remains available from the player;
3. a small live-summary strip using only provider-backed facts;
4. schedule and related-event navigation organized around followed teams/leagues;
5. spoiler controls and accessible TV/remote focus behavior;
6. optional multiview later, after stream connection limits and recovery behavior are designed.

Do not fill the page with invented possession, shots, player leaders, standings, lineups, or play-by-play. Dispatcharr's present Sportarr integration does not expose those fields. Deep stats should be a separately scoped, licensed-provider integration with a stable event-ID mapping, explicit attribution rights, caching, rate-limit handling, and honest unavailable states.

## Verified ScoreBox feature set

ScoreBox's core promise is event-first discovery: connect one or more M3U/Xtream playlists, follow leagues and teams, see live/up-next games, match each event to the user's own channels, and play inside the app. Its official site says it combines schedules, per-game listings where available, country-level network guidance, provider EPG, and channel names; it labels curated rights-holder guidance as a best guess rather than a carriage guarantee. See the [Google Play description](https://play.google.com/store/apps/details?id=net.darkforeststudios.scorebox) and [matching FAQ](https://scoreboxiptv.com/).

### Sports detail and schedule

- Live score, clock/time remaining, status, matchup, and available broadcasts remain visible on Home, schedule, and the player overlay.
- Followed live games rise above upcoming games.
- ScoreBox advertises schedules across 57 leagues and 13 sports, with country-aware ordering/filtering.
- Spoiler-free mode can hide scores, progress, and results for followed teams or globally.

This is a concise score layer, not a deep box score. The official sources do **not** claim standings, lineups, play-by-play, player/team box-score statistics, injuries, odds, shot maps, or possession.

### Channel discovery and live viewing

- Each matchup points to channels in the user's playlists, with the best match first.
- Main, backup, home, away, alternate, and event-specific feeds can stay grouped with the matchup.
- The player can switch between all feeds for the same event without returning to the full channel list.
- Users can pin the channel they prefer for a network and reuse that choice later.
- Provider categories remain available for ordinary channel/network browsing.
- The built-in player supports full-screen playback; the current Play release also advertises in-place stream recovery, lower-latency startup, and visibility into provider connection usage.
- Multiview supports up to four streams, grid or large-plus-column layouts, selectable audio, and—according to the official product site—a score per pane.

### Personalization and TV UX

- Users can follow leagues and teams; the official site also presents network/channel-oriented navigation.
- Providers, follows, and pins are restored through the user's Google account rather than a ScoreBox account.
- A Resume surface returns to the last game/channel, and the user can select a starting surface.
- The interface is described as Android-TV/D-pad-first with large, visible focus states.

No sports push, game-start, score-change, or final-result alert system is verified in the official ScoreBox sources.

## Flicka's stated feature catalogue

Flicka's official site says “Beta en preparación.” The following are first-party design claims, but should not be represented as independently verified shipped behavior.

### IPTV patterns worth borrowing

- Equivalent channels from several M3U, Xtream, or Stalker sources are compacted into one dial/card.
- Copies are ranked by quality, can be merged or split manually, and can fail over to a backup source.
- A connection governor counts active connections per provider and coordinates playback, PiP, multiview, recording, and preloading.
- PiP preserves two live streams, supports main/window swap and selectable audio, and stays visible while navigating.
- Multiview offers equal grid, focus-plus-side, and focus-plus-bottom layouts and saved sessions.
- A remote-oriented player OSD exposes EPG, more channels, audio, subtitles, quality, information, and time controls.
- The EPG includes a current-time line, program detail, past guide, and provider-source selection.
- Unified search spans channels, programs, movies, and series; background refresh and caching avoid blocking the home screen.
- Profiles retain favorites, history, source/list choice, startup screen/channel, and preferences.
- A football picture preset prioritizes motion handling; format and quality claims are explicitly conditional on device, platform, and stream support.

The site visually illustrates a score/time inside PiP, but it does not claim a sports-data subsystem. That mock content is not evidence that Flicka supplies live scores or game statistics. Flicka also does not claim league schedules, standings, lineups, play-by-play, box scores, or sports alerts.

### Roadmap-only Flicka concepts

Flicka explicitly labels the following “Futuro inteligente,” so they are inspiration rather than current feature parity:

- natural-language game discovery;
- correction of inaccurate EPG timing and last-minute changes;
- grouping one event across multiple channels with spoiler avoidance;
- proactive recordings that extend for overtime and warn about conflicts;
- smart match-day multiview that avoids duplicate events and respects connection budgets;
- AI-assisted plans that arrange PiP, reserve a connection, or schedule a recording after user review.

## What Dispatcharr has today

The current API contract exposes league identity/artwork/description, sport, event name/type/season/round/venue, schedule and optional end time, status/status text, home and away team IDs/names/abbreviations/logos/colors, home and away score, live/completed flags, and matched channels with a reason and confidence score. See [`SportsLeague`, `SportsTeam`, `SportsEvent`, and `SportsChannelMatch`](https://github.com/theramindex/silo-plugin-dispatcharr/blob/d53a2154d04651c4ebc97fa880ca5fef11a99f20/internal/plugin/sports.go#L59).

Sportarr's event response also includes `parts`, but the current adapter models them as untyped values and does not expose them to the browser. The adapter currently maps only the fields listed above. See the [Sportarr response types and mapping](https://github.com/theramindex/silo-plugin-dispatcharr/blob/d53a2154d04651c4ebc97fa880ca5fef11a99f20/internal/plugin/sports_sportarr.go#L72).

EPG can corroborate program name, current/up-next timing, description, and which channel appears to carry an event. It is not a trustworthy source for scores, clocks, lineups, play-by-play, standings, or box-score statistics.

## Feature feasibility and data requirements

| Recommended idea | Data and system requirements | Current-input fit | Main caveat |
| --- | --- | --- | --- |
| Compact event hero | Teams, logos/colors, score, state, start time, league, venue/round | **Supported now** by Sportarr fields, with honest omission of missing values | Status text may be only `Live`/`Final`; do not manufacture a game clock |
| One primary watch action | Ranked channel matches and stream route | **Supported now** | A single match should be a direct link; multiple matches need an anchored chooser |
| Ranked broadcast drawer | Channel name/logo/category, match reason/score | **Supported now** | Confidence score is not the same as confirmed carriage |
| Preferred feed memory | Persist event/network-to-channel preference; fallback if unavailable | **Mostly supported** with existing channel IDs plus new preference state | Network identity is not canonical today; pinning by raw channel name is fragile |
| Feed labels: home/away/national/backup | Canonical broadcast metadata or high-confidence naming rules | **Partial/inference only** | Current channel reason/name may suggest a label but cannot guarantee it; show “matched channel” unless confirmed |
| Live score overlay | Score and event state delivered during playback | **Supported at current refresh cadence** | A true clock/time-remaining needs provider data and faster polling/push |
| Schedule date rail and related games | Start time, league/team IDs, status | **Supported now** | Completeness depends on Sportarr/provider coverage |
| Followed-team/league prioritization | Stable team/league IDs plus stored preferences | **Supported now**; favorite team IDs already exist | Avoid hiding other live events; offer a clear “All” view |
| Spoiler mode | Display preference applied to score, state, progress, final labels | **Supported now** without new sports data | Must cover cards, detail, player overlay, titles, and accessible labels consistently |
| Pregame reminder | Start time, stable event ID, timezone, persisted scheduler, notification destination | **Sports data supported now; delivery subsystem missing** | Browser/OS permission, restarts, schedule changes, privacy, and duplicate suppression require backend design |
| Score/final alerts | Fresh event state, durable subscriptions, idempotency, destination | **Partial**; current score/status exists but may not meet alert timeliness/reliability | Needs a documented refresh SLA or a stable live-data provider before promises are made |
| PiP | Two playable streams, player/browser support, audio/focus controls | **No new sports data needed; substantial player work** | Device/browser support, decoder limits, accessibility, and source connection counts |
| Multiview | 2–4 streams, layout/session state, audio selection, connection budget | **No new sports data needed; substantial player/backend work** | Do not ship without provider connection-limit awareness and recovery behavior |
| Cross-source dedupe/failover | Canonical channel/network identity, quality/health telemetry, provider connection limits | **Not supported by present sports payload** | Requires backend stream-health and source-capability work; channel-match rank is not stream quality |
| Team/game summary statistics | Sport-specific stat schema and stable event mapping | **Not supported** | Requires a stable licensed data source; schemas differ by sport |
| Standings | Competition, season, stage/conference, record/tiebreak data | **Not supported** | Requires historical/season data and league-specific rules |
| Lineups/rosters/injuries | Player identities, roles, starter status, change timestamps | **Not supported** | High update frequency, privacy/trademark/photo rights, and sport-specific semantics |
| Play-by-play/timeline | Ordered event feed, clock/period, participants, corrections | **Not supported** | Real-time polling/streaming, corrections, latency, attribution, and significant API cost |

## Proposed sports-detail information architecture

The current page has enough room to become more useful without becoming a generic scores portal:

1. **Hero:** league, event state, score, teams, start/clock text, compact context such as round and venue, then `Watch live`.
2. **Live summary:** only verified provider facts—period/clock if eventually supplied, otherwise `Live`; no estimated clock.
3. **Where to watch:** a single direct channel or ranked grouped alternatives, preferred feed first, with the EPG's current program as corroboration.
4. **Event navigation:** previous/next game for the same league, followed teams first, without forcing the user back to the sports grid.
5. **Optional secondary tabs:** `Overview`, `Stats`, `Lineups`, and `Play-by-play` should appear only when a future data provider explicitly declares those capabilities for this event. Never render empty ornamental tabs.

For wide screens, use a two-column detail layout: event narrative on the left and a sticky `Watch`/broadcast rail on the right. On small screens and TV focus navigation, preserve a single logical order: matchup → watch → broadcasts → related games. Team color should be an accent, not the only status/identity signal.

## Accessibility and inclusive-design requirements

- Keep the primary watch action early in DOM and focus order.
- Preserve large, high-contrast focus rings for remote/keyboard operation; do not rely on scale alone.
- Announce score changes politely and avoid repeatedly interrupting screen readers with every polling refresh.
- Express score, possession/home-away role, live/final state, and selected audio in text, not color alone.
- Offer spoiler hiding consistently to visual users and assistive technology; hidden scores must not remain in `aria-label` text.
- Provide readable team names alongside crests and abbreviations.
- Make PiP/multiview pane selection, audio ownership, swapping, and closing operable without pointer gestures.
- Honor reduced-motion preferences for live-score transitions and focus movement.

## Data, licensing, and API risks

### Sports data

Neither ScoreBox source names its score/schedule/broadcast vendor, license, attribution terms, update cadence, or SLA. Its official site says reliable schedule/score sources determine league support and that some rights-holder guidance is only a best guess. Dispatcharr should not infer that ScoreBox's coverage can be reproduced with an undocumented endpoint.

Deep statistics require more than a technically reachable API. Before integration, obtain written answers for:

- commercial/self-hosted display rights and whether cached data may be shown in Silo;
- required attribution and logo/team/player-photo rights;
- allowed polling rate, redistribution/proxy restrictions, and retention limits;
- coverage by league, region, preseason/postseason, and women's/college competitions;
- event-ID stability, correction semantics, latency target, outage policy, and deprecation/versioning;
- price at the concurrency and polling rate needed for live use.

Unofficial or reverse-engineered sports endpoints are unsuitable as the sole source for promised live alerts or statistics: they can change without notice, lack an SLA, and may prohibit reuse. If used experimentally, keep them behind a provider interface, server-side cache, feature flag, explicit attribution configuration, and graceful “data unavailable” state.

### Streams and marks

ScoreBox states that it supplies no channels, hosts no streams, and requires users to provide content they are entitled to access. Flicka likewise describes itself as a player for user-supplied legal lists. Dispatcharr should preserve that boundary and avoid presenting a fuzzy channel match as a rights or availability guarantee.

League/team/broadcaster names and marks remain their owners' property. Software implementation ideas are reusable, but artwork and branding need provider terms or direct URLs with appropriate caching/attribution rules. Flicka separately states that Dolby/DTS marks are not implied certifications and that format support depends on device, platform, and stream; Dispatcharr should use the same restraint for video-quality badges.

## Delivery order

1. Redesign the detail hierarchy using existing Sportarr/EPG fields and capability-based omission.
2. Add preferred-feed memory, spoiler mode, and in-player feed switching.
3. Add schedule navigation and pregame reminders backed by durable state.
4. Instrument channel-selection success, match-confidence overrides, playback failures, and reminder delivery before adding automation.
5. Design connection budgeting and recovery before PiP/multiview.
6. Evaluate licensed sports-data providers with a small multi-sport proof of coverage; add stats/standings/lineups/play-by-play only after the rights, freshness, and event-mapping gates pass.

The immediate redesign can be meaningfully better without new sports data. The honest near-term differentiator is not “more numbers”; it is faster movement from event to the correct, reliable stream with enough live context to stay oriented.
