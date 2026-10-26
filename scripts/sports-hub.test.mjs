import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../internal/plugin/ui/sports_hub.js', import.meta.url), 'utf8');

function context(extra = {}) {
  const ctx = vm.createContext({
    state: { view: 'sports', sportsTab: 'today', sports: { leagues: [] } },
    document: { addEventListener() {} },
    items: value => Array.isArray(value) ? value : [],
    lower: value => String(value || '').toLowerCase(),
    escapeHTML: value => String(value ?? '').replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;'),
    icon: name => '<svg data-icon="' + name + '"></svg>',
    uniqueEventChannels: channels => Array.isArray(channels) ? channels : [],
    sportsTeamName: team => (team && team.name) || '',
    sportsGamePassSlug: name => String(name || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''),
    sportsEventIsRace: () => false, sportsEventIsProgram: () => false,
    sportsEventIsLive: event => !!event.live, sportsEventIsOnNow: event => !!event.live && !event.completed, sportsEventHasScores: event => !!(event.homeScore || event.awayScore),
    sportsScoresHidden: () => false, sportsStatusLabel: () => 'Live', sportsDateLabel: unix => 'at ' + unix,
    sportsEventTitle: event => event.away.name + ' at ' + event.home.name, sportsEventStateID: event => event.id,
    renderSportsTeamLogo: () => '<i></i>', sportsEventIsFollowed: () => false, sportsFavoriteLeagueMap: () => ({}),
    sportsFavoriteTeamMatches: team => !!team.favorite, myTVFollowedPeople: () => [],
    configuredSportsLibraryIDs: () => [], pluralLabel: (count, noun) => count + ' ' + noun + (count === 1 ? '' : 's'),
    emptyStateHTML: title => '<div class="empty">' + title + '</div>',
    externalLinkAttrs: url => /^https:\/\//i.test(String(url || '')) ? ' href="' + url + '" target="_blank" rel="noopener noreferrer"' : '',
    ...extra
  });
  vm.runInContext(source, ctx);
  return ctx;
}

const mets = { name: 'New York Mets' };
const braves = { name: 'Atlanta Braves' };

test('legacy sports routes map onto the hub tabs', () => {
  const ctx = context();
  assert.equal(ctx.sportsHubTab('live'), 'today');
  assert.equal(ctx.sportsHubTab('upcoming'), 'scores');
  assert.equal(ctx.sportsHubTab('favorites'), 'mysports');
  assert.equal(ctx.sportsHubTab('replays'), 'replays');
  assert.equal(ctx.sportsHubTab('nonsense'), 'today');
});

test('score rows offer Watch only when a channel carries the game', () => {
  const ctx = context();
  const watchable = ctx.renderSportsScoreRow({ id: 'a', live: true, away: mets, home: braves, awayScore: '2', homeScore: '4', channels: [{ id: 'channel:sny', name: 'SNY' }] });
  assert.match(watchable, /data-channel="channel:sny"/);
  assert.match(watchable, /SNY/);
  assert.match(watchable, /<b>2<\/b>.*<b>4<\/b>/);
  const unavailable = ctx.renderSportsScoreRow({ id: 'b', startUnix: 100, away: mets, home: braves, channels: [] });
  assert.doesNotMatch(unavailable, /data-channel=/);
  assert.match(unavailable, /Not on your channels/);
  assert.match(unavailable, /data-sports-open-event="b"/);
  const later = ctx.renderSportsScoreRow({ id: 'c', startUnix: 200, away: mets, home: braves, channels: [{ id: 'channel:espn', name: 'ESPN' }] });
  assert.doesNotMatch(later, /class="sports-score-watch"/);
  assert.match(later, /class="sports-score-channel"[^>]*>ESPN</);
});

test('team pages group a team\'s games and describe results from its side', () => {
  const ctx = context();
  const payload = { events: [
    { id: 'final', leagueId: 'mlb', leagueName: 'MLB', completed: true, startUnix: 10, away: mets, home: braves, awayScore: '5', homeScore: '3', channels: [] },
    { id: 'next', leagueId: 'mlb', leagueName: 'MLB', startUnix: Math.floor(Date.now() / 1000) + 3600, away: braves, home: mets, channels: [] }
  ] };
  const teams = ctx.sportsHubTeams(payload);
  const entry = teams['mlb~new-york-mets'];
  assert.equal(entry.events.length, 2);
  assert.equal(ctx.sportsTeamResultLine(entry, payload.events[0]), 'W 5–3 at Atlanta Braves');
  assert.equal(ctx.sportsTeamStatusLine(entry).text, 'at ' + payload.events[1].startUnix + ' vs Atlanta Braves');
  assert.deepEqual({ ...ctx.sportsTeamKeyParts('mlb~new-york-mets') }, { leagueID: 'mlb', slug: 'new-york-mets' });
});

