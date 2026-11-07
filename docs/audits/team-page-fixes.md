# Team-page fixes: completion record

Verified against `main` on 2026-11-06. The [original trail](team-page-fixes.tsv)
is archived for context; its October “open” and “did not push” entries are historical.
The implementation and regressions already shipped in 0.3.196 and remain in the
current branch. No additional team-page runtime change is required.

| Defect | Landed behavior | Evidence |
|---|---|---|
| Postseason statistics replacing the full season | Ask ESPN for `seasontype=2`, leaving the year to ESPN's current season | `internal/plugin/sports_news.go`: `teamStatSheet` |
| A transient sheet failure sticking inside a long-lived summary | Cache the counting sheet independently; retry failed requests with a short error lifetime | `TestTeamStatFailureDoesNotStickInTheSummary` |
| Guide team IDs sent directly to Sportarr | Resolve a guide-only team by name to a Sportarr hub ID | `TestRecentResultsResolvesAGuideTeamByName` |
| Similar team names selecting the wrong school | Prefer exact names and reject mismatched suffixes such as Michigan State | `TestMatchESPNTeamPrefersExactNames` |
| A doubleheader disappearing from recent results | Preserve distinct finals, deduplicating only matching start times or scores | `scripts/sports-hub.test.mjs`; `TestSportarrRecentGamesKeepsLatestFive` |

The removed raw-abbreviation fallback is intentional: `sportsTeamStatSheet`
selects the sport's supported counting columns in their defined order and omits
unrecognized columns. It does not invent a generic sheet for unsupported sports.
This closes the unlogged behavior-change entry in the original trail.

Landed commits, all ancestors of the current main branch:

- `53ae1f7`: `fix(sports): show the regular-season sheet and the right recent finals`
- `de38af5348dd1488eacf8e668229cc3bd8ca9c44`: regression cases for the sheet and results
- `9a19719a6b4574fbd8b6f65d6694617802cbb708`: doubleheader regression

The current release verification runs these Go and browser-script regressions.
The archived model-review notes are historical claims, not new verification.
