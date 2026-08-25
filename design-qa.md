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

---

# UHF-style playback redesign QA

final result: blocked

## Comparison target

- Source visual truth: `/var/folders/62/h76dg1dn1w54yvjml62vxw480000gn/T/codex-clipboard-aa909038-e7e7-4661-aade-5b93ee4a3b25.png`
- Source pixels: `3138 × 2456` (`1792 × 1402` normalized conversation preview); inferred `1569 × 1228` CSS viewport at `2x` source density.
- Implementation: production Dispatcharr installation `14`, release `v0.3.91`, at `https://silo.ramindex.org/api/v1/plugins/14/dispatcharr?theme=midnight-cinema`.
- Implementation viewport: `1511 × 1306` CSS pixels in the Codex in-app browser; temporary `1569 × 1228` override was reset after capture was unavailable.
- Implementation screenshot path: unavailable. The in-app browser returned `Unable to capture screenshot` for viewport and full-page captures, including while the player was visible and paused.
- State: authenticated Midnight Cinema theme, Monumental Sports Network, live-rewind ready, controls awake, live video playing.

## Browser-rendered verification

- Asset version `69470cfd7c99abce` confirmed the new embedded UI was loaded rather than the previous cached page.
- The browser-rendered player uses `object-fit: contain`, a `100dvh` shell, circular back control, five-item upper-right control group, compact lower-left program identity, pill-shaped lower-right actions, full-width rewind range, and right-aligned live timing.
- More menu preserved Aspect Ratio, Fullscreen, Picture in Picture, Channel Guide, Multiview, search, history, casting, copy, and external-player controls. Sports Center remains conditional on a sports launch context.
- Live playback reached ready state `4`, remained unpaused, advanced beyond `254s`, and reported no media error after the redesign.
- Console review found no current-release error; the only retained entries were timestamped failures from the prior `72825efffc55cd96` asset before the production service refresh.

## Full-view comparison evidence

- The source and rendered implementation could not be placed into the required combined visual comparison input because the selected in-app browser failed every screenshot capture.
- DOM hierarchy, computed layout, runtime state, and interaction checks support the intended UHF composition, but they are not a substitute for visual comparison.

## Focused-region comparison evidence

- Not available for the same screenshot-capture blocker. The intended focused regions are the upper-right control cluster, lower-left identity block, lower-right action pill, and live timeline.

## Findings

- [P2] Visual comparison evidence is missing.
  - Location: production player, full viewport and four focused control regions.
  - Evidence: the source image opened successfully; the rendered implementation was interactive, but browser screenshot capture returned `Unable to capture screenshot`.
  - Impact: exact typography, spacing, opacity, and video-to-chrome proportions cannot be certified against the UHF reference.
  - Fix: capture the current production player once in-app screenshot capture is available, combine it with the source image at normalized density, and rerun the five required fidelity-surface checks.

## Required fidelity surfaces

- Fonts and typography: code and computed hierarchy verified; pixel fidelity blocked by missing implementation capture.
- Spacing and layout rhythm: DOM and computed viewport behavior verified; pixel fidelity blocked.
- Colors and visual tokens: black stage, white text, glass controls, scrims, and live red verified in CSS; rendered comparison blocked.
- Image quality and asset fidelity: the real channel logo and live video are used; screenshot comparison blocked.
- Copy and content: channel name, program title, description/category, mode, timing, live state, and accessible control labels verified in the rendered DOM.

## Comparison history

1. Initial implementation changed the default stage from cropped `cover` playback to contained full-viewport playback, removed the text Exit treatment, reduced persistent chrome, moved advanced controls to overflow, compacted metadata, and anchored the rewind timeline across the bottom.
2. Automated checks passed and `v0.3.91` was released, cataloged, and activated.
3. Production initially showed the previous cached asset version. Re-entering Live TV through Silo loaded `69470cfd7c99abce`; the new hierarchy and current live stream were then verified.
4. Viewport, full-page, paused-video, visible-browser, and default-size screenshot attempts all failed at the browser capture boundary, leaving the visual comparison blocked.

## Implementation checklist

- [x] Full-viewport contained video stage.
- [x] Sparse floating top controls and circular back action.
- [x] Compact lower-left channel/program identity.
- [x] Lower-right action pill and full-width live-rewind timeline.
- [x] Advanced features preserved in overflow.
- [x] Production playback and interaction verification.
- [ ] Combined reference/implementation visual comparison and focused-region review.
