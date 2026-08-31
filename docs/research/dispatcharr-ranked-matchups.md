# `dispatcharr_ranked_matchups`: transferable ideas for Dispatcharr for Silo

Research snapshot: 2026-08-31. Upstream reviewed at [`e71a7ae`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/tree/e71a7aedfc6f98530bebba8ba39cdf32e8611e31); this plugin was compared at local commit `b1fb16b`.

## Executive conclusion

The upstream project is not an alternative metadata provider or a replacement UI. It is a **Dispatcharr-side curation and lineup mutation pipeline**: fetch fixtures from many APIs, calculate an explainable “interestingness” score, match those fixtures to EPG/channels/raw streams, then create temporary virtual channels with generated EPG rows and artwork. Its best ideas for this plugin are the normalized source contract, explainable ranking, evidence-tiered matching, and durable image-cache behavior. Its Django ORM integration, virtual-channel churn, scheduler implementation, LLM fallback, and provider-specific Python code should not be transplanted.

Our plugin has a different and safer boundary: a compiled Silo plugin with declared scheduled-task and HTTP-route capabilities ([`manifest.json`](../../manifest.json#L20-L32)); it merges Sportarr events with the cached guide and ranks matches to existing channels ([`sports.go`](../../internal/plugin/sports.go#L295-L328)). The right adaptation is therefore **a “Top games” presentation/curation layer over the existing `SportsEvent` model**, not a second system that rewrites Dispatcharr’s database.

## Architecture and runtime model

### Upstream

The pipeline has two explicit phases:

1. `refresh` builds enabled source adapters, fetches each independently, computes scores and channel matches, and writes `cache.json`; source failures are isolated so one feed does not stop the slate ([`plugin.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.py#L2457-L2495)).
2. `apply` reads the plan and uses Dispatcharr’s private Django models to create virtual `Channel`, `ChannelStream`, `EPGSource`, `EPGData`, and `ProgramData` records; source channels are left alone and stale owned virtual channels are deleted ([`plugin.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.py#L4099-L4118)). The README summarizes the write boundary explicitly ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L218-L228)).

CPU-heavy work and bulk writes do not run inside Dispatcharr’s gevent/uWSGI request worker. A daemon thread supervises a fresh Python subprocess because the Monte Carlo work holds the GIL and previously froze login/live-stream handling ([`tasks.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/tasks.py#L1-L27), [`_pipeline_runner.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/_pipeline_runner.py#L1-L20)). Redis supplies cross-worker inflight state and a destructive-work mutex; a 25-minute child timeout is kept below the 30-minute lock TTL ([`tasks.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/tasks.py#L72-L95), [`tasks.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/tasks.py#L196-L215)). Network-backed values are resolved before `transaction.atomic()` and EPG caches are invalidated only after commit ([`plugin.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.py#L4671-L4685), [`plugin.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.py#L5208-L5215)).

### Ours

Our process boundary is already cleaner. `SportsProvider` is a small Go interface, with optional event-enrichment and league-roster capabilities ([`sports.go`](../../internal/plugin/sports.go#L20-L31)). The model already separates league, team, artwork, event, and channel-match data ([`sports.go`](../../internal/plugin/sports.go#L67-L154)). Sportarr access is bounded by pagination, concurrency, retries, timeouts, caches, and single-flight maps ([`sports_sportarr.go`](../../internal/plugin/sports_sportarr.go#L17-L41), [`sports_sportarr.go`](../../internal/plugin/sports_sportarr.go#L188-L226)).

**Implication:** keep ranking/matching as pure Go transforms over `[]SportsEvent`, within the Silo plugin’s declared APIs. Do not add direct Postgres/Django coupling or let a request handler own CPU-heavy simulation.

## Metadata sources and data model

Upstream aggregates many first-party and unofficial feeds: CFBD/CBBData, Football-Data.org, official-but-undocumented NHL and MLB endpoints, unofficial ESPN feeds for NBA/MLS/NCAA/cups/friendlies/field events, The Odds API, and a RapidAPI boxing feed ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L97-L114)). Each adapter emits a sport-agnostic `GameRow` with teams, ranks, time, venue, spread/closeness, rivalry and extensible `extra` metadata ([`sources/base.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/sources/base.py#L61-L111)). Simulation is an explicit optional capability rather than assumed for every provider ([`sources/base.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/sources/base.py#L129-L173)).

The strongest contract lesson is the required stable `extra["game_id"]` for any simulated source. The source documents how falling back to `(home, away, start_time)` silently double-counted rescheduled or undated games, producing wrong importance scores ([`sources/base.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/sources/base.py#L83-L110)).

Our `SportsEvent` is richer for presentation—provider IDs, event type, season/round, status/clock/scores, structured artwork and matched channels ([`sports.go`](../../internal/plugin/sports.go#L104-L154))—and Sportarr remains the appropriate canonical fixture/identity source. If ranking gains optional odds/rank signals, expose them as typed optional fields/capabilities; do not encode provider-specific assumptions in the core event matcher.

## Parsing, matching, and ranking

### Interestingness ranking

Upstream’s score is deliberately explainable: rank strength, favorites, closeness, tournament stage, rivalry, optional narrative, and Monte Carlo standings importance each contribute visible points ([`scoring.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/scoring.py#L1-L17)). Default weights put importance first and leave LLM narrative disabled ([`scoring.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/scoring.py#L56-L85)). `score_game` retains a per-signal breakdown and notes, then compresses raw points to a bounded 0–10 score ([`scoring.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/scoring.py#L1187-L1310)); larger batches can use median-relative compression to avoid early-season low-score and late-season saturation ([`scoring.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/scoring.py#L1019-L1051)).

The Monte Carlo importance calculation is materially more complex: it samples seasons and measures how the target result changes title/playoff/relegation outcomes, including effects on favorite teams not playing in the match ([`scoring.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/scoring.py#L1313-L1344)). This is valuable editorial logic, but too costly and provider-specific for a first Silo adaptation.

### Broadcast matching

Upstream builds candidates from three evidence paths: EPG title/subtitle/description in a broadcast window, channel name, and raw stream name. Raw-stream hits attach only the matching stream, not every feed on its parent channel ([`matcher.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/matcher.py#L1-L23), [`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L266-L279)). Resolution uses confidence tiers:

- channel/stream name containing both teams: highest-confidence, stack deduplicated same-fixture variants;
- exactly one non-preview program containing both teams: accept;
- otherwise optionally batch ambiguous candidates to Claude; with no key, choose the first candidate ([`matcher.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/matcher.py#L860-L965), [`matcher.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/matcher.py#L966-L1027)).

Its false-positive defenses are especially reusable: strong vs weak team aliases, dropping generic/numeric last-word fallbacks ([`matcher.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/matcher.py#L77-L145), [`matcher.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/matcher.py#L200-L216)); requiring both sides in one provider-label segment; whole-word handling for short abbreviations; and rejecting streams explicitly dated before the fixture ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L316-L338)).

Our matcher already has a good foundation: team names/abbreviations/event titles receive explicit weights, league names are excluded as overly broad terms, abbreviations require league context, and results include human-readable reasons ([`sports.go`](../../internal/plugin/sports.go#L1108-L1162), [`sports.go`](../../internal/plugin/sports.go#L1182-L1234)). It also requires structural evidence or a strong guide match and constrains EPG programs to a time window ([`sports.go`](../../internal/plugin/sports.go#L1205-L1281)).

**Best adaptation:** add an evidence class/confidence to the existing `SportsChannelMatch`, then port the upstream *tests and concepts* for same-segment both-team matching, preview/ancillary demotion, stale date filtering, and strong/weak aliases. Do not use `fallback_first`; an ambiguous false positive is worse than an honest unmatched event. LLM resolution should remain optional and off by default if ever introduced, because it exports fixture and lineup candidate metadata ([`matcher.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/matcher.py#L966-L986)).

## Channel/event selection behavior

Upstream creates a bounded curated group, supports a reserve “bench,” removes completed games, and can create high-scoring placeholders before a broadcast match appears ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L218-L228), [`plugin.json`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.json#L638-L649)). Matched feeds are ordered by quality and optional language/group policy; user-curated channel streams lead raw-M3U matches ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L281-L307)). Virtual channel numbers can be kickoff-derived and stable because Dispatcharr clients bind EPG by integer channel number ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L201-L216)).

For Silo, reuse the **editorial behavior**, not the channel mutation: a ranked “Top games” shelf, explicit “why,” favorites boost, live-first ordering, and a bench to keep the shelf populated. Existing event-to-channel matches should remain playback targets. Stream fallback ordering belongs in Dispatcharr/source configuration unless Silo’s API exposes distinct, authorized variants.

## Logos and artwork

Upstream’s fallback chain is TheSportsDB matchup graphic → optional game-thumbs composite → league/tournament badge → matched source-channel logo ([`plugin.json`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.json#L652-L664)). The important engineering is not the particular vendor but the cache discipline:

- download once to local durable storage rather than serving volatile remote URLs ([`logos.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/logos.py#L1-L16));
- positive and negative TTLs differ because artwork can appear late ([`logos.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/logos.py#L58-L66));
- validate file signatures and atomically replace a temp file so HTML/error bodies or partial downloads never become permanent logos ([`logos.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/logos.py#L390-L460));
- preserve HTTP status so a deterministic miss can be negative-cached while `429` remains retryable ([`logos.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/logos.py#L407-L418));
- keep optional third-party composition off by default and recommend self-hosting, because it reveals the curated fixtures and adds an uptime dependency ([`gamethumbs.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/gamethumbs.py#L20-L33), [`gamethumbs.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/gamethumbs.py#L52-L67)).

Our current identity fallback returns remote URLs for referenced/NCAA/AFL/country/game-thumbs assets ([`sports_identity.go`](../../internal/plugin/sports_identity.go#L154-L214)). A Silo-owned image proxy/cache implementing the validation, atomic writes, differentiated TTLs and rate-limit semantics above would eliminate the observed flashing/broken-image behavior more reliably than adding more URL aliases.

## Configuration and persistence

Upstream is highly configurable through `plugin.json`: source toggles and credentials, favorites, lookahead/cap/bench, signal weights, matching and stream policy, naming/numbering, placeholders, artwork, dry-run, and scheduled times. Destructive apply defaults to dry-run ([`plugin.json`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.json#L638-L671), [`plugin.json`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.json#L715-L739)). Settings may hold secrets; plaintext `chmod 600` files are fallback storage ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L160-L184), [`plugin.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.py#L1677-L1699)). Deterministic scores, LLM descriptions, SportsDB lookups, and game-thumbs lookups are separate sidecars; cache publication uses per-writer temp files plus atomic replace ([`plugin.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.py#L323-L342), [`plugin.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/plugin.py#L1702-L1729)).

For ours, keep the first version small: enabled ranking, favorite teams, maximum shelf size, and a few bounded weights. Persist only user preferences through the Silo-supported settings/state contract. Keep provider secrets out of reportable cache files and do not introduce plugin-directory plaintext secrets unless Silo offers no secret-store capability.

## API, deployment, licensing, and risks

Upstream assumes it is cloned into `/data/plugins` inside the Dispatcharr container and runs against Dispatcharr’s Python/Django/Redis internals ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L160-L199), [`_pipeline_runner.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/_pipeline_runner.py#L45-L61)). There is no standalone dependency manifest in the reviewed tree; its effective runtime ABI is the host’s private Django models and Redis helpers. That is fundamentally different from our SDK-based binary and is the main reason not to copy the runtime design.

Principal risks:

- **Host-version coupling:** direct imports from `apps.channels.models`, `apps.epg.models`, and `core.utils.RedisClient` can break on Dispatcharr upgrades.
- **Destructive ownership:** `apply` creates/deletes/renumbers lineup and EPG records. The upstream mitigates with dry-run, ownership markers, transactions, locks and fail-closed destructive work, but the blast radius remains much larger than a read-only shelf.
- **Provider stability/terms/quotas:** several inputs are explicitly unofficial or undocumented; Football-Data and Odds API have low free-tier limits ([`README.md`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/README.md#L97-L114)).
- **Secrets:** SportsDB embeds its key in the URL path, so composed URLs and even HTTP exceptions can leak credentials unless scrubbed ([`logos.py`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/logos.py#L35-L51)).
- **Privacy/cost:** optional Claude matching sends team, time, channel and program candidate data; optional public artwork composition reveals requested fixtures.
- **Correctness:** language inference from stream names is admitted to be best-effort, and the no-key `fallback_first` path can select an unrelated broadcast.
- **Performance:** importance simulation is CPU-heavy enough to require subprocess isolation; multiplying that across 39 competitions is not appropriate in an HTTP request path.

The upstream code is MIT licensed and requires retaining its copyright/license notice in copies or substantial portions ([`LICENSE`](https://github.com/Jacob-Lasky/dispatcharr_ranked_matchups/blob/e71a7aedfc6f98530bebba8ba39cdf32e8611e31/LICENSE#L1-L20)). Our repository is AGPL-3.0 ([`LICENSE`](../../LICENSE#L1-L18)). The licenses are compatible for incorporation with notice, but the recommended path is to reimplement the concepts in idiomatic Go and cite the inspiration; do not copy the Django-specific implementation, large alias datasets, vendor keys, or generated artwork without separately verifying their provenance/terms.

## Recommended reuse plan

| Priority | Reuse safely | Do not copy |
|---|---|---|
| 1 | Add a pure, deterministic `SportsEvent` ranking stage with visible signal breakdown; start with favorites, live/upcoming state, tournament/final stage and any provider-supplied rank/closeness fields. | Full Monte Carlo season simulator or LLM narrative in the request process. |
| 2 | Extend channel matching with explicit evidence class/confidence, same-segment both-team matching, strong/weak aliases, preview/ancillary demotion, and dated-feed rejection. | `fallback_first`, broad raw-M3U sweep without an authorized API, or Python matcher code. |
| 3 | Add a “Top games” shelf/filters over existing events and channel matches, with “Why this game?” details and an unmatched-but-not-playable placeholder state. | Temporary virtual channels, Dispatcharr ORM writes, channel renumbering, or cloned EPG rows from the Silo plugin. |
| 4 | Build a Silo-owned durable image proxy/cache with magic-byte validation, atomic replace, positive/negative TTLs, retry-aware `429` handling and bounded cleanup. | Remote vendor URL as permanent identity, public composition enabled by default, or embedded third-party API keys. |
| 5 | Add read-only matching diagnostics that report candidate evidence and rejection reasons; keep concurrency/time budgets explicit. | Upstream daemon-thread/subprocess/Redis scheduler machinery, which solves Dispatcharr’s private gevent deployment rather than ours. |

The central design rule is simple: **Sportarr owns canonical sports identity; Dispatcharr/EPG supplies what can be watched; Silo ranks and presents the intersection.**
