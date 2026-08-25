# Dispatcharr sports-card design QA

final result: blocked

## Scope

- Reference: Apple TV live-sports cards supplied by the user and the Channels DVR Sports Section.
- Build: Dispatcharr `v0.3.88`, using Sportarr event artwork with team/league marks, full team names centered beneath their logos, stable team shelf cards, status pills, a consistent featured-event fallback, and animated inline channel disclosure on event cards.

## Automated verification

- `node --check internal/plugin/ui/app.js`: passed
- `go test ./...`: passed
- release verification for `0.3.88`: passed
- plugin release workflow: pending
- Ramindex catalog validation: passed

## Blocking condition

Production activation has not been authorized. The live browser therefore remains on `v0.3.83`, so a same-state prototype capture and reference/prototype visual comparison cannot be completed yet.

## Required next step

Activate plugin installation `14` on `v0.3.88`, refresh the Sports page until Sportarr artwork enrichment is present, exercise the team rail and animated channel tray at desktop and narrow layouts, and compare the cards with the supplied Apple TV reference before marking this QA passed.
