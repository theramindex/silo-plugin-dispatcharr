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
    sportsEventIsLive: event => !!event.live, sportsEventHasScores: event => !!(event.homeScore || event.awayScore),
    sportsScoresHidden: () => false, sportsStatusLabel: () => 'Live', sportsDateLabel: unix => 'at ' + unix,
    sportsEventTitle: event => event.away.name + ' at ' + event.home.name, sportsEventStateID: event => event.id,
    renderSportsTeamLogo: () => '<i></i>', sportsEventIsFollowed: () => false, sportsFavoriteLeagueMap: () => ({}),
    sportsFavoriteTeamMatches: team => !!team.favorite, myTVFollowedPeople: () => [],
    configuredSportsLibraryIDs: () => [], pluralLabel: (count, noun) => count + ' ' + noun + (count === 1 ? '' : 's'),
    emptyStateHTML: title => '<div class="empty">' + title + '</div>',
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
  assert.equal(ctx.sportsHubTab('favorites'), 'teams');
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
  const ctx = context({ myTVFollowedPeople: () => [{ id: 'gamepass:mlb:new-york-mets', name: 'New York Mets', leagueName: 'MLB' }] });
  ctx.state.sports = { leagues: [], events: [] };
  ctx.state.sportsTab = 'scores';
  const html = ctx.renderSportsAppHeader();
  assert.match(html, /data-sports-tab="scores" class="active" aria-current="page">Scores/);
  assert.match(html, /data-sports-team-open="mlb~new-york-mets"/);
  assert.match(html, /aria-label="Follow teams"/);
  ctx.state.sportsTeam = 'mlb~new-york-mets';
  const onTeam = ctx.renderSportsAppHeader();
  assert.doesNotMatch(onTeam, /class="active" aria-current="page">Scores/);
  assert.match(onTeam, /sports-header-team active/);
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
