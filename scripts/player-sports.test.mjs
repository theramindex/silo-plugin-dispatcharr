import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

test('sports drawer waits for data and avoids redundant redraws', () => {
  const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
  const render = source.slice(source.indexOf('function renderPlayerSportsDrawer()'), source.indexOf('function stopPlayerSportsRefresh()'));
  let writes = 0, html = '', body = {scrollTop: 80};
  const root = {classList: {toggle() {}}, querySelector: () => body,
    get innerHTML() { return html; },
    set innerHTML(value) { writes++; html = value; body = {scrollTop: 0}; }};
  const state = {playerSportsOpen: true, sports: null, sportsLoading: false};
  const ctx = vm.createContext({state, document: {querySelector: () => null},
    byId: id => id === 'player-sports-drawer' ? root : null,
    playerSportsEvents: () => [], playerSportsCurrentEvent: () => null,
    playerSportsRelatedEvents: () => [], playerSportsChannels: () => [],
    sportsScoresHidden: () => false, icon: () => '', observePlayerSportsDrawerLayout() {}});
  vm.runInContext(render, ctx);
  const draw = () => vm.runInContext('renderPlayerSportsDrawer()', ctx);
  draw();
  assert.match(html, /Loading related sports/);
  assert.doesNotMatch(html, /No live sports/);
  assert.equal(body.scrollTop, 80);
  draw();
  assert.equal(writes, 1, 'unchanged data must retain the existing DOM');
  state.sports = {events: []};
  state.sportsLoading = true;
  draw();
  assert.match(html, /Loading related sports/);
  state.sportsLoading = false;
  draw();
  assert.match(html, /No live sports available/);
  assert.equal(body.scrollTop, 80);
  state.sports.error = 'timeout';
  draw();
  assert.match(html, /temporarily unavailable/);
});
