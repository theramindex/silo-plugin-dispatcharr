import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const player = fs.readFileSync(new URL('../internal/plugin/ui/player.js', import.meta.url), 'utf8');
const app = ['core', 'prefs', 'sports', 'admin', 'multiview', 'app'].map(name => fs.readFileSync(new URL('../internal/plugin/ui/' + name + '.js', import.meta.url), 'utf8')).join('\n');
function node() {
  return {innerHTML: '', textContent: '', attrs: {}, classList: {toggle() {}}, setAttribute(k, v) {this.attrs[k] = v;}};
}
function fixture() {
  const nodes = Object.fromEntries(['player-center-button', 'player-timeshift-play', 'player-timeshift-controls', 'player-timeshift-range', 'player-timeshift-label', 'player-mode-tag'].map(id => [id, node()]));
  let end = 120;
  const video = nodes.player = {paused: false, ended: false, currentTime: 114, seekable: {length: 1, start: () => 0, end: () => end}, play() {this.paused = false; return Promise.resolve();}};
  const state = {timeShiftSession: {ready: true}, hls: {latestLevelDetails: {targetduration: 6}, liveSyncPosition: 114}, playerWaiting: false};
  const ctx = vm.createContext({state, document: {addEventListener() {}}, byId: id => nodes[id], icon: name => name, isRewindableChannel: () => false});
  vm.runInContext(player.slice(0, player.indexOf('document.addEventListener("fullscreenchange"')), ctx);
  vm.runInContext(app.slice(app.indexOf('function timeShiftSeek('), app.indexOf('function boundedHLSBufferSeconds(')), ctx);
  return {nodes, video, state, ctx, setEnd(value) {end = value;}};
}

test('transport reflects autoplay, buffering, pause, resume and ended state', () => {
  const f = fixture();
  for (const [paused, waiting, ended, label] of [[false,false,false,'Pause'],[false,true,false,'Pause'],[true,false,false,'Play'],[false,false,false,'Pause'],[false,false,true,'Play']]) {
    Object.assign(f.video, {paused, ended}); f.state.playerWaiting = waiting;
    vm.runInContext('updateCenterPlayButton()', f.ctx);
    assert.equal(f.nodes['player-timeshift-play'].attrs['aria-label'], label);
    assert.equal(f.nodes['player-timeshift-play'].innerHTML, label.toLowerCase());
  }
});

test('LIVE stays stable across segment publication while actual rewind remains visible', () => {
  const f = fixture();
  for (const [end, time] of [[120,114],[126,114.1],[126,118],[126,123],[132,123.1]]) {
    f.setEnd(end); f.video.currentTime = time;
    vm.runInContext('updateTimeShiftUI()', f.ctx);
    assert.equal(f.nodes['player-timeshift-label'].textContent, 'LIVE');
  }
  vm.runInContext('timeShiftSeek(-30)', f.ctx);
  assert.match(f.nodes['player-timeshift-label'].textContent, /^-0:/);
  f.video.currentTime = 0;
  vm.runInContext('updateTimeShiftUI()', f.ctx);
  assert.equal(f.nodes['player-timeshift-range'].value, '0', 'zero is a valid playback position');
  vm.runInContext('timeShiftGoLive()', f.ctx);
  assert.equal(f.video.currentTime, 114, 'go live uses the HLS safe sync position');
});

test('paused playback does not advertise LIVE', () => {
  const f = fixture(); f.video.paused = true; f.video.currentTime = 119;
  vm.runInContext('updateTimeShiftUI()', f.ctx);
  assert.notEqual(f.nodes['player-timeshift-label'].textContent, 'LIVE');
});

test('LIVE follows the increased HLS latency target after a startup stall', () => {
  const f = fixture();
  f.state.hls.latestLevelDetails.targetduration = 2;
  f.state.hls.targetLatency = 4;
  for (const time of [112.1, 114, 116, 118]) {
    f.video.currentTime = time;
    vm.runInContext('updateTimeShiftUI()', f.ctx);
    assert.equal(f.nodes['player-timeshift-label'].textContent, 'LIVE');
  }
  f.video.paused = true;
  vm.runInContext('updateCenterPlayButton()', f.ctx);
  f.video.paused = false; f.setEnd(126);
  vm.runInContext('updateCenterPlayButton()', f.ctx);
  assert.equal(f.nodes['player-timeshift-label'].textContent, '-0:08', 'resuming a deliberate pause retains the offset');
});

