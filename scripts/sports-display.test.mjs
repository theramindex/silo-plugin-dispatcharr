import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

test('event details use browse breadcrumbs and sport art without generic league copy', () => {
  const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
  const ctx = vm.createContext({
    state: {sportsTab: 'live', sportsFailedMedia: {}},
    escapeHTML: value => String(value || ''), safeSportsMediaURL: value => String(value || ''),
    appRoutePart: encodeURIComponent, sportsTeamName: team => team?.name || '',
    sportsEventTitle: event => event.name, icon: () => '', sportsScoresHidden: () => false,
    rankedSportsBroadcasts: () => [], sportsReplayMatchesForEvent: () => [],
    sportsEventArtwork: event => event.art || '', sportsArtworkDimensions: () => '',
    sportsLeagueEvents: () => [], sportsEventHasPlayableAccess: () => true,
    sportsEventIsLive: () => false, sportsFavoriteLeagueMap: () => ({}),
    sportsBroadcastGroup: () => 'Other feeds', renderSportsBroadcastCard: channel => '<button data-channel="' + channel.id + '">Watch ' + channel.name + '</button>',
    renderSportsDetailScore: () => '', renderSportsGameStats: () => '', sportsSectionHTML: () => ''
  });
  for (const name of ['appRouteHash', 'sportsTabLabel', 'sportsDetailLeagueLabel', 'renderSportsEventNavigation', 'sportsMediaFailed', 'sportsFieldBackgroundKind', 'sportsFieldBackgroundURL', 'markSportsDetailBackgroundFailed', 'renderSportsBroadcastGroups', 'renderSportsEventDetail']) {
    const start = source.indexOf('function ' + name + '('), end = source.indexOf('\nfunction ', start + 1);
    assert.ok(start >= 0 && end > start, name);
    vm.runInContext(source.slice(start, end), ctx);
  }
  const event = {leagueId: 'sports', leagueName: 'Sports', sportName: 'Sports', name: 'NWSL Soccer: Palmeiras vs Chicago Stars', away: {name: 'Palmeiras'}, home: {name: 'Chicago Stars'}};
  const html = ctx.renderSportsEventDetail({}, event);
  assert.match(html, /aria-label="Breadcrumb"/);
  assert.match(html, /href="#\/sports\/live"/);
  assert.match(html, /aria-current="page"[^>]*>Palmeiras vs Chicago Stars/);
  assert.doesNotMatch(html, />Previous<|>Next<|sports-eyebrow|data-sports-favorite-league/);
  assert.match(html, /sports-event-hero-art.*images\.unsplash\.com/);
  ctx.rankedSportsBroadcasts = () => [{id: 'watch-channel', name: 'Womens Sports Network'}];
  const watchPage = ctx.renderSportsEventDetail({}, event);
  assert.match(watchPage, /data-channel="watch-channel"/);
  assert.match(watchPage.slice(0, watchPage.indexOf('</header>')), /sports-hero-feeds.*data-channel="watch-channel"/);
  assert.equal((watchPage.match(/data-channel="watch-channel"/g) || []).length, 1);
  assert.doesNotMatch(watchPage, /Matched channels|candidate|Other feeds/);
  const nhl = {...event, leagueId: 'nhl', leagueName: 'NHL', sportName: 'Hockey'};
  assert.match(ctx.renderSportsEventNavigation({}, nhl), /href="#\/sports\/live\/league\/nhl">NHL/);
  const supplied = ctx.renderSportsEventDetail({}, {...event, art: 'https://example.com/event.jpg'});
  assert.match(supplied, /src="https:\/\/example.com\/event.jpg"/);
  assert.match(supplied, /data-sports-detail-fallback="https:\/\/images\.unsplash\.com/);
  const attributes = new Map([['src', 'https://example.com/event.jpg'], ['data-sports-detail-fallback', 'https://example.com/pitch.jpg']]);
  const classes = new Set(['has-art']);
  const image = {hidden: false, getAttribute: key => attributes.get(key), setAttribute: (key, value) => attributes.set(key, value), removeAttribute: key => attributes.delete(key), parentElement: {classList: {remove: key => classes.delete(key), add: key => classes.add(key)}}};
  ctx.markSportsDetailBackgroundFailed(image);
  assert.equal(attributes.get('src'), 'https://example.com/pitch.jpg');
  assert.equal(image.hidden, false);
  ctx.markSportsDetailBackgroundFailed(image);
  assert.equal(image.hidden, true);
  assert.ok(classes.has('no-art'));
  assert.ok(!classes.has('has-art'));
});

test('MLB details render innings, final stats, and honor hidden scores', () => {
  const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
  const event = {id: 'mlb', leagueId: 'mlb', away: {name: 'Reds'}, home: {name: 'Dodgers'}};
  const data = {available: true, completed: true, homeScore: '6', awayScore: '3', updatedAtUnix: 1, sourceUrl: 'https://www.espn.com/mlb/boxscore/', innings: [{number: 1, away: '0', home: '0'}, {number: 9, away: '0', home: ''}], rows: [{label: 'Hits', away: '7', home: '10'}]};
  const ctx = vm.createContext({sportsGameStatsState: {id: 'mlb', data}, items: value => value || [], escapeHTML: value => String(value ?? ''), sportsScoresHidden: () => false, sportsEventStateID: e => e.id, sportsTeamName: t => t.name, sportsSectionHTML: (title, source, body) => title + source + body});
  for (const name of ['sportsHasGameStats', 'renderSportsGameStats', 'renderSportsInnings']) {
    const start = source.indexOf('function ' + name + '('), end = source.indexOf('\nfunction ', start + 1);
    vm.runInContext(source.slice(start, end), ctx);
  }
  const html = ctx.renderSportsGameStats(event);
  assert.match(html, /^Final stats/);
  assert.match(html, /aria-label="Inning scores"/);
  assert.match(html, /<td>0<\/td><td>–<\/td>/);
  assert.match(html, /Hits/);
  data.completed = false; data.live = true;
  assert.match(ctx.renderSportsGameStats(event), /^Live stats/);
  ctx.sportsScoresHidden = () => true;
  assert.equal(ctx.renderSportsGameStats(event), '');
  assert.equal(ctx.sportsHasGameStats({leagueId: 'nhl'}), false);
});

test('game-thumbs failures fall back once and retain the local layout', () => {
  const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
  const state = {sportsFailedMedia: {}};
  const ctx = vm.createContext({state, safeSportsMediaURL: value => String(value || ''), escapeHTML: value => String(value || '')});
  for (const name of ['sportsMediaFailed', 'markSportsMediaFailed', 'sportsPreferredLogo', 'sportsGeneratedBackground', 'markSportsBackgroundFailed']) {
    const start = source.indexOf('function ' + name + '(');
    const end = source.indexOf('\nfunction ', start + 1);
    vm.runInContext(source.slice(start, end), ctx);
  }
  const primary = 'https://game-thumbs.swvn.io/nba/lakers/logo.png?variant=dark';
  const fallback = 'https://example.com/lakers.png';
  assert.equal(ctx.sportsPreferredLogo(primary, fallback), primary);
  let src = primary, removed = false;
  const image = {hidden: false, getAttribute: () => src, setAttribute: (_, value) => { src = value; }, nextElementSibling: {hidden: true}, parentElement: {classList: {contains: () => true, remove: () => {removed = true;}}}};
  ctx.markSportsMediaFailed(image);
  assert.equal(src, fallback);
  assert.equal(image.hidden, false);
  assert.equal(ctx.sportsPreferredLogo(primary, fallback), fallback);
  ctx.markSportsMediaFailed(image);
  assert.equal(image.hidden, true);
  assert.equal(image.nextElementSibling.hidden, false);
  assert.equal(ctx.sportsPreferredLogo(primary, fallback), '');
  assert.equal(removed, true);
  src = 'https://game-thumbs.swvn.io/nba/lakers/celtics/thumb.png';
  const event = {gameThumbsBackgroundUrl: src};
  assert.match(ctx.sportsGeneratedBackground(event), /loading="lazy"/);
  ctx.markSportsBackgroundFailed(image);
  assert.equal(ctx.sportsGeneratedBackground(event), '');
});

test('upcoming promotions show a matchup heading and separate start time', () => {
  const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
  const ctx = vm.createContext({
    sportsTeamName: team => team?.name || 'Team', uniqueEventChannels: () => [],
    sportsEventArtwork: () => '', sportsArtworkDimensions: () => '',
    sportsEventIsLive: () => false, sportsEventIsOnNow: () => false,
    renderSportsMatchupThumbnail: () => '', sportsDateLabel: () => 'Sun, Sep 6 4:10 PM',
    escapeHTML: value => String(value), sportsEventStateID: () => 'test', icon: () => ''
  });
  for (const [start, end] of [
    ['function sportsEventTitle(', 'function sportsEventStateID('],
    ['function renderSportsFeature(', 'function renderStandaloneSportsReplayFeature(']
  ]) vm.runInContext(source.slice(source.indexOf(start), source.indexOf(end)), ctx);
  const event = {name: 'Next Game: New York Yankees @ San Diego Padres on 2026-09-06 at 04:10PM EDT', startUnix: 1788725400};
  assert.equal(ctx.sportsEventTitle(event), 'New York Yankees @ San Diego Padres');
  const html = ctx.renderSportsFeature(event);
  assert.match(html, /<h1>New York Yankees @ San Diego Padres<\/h1>/);
  assert.match(html, /sports-feature-schedule.*Sun, Sep 6 4:10 PM/);
  assert.equal(ctx.sportsEventTitle({name: 'Classic MLB: 1986 Mets at Boston'}), 'Classic MLB: 1986 Mets at Boston');
  assert.equal(ctx.sportsEventTitle({name: 'Next Game: Yankees @ Padres'}), 'Yankees @ Padres');
  assert.doesNotMatch(ctx.renderSportsFeature({...event, startUnix: 0}), /sports-feature-schedule/);
});