test('the standalone Sports header carries sections and followed team shortcuts', () => {
  const ctx = context({ myTVFollowedPeople: () => [{ id: 'gamepass:mlb:new-york-mets', name: 'New York Mets', leagueName: 'MLB' }], sportsReplayStatusLabel: () => '', sportsDataSourceLabel: () => 'Sportarr' });
  ctx.state.sports = { leagues: [], events: [] };
  ctx.state.sportsTab = 'scores';
  const html = ctx.renderSportsAppHeader();
  assert.match(html, /data-sports-tab="scores" class="active" aria-current="page">Scores/);
  assert.match(html, /data-sports-team-open="mlb~new-york-mets"/);
  assert.match(html, /aria-label="Follow teams"/);
  assert.match(html, /data-sports-refresh="true" aria-label="Refresh scores · Data by Sportarr"/);
  ctx.state.sportsTeam = 'mlb~new-york-mets';
  const onTeam = ctx.renderSportsAppHeader();
  assert.doesNotMatch(onTeam, /class="active" aria-current="page">Scores/);
  assert.match(onTeam, /sports-header-team active/);
});

test('a doubleheader stays beside the final already on the page', () => {
  const ctx = context();
  const yankees = { name: 'New York Yankees' };
  const sox = { name: 'Boston Red Sox' };
  const entry = {
    team: yankees,
    events: [{ id: 'first', completed: true, startUnix: 1700000000, home: yankees, away: sox, homeScore: '8', awayScore: '7' }]
  };
  const summary = {
    recent: [
      { startUnix: 1700000000 + 4 * 3600, opponent: 'Boston Red Sox', score: '1–0', result: 'W', home: true },
      { startUnix: 1700000000 + 10 * 3600, opponent: 'Boston Red Sox', score: '8–7', result: 'W', home: true }
    ]
  };
  const html = ctx.sportsTeamRecentRows(entry, summary);
  assert.match(html, /1–0/);
  assert.doesNotMatch(html, /8–7/);
});

test('news from several feeds is deduplicated and newest first', () => {
  const ctx = context();
  const merged = ctx.mergeSportsNews([
    { articles: [{ id: '1', published: '2026-09-24T10:00:00Z' }, { id: '2', published: '2026-09-24T12:00:00Z' }] },
    { articles: [{ id: '2', published: '2026-09-24T12:00:00Z' }, { id: '3', published: '2026-09-23T12:00:00Z' }] }
  ]);
  assert.deepEqual(Array.from(merged, article => article.id), ['2', '1', '3']);
  const html = ctx.renderSportsNewsList([{ id: '1', headline: 'Mets win', url: 'https://www.espn.com/story', leagueName: 'MLB' }], 5);
  assert.match(html, /target="_blank" rel="noopener noreferrer"/);
  assert.match(html, /Mets win/);
});