test('managed rewind disables automatic catch-up while regular live playback retains it', () => {
  class Hls {
    static isSupported() {return true;}
    static Events = {};
    constructor(config) {this.config = config;}
    on() {} loadSource() {} attachMedia() {}
  }
  const ctx = vm.createContext({window: {Hls}, Hls, applyCoreMediaRequest() {}});
  vm.runInContext(app.slice(app.indexOf('function boundedHLSBufferSeconds('), app.indexOf('function overflowTooltip(')), ctx);
  const attach = options => ctx.attachVideoSource({}, '/test.m3u8', options).hls.config;
  assert.equal(attach({managedTimeShift: true}).liveMaxLatencyDurationCount, Infinity);
  assert.equal(attach({hlsBufferSeconds: 12}).liveMaxLatencyDuration, 24);
});

test('menu retains useful actions and recent channels without duplicate casting', () => {
  const f = fixture();
  f.nodes['player-more-menu'] = node(); f.state.moreMenuOpen = true;
  Object.assign(f.ctx, {updatePlayerChrome() {}, prefs: () => ({recentChannels: ['one']}), items: x => x,
    channelByID: id => ({id, name: 'A long channel name', categoryName: 'Sports'}),
    sportsFirstPlayerActive: () => false, menuIcon: () => '', logoHTML: () => '<img>', escapeHTML: x => x});
  vm.runInContext('renderPlayerMoreMenu()', f.ctx);
  const html = f.nodes['player-more-menu'].innerHTML;
  assert.doesNotMatch(html, /AirPlay|data-player-action="cast"|Video & audio casting/);
  assert.match(html, /Recent channels/);
  assert.match(html, /data-player-action="fullscreen"/);
  assert.match(html, /data-player-action="copy-stream"/);
});

test('codec failure converts once; stop and channel changes cancel late conversion', async () => {
  const posts = [];
  let finishStart;
  let convertState = 'ready';
  const state = {view: 'player', currentChannel: {id: 'one'}, timeShiftAttempt: 0};
  const sources = [];
  const errors = [];
  const ctx = vm.createContext({state, document: {addEventListener() {}}, byId: () => null,
    postJSON(path, body) {
      posts.push([path, body]);
      if (path.endsWith('/start')) return new Promise(resolve => {finishStart = resolve;});
      return Promise.resolve({});
    },
    getJSON: async () => ({state: convertState}),
    ensurePlayerLibraries: async () => {},
    clearPlayerError() {}, showPlayerToast() {}, updateTimeShiftUI() {},
    setVideoSource: (url, options) => sources.push([url, options]),
    showPlayerError: message => errors.push(message), route: path => '/plugin' + path,
    handlePlaybackFatalError() {}, setTimeout, clearTimeout,
    setInterval: () => 1, clearInterval() {}
  });
  vm.runInContext(player.slice(0, player.indexOf('document.addEventListener("fullscreenchange"')), ctx);
  // Keep the real conversion lifecycle while excluding presentation helpers.
  ctx.setVideoSource = (url, options) => sources.push([url, options]);
  ctx.showPlayerError = message => errors.push(message);
  ctx.showPlayerToast = () => {};
  ctx.clearPlayerError = () => {};
  ctx.updateTimeShiftUI = () => {};
  assert.equal(ctx.tryCompatiblePlayback({type: 'NetworkError', status: 503}), false);
  assert.equal(ctx.tryCompatiblePlayback({type: 'mediaError'}), true);
  assert.equal(ctx.tryCompatiblePlayback({code: 4}), false, 'one attempt per selection');
  finishStart({sessionId: 'old'});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(sources.length, 1);
  assert.equal(sources[0][1].format, 'hls');
  assert.match(sources[0][0], /session_id=old/);
  assert.equal(ctx.tryCompatiblePlayback({code: 3}), false, 'conversion cannot recursively convert itself');
  ctx.stopCompatibilitySession();
  assert.ok(posts.some(([path, body]) => path.endsWith('/stop') && body.sessionId === 'old'));
  state.currentChannel = {id: 'two'};
  assert.equal(ctx.tryCompatiblePlayback(null, true), true, 'manual audio compatibility uses the same lifecycle');
  ctx.stopCompatibilitySession();
  state.currentChannel = {id: 'three'};
  finishStart({sessionId: 'late'});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(sources.length, 1, 'late completion did not replace the new channel');
  assert.ok(posts.some(([path, body]) => path.endsWith('/stop') && body.sessionId === 'late'));
  assert.deepEqual(errors, []);
});
