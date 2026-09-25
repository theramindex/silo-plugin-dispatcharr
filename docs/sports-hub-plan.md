# Sports hub plan

Goal: make Sports a sports app organized around teams and games, with TV as the
"Watch" action on a game instead of the thing that decides what appears.

## What changes for the user

| Tab | Content |
| --- | --- |
| Today (default) | Your teams strip, featured watchable game, "On your TV now", today's scores across leagues, top stories |
| Scores | Every game in the window grouped by league: live, final, upcoming. Watch chip when a channel carries it, "Not on your channels" otherwise |
| My Teams | Followed teams with next game and last result, plus team search to follow more |
| News | Headlines for followed teams and leagues, filterable by league |
| Replays | Existing Silo library replays (only when configured) |

Team page (`#/sports/<tab>/team/<teamKey>`): logo, record, standing, next game
with Watch button, recent results, upcoming schedule, team news.

Old routes (`live`, `upcoming`, `favorites`, `all`) map to the new tabs.

## Data

- Sportarr already returns games our channels do not carry; the server dropped
  them in `filterPlayableSportsEvents`. It now keeps them, without channels, when
  they are in a useful window (live, next 36 hours, or finished in the last 24
  hours) and in a league the lineup covers or a supported news league.
  Unwatchable games are capped so the payload stays small.
- News, record, and standing come from ESPN's public site API, which the plugin
  already uses for box scores. Two cached endpoints:
  - `GET /dispatcharr/api/sports/news?league=<id>&team=<name>` (10 minute cache)
  - `GET /dispatcharr/api/sports/team?league=<id>&name=<name>` (team summary and
    team news; ESPN team list cached for 24 hours)
- Supported news leagues: MLB, NFL, NBA, WNBA, NHL, MLS, Premier League,
  Champions League, college football, men's and women's college basketball,
  Formula 1, PGA golf, UFC. Other leagues still get scores and schedules.

## Game pages, standings, and schedules

- Game pages use ESPN's scoreboard and `summary` data for MLB, NFL, NBA, WNBA,
  NHL, MLS, Premier League, Champions League, and college football and
  basketball: team stats, recent plays, game leaders, and highlight clips.
- MLB adds the live situation (count, outs, runners, batter, pitcher) and
  probable pitchers before the game. ESPN carries this, so the MLB Stats API and
  NBA live CDN used by raquest are not needed.
- League and team pages show standings (`GET /dispatcharr/api/sports/standings`),
  with the team's own group highlighted on its page.
- Team pages and My Teams list upcoming games from the ESPN team schedule, beyond
  the 36-hour score window. Games on a matched channel with a guide listing get
  a Record button when Dispatcharr Direct recording is available.

## Team channels

Admins can pin a team to one channel (Admin > Sports > Team channels). The pinned
channel becomes the first Watch option for that team's live and upcoming games.
Stored in admin settings as `sportsTeamChannels`.

## Standalone Sports app header

When Sports runs as its own Silo app, the header shows the Sports sections and
logo shortcuts for followed teams (red ring while live) instead of Live TV
navigation.

## Code layout

- `internal/plugin/sports.go`: keep unwatchable games (`selectSportsEvents`).
- `internal/plugin/sports_news.go`: ESPN league mapping, news, team summary, cache.
- `internal/plugin/ui/sports_hub.js`: new tabs, scoreboard, teams, news, team page.
  Kept out of `app.js`, which is already over 8,000 lines.
- Existing featured hero, event tiles, league detail, and event detail are reused.

## Risks

- ESPN's site API is public but undocumented and can change. Failures degrade to
  "News is unavailable right now" without affecting scores or playback.
- More games in the payload: capped and windowed; watchable games keep priority
  for featured and "On your TV now".
