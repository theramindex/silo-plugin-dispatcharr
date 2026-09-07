import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

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
