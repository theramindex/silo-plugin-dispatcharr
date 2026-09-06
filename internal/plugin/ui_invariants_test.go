package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSportsScoresVisibleWithStaleProviderStatus(t *testing.T) {
	t.Parallel()
	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs() };`,
		`const event = { status:"airing", live:true, home:{name:"Michigan"}, away:{name:"Western Michigan"}, homeScore:"7", awayScore:"3" };`,
		`const html = renderSportsDetailScore(event);`,
		`state.app.preferences.sportsSpoilersHidden = true;`,
		`const hidden = renderSportsDetailScore(event);`,
		`globalThis.__result = { stableResults: html.includes("<b>7</b>") && html.includes("<b>3</b>") && !hidden.includes("<b>7</b>") && !sportsEventHasScores({live:true}) && sportsEventHasScores({homeScore:0, awayScore:0}) };`,
	})
	if !result.StableResults {
		t.Fatal("available scores must display despite stale status, respect spoilers, and avoid invented zeroes")
	}
}

func TestMyTVCollegePassesShowSeparateCompetitions(t *testing.T) {
	t.Parallel()
	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs() };`,
		`state.sports = { events: [
		  { leagueName:"College Football", home:{id:"michigan-football", name:"Michigan"} },
		  { leagueName:"Men's College Basketball", home:{id:"michigan-mens-basketball", name:"Michigan"} },
		  { leagueName:"Women's College Basketball", home:{id:"michigan-womens-basketball", name:"Michigan"} }
		], leagues: [] };`,
		`state.app.preferences.sportsFavoriteTeams["michigan-football"] = true;`,
		`const teams = myTVSportsPeople().filter(team => team.name === "Michigan");`,
		`globalThis.__result = { stableResults: teams.length === 3 && new Set(teams.map(myTVSportsPassLabel)).size === 3 && teams.filter(sportsFavoriteTeamMatches).length === 1 && myTVFollowedPeople()[0].leagueName === "College Football" };`,
	})
	if !result.StableResults {
		t.Fatal("college search results and active passes must distinguish sports and men's/women's competitions")
	}
}

func TestSavedGamePassKeepsIdentityWhenLiveTeamArrives(t *testing.T) {
	t.Parallel()
	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs() };`,
		`const id = "gamepass:mlb:new-york-yankees";`,
		`state.app.preferences.sportsFavoriteTeams[id] = true;`,
		`state.sports = { events: [{ leagueName:"MLB", home: { id:"live-yankees", name:"New York Yankees" } }], leagues: [{id:"mlb", name:"MLB"}] };`,
		`state.app.preferences.sportsFavoriteLeagues.mlb = true;`,
		`const html = myTVFollowingHTML();`,
		`globalThis.__result = { stableResults: html.includes("New York Yankees") && html.includes("MLB game pass") && html.includes(id) && html.includes("teamlogo.png") && html.includes("Remove MLB pass") && !html.includes("Saved team") };`,
	})
	if !result.StableResults {
		t.Fatal("saved game passes must retain their team identity when search deduplicates a live roster entry")
	}
}

func TestMyTVRosterUpdatesPreserveUnchangedResults(t *testing.T) {
	t.Parallel()
	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs(), channels: [], categories: [], programs: [], status: {} };`,
		`state.sports = { events: [], leagues: [{id:"mlb", name:"MLB"}, {id:"nba", name:"NBA"}] };`,
		`state.view = "mytv"; state.myTVQuery = "nationals";`,
		`const results = document.getElementById("my-tv-search-results");`,
		`let writes = 0;`,
		`Object.defineProperty(results, "innerHTML", { get() { return this.html || ""; }, set(value) { this.html = value; writes++; } });`,
		`updateMyTVSearchSurface();`,
		`state.myTVTeamCatalogLoading = Promise.resolve();`,
		`state.sportsLeagueTeams.nba = [{ id:"nba-1", name:"Boston Celtics" }];`,
		`updateMyTVSearchSurface(); updateMyTVSearchSurface();`,
		`state.myTVTeamCatalogLoading = null; updateMyTVSearchSurface();`,
		`const stableResults = writes === 1 && results.innerHTML.includes("Washington Nationals");`,
		`state.myTVQuery = "yankees"; updateMyTVSearchSurface();`,
		`globalThis.__result = { stableResults: stableResults && writes === 2 && results.innerHTML.includes("New York Yankees") && !results.innerHTML.includes("Washington Nationals") };`,
	})
	if !result.StableResults {
		t.Fatal("unrelated roster updates must preserve the existing game-pass controls, while a new query must update them")
	}
}

