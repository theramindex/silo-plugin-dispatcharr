# Live TV (Dispatcharr) UX Audit

Date: July 25, 2026

Surfaces reviewed:

- Silo plugin entry and catalog card
- Home
- My Channels and My Groups
- Full Guide
- On Later
- Search
- Sports
- Events
- Dispatcharr Admin
- 1280x720 and 1728x1000 desktop viewports

## Executive Summary

The Guide and core navigation are in solid shape, but the app currently has three
competing organizational concepts: profiles, groups, and saved channel lists.
That makes the Home page feel fragmented and makes "My Channels" appear to be a
small favorites list when it is actually a 717-channel saved profile.

The biggest trust problem is not visual. Sports and Events regularly surface stale
events, incorrect channel matches, description-only false positives, duplicates,
and low-value automatic featured content. These should be corrected before making
the sports presentation more prominent.

The reported wide-screen and sports scrolling issues are both supported by the
current layout. Sports and Events use capped, left-aligned containers, while Sports
also owns a tall nested vertical scroller that can be reset by data refreshes.

## Priority Findings

### P0: Sports and Events Need a Confidence Gate

Observed examples:

- Korn Ferry and PGA Tour events matched to AyazTV, a Persian music channel.
- Jeff Dunham content was classified as Combat Sports because its description
  mentioned a former UFC champion.
- Festival language promoted unrelated entertainment into Events.
- Old Apple MLS entries were still marked live, including events months old.
- The automatically featured live event was a high-school softball game while
  higher-salience events were available.

Recommended pipeline:

1. Identify candidates from structured Sportarr or EPG fields, title, time, and
   channel category.
2. Match channels using normalized league, team, network, locale, and event time.
3. Reject description-only keyword matches unless supported by title or channel.
4. Assign a confidence score and hide low-confidence matches.
5. Deduplicate by stable event and channel IDs.
6. Apply feature ranking only after validation.

### P0: Fix Sports Vertical Scrolling

Sports uses a nested `.sports-score-scroll` vertical container with a large internal
scroll height. Refreshes or rerenders can replace that container and return it to
the top, matching the reported behavior.

Recommended change:

- Let the page own vertical scrolling.
- Keep only horizontal rails independently scrollable.
- Patch changed score cards instead of rebuilding the whole list.
- If a rebuild is unavoidable, preserve route, filters, focus, and scroll position.

Acceptance criterion: a user can scroll through Sports while automatic score
updates occur without any vertical movement or focus loss.

### P1: Combine My Channels and My Groups

Home presents My Channels at the top and My Groups below the Guide. They are two
versions of the same user intent: save a lineup for quick access.

Replace them with one top-level **Saved Lineups** section containing:

- Profile-based lineups
- Group-based lineups
- Custom mixed lineups

Each item should show its source type, channel count, dedupe status, and a single
edit menu. Existing storage can remain compatible while the UI presents one model.

The current saved `US TV | NY` lineup also produced duplicate FOX Sports 2 and NBA
TV rows. Apply stable channel-ID deduplication before rendering or saving a lineup.

### P1: Add Admin and User Event Featuring

The Events admin currently configures keyword detection but cannot curate results.

Admin controls should include:

- Event search and preview
- Featured toggle
- Display start and end
- Rank/order
- Optional title and artwork override
- Validation status and matched channels

Users should be able to follow or feature events privately. Recommended priority:

1. Active admin pin
2. User-featured or followed event
3. Major-event importance score
4. Validated live event
5. Automatic relevance score

This allows a Presidential Debate or the Grammys to outrank routine PGA coverage.
Use separate **Featured** and **Following** rails rather than one fragile hero slot.

### P1: Remove Common Desktop Width Caps

At 1728px:

- Home and On Later filled the viewport with 20px gutters.
- Sports was capped at 1536px and left 172px unused on the right.
- Events was capped at 1472px and left 236px unused on the right.

Use the same fluid page shell everywhere:

```css
width: 100%;
max-width: none;
```

If a cap is desired for very large displays, center it and apply it above 1920px.
Use responsive `auto-fit` grids so additional width becomes another column or
comfortable card width instead of empty space.

## Surface Findings

### Home

- A single My Channels card leaves excessive unused space.
- Recently watched is fixed to five cards even when more width is available.
- My Groups is separated from My Channels by the entire Guide.
- The Home Guide is useful, but saved content should appear as one coherent shelf.

### Guide

- The grid is virtualized; only a small row window is mounted for a very large
  guide. The problem is duplicate source membership, not missing virtualization.
- Duplicate channel rows reduce trust in category and saved-lineup filtering.
- The category and program search toolbar is one of the strongest layouts.
- Preserve sticky time/channel panes and scroll state during guide refresh.

### On Later

- Time and type controls change active state correctly.
- Sports filtering returns game shows, films, children's shows, and sitcoms.
- Guide Picks includes placeholders such as "To Be Announced" and "Sendepause."
- Suppress placeholders and rank exact structured type matches above keyword hits.

### Search

- The page is visually cohesive.
- Exact local results can rank below many international channels.
- Duplicate airings and channel variants remain in results.
- Rank exact title first, then current saved lineup/profile, then global catalog.
- Deduplicate by stable channel and airing IDs and offer "Show all" per section.

### Sports

- The current browse layout is promising, but the oversized no-art hero wastes
  space and amplifies low-confidence data.
- Several event cards expose raw timestamps, provider slot labels, or stale scores.
- Whole-card and nested Open actions duplicate the same interaction.
- Generic labels such as "Sports / Sports" do not help users understand the source.

### Events

- Cards are readable but visually repetitive because all currently lack artwork.
- Classification and channel matching produce obvious false positives.
- Cards need validation state, follow/feature action, and a clearer primary action.
- Low-confidence events should not be shown merely to fill the grid.

### Admin

- Connection, channel, guide, and profile health are understandable.
- Profiles and Groups are functional but very dense when every item is enabled.
- Events needs a curation view in addition to keyword rules.
- The installed plugin catalog card still advertises VOD and series routes despite
  the no-VOD product decision.
- The catalog card still exposes Refresh Live TV Channels, Refresh Live TV Guide,
  and Live TV App actions that were previously designated for removal.

## Recommended Implementation Order

1. Remove the Sports nested vertical scroller and preserve state during refresh.
2. Remove Sports and Events desktop width caps.
3. Merge My Channels and My Groups into Saved Lineups and dedupe channels.
4. Add event confidence scoring, stale-event rejection, and channel-match validation.
5. Add admin event curation and private user follows/features.
6. Correct On Later classification and search relevance.
7. Clean stale catalog description and actions.

## Release Acceptance Checklist

- Sports never jumps vertically during automatic or manual refresh.
- Sports and Events fill a 1728px viewport with standard gutters.
- Saved Lineups contains profile, group, and custom entries in one section.
- A channel appears once per saved lineup unless variants are intentionally distinct.
- Description-only event keywords cannot create an unsupported match.
- Stale events cannot appear live.
- Admin pins always outrank automatic featured events during their active window.
- User-followed events have a dedicated, persistent view.
- Exact local search results rank above partial global matches.
- No-VOD catalog copy and obsolete plugin actions are removed.
