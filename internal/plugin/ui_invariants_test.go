package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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

type uiInvariantResult struct {
	KeptToolbar           bool `json:"keptToolbar"`
	ViewWritesAfterSearch int  `json:"viewWritesAfterSearch"`
	EPGWrites             int  `json:"epgWrites"`
	WroteGuideShellOnce   bool `json:"wroteGuideShellOnce"`
	ScrollTop             int  `json:"scrollTop"`
	SportsHidden          bool `json:"sportsHidden"`
	EventsHidden          bool `json:"eventsHidden"`
}

func runUIInvariantScript(t *testing.T, statements []string) uiInvariantResult {
	t.Helper()

	dir := t.TempDir()
	appScriptPath := filepath.Join(dir, "app.js")
	runnerPath := filepath.Join(dir, "runner.js")
	if err := os.WriteFile(appScriptPath, []byte(extractPlayerScript(t)), 0o600); err != nil {
		t.Fatalf("write app script: %v", err)
	}
	body := ""
	for _, statement := range statements {
		body += statement + "\n"
	}
	nodeScript := fmt.Sprintf(`
const fs = require("fs");
const vm = require("vm");
const source = fs.readFileSync(%q, "utf8").replace(/startGuideAutoRefresh\(\);[\s\S]*$/, "");
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
const sandbox = {
  window: { location: { pathname: "/api/v1/plugins/14/dispatcharr", search: "" }, addEventListener: () => {}, innerHeight: 800, scrollY: 0 },
  document: {
    documentElement: { dataset: {} },
    body: makeElement(),
    activeElement: null,
    hidden: false,
    querySelectorAll: () => [],
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
vm.runInContext(source + "\n" + %s, sandbox);
process.stdout.write(JSON.stringify(sandbox.__result || {}));
`, appScriptPath, strconvQuote(body))
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