func TestMenuNavigationAfterSeparateScriptLoads(t *testing.T) {
	t.Parallel()
	result := runUIInvariantScript(t, []string{
		`render = function() {};`,
		`const getElementById = document.getElementById;`,
		`document.getElementById = function(id) { return id === "player" ? null : getElementById(id); };`,
		`const button = document.getElementById("primary-browse-nav");`,
		`button.onclick();`,
		`globalThis.__result = { menuWorks: state.view === "channels" };`,
	})
	if !result.MenuWorks {
		t.Fatal("Channels menu must navigate after loading the page scripts separately")
	}
}

func TestGuideSearchDoesNotRecreateCommandBar(t *testing.T) {
	t.Parallel()

	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs(), source: { mode: "direct_login", profiles: [] }, channels: [{ id: "ch-1", name: "CNN", categoryId: "news", categoryName: "News" }, { id: "ch-2", name: "ESPN", categoryId: "sports", categoryName: "Sports" }], categories: [{ id: "news", name: "News" }, { id: "sports", name: "Sports" }], status: {} };`,
		`state.view = "guide";`,
		`rebuildProgramIndex();`,
		`const view = document.getElementById("view");`,
		`let viewWrites = 0;`,
		`let epgWrites = 0;`,
		`Object.defineProperty(view, "innerHTML", { configurable: true, get() { return this._html || ""; }, set(value) { this._html = String(value); viewWrites += 1; } });`,
		`renderGuidePage();`,
		`const epg = document.getElementById("epg");`,
		`Object.defineProperty(epg, "innerHTML", { configurable: true, get() { return this._html || ""; }, set(value) { this._html = String(value); epgWrites += 1; } });`,
		`const writesAfterGuide = viewWrites;`,
		`state.query = "cnn";`,
		`const keptToolbar = refreshGuideRowsForQuery();`,
		`globalThis.__result = { keptToolbar, viewWritesAfterSearch: viewWrites, epgWrites, wroteGuideShellOnce: writesAfterGuide === 1 };`,
	})
	if !result.KeptToolbar || !result.WroteGuideShellOnce || result.ViewWritesAfterSearch != 1 || result.EPGWrites == 0 {
		t.Fatalf("guide search must update rows without rebuilding the command bar: %+v", result)
	}
}

func TestSportsPollPreservesViewScroll(t *testing.T) {
	t.Parallel()

	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs(), source: { mode: "direct_login", profiles: [] }, channels: [{ id: "ch-1", name: "ESPN", categoryId: "sports", categoryName: "Sports" }], categories: [], status: {} };`,
		`state.adminCategorySettings = defaultAdminCategorySettings();`,
		`state.view = "sports";`,
		`state.sports = { events: [{ id: "game-1", name: "Jets at Giants", leagueId: "nfl", startUnix: Math.floor(Date.now() / 1000) - 60, live: true, channels: [{ id: "ch-1", name: "ESPN", confidence: "high" }] }], leagues: [], source: "sportarr" };`,
		`const view = document.getElementById("view");`,
		`Object.defineProperty(view, "innerHTML", { configurable: true, get() { return this._html || ""; }, set(value) { this._html = String(value); this.scrollTop = 0; } });`,
		`renderSportsPage();`,
		`view.scrollTop = 240;`,
		`renderSportsPage();`,
		`globalThis.__result = { scrollTop: view.scrollTop };`,
	})
	if result.ScrollTop != 240 {
		t.Fatalf("sports rerender must keep the page scroll, got %+v", result)
	}
}

