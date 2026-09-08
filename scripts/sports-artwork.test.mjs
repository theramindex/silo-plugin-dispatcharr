import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../internal/plugin/ui/app.js', import.meta.url), 'utf8');
function context() {
  const ctx = vm.createContext({
    state: {sportsFailedMedia: {}}, safeSportsMediaURL: value => String(value || ''),
    escapeHTML: value => String(value || ''), sportsPreferredLogo: () => '',
    sportsLeagueFallbackMark: () => 'League', icon: () => '',
    sportsTeamName: team => team?.name || '', sportsEventTitle: event => event.name || 'Event'
  });
  for (const name of ['sportsMediaFailed', 'sportsFieldBackgroundKind', 'sportsFieldBackgroundURL', 'renderSportsBackground', 'sportsGeneratedBackground', 'markSportsBackgroundFailed', 'renderSportsProgramThumbnail', 'renderSportsRaceThumbnail']) {
    const start = source.indexOf('function ' + name + '(');
    const end = source.indexOf('\nfunction ', start + 1);
    assert.ok(start >= 0 && end > start, name);
    vm.runInContext(source.slice(start, end), ctx);
  }
  return ctx;
}

test('sport-specific photos cover field, court, combat, race and individual events', () => {
  const ctx = context();
  for (const [sportName, kind] of [
    ['Baseball', 'baseball'], ['Softball', 'baseball'], ['Basketball', 'basketball'],
    ['American Football', 'football'], ['Soccer', 'soccer'], ['Ice Hockey', 'hockey'],
    ['Field Hockey', 'field-hockey'], ['Tennis', 'tennis'], ['Table Tennis', 'table-tennis'],
    ['Cricket', 'cricket'], ['Rugby Union', 'rugby'], ['Rugby League', 'rugby'],
    ['Golf', 'golf'], ['Volleyball', 'volleyball'], ['Badminton', 'badminton'],
    ['Motorsport', 'motorsport'], ['Motorcycle Racing', 'motorsport'],
    ['Boxing', 'boxing'], ['Mixed Martial Arts', 'mma'], ['Swimming', 'swimming'],
    ['Water Polo', 'swimming'], ['Cycling', 'cycling'], ['Skiing', 'skiing'],
    ['Snowboarding', 'skiing'], ['Track and Field', 'athletics'],
    ['Horse Racing', 'equestrian'], ['Darts', 'darts'], ['Snooker', 'snooker']
  ]) {
    assert.equal(ctx.sportsFieldBackgroundKind({sportName}), kind, sportName);
    assert.match(ctx.sportsFieldBackgroundURL({sportName}), /^https:\/\/images\.unsplash\.com\/(?:flagged\/)?photo-[^?]+\?auto=format&fit=crop&w=1200&q=80$/, sportName);
  }
});

test('league aliases disambiguate football and preserve specific sport metadata', () => {
  const ctx = context();
  for (const [event, expected] of [
    [{sportName: 'Football', leagueId: 'nfl'}, 'football'],
    [{sportName: 'Football', leagueId: 'cfl'}, 'football'],
    [{sportName: 'Football', leagueId: 'epl'}, 'soccer'],
    [{sportName: 'Football', leagueId: 'afl'}, 'cricket'],
    [{leagueId: 'college-womens-field-hockey'}, 'field-hockey'],
    [{leagueId: 'college-mens-basketball'}, 'basketball'],
    [{leagueId: 'fiba-women'}, 'basketball'],
    [{leagueId: 'wnba'}, 'basketball'],
    [{leagueId: 'milb-pacific-coast-league'}, 'baseball'],
    [{leagueId: 'npb'}, 'baseball'],
    [{leagueId: 'nhl'}, 'hockey'],
    [{leagueId: 'atp'}, 'tennis'],
    [{leagueId: 'lpga'}, 'golf'],
    [{leagueId: 'f1'}, 'motorsport'],
    [{leagueId: 'motogp'}, 'motorsport'],
    [{leagueId: 'ufc'}, 'mma'],
    [{leagueId: 'pdc'}, 'darts'],
    [{sportName: 'Cricket', leagueName: 'Premier League'}, 'cricket'],
    [{sportName: 'Table Tennis', leagueName: 'Tennis'}, 'table-tennis']
  ]) assert.equal(ctx.sportsFieldBackgroundKind(event), expected, JSON.stringify(event));
  assert.equal(ctx.sportsFieldBackgroundURL({sportName: 'Unknown sport'}), '');
  assert.equal(ctx.sportsFieldBackgroundURL(null), '');
});

test('missing or failed photos retain GameThumbs backgrounds', () => {
  const ctx = context();
  const event = {sportName: 'Basketball', gameThumbsBackgroundUrl: 'https://game-thumbs.swvn.io/nba/lakers/celtics/thumb.png'};
  const photo = ctx.sportsFieldBackgroundURL(event);
  assert.match(ctx.renderSportsBackground(event), /class="sports-field-bg"/);
  assert.match(ctx.renderSportsBackground(event), /data-sports-background-fallback="https:\/\/game-thumbs/);
  ctx.state.sportsFailedMedia[photo] = true;
  assert.match(ctx.renderSportsBackground(event), /class="sports-generated-bg"/);
  assert.match(ctx.renderSportsBackground({...event, sportName: 'Unknown'}), /class="sports-generated-bg"/);
  ctx.state.sportsFailedMedia[event.gameThumbsBackgroundUrl] = true;
  assert.equal(ctx.renderSportsBackground(event), '');
});

test('photo failure falls back once, then preserves the card when both images fail', () => {
  const ctx = context();
  const photo = ctx.sportsFieldBackgroundURL({sportName: 'Soccer'});
  const fallback = 'https://game-thumbs.swvn.io/epl/thumb.png';
  const attrs = {src: photo, 'data-sports-background-fallback': fallback};
  const classes = new Set(['sports-field-bg']);
  let removed = '';
  const image = {
    hidden: false,
    getAttribute: name => attrs[name], setAttribute: (name, value) => {attrs[name] = value;},
    removeAttribute: name => {delete attrs[name];},
    classList: {remove: value => classes.delete(value), add: value => classes.add(value)},
    parentElement: {classList: {remove: value => {removed = value;}}}
  };
  ctx.markSportsBackgroundFailed(image);
  assert.equal(attrs.src, fallback);
  assert.equal(attrs['data-sports-background-fallback'], undefined);
  assert.equal(image.hidden, false);
  assert.ok(classes.has('sports-generated-bg'));
  assert.ok(ctx.state.sportsFailedMedia[photo]);
  ctx.markSportsBackgroundFailed(image);
  assert.equal(image.hidden, true);
  assert.equal(removed, 'has-generated-art');
  assert.ok(ctx.state.sportsFailedMedia[fallback]);
});

test('race and program cards use the same photo selection as matchups', () => {
  const ctx = context();
  const race = ctx.renderSportsRaceThumbnail({sportName: 'Motorsport', leagueId: 'f1'});
  const program = ctx.renderSportsProgramThumbnail({sportName: 'Boxing', leagueId: 'boxing'});
  assert.match(race, /sports-field-bg/);
  assert.match(race, /sports-race-copy/);
  assert.match(program, /sports-field-bg/);
  assert.match(program, /sports-program-copy/);
});
