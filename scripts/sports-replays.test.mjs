import assert from "node:assert/strict";
import test from "node:test";
import { SportsReplayMatcher } from "../internal/plugin/ui/sports_replays.js";

const event = (overrides = {}) => ({
  id: "wnba-aces-liberty",
  leagueId: "wnba",
  leagueName: "WNBA",
  name: "Las Vegas Aces vs New York Liberty",
  shortName: "LVA vs NYL",
  startUnix: 1752447600,
  home: { name: "New York Liberty", abbreviation: "NYL" },
  away: { name: "Las Vegas Aces", abbreviation: "LVA" },
  ...overrides
});

test("matches a strong WNBA replay and an MLB replay using alternate date fields", () => {
  const events = [event(), event({
    id: "mlb-yankees-red-sox",
    leagueId: "mlb",
    leagueName: "MLB",
    name: "New York Yankees at Boston Red Sox",
    shortName: "NYY @ BOS",
    startUnix: 1752534000,
    home: { name: "Boston Red Sox", abbreviation: "BOS" },
    away: { name: "New York Yankees", abbreviation: "NYY" }
  })];
  const catalog = [
    { id: "vod:aces-old", content_id: "aces-replay", name: "WNBA Las Vegas Aces vs New York Liberty Full Game", release_date: "2025-07-14" },
    { id: "vod:aces-best", content_id: "aces-replay", name: "WNBA Las Vegas Aces vs New York Liberty Full Game Replay", first_air_date: "2025-07-14" },
    { id: "vod:mlb", content_id: "mlb-replay", name: "MLB New York Yankees at Boston Red Sox", sort_metrics: { release_date: "2025-07-15" } }
  ];

  const matches = SportsReplayMatcher.matchEvents(events, catalog);
  assert.equal(matches[events[0].id].length, 1);
  assert.equal(matches[events[0].id][0].item.id, "vod:aces-old");
  assert.equal(matches[events[0].id][0].confidence, "high");
  assert.equal(matches[events[1].id][0].item.id, "vod:mlb");
  assert.ok(matches[events[1].id][0].reasons.includes("league matches"));
});

test("rejects a wrong-date replay and a candidate with only one team", () => {
  const matches = SportsReplayMatcher.matchEvents([event()], [
    { content_id: "wrong-date", name: "WNBA Las Vegas Aces vs New York Liberty", release_date: "2025-09-01" },
    { content_id: "one-team", name: "WNBA Las Vegas Aces season highlights", release_date: "2025-07-14" }
  ]);
  assert.deepEqual(matches[event().id], []);
});

test("rejects undated candidates even when both teams and league match", () => {
  const matches = SportsReplayMatcher.matchEvents([event()], [
    { content_id: "undated", name: "WNBA Las Vegas Aces vs New York Liberty Full Game Replay" }
  ]);
  assert.deepEqual(matches[event().id], []);
});

test("does not treat EDM music or Big Ten as team abbreviations", () => {
  const matches = SportsReplayMatcher.matchEvents([
    event({
      id: "nhl-oilers-jets",
      leagueId: "nhl",
      leagueName: "NHL",
      name: "Winnipeg Jets at Edmonton Oilers",
      shortName: "WPG @ EDM",
      startUnix: 1752447600,
      home: { name: "Edmonton Oilers", abbreviation: "EDM" },
      away: { name: "Winnipeg Jets", abbreviation: "WPG" }
    }),
    event({
      id: "nfl-titans-jets",
      leagueId: "nfl",
      leagueName: "NFL",
      name: "Tennessee Titans at New York Jets",
      shortName: "TEN @ NYJ",
      startUnix: 1752447600,
      home: { name: "New York Jets", abbreviation: "NYJ" },
      away: { name: "Tennessee Titans", abbreviation: "TEN" }
    })
  ], [
    { content_id: "music", name: "MC Dance EDM", release_date: "2025-07-14" },
    { content_id: "big-ten", name: "Big Ten Network", release_date: "2025-07-14" },
    { content_id: "real-nhl", name: "NHL Winnipeg Jets at Edmonton Oilers", release_date: "2025-07-14" },
    { content_id: "real-nfl", name: "NFL Tennessee Titans at New York Jets", release_date: "2025-07-14" }
  ]);
  assert.equal(matches["nhl-oilers-jets"].length, 1);
  assert.equal(matches["nhl-oilers-jets"][0].item.content_id, "real-nhl");
  assert.equal(matches["nfl-titans-jets"].length, 1);
  assert.equal(matches["nfl-titans-jets"][0].item.content_id, "real-nfl");
});

test("sorts by score and deduplicates repeated content_id values", () => {
  const matches = SportsReplayMatcher.matchEvents([event()], [
    { id: "weak", content_id: "same", name: "WNBA Las Vegas Aces vs New York Liberty", release_date: "2025-07-20" },
    { id: "best", content_id: "same", name: "WNBA Las Vegas Aces vs New York Liberty Full Game Replay", release_date: "2025-07-14" },
    { id: "second", content_id: "second", name: "WNBA Las Vegas Aces vs New York Liberty Replay", release_date: "2025-07-14" }
  ]);
  assert.deepEqual(matches[event().id].map((match) => match.item.id), ["best", "second"]);
  assert.ok(matches[event().id][0].score >= matches[event().id][1].score);
});

test("queries only configured libraries accessible to the current user", () => {
  assert.deepEqual(
    SportsReplayMatcher.accessibleLibraryIDs([9, 2, 9, 5, -1, "bad"], [{ id: 2, name: "Sports" }, { id: 7, name: "Kids" }, { id: 9, name: "Team Replays" }]),
    [9, 2]
  );
});

test("builds bounded catalog windows only around recent or live events", () => {
  const windows = SportsReplayMatcher.catalogWindows([
    event({ id: "recent", startUnix: 1752447600 }),
    event({ id: "same-window", startUnix: 1752534000 }),
    event({ id: "old", startUnix: 1742079600 }),
    event({ id: "future", startUnix: 1757804400 })
  ], { nowUnix: 1752447600 });
  assert.deepEqual(windows, [["2025-07-06", "2025-07-20"]]);
});

test("paginates catalog results with a hard cap and reports truncation", async () => {
  const offsets = [];
  const result = await SportsReplayMatcher.collectCatalogPages(async (offset) => {
    offsets.push(offset);
    return { items: [{ content_id: `item-${offset}` }], has_more: true };
  }, 3);
  assert.deepEqual(offsets, [0, 1, 2]);
  assert.equal(result.items.length, 3);
  assert.equal(result.truncated, true);

  const complete = await SportsReplayMatcher.collectCatalogPages(async (offset) => ({
    items: offset === 0 ? [{ content_id: "only" }] : [],
    has_more: false
  }), 3);
  assert.equal(complete.truncated, false);
});
