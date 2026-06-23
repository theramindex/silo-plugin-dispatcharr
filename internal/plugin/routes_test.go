package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestHTTPRoutesServerStatusRoute(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{{ID: "xtream:1", Name: "News HD"}},
			Programs: []model.Program{{ID: "program:1", ChannelID: "xtream:1", Title: "Morning News", StartUnix: 1700000000}},
		},
		Health: model.SyncHealth{LastSuccessUnix: 123},
	})
	server := NewHTTPRoutesServer(store)

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/status"})
	if err != nil {
		t.Fatalf("handle route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}

	var payload HealthPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.SourceName != "Live TV" || payload.ChannelCount != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestHTTPRoutesServerChannelsAndGuideRoutes(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{
				{ID: "xtream:1", Name: "News HD"},
			},
			Programs: []model.Program{
				{ID: "program:2", ChannelID: "xtream:1", Title: "Late News", StartUnix: 1700003600},
				{ID: "program:1", ChannelID: "xtream:1", Title: "Morning News", StartUnix: 1700000000},
			},
		},
	})
	server := NewHTTPRoutesServer(store)

	channelsResponse, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/channels"})
	if err != nil {
		t.Fatalf("channels route: %v", err)
	}
	if channelsResponse.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", channelsResponse.GetStatusCode())
	}
	var channelsPayload ChannelsPayload
	if err := json.Unmarshal(channelsResponse.GetBody(), &channelsPayload); err != nil {
		t.Fatalf("unmarshal channels payload: %v", err)
	}
	if len(channelsPayload.Channels) != 1 || channelsPayload.Channels[0].Name != "News HD" {
		t.Fatalf("unexpected channels payload: %+v", channelsPayload)
	}

	query, _ := structpb.NewStruct(map[string]any{"channel_id": "xtream:1"})
	guideResponse, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/guide", Query: query})
	if err != nil {
		t.Fatalf("guide route: %v", err)
	}
	if guideResponse.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", guideResponse.GetStatusCode())
	}
	var guidePayload GuidePayload
	if err := json.Unmarshal(guideResponse.GetBody(), &guidePayload); err != nil {
		t.Fatalf("unmarshal guide payload: %v", err)
	}
	if len(guidePayload.Programs) != 2 || guidePayload.Programs[0].Title != "Morning News" {
		t.Fatalf("unexpected guide payload: %+v", guidePayload)
	}
}

func TestHTTPRoutesServerAppRouteIncludesAppLayerPayload(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{
				{ID: "xtream:1", Name: "News HD", CategoryID: "10", CategoryName: "News"},
			},
			Programs: []model.Program{
				{ID: "program:1", ChannelID: "xtream:1", Title: "Morning News", StartUnix: 100, EndUnix: 200},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{{ID: "10", Name: "News", Kind: "live"}},
				VODCategories:  []model.Category{{ID: "movies", Name: "Movies", Kind: "vod"}},
				VODItems:       []model.VODItem{{ID: "vod:2001", Name: "Example Movie", Container: "mp4"}},
				SeriesItems:    []model.SeriesItem{{ID: "series:3001", Name: "Example Series"}},
			},
		},
	})
	server := NewHTTPRoutesServer(store)

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/api/app"})
	if err != nil {
		t.Fatalf("app route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	if !strings.Contains(string(response.GetBody()), `"id":"xtream:1"`) {
		t.Fatalf("expected lower-case JSON field names, got %s", string(response.GetBody()))
	}
	var payload AppPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal app payload: %v", err)
	}
	if !payload.Capabilities.LiveTV || payload.Capabilities.NativeLiveTVExport || payload.Capabilities.Recordings {
		t.Fatalf("unexpected capabilities: %+v", payload.Capabilities)
	}
	if len(payload.Categories) != 1 || len(payload.Channels) != 1 || len(payload.Programs) != 1 {
		t.Fatalf("unexpected app payload: %+v", payload)
	}
}

