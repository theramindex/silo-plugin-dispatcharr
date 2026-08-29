# Sportarr Metadata Plugin and Sports Library Brief: Transferable Learnings

Date: 2026-08-29  
Sportarr plugin snapshot: [`c25282c`](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/tree/c25282c72df9911a487c9f886fd8b92649b20b3e)  
Dispatcharr snapshot reviewed: `d721df367ab5603eecbaa5c2ad68ddc304d5894e`

## Executive answer

Yes. The Sportarr plugin demonstrates a sound provider adapter: stable provider identity, a typed artwork catalog, graceful public/private artwork fallback, rate-limit handling, and careful credential/redirect boundaries. The visual brief supplies the larger product model that the plugin does **not** implement: `sport -> league -> team -> event -> optional event part`, with Silo owning VOD-library presentation and per-user behavior.

The practical conclusion is to keep Dispatcharr's current Sports area focused on live/EPG/scores/broadcast selection, then improve its bridge to Silo's VOD library. Do not grow Dispatcharr into a second independent Sports library. The reusable library hierarchy belongs in Silo core (or a future core library-type contract), while Sportarr supplies metadata and Dispatcharr supplies live context.

## What the first-party plugin actually provides

- It is a `metadata_provider.v1`, disabled by default, with priority only for series, seasons, and episodes ([manifest](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/manifest.json#L33-L47)). Its provider advertises only `series` ([provider.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/provider.go#L32-L35)).
- Its compatibility mapping is `league -> series`, `season -> season`, and `event -> episode` ([README](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/README.md#L1-L4)). That makes existing TV-library machinery useful, but it is not a first-class sports domain model.
- The exposed response types contain league, season, and event metadata plus poster/banner/fanart/still fields ([provider/types.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/types.go#L16-L65)). There are no first-class sport, team, follow, collection, or event-part relationships in the metadata contract.
- `AgentEpisode` receives `part_name`, but `GetEpisodes` drops it when mapping to `EpisodeResult` ([types.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/types.go#L54-L65), [provider.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/provider.go#L277-L304)). Therefore the plugin cannot currently express prelim/main-card/post-show or practice/qualifying/race grouping.

## Strongest transferable learnings

### 1. Preserve Sportarr IDs end to end; match by text only as a fallback

The plugin deliberately distinguishes the durable short ID used by metadata-agent calls from `hub_id`, used for UUID-oriented image lookup ([provider.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/provider.go#L123-L155)). Dispatcharr already carries `ProviderID` and `ProviderLeagueID` on events ([sports.go](../../internal/plugin/sports.go#L98-L129)), but its Silo replay bridge currently uses team/title/date scoring ([sports_replays.js](../../internal/plugin/ui/sports_replays.js#L255-L307)). That matcher cannot reliably handle races, fight cards, stages, or renamed replays.

Recommended match order:

1. exact Sportarr event provider ID;
2. Sportarr league ID + season/round + event date;
3. exact participant pair + league + date;
4. the current normalized-title heuristic.

This requires retaining Sportarr provider IDs on enriched Silo catalog items or exposing them through the catalog query response. Until that host contract exists, keep heuristic matches visibly best-effort and never silently merge low-confidence recordings.

### 2. Adopt a typed artwork set instead of one generic event image

Sportarr models `poster`, `backdrop`, `logo`, `banner`, and `thumbnail`, including dimensions, primary status, and priority ([provider/types.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/types.go#L67-L80)). The provider sorts primary images first, then priority, and maps each type explicitly ([provider.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/provider.go#L40-L109)). It prefers entity-image artwork but falls back to public agent URLs when richer images are unavailable ([provider.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/provider.go#L209-L227)).

Dispatcharr currently collapses an event to `ImageURL` plus separate team/league logos ([sports.go](../../internal/plugin/sports.go#L98-L129)). Its event fetch also selects one image with a fixed backdrop/thumbnail/banner/poster ranking ([sports_sportarr.go](../../internal/plugin/sports_sportarr.go#L570-L644)). A small `SportsArtwork` model would let the UI choose correctly by slot:

- league browse card: logo;
- event grid card: thumbnail or backdrop;
- event detail hero: backdrop;
- VOD archive item: poster;
- compact rails: logo/banner as appropriate.

Carry width and height so layout can reserve the right aspect ratio before load. Do not display a stretched team/channel logo as faux event art; use the structured no-art layout when no suitable image exists.

### 3. Keep the core Sports library separate from Dispatcharr's live companion

The visual brief explicitly assigns hierarchy to Sportarr and library shell, playback, follows, collections, and access control to Silo ([local brief, lines 113-120](file:///Users/jonathanfinley/Developer/LeZen/docs/sports-library-visual-brief.html#L113)). It proposes Sports/Leagues browse modes, contextual teams, league/team archives, grouped VOD events, and a normal recording-grid fallback ([lines 152-164](file:///Users/jonathanfinley/Developer/LeZen/docs/sports-library-visual-brief.html#L152)). It also declares that the reusable library-type framework belongs in Silo core and that the proposal is strictly VOD, not live/EPG/scores ([lines 160-162](file:///Users/jonathanfinley/Developer/LeZen/docs/sports-library-visual-brief.html#L160)).

Dispatcharr should therefore own:

- live/upcoming/ended status and scores;
- EPG-to-event reconciliation;
- broadcast/channel matching and preferred feeds;
- the live player and related-event drawer;
- a bridge/deep link to matching Silo recordings.

Silo core should own:

- persistent Sports and Leagues library browse modes;
- team/league follows and access-aware collections;
- season/year archives;
- event pages grouping multiple VOD recordings;
- playback progress and library item actions.

Dispatcharr already queries user-accessible libraries and restricts selected replay libraries accordingly ([app.js](../../internal/plugin/ui/app.js#L2707-L2752)). Preserve that boundary. Avoid duplicating a second long-lived VOD catalog or a separate follow system once core follows exist.

### 4. Event parts and teams are the key missing contract, not another visual redesign

The brief's highest-value screens depend on first-class relationships: league detail brings teams forward; event detail groups pre-show/main event/post-show; and a Grand Prix groups practice, qualifying, sprint, and race ([lines 115-120](file:///Users/jonathanfinley/Developer/LeZen/docs/sports-library-visual-brief.html#L115)). The proposed minimum metadata contract is sport, league, team, event, and optional event-part relationships ([lines 168-175](file:///Users/jonathanfinley/Developer/LeZen/docs/sports-library-visual-brief.html#L168)).

The first-party metadata plugin does not currently expose teams or parts, even though its upstream episode type sees `part_name`. Dispatcharr's public-event adapter also receives `Parts []any` but does not carry it into `SportsEvent` ([sports_sportarr.go](../../internal/plugin/sports_sportarr.go#L89-L115), [conversion](../../internal/plugin/sports_sportarr.go#L360-L410)).

The next cross-repo contract should add typed entities such as:

```text
Sport { id, name, artwork }
League { id, sportId, name, artwork }
Team { id, leagueIds, names/aliases, colors, artwork }
Event { id, leagueId, seasonId, participants, date, venue, artwork }
EventPart { id, eventId, kind, title, order, providerIds }
```

That unlocks the brief directly. More logo fallbacks and title parsing improve symptoms but do not create the missing hierarchy.

### 5. Copy the provider safety model, but keep Dispatcharr's stronger bounded cache behavior

The metadata plugin rate-limits requests, retries 429/5xx responses, honors `Retry-After`, caps response bodies, and prevents JSON redirects from leaking API keys ([client.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/client.go#L19-L45), [request loop](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/client.go#L102-L189)). Its image resolver accepts only public HTTPS redirect targets and rejects non-routable addresses ([client.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/client.go#L284-L359)). Configuration also refuses to send a self-hosted API key to the default public hub ([main.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/main.go#L135-L166)). Adopt these rules if Dispatcharr gains a configurable/self-hosted Sportarr endpoint.

Do **not** copy the metadata plugin's unbounded one-goroutine-per-ID image batch ([client.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/client.go#L247-L281)). Dispatcharr already has bounded enrichment workers, request coalescing, capped caches, 24-hour positive image caching, and 15-minute negative image caching ([sports_sportarr.go](../../internal/plugin/sports_sportarr.go#L471-L568), [image cache](../../internal/plugin/sports_sportarr.go#L570-L608), [limits](../../internal/plugin/sports_sportarr.go#L17-L26)). Keep those mechanisms and add a global token bucket plus stale-while-revalidate behavior if source limits remain visible.

## Recommended sequence

1. **Contract spike:** verify whether Silo catalog results expose Sportarr provider IDs. If not, add that host capability before changing replay matching.
2. **Typed artwork:** extend the Dispatcharr sports payload with slot-aware artwork and dimensions while retaining `ImageURL` for compatibility.
3. **Typed event parts:** decode Sportarr `parts`, preserve `part_name`, and group matched recordings on event detail pages.
4. **ID-first replay bridge:** move matching behind a backend boundary or shared core service; retain the current title matcher only as fallback.
5. **Core proposal:** use the visual brief as the Silo-core feature request for a library-type/entity-relationship contract. Keep Dispatcharr's UI live-first and deep-link into the core VOD experience.

## Risks and constraints

- The Sportarr metadata plugin is evidence of a TV-series compatibility adapter, not evidence that Silo already supports an extensible Sports library type.
- Rich entity images may depend on a configured/self-hosted API key; the public agent artwork is intentionally a reduced fallback ([provider.go](https://github.com/Silo-Server/silo-plugin-metadata-sportarr/blob/c25282c72df9911a487c9f886fd8b92649b20b3e/provider/provider.go#L307-L359)).
- `hub_id` and public short IDs serve different APIs. Persisting the wrong one will break either metadata refresh or image lookup.
- Text/date matching will remain ambiguous for replays, highlights, races, stages, and multi-part cards until provider IDs and event-part IDs reach the Silo catalog.
- Both repositories are AGPL-3.0-or-later, but substantial copied code should still retain the original notices and attribution. The recommendations above are architectural learnings, not copied implementation.