func TestSportsAndEventsNavHideWhenEmpty(t *testing.T) {
	t.Parallel()

	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs(), source: { mode: "direct_login", profiles: [] }, channels: [], categories: [], status: {} };`,
		`state.adminCategorySettings = defaultAdminCategorySettings();`,
		`state.sports = { events: [], leagues: [] };`,
		`state.events = { events: [], categories: [] };`,
		`const buttons = {`,
		`  sports: { dataset: { view: "sports" }, hidden: false, classList: { toggle: () => {} }, setAttribute: () => {}, removeAttribute: () => {} },`,
		`  events: { dataset: { view: "events" }, hidden: false, classList: { toggle: () => {} }, setAttribute: () => {}, removeAttribute: () => {} }`,
		`};`,
		`document.querySelectorAll = function(selector) { return selector.indexOf("data-view") >= 0 ? [buttons.sports, buttons.events] : []; };`,
		`renderRail();`,
		`globalThis.__result = { sportsHidden: !!buttons.sports.hidden, eventsHidden: !!buttons.events.hidden };`,
	})
	if !result.SportsHidden || !result.EventsHidden {
		t.Fatalf("empty Sports and Events must leave the nav, got %+v", result)
	}
}

func TestSportsAndEventsNavIgnoreCurrentTabFilters(t *testing.T) {
	t.Parallel()

	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs(), source: { mode: "direct_login", profiles: [] }, channels: [], categories: [], status: {} };`,
		`state.adminCategorySettings = defaultAdminCategorySettings();`,
		`state.sportsTab = "live";`,
		`state.eventsTab = "live";`,
		`state.sports = { events: [{ id: "game-1", name: "Jets at Giants", leagueId: "nfl", startUnix: Math.floor(Date.now() / 1000) + 3600, live: false, channels: [{ id: "ch-1", name: "ESPN", confidence: "high" }] }], leagues: [] };`,
		`state.events = { events: [{ id: "awards-1", name: "The Oscars", startUnix: Math.floor(Date.now() / 1000) + 3600, live: false, channels: [{ id: "ch-1", name: "ABC", confidence: "high" }] }], categories: [] };`,
		`const buttons = {`,
		`  sports: { dataset: { view: "sports" }, hidden: false, classList: { toggle: () => {} }, setAttribute: () => {}, removeAttribute: () => {} },`,
		`  events: { dataset: { view: "events" }, hidden: false, classList: { toggle: () => {} }, setAttribute: () => {}, removeAttribute: () => {} }`,
		`};`,
		`document.querySelectorAll = function(selector) { return selector.indexOf("data-view") >= 0 ? [buttons.sports, buttons.events] : []; };`,
		`renderRail();`,
		`globalThis.__result = { sportsHidden: !!buttons.sports.hidden, eventsHidden: !!buttons.events.hidden };`,
	})
	if result.SportsHidden || result.EventsHidden {
		t.Fatalf("upcoming playable Sports and Events must keep the nav on the Live tab, got %+v", result)
	}
}

func TestBootFailureKeepsHydratedApp(t *testing.T) {
	t.Parallel()

	result := runUIInvariantScript(t, []string{
		`state.app = { preferences: defaultPrefs(), source: { mode: "direct_login", profiles: [] }, channels: [{ id: "ch-1", name: "CNN", categoryId: "news", categoryName: "News" }], categories: [{ id: "news", name: "News" }], status: {} };`,
		`const view = document.getElementById("view");`,
		`view.innerHTML = "cached-home";`,
		`render = function() { throw new Error("render failed"); };`,
		`console.error = function() {};`,
		`handleAppBootFailure(new Error("DataCloneError"));`,
		`globalThis.__result = { keptApp: !!state.app && view.innerHTML === "cached-home" };`,
	})
	if !result.KeptApp {
		t.Fatalf("boot failure must keep a hydrated app instead of the empty state: %+v", result)
	}
}

func TestBootFailureWithoutAppShowsRecoveryMessage(t *testing.T) {
	t.Parallel()

	result := runUIInvariantScript(t, []string{
		`console.error = function() {};`,
		`handleAppBootFailure(new Error("network failed"));`,
		`const view = document.getElementById("view");`,
		`globalThis.__result = { showedEmpty: view.innerHTML.includes("Unable to load Live TV.") && view.innerHTML.includes("Check your Dispatcharr connection") && view.getAttribute("role") === "status" };`,
	})
	if !result.ShowedEmpty {
		t.Fatal("boot failure without app data must show an accessible recovery message")
	}
}

func TestCommitAppRouteDoesNotCloneHistoryState(t *testing.T) {
	t.Parallel()

	result := runUIInvariantScript(t, []string{
		`window.history = {`,
		`  state: { silo: function() {}, nested: { fn: function() {} } },`,
		`  pushState: function(next) { globalThis.__pushed = next; },`,
		`  replaceState: function(next) { globalThis.__replaced = next; }`,
		`};`,
		`window.location.hash = "";`,
		`commitAppRoute("replace");`,
		`const replaced = globalThis.__replaced || {};`,
		`globalThis.__result = { clonedSilo: Object.prototype.hasOwnProperty.call(replaced, "silo"), routeKeyCount: Object.keys(replaced).length };`,
	})
	if result.ClonedSilo || result.RouteKeyCount != 1 {
		t.Fatalf("commitAppRoute must write only plugin route state: %+v", result)
	}
}

