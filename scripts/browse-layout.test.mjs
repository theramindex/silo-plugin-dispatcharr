import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
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
  const ctx = load(['renderSportsHighlights'], {
    sportsSectionHTML: (title, meta, body) => body
  });
  const html = ctx.renderSportsHighlights([
    { title: 'Game Highlights', url: 'https://www.espn.com/video/clip?id=1', stream: 'https://media.video-cdn.espn.com/clip.mp4' },
    { title: 'Web only', url: 'https://www.espn.com/video/clip?id=2' }
  ]);
  assert.match(html, /<button type="button" class="sports-highlight" data-sports-highlight-src="https:\/\/media\.video-cdn\.espn\.com\/clip\.mp4"/);
  assert.match(html, /<a class="sports-highlight" href="https:\/\/www\.espn\.com\/video\/clip\?id=2" target="_blank"/);
});
