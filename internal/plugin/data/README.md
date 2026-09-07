# Game Thumbs league registry

`game_thumbs_leagues.json` contains artwork namespace names and aliases, not a replacement for the plugin's competition IDs or schedule data.

Sources reviewed on 2026-09-07:

- [game-thumbs](https://github.com/sethwv/game-thumbs), revision `ae4964ac11a090536d295995414e6cec0648520c`: `leagues.json`, NCAA routes, and its Teamarr league generator.
- [Teamarr league definitions](https://github.com/Pharaoh-Labs/teamarr/blob/dev/teamarr/database/schema.sql): compatible team leagues from the same seed source used by game-thumbs. ESPN namespaces complement the built-in registry.

The runtime uses a bundled snapshot. It does not fetch source files or perform league discovery during guide rendering. Names are matched conservatively; specific NCAA and World Cup competitions take precedence over generic aliases. Unsupported competitions keep their existing images.

Image URLs use the unified `logo.png` and `thumb.png` routes. The optional `.png` extension is required by the public service's current edge routing. Team logos request the dark variant; league marks retain the default light variant for their light tiles. Generated backgrounds are only shown where provider/replay artwork is absent. Failed images fall back to existing logos or the local matchup layout.

When refreshing the registry, review alias collisions and verify representative public image responses. Upstream built-in leagues are in `leagues.json`; its `scripts/generate-teamarr-leagues.js` describes compatible Teamarr additions. Registry coverage does not guarantee that every team or logo resolves at the public service.