// Exercise the same shared follow preferences and event matching used by the
// Sports screen. The directory must stay usable without any follows.
test('Teams discovers everyone while My Sports contains only personal follows and their events', () => {
  const preferences = {
    sportsFavoriteTeams: { 'gamepass:mlb:new-york-mets': true, 'fighter-1': true },
    sportsFavoriteTeamLabels: { 'fighter-1': { name: 'Alex Fighter', kind: 'Fighter', leagueName: 'UFC' } },
    sportsFavoriteLeagues: { nba: true }
  };
  const ctx = context({
    prefs: () => preferences,
    uniqueIDs: values => [...new Set(values)], isSportsApp: () => true,
    sportsFavoriteTeamMap: () => preferences.sportsFavoriteTeams,
    sportsFavoriteLeagueMap: () => preferences.sportsFavoriteLeagues,
    renderSportsLeagueMark: league => '<i>' + league.name + '</i>',
    sportsTeamAbbreviation: team => team.abbreviation || '',
    collapseEquivalentSportsEvents: events => events,
    compareSportsEventsForTab: (a, b) => a.startUnix - b.startUnix,
    sportsTeamSummaryFor: () => ({ upcoming: [] }), espnNewsLeague: () => false
  });
  vm.runInContext(fs.readFileSync(new URL('../internal/plugin/ui/sports.js', import.meta.url), 'utf8'), ctx);
  // Artwork and score formatting have their own tests; keep this scenario
  // focused on which real events and follows the page exposes.
  ctx.sportsTeamSummaryFor = () => ({ upcoming: [] });
  ctx.renderSportsTeamLogo = () => "<i></i>";
  ctx.renderSportsLeagueMark = league => "<i>" + league.name + "</i>";
  ctx.renderSportsEventTile = event => '<article data-event="' + event.id + '"></article>';
  const now = Math.floor(Date.now() / 1000);
  const payload = {
    leagues: [{ id: 'mlb', name: 'MLB' }, { id: 'nba', name: 'NBA' }, { id: 'ufc', name: 'UFC' }],
    events: [
      { id: 'team-game', leagueId: 'mlb', home: mets, away: braves, live: true, startUnix: now - 60 },
      { id: 'league-game', leagueId: 'nba', home: { name: 'New York Knicks' }, away: { name: 'Boston Celtics' }, startUnix: now + 3600 },
      { id: 'fight', leagueId: 'ufc', home: { id: 'fighter-1', name: 'Alex Fighter' }, away: { name: 'Blair Fighter' }, startUnix: now + 7200 },
      { id: 'unrelated', leagueId: 'mlb', home: { name: 'Chicago Cubs' }, away: braves, startUnix: now + 3600 },
      { id: 'old-game', leagueId: 'mlb', home: mets, away: braves, completed: true, startUnix: now - 7200 }
    ]
  };
  ctx.state.sports = payload;
  ctx.state.sportsLeagueTeams = { mlb: [mets, braves], nba: [], ufc: [] };
  ctx.state.sportsTab = 'teams';
  const directory = ctx.renderSportsHub(payload);
  assert.match(directory, /Atlanta Braves/);
  assert.doesNotMatch(directory, /data-event="team-game"/);
  assert.equal(ctx.sportsHubTab('mysports'), 'mysports');
  ctx.state.sportsTab = 'mysports';
  const personal = ctx.renderSportsHub(payload);
  assert.match(personal, /Unfollow New York Mets/);
  assert.match(personal, /Unfollow Alex Fighter/);
  assert.match(personal, /Unfollow NBA/);
  assert.match(personal, /data-event="team-game"/);
  assert.match(personal, /data-event="league-game"/);
  assert.match(personal, /data-event="fight"/);
  assert.doesNotMatch(personal, /data-event="(?:unrelated|old-game)"/);
  preferences.sportsFavoriteTeams = {};
  preferences.sportsFavoriteLeagues = {};
  const empty = ctx.renderSportsHub(payload);
  assert.doesNotMatch(empty, /data-event=/);
  assert.match(empty, /data-sports-tab="teams"/);
  ctx.state.sportsTab = 'teams';
  assert.match(ctx.renderSportsHub(payload), /Atlanta Braves/);
});

test('Sports keeps polling through a slow background build, then stops at a bounded deadline', () => {
  let pending = null;
  let elapsed = 0;
  let polls = 0;
  const ctx = context({
    setTimeout: (callback, delay) => { pending = callback; elapsed += delay; return 1; },
    clearTimeout: () => { pending = null; }
  });
  vm.runInContext(fs.readFileSync(new URL('../internal/plugin/ui/sports.js', import.meta.url), 'utf8'), ctx);
  ctx.state.sportsPollAttempts = 0;
  ctx.loadSports = (force, preparedOnly) => {
    assert.equal(force, true);
    assert.equal(preparedOnly, true);
    polls += 1;
  };
  for (let tick = 0; tick < 200; tick += 1) {
    ctx.scheduleSportsPoll();
    if (!pending) break;
    const callback = pending;
    pending = null;
    callback();
  }
  // The server allows a minute for guide/provider/matching/enrichment work;
  // the UI needs room beyond that without polling forever.
  assert.ok(elapsed >= 90_000, `polling stopped after ${elapsed} ms`);
  assert.ok(elapsed <= 180_000, `polling ran for ${elapsed} ms`);
  assert.ok(polls < 100, 'polling never stopped');
  ctx.stopSportsPoll(true);
  ctx.scheduleSportsPoll();
  assert.ok(pending, 'a later refresh must start polling again');
});
