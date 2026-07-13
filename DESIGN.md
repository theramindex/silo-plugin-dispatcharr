# Dispatcharr Design System

## Direction

Dispatcharr should feel like a focused Live TV workspace inside Silo. Silo owns the global product experience; the plugin adds only the navigation and controls required for live television.

## Silo Alignment

- Use Silo theme variables whenever they are available.
- Dark fallbacks use near-black and zinc neutrals: `#0a0a0b`, `#18181b`, `#202023`, `#e4e4e7`, and `#a1a1aa`.
- Page titles use `1.875rem` at weight `700`.
- Section titles use `1.25rem` at weight `600`.
- Body, metadata, and controls use weights from `500` through `700`; reserve heavier weight for exceptional media states only.
- Segmented controls are `2rem` to `2.2rem` tall. The selected item uses the text color as its surface and the page background as its text color.
- Cards use an `0.5rem` radius at most. Prefer unframed media rails and quiet neutral surfaces.
- Accent color communicates selection, live state, focus, or a primary action. It is not decoration.

## Layout

- The plugin route cannot inject Silo's native sidebar, so its sticky top bar acts as a compact local header.
- Keep the local header on one line at desktop and medium widths. On narrow screens, the navigation becomes a horizontally scrollable second row.
- Browse pages use a stable content inset and compact vertical rhythm.
- Guide and scoreboard surfaces may use denser layouts because comparison is the task.
- Search, On Later, Sports, and Events use the same page-title, segmented-filter, section-title, and empty-state vocabulary.

## Components

- **Media rails:** artwork first, title and muted metadata below, no outer card frame.
- **Folder tiles:** quiet neutral surface, compact label and count, visible hover/focus state.
- **Search results:** dense rows for direct matches; compact cards only when artwork and grouped airing information justify them.
- **Sports and events:** compact Calendar-like filters above domain-specific score or event content.
- **Admin:** use the same segmented controls, typography, neutral surfaces, and semantic status colors as the user app.

## Interaction

- Keep focus rings visible and keyboard order predictable.
- Use 150-250ms state transitions only where they clarify selection or reveal.
- Respect reduced motion and reduced transparency.
- Preserve Guide timeline position and player return state when navigating.
