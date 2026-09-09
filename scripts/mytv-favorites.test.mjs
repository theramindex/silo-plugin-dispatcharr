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
test('search rows have a distinct accessible favorite action and the empty hub explains how to save', () => {
  const ctx = setup();
  const html = ctx.myTVChannelRow({ id: 'other', name: 'New channel' });
  assert.match(html, /data-channel="other"/);
  assert.match(html, /data-save-channel="other"/);
  assert.match(html, /aria-pressed="false"/);
  assert.match(html, /Save New channel to My TV favorites/);
  ctx.prefs().favorites = {};
  ctx.prefs().favoriteOrder = [];
  assert.match(ctx.myTVFavoriteChannelsHTML(), /Save channels with the heart button/);
});