type uiInvariantResult struct {
	StableResults         bool `json:"stableResults"`
	MenuWorks             bool `json:"menuWorks"`
	KeptToolbar           bool `json:"keptToolbar"`
	ViewWritesAfterSearch int  `json:"viewWritesAfterSearch"`
	EPGWrites             int  `json:"epgWrites"`
	WroteGuideShellOnce   bool `json:"wroteGuideShellOnce"`
	ScrollTop             int  `json:"scrollTop"`
	SportsHidden          bool `json:"sportsHidden"`
	EventsHidden          bool `json:"eventsHidden"`
	ShowedEmpty           bool `json:"showedEmpty"`
	KeptApp               bool `json:"keptApp"`
	ClonedSilo            bool `json:"clonedSilo"`
	RouteKeyCount         int  `json:"routeKeyCount"`
}

func runUIInvariantScript(t *testing.T, statements []string) uiInvariantResult {
	t.Helper()

	dir := t.TempDir()
	runnerPath := filepath.Join(dir, "runner.js")
	page, err := playerUIAssets.ReadFile("ui/page.html")
	if err != nil {
		t.Fatal(err)
	}
	var scripts []string
	for _, match := range regexp.MustCompile(`<script defer src="__ASSET_PREFIX__/([^?]+)\?`).FindAllStringSubmatch(string(page), -1) {
		source, err := playerUIAssets.ReadFile("ui/" + match[1])
		if err != nil {
			t.Fatal(err)
		}
		scripts = append(scripts, string(source))
	}
	if len(scripts) == 0 {
		t.Fatal("page has no deferred scripts")
	}
	scriptsJSON, err := json.Marshal(scripts)
	if err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, statement := range statements {
		body += statement + "\n"
	}
	nodeScript := fmt.Sprintf(`
const vm = require("vm");
const scripts = %s;
function makeElement() {
  const attributes = {};
  const element = {
    innerHTML: "", textContent: "", value: "", scrollTop: 0, scrollLeft: 0, hidden: false, style: {}, dataset: {},
    classList: { add: () => {}, remove: () => {}, toggle: () => {}, contains: () => false },
    setAttribute: (name, value) => { attributes[name] = String(value); },
    getAttribute: (name) => attributes[name] || null,
    removeAttribute: (name) => { delete attributes[name]; },
    querySelector: () => makeElement(),
    querySelectorAll: () => [],
    addEventListener: () => {},
    closest: () => null,
    focus: () => {}
  };
  return element;
}
const elements = {};
elements["primary-browse-nav"] = makeElement();
elements["primary-browse-nav"].dataset.view = "channels";
const sandbox = {
  window: { location: { pathname: "/api/v1/plugins/14/dispatcharr", search: "" }, addEventListener: () => {}, innerHeight: 800, scrollY: 0 },
  document: {
    documentElement: { dataset: {} },
    body: makeElement(),
    activeElement: null,
    hidden: false,
    querySelectorAll: (selector) => selector === "[data-view]" ? [elements["primary-browse-nav"]] : [],
    querySelector: () => makeElement(),
    getElementById: (id) => elements[id] = elements[id] || makeElement(),
    addEventListener: () => {},
    contains: () => true
  },
  localStorage: { getItem: () => null, setItem: () => {} },
  sessionStorage: { getItem: () => null, setItem: () => {}, removeItem: () => {} },
  navigator: { sendBeacon: () => true },
  console, URLSearchParams,
  requestAnimationFrame: (callback) => { callback(); return 1; },
  cancelAnimationFrame: () => {},
  getComputedStyle: () => ({ getPropertyValue: () => "", fontSize: "16px" }),
  setTimeout, clearTimeout, setInterval, clearInterval,
  fetch: async () => ({ ok: true, status: 200, text: async () => "{}", json: async () => ({}) })
};
vm.createContext(sandbox);
for (const source of scripts) {
  vm.runInContext(source.replace(/startGuideAutoRefresh\(\);[\s\S]*$/, ""), sandbox);
}
vm.runInContext(%s, sandbox);
process.stdout.write(JSON.stringify(sandbox.__result || {}));
`, scriptsJSON, strconvQuote(body))
	if err := os.WriteFile(runnerPath, []byte(nodeScript), 0o600); err != nil {
		t.Fatalf("write ui invariant runner: %v", err)
	}
	output, err := exec.Command("node", runnerPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run ui invariant script: %v\n%s", err, output)
	}
	var result uiInvariantResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode ui invariant result: %v\n%s", err, output)
	}
	return result
}

func strconvQuote(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
