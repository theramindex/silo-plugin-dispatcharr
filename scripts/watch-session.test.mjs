import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const read = name => fs.readFileSync(new URL('../internal/plugin/ui/' + name, import.meta.url), 'utf8');
const player = read('player.js');
const app = read('app.js');

function fn(source, name) {
  const start = source.indexOf('function ' + name + '(');
  assert.ok(start >= 0, name + ' must exist');
  // Function bodies end at the first top-level closing brace.
  return source.slice(start, source.indexOf('\n}\n', start) + 2);
}

function setup() {
  const calls = [];
  const pending = [];
  let next = 0;
  const ctx = vm.createContext({
    state: { currentSession: null, heartbeat: null },
    setInterval: () => 1, clearInterval() {},
    recordWatchPreference() {}, renderRail() {},
    postJSON(url, body) {
      calls.push({ url, body });
      if (url !== '/dispatcharr/api/watch/start') return Promise.resolve({});
      let resolve;
      const promise = new Promise(r => { resolve = r; });
      pending.push(() => resolve({ session: { id: 'session-' + (++next) } }));
      return promise;
    }
  });
  vm.runInContext(fn(player, 'startWatch') + '\n' + fn(app, 'stopCurrentWatch'), ctx);
  return { ctx, calls, pending };
}
const flush = () => new Promise(resolve => setImmediate(resolve));
const stops = calls => calls.filter(call => call.url === '/dispatcharr/api/watch/stop').map(call => call.body.sessionId);

test('a /watch/start that resolves after a newer switch is stopped immediately', async () => {
  const { ctx, calls, pending } = setup();
  ctx.startWatch({ id: 'a', name: 'A' });
  ctx.startWatch({ id: 'b', name: 'B' });
  pending[1]();
  await flush();
  pending[0]();
  await flush();
  assert.equal(ctx.state.currentSession.id, 'session-1', 'B (resolved first) stays current');
  assert.deepEqual(stops(calls), ['session-2'], 'the stale A session is stopped, not orphaned');
});

test('leaving the player while /watch/start is in flight stops the late session', async () => {
  const { ctx, calls, pending } = setup();
  ctx.startWatch({ id: 'a', name: 'A' });
  ctx.stopCurrentWatch('leave_player');
  pending[0]();
  await flush();
  assert.equal(ctx.state.currentSession, null);
  assert.deepEqual(stops(calls), ['session-1']);
});