func TestHTTPRoutesServerAppPageIncludesVirtualFolderDrilldown(t *testing.T) {
	t.Parallel()

	response, err := NewHTTPRoutesServer(cache.NewStore()).Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "GET",
		Path:   "/dispatcharr",
		Query:  &structpb.Struct{Fields: map[string]*structpb.Value{"theme": structpb.NewStringValue("midnight-cinema")}},
	})
	if err != nil {
		t.Fatalf("app route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	body := string(response.GetBody())
	for _, want := range []string{
		`function sourceVirtualChildCategories(parentPath, includeChannel)`,
		`function virtualCategoriesFromPaths(parentPath, includeChannel, includeAllDescendants)`,
		`if (state.category.indexOf("virtual:") === 0)`,
		`const children = virtualChildCategories(path,`,
		`virtualFolderBreadcrumbs(path)`,
		`Virtual Categories</button>`,
		`const showSourceCategorySettings = !virtualCategoriesActive()`,
		`Saved on this device, but not to your Silo profile.`,
		`<span>Preferences</span>`,
		`const isAdminRoute = path.endsWith("/dispatcharr/admin")`,
		`if (state.view === "admin" && !isAdminRoute) state.view = "home"`,
		`delimiter: "pipe"`,
		`if (!settings.delimiter) settings.delimiter = "pipe"`,
		`function renderVirtualCategoryGuide(channels)`,
		`renderVirtualCategoryGuide(channels)`,
		`No channels in this virtual category yet.`,
		`sectionHeader("Virtual Categories")`,
		`data-silo-theme="midnight-cinema"`,
		`function applySiloTheme()`,
		`--silo-bg`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected app page to include virtual folder drilldown marker %q", want)
		}
	}
	if strings.Contains(body, "Saved on this device. Silo profile sync is unavailable here.") {
		t.Fatalf("expected local-only profile save message to use the standard warning")
	}
}

