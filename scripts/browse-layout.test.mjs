import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = ['core', 'prefs', 'sports', 'admin', 'multiview', 'app'].map(name => fs.readFileSync(new URL('../internal/plugin/ui/' + name + '.js', import.meta.url), 'utf8')).join('\n');
function load(names, globals) {
  const ctx = vm.createContext(Object.assign({
    state: {},
    items: value => Array.isArray(value) ? value : [],
    lower: value => String(value ?? '').toLowerCase(),
    escapeHTML: value => String(value ?? '').replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;'),
    icon: name => '<svg data-icon="' + name + '"></svg>'
  }, globals));
  for (const name of names) {
    const start = source.indexOf('function ' + name + '(');
    assert.ok(start >= 0, name + ' must exist');
    vm.runInContext(source.slice(start, source.indexOf('\nfunction ', start + 1)), ctx);
  }
  return ctx;
}

test('a folder with child groups stays intact instead of splitting its name', () => {
  const ctx = load(['categoryPrefixSplit', 'channelsBrowserSetups'], {
    channelsBrowserChildGroups: category => category.children || []
  });
  const setups = ctx.channelsBrowserSetups([
    { id: 'virtual:US TV', name: 'US TV', children: [{ category: { id: 'virtual:US TV / Sports', count: 36 }, label: 'Sports' }] },
    { id: 'c1', name: 'US News' }
  ]);
  assert.equal(setups[0].prefix, 'US TV');
  assert.equal(setups[0].groups[0].label, 'Sports');
  assert.equal(setups[1].prefix, 'US');
  assert.equal(setups[1].groups[0].label, 'News');
});

test('selecting a nested channel group reveals its children and keeps its full context', () => {
  const child = (path, label, count) => ({category: {id: 'virtual:' + path, name: label, count}, label});
  const va = child('US TV / VA', 'VA', 12);
  const sports = child('US TV / VA / Sports', 'Sports', 5);
  const news = child('US TV / VA / News', 'News', 7);
  const nba = child('US TV / VA / Sports / NBA', 'NBA', 2);
  const children = {'virtual:US TV / VA': [sports, news], 'virtual:US TV / VA / Sports': [nba]};
  const state = {channelsBrowserGroup: nba.category.id};
  const ctx = load(['channelsBrowserVisibleGroups', 'channelsBrowserSelection', 'channelsBrowserContext', 'channelsBrowserGroupButton'], {
    state,
    channelsBrowserChildGroups: category => children[category.id] || []
  });
  const setups = [{prefix: 'US TV', groups: [va]}];
  const selected = ctx.channelsBrowserSelection(setups);
  assert.equal(selected.group.category.id, nba.category.id, 'deep selection must not reset to VA');
  assert.equal(ctx.channelsBrowserContext(selected.setup, selected.group), 'US TV · VA · Sports · NBA');
  assert.match(ctx.channelsBrowserGroupButton(selected.setup, selected.group, true), /data-channels-group="virtual:US TV \/ VA \/ Sports \/ NBA"/);
  assert.match(ctx.channelsBrowserGroupButton(selected.setup, selected.group, true), /aria-pressed="true"/);
  state.channelsBrowserGroup = va.category.id;
  assert.deepEqual([...ctx.channelsBrowserVisibleGroups(setups[0]).map(group => group.category.id)], [va.category.id, sports.category.id, news.category.id]);
  state.channelsBrowserGroup = news.category.id;
  assert.equal(ctx.channelsBrowserSelection(setups).group.category.id, news.category.id);
  assert.ok(!ctx.channelsBrowserVisibleGroups(setups[0]).some(group => group.category.id === nba.category.id), 'inactive branches stay compact');
});

test('channel groups sharing a region prefix are grouped and shown without the prefix', () => {
  const ctx = load(['categoryPrefixSplit', 'categoryPrefixGroups']);
  const categories = ['Events', 'Replays', 'US Kids', 'US News', 'International Kids', 'International News', 'Solo Group'].map((name, index) => ({ id: 'c' + index, name }));
  const grouped = ctx.categoryPrefixGroups(categories);
  assert.deepEqual([...grouped.groups.map(group => group.prefix)], ['US', 'International']);
  assert.equal(grouped.groups[0].names.c2, 'Kids');
  assert.deepEqual([...grouped.loose.map(category => category.name)], ['Events', 'Replays', 'Solo Group']);
});

test('repeat airings of the same show on the same channel collapse into one card', () => {
  const ctx = load(['collapseRepeatAirings'], {
    uniqueEventChannels: channels => channels || [],
    sportsEventIsLive: event => !!event.live
  });
  const channel = [{ id: 'cfn' }];
  const events = [
    { id: 'a', name: 'CFN Special Presentation', startUnix: 100, channels: channel },
    { id: 'b', name: 'CFN Special Presentation', startUnix: 200, channels: channel },
    { id: 'c', name: 'MTV Video Music Awards', startUnix: 150, channels: [{ id: 'mtv' }] },
    { id: 'd', name: 'CFN Special Presentation', startUnix: 300, channels: channel }
  ];
  const collapsed = ctx.collapseRepeatAirings(events);
  assert.deepEqual([...collapsed.map(event => event.id)], ['a', 'c']);
  assert.deepEqual([...collapsed[0].laterAirings], [200, 300]);
  assert.equal(events[0].laterAirings, undefined, 'source events must not be mutated');
});

test('highlights with an mp4 stream open in the modal, others keep the ESPN link', () => {
  const ctx = load(['safeHTTPS', 'externalLinkAttrs', 'renderSportsHighlights'], {
    sportsSectionHTML: (title, meta, body) => body
  });
  const html = ctx.renderSportsHighlights([
    { title: 'Game Highlights', url: 'https://www.espn.com/video/clip?id=1', stream: 'https://media.video-cdn.espn.com/clip.mp4' },
    { title: 'Web only', url: 'https://www.espn.com/video/clip?id=2' }
  ]);
  assert.match(html, /<button type="button" class="sports-highlight" data-sports-highlight-src="https:\/\/media\.video-cdn\.espn\.com\/clip\.mp4"/);
  assert.match(html, /<a class="sports-highlight" href="https:\/\/www\.espn\.com\/video\/clip\?id=2" target="_blank"/);
});
