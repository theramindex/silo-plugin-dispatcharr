import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
function setup() {
  const preferences = { favorites: { second: true, first: true }, favoriteOrder: ['second', 'first'] };
  const channels = [{ id: 'first', name: 'ESPN' }, { id: 'second', name: 'ESPN 2' }, { id: 'suggested', name: 'Suggested' }];
  const ctx = vm.createContext({
    state: { app: { preferences }, view: 'mytv' },
    prefs: () => preferences, favoriteMap: () => preferences.favorites,
    autoFavoriteMap: () => ({ suggested: true }), searchableChannels: () => channels,
    items: value => Array.isArray(value) ? value : [], uniqueIDs: value => [...new Set(value)],
    escapeHTML: value => String(value ?? '').replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;'),
    icon: name => '<svg data-icon="' + name + '"></svg>', logoHTML: () => '<span>TV</span>',
    currentProgram: () => ({ title: 'Live tennis' }), normalizePreferences: () => {},
    savePrefs: () => { ctx.saves = (ctx.saves || 0) + 1; }, showAppToast: () => {},
    updateMyTVSearchSurface: () => { ctx.refreshes = (ctx.refreshes || 0) + 1; }
  });
  for (const name of ['orderedFavoriteChannels', 'setChannelFavorite', 'channelFavoriteButton', 'renderSearchResultRow', 'myTVChannelRow', 'myTVFavoriteChannelsHTML']) {
    const start = source.indexOf('function ' + name + '(');
    vm.runInContext(source.slice(start, source.indexOf('\nfunction ', start + 1)), ctx);
  }
  return ctx;
}
test('My TV uses existing explicit favorites in saved order and shows current programming', () => {
  const ctx = setup(), html = ctx.myTVFavoriteChannelsHTML();
  assert.ok(html.indexOf('data-channel="second"') < html.indexOf('data-channel="first"'));
  assert.ok(!html.includes('data-channel="suggested"'));
  assert.match(html, /Live tennis/);
  assert.match(html, /Remove ESPN 2 from My TV favorites/);
});
test('the actual favorite click handler saves and removes a channel using shared preferences', () => {
  const ctx = setup();
  ctx.event = { preventDefault() {}, target: { closest: () => ({ getAttribute: () => 'new-channel' }) } };
  const start = source.indexOf('  const saveChannel = event.target.closest');
  const end = source.indexOf('  const favoriteMove =', start);
  const click = () => vm.runInContext('(function(){' + source.slice(start, end) + '})()', ctx);
  click();
  assert.equal(ctx.prefs().favorites['new-channel'], true);
  assert.equal(ctx.prefs().favoriteOrder.at(-1), 'new-channel');
  click();
  assert.equal(ctx.prefs().favorites['new-channel'], undefined);
  assert.equal(ctx.saves, 2);
  assert.equal(ctx.refreshes, 2);
});
test('search rows have a distinct favorite action and the empty favorites section is hidden', () => {
  const ctx = setup();
  const html = ctx.myTVChannelRow({ id: 'other', name: 'New channel' });
  assert.match(html, /data-channel="other"/);
  assert.match(html, /data-save-channel="other"/);
  assert.match(html, /aria-pressed="false"/);
  assert.match(html, /Save New channel to My TV favorites/);
  ctx.prefs().favorites = {};
  ctx.prefs().favoriteOrder = [];
  assert.equal(ctx.myTVFavoriteChannelsHTML(), '');
});

function teamSetup() {
  const ctx = setup();
  ctx.prefs().sportsFavoriteTeams = {};
  ctx.lower = value => String(value || '').toLowerCase();
  ctx.sportsFavoriteTeamMap = () => ctx.prefs().sportsFavoriteTeams;
  ctx.myTVSportsPeople = () => [{ id: 'epg-yankees', name: 'New York Yankees' }];
  ctx.applySportsFavoritesToPayload = () => {};
  for (const name of ['sportsGamePassSlug', 'myTVBuiltInSportsPeople', 'sportsFavoriteTeamMatches', 'sportsSavedTeamID', 'sportsTeamFavoriteButton', 'toggleSportsTeamFavorite']) {
    const start = source.indexOf('function ' + name + '(');
    vm.runInContext(source.slice(start, source.indexOf('\nfunction ', start + 1)), ctx);
  }
  return ctx;
}
test('team hearts save a stable game pass and reflect existing follows', () => {
  const ctx = teamSetup(), team = { id: 'epg-yankees', name: 'New York Yankees' };
  const key = 'gamepass:mlb:new-york-yankees';
  assert.match(ctx.sportsTeamFavoriteButton(team), /data-sports-favorite-team="gamepass:mlb:new-york-yankees"/);
  ctx.toggleSportsTeamFavorite(key, true);
  assert.match(ctx.sportsTeamFavoriteButton(team), /aria-pressed="true"/);
  assert.match(ctx.sportsTeamFavoriteButton(team), /data-icon="heart-solid"/);
  ctx.prefs().sportsFavoriteTeams[team.id] = true;
  ctx.toggleSportsTeamFavorite(team.id, false);
  assert.equal(Object.keys(ctx.prefs().sportsFavoriteTeams).length, 0);
  assert.match(ctx.sportsTeamFavoriteButton(team), /aria-pressed="false"/);
  assert.equal(ctx.sportsTeamFavoriteButton({ name: 'Unknown' }), '');
});
test('card team hearts are outside the event-opening button and absent for program cards', () => {
  const ctx = teamSetup();
  Object.assign(ctx, {
    sportsEventIsLive: () => false, sportsEventArtwork: () => '', sportsEventChannelsExpanded: () => false,
    renderSportsMatchupThumbnail: () => '<span>Team artwork</span>', sportsStatusLabel: () => 'Live',
    sportsEventTitle: () => 'Yankees vs Nationals', sportsEventStateID: () => 'event',
    renderSportsTileAvailability: () => '', sportsEventIsRace: () => false, sportsEventIsProgram: () => false
  });
  const start = source.indexOf('function renderSportsEventTile(');
  vm.runInContext(source.slice(start, source.indexOf('\nfunction ', start + 1)), ctx);
  const event = { away: {id:'epg-yankees',name:'New York Yankees'}, home:{id:'nationals',name:'Washington Nationals'} };
  const html = ctx.renderSportsEventTile(event);
  assert.equal((html.match(/class="sports-team-favorite /g) || []).length, 2);
  assert.ok(html.indexOf('sports-tile-team-favorites') > html.indexOf('</button>'));
  ctx.sportsEventIsProgram = () => true;
  assert.doesNotMatch(ctx.renderSportsEventTile(event), /sports-tile-team-favorites/);
});