func TestHTTPRoutesServerAdminPageIncludesCategoryMapping(t *testing.T) {
	t.Parallel()

	response, err := NewHTTPRoutesServer(cache.NewStore()).Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/admin"})
	if err != nil {
		t.Fatalf("admin route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	body := string(response.GetBody())
	for _, want := range []string{
		`<title>Live TV Admin</title>`,
		`<h1>Live TV Admin</h1>`,
		`<div class="shell is-admin">`,
		`.shell.is-admin .nav, .shell.is-admin .topbar { display: none; }`,
		`const adminSettingsKey = "adminCategorySettings"`,
		`function defaultAdminCategorySettings()`,
		`function renderAdminPage()`,
		`Category method`,
		`Normal`,
		`By delimiter`,
		`Admin virtual folders + delimiter`,
		`data-admin-category-field=\"mode\"`,
		`data-admin-group-action=\"create\"`,
		`adminGroupMemberships`,
		`presentationOverrides`,
		`Admin virtual folder aliases`,
		`New alias folder`,
		`Edit alias folder`,
		`function effectiveChannel(channel)`,
		`function renderAdminPresentationSettings()`,
		`Presentation overrides`,
		`data-admin-presentation-field=\"name\"`,
		`data-admin-presentation-field=\"logoUrl\"`,
		`data-admin-presentation-field=\"hidden\"`,
		`data-admin-presentation-field=\"order\"`,
		`/dispatcharr/api/admin-settings`,
		`const adminSettingsToken = "`,
		`x-dispatcharr-admin-token`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected admin page to include category mapping marker %q", want)
		}
	}
	for _, hidden := range []string{`<span>Home</span>`, `<span>Favorites</span>`, `<span>TV Guide</span>`, `<span>Preferences</span>`} {
		if strings.Contains(body, hidden) {
			t.Fatalf("expected admin page shell to hide user nav marker %q", hidden)
		}
	}
}

func TestAdminVirtualFolderAliasesKeepSourceAndAliasPaths(t *testing.T) {
	t.Parallel()

	script := extractPlayerScript(t)
	context := map[string]any{
		"state": map[string]any{
			"app": map[string]any{
				"channels": []map[string]any{
					{"id": "channel:argentina-sports", "name": "Argentina Sports", "categoryId": "cat:argentina-sports", "categoryName": "International | Argentina | Sports"},
				},
				"categories": []map[string]any{
					{"id": "cat:argentina-sports", "name": "International | Argentina | Sports"},
				},
			},
			"adminCategorySettings": map[string]any{
				"mode":      "admin_delimiter",
				"delimiter": "pipe",
				"adminGroups": []map[string]any{
					{"id": "admin:sports-argentina", "name": "Sports | Argentina", "order": 1},
				},
				"adminGroupMemberships": map[string]any{
					"admin:sports-argentina": []string{"channel:argentina-sports"},
				},
				"presentationOverrides": map[string]any{},
			},
		},
	}

	result := runVirtualAliasScript(t, script, context)
	if !result.SourcePath {
		t.Fatalf("expected source path to remain visible: %+v", result)
	}
	if !result.AliasPath {
		t.Fatalf("expected admin alias path to be added: %+v", result)
	}
	if result.SourceCount != 1 || result.AliasCount != 1 {
		t.Fatalf("expected source and alias counts to be one channel each: %+v", result)
	}
}

func TestHTTPRoutesServerAppPageIncludesOrderedFavorites(t *testing.T) {
	t.Parallel()

	response, err := NewHTTPRoutesServer(cache.NewStore()).Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr"})
	if err != nil {
		t.Fatalf("app route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	body := string(response.GetBody())
	for _, want := range []string{
		`favoriteOrder: []`,
		`function orderedFavoriteChannels(`,
		`function moveFavorite(channelID, direction)`,
		`data-favorite-move=\"up\"`,
		`data-favorite-move=\"down\"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected app page to include ordered favorites marker %q", want)
		}
	}
}

type virtualAliasResult struct {
	SourcePath  bool `json:"sourcePath"`
	AliasPath   bool `json:"aliasPath"`
	SourceCount int  `json:"sourceCount"`
	AliasCount  int  `json:"aliasCount"`
}

func extractPlayerScript(t *testing.T) string {
	t.Helper()

	response, err := NewHTTPRoutesServer(cache.NewStore()).Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/admin"})
	if err != nil {
		t.Fatalf("admin route: %v", err)
	}
	body := string(response.GetBody())
	start := strings.Index(body, "<script>\n      const path")
	end := strings.LastIndex(body, "</script>")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("expected embedded script in admin page")
	}
	return body[start+len("<script>") : end]
}

func runVirtualAliasScript(t *testing.T, script string, context map[string]any) virtualAliasResult {
	t.Helper()

	payload, err := json.Marshal(context)
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}
	dir := t.TempDir()
	appScriptPath := filepath.Join(dir, "app.js")
	runnerPath := filepath.Join(dir, "runner.js")
	if err := os.WriteFile(appScriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write app script: %v", err)
	}
	nodeScript := fmt.Sprintf(`
const fs = require("fs");
const vm = require("vm");
const input = %s;
const script = fs.readFileSync(%q, "utf8");
const sandbox = {
  window: { location: { pathname: "/api/v1/plugins/14/dispatcharr/admin", search: "" }, innerHeight: 800, scrollY: 0, addEventListener: () => {} },
  document: { documentElement: { dataset: {} }, querySelectorAll: () => [], querySelector: () => ({ classList: { toggle: () => {} } }), getElementById: () => ({ innerHTML: "", classList: { add: () => {}, remove: () => {}, toggle: () => {} }, textContent: "" }), addEventListener: () => {} },
  localStorage: { getItem: () => null, setItem: () => {} },
  navigator: { sendBeacon: () => true },
  console: { log: () => {}, warn: () => {}, error: () => {} },
  URLSearchParams,
  setTimeout,
  clearTimeout,
};
sandbox.input = input;
vm.createContext(sandbox);
vm.runInContext(script, sandbox);
const result = vm.runInContext(`+"`"+`
Object.assign(state, input.state);
JSON.stringify((function() {
  const all = virtualCategoriesFromPaths("", function() { return true; }, true);
  const source = all.find(function(item) { return item.name === "International / Argentina / Sports"; });
  const alias = all.find(function(item) { return item.name === "Sports / Argentina"; });
  const channelsInSource = effectiveChannels(false).filter(function(channel) {
    return virtualPathsForChannel(channel).indexOf("International / Argentina / Sports") !== -1;
  });
  const channelsInAlias = effectiveChannels(false).filter(function(channel) {
    return virtualPathsForChannel(channel).indexOf("Sports / Argentina") !== -1;
  });
  return {
    sourcePath: !!source,
    aliasPath: !!alias,
    sourceCount: channelsInSource.length,
    aliasCount: channelsInAlias.length
  };
})())
`+"`"+`, sandbox);
process.stdout.write(result);
`, string(payload), appScriptPath)
	if err := os.WriteFile(runnerPath, []byte(nodeScript), 0o600); err != nil {
		t.Fatalf("write runner script: %v", err)
	}
	cmd := exec.Command("node", runnerPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run node script: %v\n%s", err, output)
	}
	var result virtualAliasResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode node result: %v\n%s", err, output)
	}
	return result
}

func TestHTTPRoutesServerRecordingsDisabledForXtream(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{{ID: "xtream:1", Name: "News HD"}},
		},
	})
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeXtream,
			XtreamBaseURL:   "https://dispatcharr.example.com",
			XtreamUsername:  "demo",
			XtreamPassword:  "secret",
			ChannelRefreshH: config.DefaultChannelRefreshHours,
			EPGRefreshH:     config.DefaultEPGRefreshHours,
		}
	})

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/api/recordings"})
	if err != nil {
		t.Fatalf("recordings route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	var payload RecordingsPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal recordings payload: %v", err)
	}
	if payload.Available || !strings.Contains(payload.Reason, "Dispatcharr Direct") {
		t.Fatalf("expected recordings disabled for xtream, got %+v", payload)
	}

	response, err = server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/recordings",
		Body:   []byte(`{"channelId":"xtream:1","title":"News","startUnix":1700000000,"endUnix":1700003600}`),
	})
	if err != nil {
		t.Fatalf("recordings schedule route: %v", err)
	}
	if response.GetStatusCode() != 409 {
		t.Fatalf("expected 409, got %d", response.GetStatusCode())
	}
}

func TestHTTPRoutesServerAppRouteHydratesColdCatalog(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	syncer := &stubCatalogSyncer{store: store}
	server := NewHTTPRoutesServerWithSyncer(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeDirectLogin,
			DispatcharrURL:  "https://dispatcharr.example.com",
			DispatcharrUser: "demo",
			DispatcharrPass: "secret",
			ChannelRefreshH: 24,
			EPGRefreshH:     24,
		}
	}, syncer)

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/api/app"})
	if err != nil {
		t.Fatalf("app route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	if syncer.calls != 1 {
		t.Fatalf("expected cold catalog sync once, got %d", syncer.calls)
	}

	var payload AppPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal app payload: %v", err)
	}
	if len(payload.Channels) != 1 || payload.Channels[0].ID != "dispatcharr:news" {
		t.Fatalf("expected hydrated channel payload, got %+v", payload.Channels)
	}

	_, err = server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/api/app"})
	if err != nil {
		t.Fatalf("second app route: %v", err)
	}
	if syncer.calls != 1 {
		t.Fatalf("expected warm catalog to skip sync, got %d calls", syncer.calls)
	}
}

func TestHTTPRoutesServerRefreshRouteForcesCatalogSync(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeDirectLogin),
			Channels: []model.Channel{{ID: "dispatcharr:old", Name: "Old Channel"}},
		},
	})
	syncer := &stubCatalogSyncer{store: store}
	server := NewHTTPRoutesServerWithSyncer(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeDirectLogin,
			DispatcharrURL:  "https://dispatcharr.example.com",
			DispatcharrUser: "demo",
			DispatcharrPass: "secret",
			ChannelRefreshH: 24,
			EPGRefreshH:     24,
		}
	}, syncer)

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "POST", Path: "/dispatcharr/api/refresh"})
	if err != nil {
		t.Fatalf("refresh route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	if syncer.calls != 1 {
		t.Fatalf("expected refresh to force one sync, got %d calls", syncer.calls)
	}

	var payload AppPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal app payload: %v", err)
	}
	if len(payload.Channels) != 1 || payload.Channels[0].ID != "dispatcharr:news" {
		t.Fatalf("expected refreshed channel payload, got %+v", payload.Channels)
	}
}

func TestHTTPRoutesServerFavoriteRouteUpdatesPreferences(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/favorites",
		Body:   []byte(`{"id":"xtream:1","enabled":true}`),
	})
	if err != nil {
		t.Fatalf("favorite route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	var prefs cache.Preferences
	if err := json.Unmarshal(response.GetBody(), &prefs); err != nil {
		t.Fatalf("unmarshal preferences: %v", err)
	}
	if !prefs.Favorites["xtream:1"] {
		t.Fatalf("expected favorite to be enabled: %+v", prefs)
	}
}

type stubCatalogSyncer struct {
	store *cache.Store
	calls int
}

func (s *stubCatalogSyncer) SyncNow(_ context.Context, _ config.Settings, nowUnix int64) error {
	s.calls++
	s.store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeDirectLogin),
			Channels: []model.Channel{{ID: "dispatcharr:news", Name: "News HD"}},
			Programs: []model.Program{{ID: "program:1", ChannelID: "dispatcharr:news", Title: "Morning News", StartUnix: 100, EndUnix: 200}},
		},
		Health: model.SyncHealth{LastSuccessUnix: nowUnix},
	})
	return nil
}

func TestHTTPRoutesServerPreferencesRoutePersistsFullPayload(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/preferences",
		Body:   []byte(`{"favorites":{"channel:1":true},"autoFavorites":{"channel:2":true},"hiddenCategories":{"sports":true},"recentChannels":["channel:1"],"continueWatching":{"channel:1":{"plays":3}},"playback":{"streamMode":"redirect","outputFormat":"hls"},"categoryParsing":{"enabled":true,"mode":"delimiter","delimiter":"pipe","regex":"","output":""},"customGroups":[{"id":"group:spanish","name":"Spanish","order":10}],"customGroupMemberships":{"group:spanish":["channel:1","channel:2"]}}`),
	})
	if err != nil {
		t.Fatalf("preferences route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	var prefs cache.Preferences
	if err := json.Unmarshal(response.GetBody(), &prefs); err != nil {
		t.Fatalf("unmarshal preferences: %v", err)
	}
	if !prefs.Favorites["channel:1"] || !prefs.AutoFavorites["channel:2"] || !prefs.HiddenCategories["sports"] {
		t.Fatalf("expected full preferences to persist: %+v", prefs)
	}
	if len(prefs.RecentChannels) != 1 || prefs.RecentChannels[0] != "channel:1" {
		t.Fatalf("expected recent channel to persist: %+v", prefs)
	}
	if !prefs.CategoryParsing.Enabled || prefs.CategoryParsing.Delimiter != "pipe" {
		t.Fatalf("expected category parsing settings to persist: %+v", prefs.CategoryParsing)
	}
	if len(prefs.CustomGroups) != 1 || prefs.CustomGroups[0].Name != "Spanish" {
		t.Fatalf("expected custom groups to persist: %+v", prefs.CustomGroups)
	}
	if got := prefs.CustomGroupMemberships["group:spanish"]; len(got) != 2 || got[0] != "channel:1" || got[1] != "channel:2" {
		t.Fatalf("expected custom group memberships to persist: %+v", prefs.CustomGroupMemberships)
	}
}

func TestHTTPRoutesServerAdminSettingsRoutePersistsPayload(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	var persisted map[string]any
	server.adminPersister = func(_ context.Context, payload map[string]any) error {
		persisted = payload
		return nil
	}
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method:  "POST",
		Path:    "/dispatcharr/api/admin-settings",
		Headers: map[string]string{"x-dispatcharr-admin-token": server.adminToken},
		Body:    []byte(`{"mode":"admin_delimiter","delimiter":"pipe","adminGroups":[{"id":"admin:sports","name":"Sports | Argentina","order":1}],"adminGroupMemberships":{"admin:sports":["channel:1"]},"presentationOverrides":{"channel:1":{"name":"Sports Alt","order":2}}}`),
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}

	response, err = server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method:  "GET",
		Path:    "/dispatcharr/api/admin-settings",
		Headers: map[string]string{"x-dispatcharr-admin-token": server.adminToken},
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal admin settings: %v", err)
	}
	if payload["mode"] != "admin_delimiter" || payload["delimiter"] != "pipe" {
		t.Fatalf("expected admin settings to persist: %+v", payload)
	}
	if persisted["mode"] != "admin_delimiter" || persisted["delimiter"] != "pipe" {
		t.Fatalf("expected admin settings to write through to host config: %+v", persisted)
	}
}

func TestHTTPRoutesServerAdminSettingsRouteReadsConfiguredPayload(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServerWithSettings(cache.NewStore(), func() config.Settings {
		return config.Settings{AdminSettings: json.RawMessage(`{"mode":"delimiter","delimiter":"pipe"}`)}
	})
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method:  "GET",
		Path:    "/dispatcharr/api/admin-settings",
		Headers: map[string]string{"x-dispatcharr-admin-token": server.adminToken},
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal admin settings: %v", err)
	}
	if payload["mode"] != "delimiter" || payload["delimiter"] != "pipe" {
		t.Fatalf("expected configured admin settings: %+v", payload)
	}
}

func TestHTTPRoutesServerAdminSettingsRouteRequiresAdminPageToken(t *testing.T) {
	t.Parallel()

	response, err := NewHTTPRoutesServer(cache.NewStore()).Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "GET",
		Path:   "/dispatcharr/api/admin-settings",
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	if response.GetStatusCode() != 403 {
		t.Fatalf("expected 403 without admin settings token, got %d", response.GetStatusCode())
	}
}

func TestHTTPRoutesServerWatchLifecycleUpdatesSessionState(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	startResponse, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/watch/start",
		Body:   []byte(`{"itemKind":"channel","itemId":"xtream:1","itemName":"News HD"}`),
	})
	if err != nil {
		t.Fatalf("watch start route: %v", err)
	}
	if startResponse.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", startResponse.GetStatusCode())
	}
	var startPayload struct {
		Session     cache.WatchSession `json:"session"`
		Preferences cache.Preferences  `json:"preferences"`
	}
	if err := json.Unmarshal(startResponse.GetBody(), &startPayload); err != nil {
		t.Fatalf("unmarshal watch start payload: %v", err)
	}
	if startPayload.Session.ID == "" || startPayload.Session.ItemID != "xtream:1" {
		t.Fatalf("unexpected watch session: %+v", startPayload.Session)
	}
	if len(startPayload.Preferences.RecentChannels) != 1 || startPayload.Preferences.RecentChannels[0] != "xtream:1" {
		t.Fatalf("expected recent channel update: %+v", startPayload.Preferences)
	}

	heartbeatResponse, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/watch/heartbeat",
		Body:   []byte(`{"sessionId":"` + startPayload.Session.ID + `"}`),
	})
	if err != nil {
		t.Fatalf("watch heartbeat route: %v", err)
	}
	if heartbeatResponse.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", heartbeatResponse.GetStatusCode())
	}

	stopResponse, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/watch/stop",
		Body:   []byte(`{"sessionId":"` + startPayload.Session.ID + `","reason":"test"}`),
	})
	if err != nil {
		t.Fatalf("watch stop route: %v", err)
	}
	if stopResponse.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", stopResponse.GetStatusCode())
	}
}

func TestHTTPRoutesServerStreamM3URoute(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeM3UXMLTV),
			Channels: []model.Channel{
				{ID: "m3u:news.hd", Name: "News HD", StreamURL: "https://dispatcharr.example.com/live/news.m3u8"},
			},
		},
	})
	server := NewHTTPRoutesServer(store)
	query, _ := structpb.NewStruct(map[string]any{"channel_id": "m3u:news.hd"})

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/stream", Query: query})
	if err != nil {
		t.Fatalf("stream route: %v", err)
	}
	if response.GetStatusCode() != 302 {
		t.Fatalf("expected 302, got %d", response.GetStatusCode())
	}
	if response.GetHeaders()["location"] != "https://dispatcharr.example.com/live/news.m3u8" {
		t.Fatalf("unexpected location header: %q", response.GetHeaders()["location"])
	}
}

func TestHTTPRoutesServerStreamXtreamRoute(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{
				{ID: "xtream:1001", Name: "News HD"},
			},
		},
	})
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeXtream,
			XtreamBaseURL:   "https://dispatcharr.example.com",
			XtreamUsername:  "demo",
			XtreamPassword:  "secret",
			ChannelRefreshH: config.DefaultChannelRefreshHours,
			EPGRefreshH:     config.DefaultEPGRefreshHours,
		}
	})
	query, _ := structpb.NewStruct(map[string]any{"channel_id": "xtream:1001"})

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/stream", Query: query})
	if err != nil {
		t.Fatalf("stream route: %v", err)
	}
	if response.GetStatusCode() != 302 {
		t.Fatalf("expected 302, got %d", response.GetStatusCode())
	}
	if !strings.Contains(response.GetHeaders()["location"], "/live/demo/secret/1001") {
		t.Fatalf("unexpected location header: %q", response.GetHeaders()["location"])
	}
}

func TestHTTPRoutesServerStreamPreservesBrowserPlaybackQuery(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{
				{ID: "xtream:1001", Name: "News HD"},
			},
		},
	})
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeXtream,
			XtreamBaseURL:   "https://dispatcharr.example.com",
			XtreamUsername:  "demo",
			XtreamPassword:  "secret",
			ChannelRefreshH: config.DefaultChannelRefreshHours,
			EPGRefreshH:     config.DefaultEPGRefreshHours,
		}
	})
	query, _ := structpb.NewStruct(map[string]any{
		"channel_id":     "xtream:1001",
		"output_profile": "2",
	})

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/stream", Query: query})
	if err != nil {
		t.Fatalf("stream route: %v", err)
	}
	location := response.GetHeaders()["location"]
	if !strings.Contains(location, "output_profile=2") {
		t.Fatalf("expected browser playback query in location header: %q", location)
	}
}

func TestHTTPRoutesServerVODStreamXtreamRoute(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeXtream),
			Content: model.ContentState{
				VODItems: []model.VODItem{{ID: "vod:2001", Name: "Movie", Container: "mp4"}},
			},
		},
	})
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeXtream,
			XtreamBaseURL:   "https://dispatcharr.example.com",
			XtreamUsername:  "demo",
			XtreamPassword:  "secret",
			ChannelRefreshH: config.DefaultChannelRefreshHours,
			EPGRefreshH:     config.DefaultEPGRefreshHours,
		}
	})
	query, _ := structpb.NewStruct(map[string]any{"item_id": "vod:2001"})

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/vod/stream", Query: query})
	if err != nil {
		t.Fatalf("vod stream route: %v", err)
	}
	if response.GetStatusCode() != 302 {
		t.Fatalf("expected 302, got %d", response.GetStatusCode())
	}
	if !strings.Contains(response.GetHeaders()["location"], "/movie/demo/secret/2001.mp4") {
		t.Fatalf("unexpected location header: %q", response.GetHeaders()["location"])
	}
}

func TestHTTPRoutesServerPlayerRoute(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/player"})
	if err != nil {
		t.Fatalf("player route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	if !strings.Contains(string(response.GetBody()), "TV Guide") {
		t.Fatalf("expected app shell html body")
	}
	if !strings.Contains(string(response.GetBody()), `href="/" aria-label="Back to Silo"`) {
		t.Fatalf("expected back to Silo link")
	}
	body := string(response.GetBody())
	if strings.Contains(body, "cdn.jsdelivr.net") {
		t.Fatalf("expected player libraries to be served locally")
	}
	if strings.Contains(body, `src="dispatcharr/assets/`) || strings.Contains(body, "__PLAYER_LIBRARIES__") {
		t.Fatalf("expected embedded player libraries")
	}
	if !strings.Contains(body, "mpegts.js") || !strings.Contains(body, "Hls") {
		t.Fatalf("expected inline player library content")
	}
	if !strings.Contains(body, "output_profile=2") {
		t.Fatalf("expected browser playback to request AAC Xtream profile")
	}
}

func TestHTTPRoutesServerPlayerAssetRoutes(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	for _, path := range []string{"/dispatcharr/assets/hls.min.js", "/dispatcharr/assets/mpegts.min.js", "/assets/hls.min.js", "/assets/mpegts.min.js"} {
		response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: path})
		if err != nil {
			t.Fatalf("asset route %s: %v", path, err)
		}
		if response.GetStatusCode() != 200 {
			t.Fatalf("expected 200 for %s, got %d", path, response.GetStatusCode())
		}
		if response.GetHeaders()["content-type"] != "application/javascript; charset=utf-8" {
			t.Fatalf("unexpected content type for %s: %q", path, response.GetHeaders()["content-type"])
		}
		if len(response.GetBody()) < 1024 {
			t.Fatalf("expected embedded player asset body for %s", path)
		}
	}
}
