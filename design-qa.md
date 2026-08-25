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

## Production verification

- Installation `14` is active on `v0.3.90` and Silo is healthy.
- Monumental Sports Network reproduced the original MPEG-TS playback path successfully: rewind start returned `202`, authenticated manifests and segments returned `200`, the video reached ready state `4`, remained unpaused, and its playback position advanced without a media error.
- Provider-native HLS channels still use the direct redirect fallback and require a separate server-side HLS proxy improvement.

## Required next step

Add an authenticated server-side proxy for provider-native HLS channels. For the remaining visual QA, refresh the Sports page until Sportarr artwork enrichment is present, exercise the three/two/one-column event grid, team rail, and animated channel tray at desktop and narrow layouts, and compare the cards with the supplied Apple TV reference before marking this QA passed.
