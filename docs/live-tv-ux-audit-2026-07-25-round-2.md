# Dispatcharr Live TV UX Audit: Round 2

Date: July 25, 2026

Target: `https://capp.ramindex.org/api/v1/plugins/14/dispatcharr`

Reviewed at 1280x720, 1512x982, and 1920x1080.

## Release Caveat

Production is still showing the pre-`0.3.76` experience. The local fixes in commit
`fef48df` and catalog correction in `77df204` have not been pushed or deployed, so
the audit separates release lag from remaining work.

## Fixed Locally, Not Yet Verifiable In Production

### Home organization

- Production still shows separate **My Channels** and **My Groups** sections.
- Local `0.3.76` combines these into **Saved Lineups**.

### Sports scrolling and stale live state

- Production sports uses a nested 560px scroll container.
- A wheel scroll moved the sports container to `scrollTop=1825`, then a rerender
  reset it to `0` about 1.8 seconds later.
- Production also shows old May and July fixtures as live.
- Local `0.3.76` removes the nested vertical scroller and normalizes stale live
  events to final.

### On Later classification

- Selecting **Sports** or **Events** still leaves 76 cards visible.
- The filtered results include unrelated programs such as *Henry Danger*,
  *Mandela: Long Walk to Freedom*, teleshopping, and sitcoms.
- Local `0.3.76` narrows event classification to titles and improves sports
  matching with channel context.

### Search ranking

- Searching `HBO` returns the exact `HBO` channel after 11 regional variants.
- Local `0.3.76` adds deterministic relevance ranking and deduplication.

### Featured events

- Production Events has **Upcoming**, **Live**, and **All**, but no featured
  section or feature controls.
- Local `0.3.76` adds administrator and user event featuring.

## Still Open

### P1: Guide time headers overlap during horizontal scrolling

The guide scrolls horizontally, but every non-first time header is sticky at the
same `left` position. At `scrollLeft=704`, the 8:30 PM and 9:00 PM headings both
resolve to the guide's first program-column boundary.

The current CSS applies:

```css
.time-head span:not(:first-child) {
  position: sticky;
  left: var(--epg-logo-col);
}
```

Only **Today** and the channel column should stay pinned. Time slots should move
normally with the program grid.

### P1: Events can present channel-name artifacts as event content

Production event cards include items such as old F1 channel names dated July 16
or July 23 while displaying July 25 as the airing date. Their descriptions can
be unrelated programs such as *The "Why Am I Still Awake?" Show*.

Local event extraction now uses the guide program title, which should improve
this, but it still needs live verification with the real catalog after deploy.
If artifacts remain, event candidates should reject:

- titles dominated by source prefixes, timestamps, and playlist syntax;
- events whose displayed date conflicts materially with the embedded title date;
- generic or unrelated guide summaries.

### P2: Recently watched can promote filler or off-air programs

The home rail displayed **San Diego Padres — Signing Off**. Recently watched is
the most prominent channel-resume surface, so filler states such as *Signing
Off*, *Off Air*, *To Be Announced*, and similar placeholders should fall back to
**Live channel**.

### P2: Exact search matches need deployment verification

The local relevance function should solve the current ordering, but the deployed
result must be checked after release. Expected order for `HBO`:

1. Exact channel name `HBO`
2. Names beginning with `HBO`
3. Names containing `HBO`
4. Guide and upcoming program matches

## Confirmed Good

- The guide is vertically virtualized: 25 EPG row nodes represent the much
  larger channel set.
- Vertical wheel input inside the guide scrolls the guide without moving the
  page.
- On Later fills the available width at 1280px.
- Home and Search fill the viewport at 1512px and 1920px with 20px page margins.
- No horizontal body overflow was found at the tested widths.
- The 1920px Home content width was 1880px, so the previous large-screen width
  cap is no longer present.

## Recommended Verification Order

1. Push and deploy `0.3.76`.
2. Confirm Saved Lineups replaces My Channels and My Groups.
3. Re-test Sports scrolling for at least 10 seconds while score refreshes run.
4. Re-test stale live sports normalization.
5. Re-test On Later Sports and Events filters.
6. Re-test `HBO` search ordering.
7. Verify admin and user event featuring.
8. Fix and re-test the guide time-header overlap.
9. Add filler-title suppression to Recently watched.
