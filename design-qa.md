# Dispatcharr sports-card design QA

final result: blocked

## Scope

- Reference: Apple TV live-sports cards supplied by the user and the Channels DVR Sports Section.
- Build: Dispatcharr `v0.3.90`, using Sportarr event artwork with team/league marks, full team names centered beneath their logos, stable team shelf cards, a denser responsive event grid, status pills, a consistent featured-event fallback, animated inline channel disclosure on event cards, and manifest-declared authenticated timeshift media routes.

## Automated verification

- `node --check internal/plugin/ui/app.js`: passed
- `go test ./...`: passed
- release verification for `0.3.90`: passed
- plugin release workflow: passed
- Ramindex catalog validation: passed

## Blocking condition

Production activation has not been authorized. The live browser therefore remains on `v0.3.83`, so a same-state prototype capture and reference/prototype visual comparison cannot be completed yet.

## Required next step

Activate plugin installation `14` on `v0.3.90`, verify a timeshifted MPEG-TS channel loads its declared manifest and segment routes, then refresh the Sports page until Sportarr artwork enrichment is present, exercise the three/two/one-column event grid, team rail, and animated channel tray at desktop and narrow layouts, and compare the cards with the supplied Apple TV reference before marking this QA passed.
