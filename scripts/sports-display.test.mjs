import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

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
