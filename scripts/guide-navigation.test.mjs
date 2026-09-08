import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../internal/plugin/ui/guide.js', import.meta.url), 'utf8');
function context(extra = {}) {
  const ctx = vm.createContext({
    state: {view: 'guide'}, items: value => Array.isArray(value) ? value : [],
    programIsGuidePlaceholder: program => program.title === 'Data not available',
    ...extra
  });
  vm.runInContext(source, ctx);
  return ctx;
}

test('program search respects channel scope, time window, deduplication and result limit', () => {
  const ctx = context();
  const program = (id, start, title = 'Evening News') => ({id, title, startUnix: start, endUnix: start + 30});
  const programs = {
    allowed: [program('expired', 50), program('duplicate', 100), program('duplicate', 100), program('future', 1000), program('placeholder', 120, 'Data not available'),
      ...Array.from({length: 25}, (_, i) => program('p' + i, 150 + i * 20)).reverse()],
    hidden: [program('hidden', 110)]
  };
  const found = ctx.guideSearchPrograms([{id: 'allowed'}], programs, 'nEwS', {start: 100, end: 1000}, 20);
  assert.equal(found.total, 26);
  assert.equal(found.entries.length, 20);
  assert.equal(found.entries[0].program.id, 'duplicate');
  assert.equal(found.entries[19].program.id, 'p18');
  assert.equal(ctx.guideSearchPrograms([{id: 'allowed'}], programs, 'n', {start: 100, end: 1000}).total, 0);
  assert.equal(ctx.guideSearchPrograms([{id: 'allowed'}], programs, 'available', {start: 100, end: 1000}).total, 0);
  programs.allowed.push({...program('description', 180, 'Magazine'), description: 'News analysis'});
  assert.equal(ctx.guideSearchPrograms([{id: 'allowed'}], programs, 'analysis', {start: 100, end: 1000}).entries[0].program.id, 'description');
});

test('viewport stays bounded and displays rows after a deep-scroll filter shrinks the list', () => {
  const ctx = context();
  for (const total of [0, 1, 5, 19, 1000]) {
    const range = ctx.guideVisibleRange(total, 60000, 900, 70, 42);
    assert.ok(range.start >= 0 && range.end <= total);
    assert.ok(range.end - range.start <= 40);
    if (total) assert.ok(range.end > range.start);
  }
  assert.equal(ctx.guideVisibleRange(5, 60000, 900, 70, 42).start, 0);
});

test('vertical navigation follows the same time across different program boundaries and gaps', () => {
  const ctx = context();
  const cells = [{kind: 'channel'}, {kind: 'program', start: 100, end: 150}, {kind: 'gap', start: 150, end: 200}, {kind: 'program', start: 200, end: 300}];
  assert.equal(ctx.guideCellIndexAtTime(cells, 125), 1);
  assert.equal(ctx.guideCellIndexAtTime(cells, 150), 2);
  assert.equal(ctx.guideCellIndexAtTime(cells, 200), 3);
  assert.equal(ctx.guideCellIndexAtTime(cells, 350), 3);
});

test('debounce ignores canceled, detached and navigated-away searches', () => {
  const timers = [], canceled = [];
  const ctx = context({setTimeout: (fn, delay) => {timers.push({fn, delay}); return timers.length;}, clearTimeout: id => canceled.push(id)});
  let applied = 0;
  ctx.applyGuideSearch = () => applied++;
  const input = {value: 'news', isConnected: true};
  ctx.scheduleGuideSearch(input);
  input.value = 'football';
  ctx.scheduleGuideSearch(input);
  assert.equal(timers[1].delay, 300);
  assert.ok(canceled.includes(1));
  timers[0].fn();
  assert.equal(applied, 0);
  timers[1].fn();
  assert.equal(applied, 1);
  ctx.scheduleGuideSearch(input);
  input.isConnected = false;
  timers[2].fn();
  assert.equal(applied, 1);
  input.isConnected = true;
  ctx.scheduleGuideSearch(input);
  ctx.state.view = 'home';
  timers[3].fn();
  assert.equal(applied, 1);
});

test('keyboard moves beyond a rendered viewport and retains the time anchor', () => {
  const target = {closest: () => row};
  const row = {getAttribute: () => '27'};
  const ctx = context({byId: () => ({clientHeight: 700})});
  ctx.state.guideChannels = Array.from({length: 100}, (_, i) => ({id: String(i)}));
  ctx.state.guideFocus = {anchorUnix: 550};
  ctx.guideFocusInfo = () => ({startUnix: 500, kind: 'program'});
  ctx.guideRowFocusCells = () => [target];
  ctx.guideRowHeight = () => 70;
  const moves = [];
  ctx.focusGuideRow = (...args) => moves.push(args);
  for (const key of ['ArrowDown', 'PageDown', 'PageUp']) {
    assert.equal(ctx.handleGuideKeyboard({target, key, preventDefault() {}}), true);
  }
  assert.deepEqual(moves, [[28, 550, 'program'], [36, 550, 'program'], [18, 550, 'program']]);
});

test('program and gap markup expose time boundaries for navigation', () => {
  const ctx = context({
    guideWindow: () => ({start: 100, end: 300}), channelMatchesQuery: () => true,
    programsFor: () => [{id: 'test', title: 'News', startUnix: 120, endUnix: 240}],
    recordingSchedulingEnabled: () => false, guideUnavailableLabel: () => 'Unavailable',
    timeLabel: value => String(value), escapeHTML: value => String(value), epgCellStyle: () => ''
  });
  const markup = ctx.renderEPGCells({id: 'channel'}, 0);
  assert.match(markup, /data-guide-focus="program" data-guide-start="120" data-guide-end="240"/);
  assert.match(markup, /data-guide-focus="gap" data-guide-start="100" data-guide-end="120"/);
});
