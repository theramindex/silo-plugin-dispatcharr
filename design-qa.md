# Dispatcharr sports-card design QA

final result: blocked

## Scope

- Reference: Apple TV live-sports cards supplied by the user and the Channels DVR Sports Section.
- Build: Dispatcharr `v0.3.85`, using Sportarr event artwork with team/league marks, full team names centered beneath their logos, status pills, and the existing no-art fallback.

## Automated verification

- `node --check internal/plugin/ui/app.js`: passed
- `go test ./...`: passed
- release verification for `0.3.85`: passed
- plugin release workflow: passed
- Ramindex catalog validation: passed

## Blocking condition

Production activation has not been authorized. The live browser therefore remains on `v0.3.83`, so a same-state prototype capture and reference/prototype visual comparison cannot be completed yet.

## Required next step

Activate plugin installation `14` on `v0.3.85`, refresh the Sports page until Sportarr artwork enrichment is present, then capture desktop and narrow layouts and compare them with the supplied Apple TV reference before marking this QA passed.
