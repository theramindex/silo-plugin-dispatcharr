package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	if response.GetHeaders()["cache-control"] != "no-store" {
		t.Fatalf("expected app shell to disable browser caching, got %q", response.GetHeaders()["cache-control"])
	}
	body := string(response.GetBody())
	for _, want := range []string{
		`function sourceVirtualChildCategories(parentPath, includeChannel)`,
		`function featuredChildCategories(parentPath, includeChannel)`,
		`function virtualCategoriesFromPaths(parentPath, includeChannel, includeAllDescendants)`,
		`function featuredCategoriesFromPaths(parentPath, includeChannel, includeAllDescendants)`,
		`function guideFilterCategories()`,
		`featuredCategoriesFromPaths("", includeChannel, true)`,
		`virtualCategoriesFromPaths("", includeChannel, true)`,
		`const categories = guideFilterCategories();`,
		`if (state.category.indexOf("virtual:") === 0 || state.category.indexOf("featured:") === 0)`,
		`const children = (featured ? featuredChildCategories : virtualChildCategories)(path,`,
		`virtualFolderBreadcrumbs(path, featured)`,
		`const rootLabel = featured ? featuredGroupLabel() : virtualGroupLabel()`,
		`function featuredGroupLabel()`,
		`function allGroupLabel()`,
		`data-admin-category-field=\"virtualGroupLabel\"`,
		`const showSourceCategorySettings = !virtualCategoriesActive()`,
		`Saved on this device, but not to your Silo profile.`,
		`aria-label="Live TV sections"`,
		`<span>Guide</span>`,
		`<span>On Later</span>`,
		`<span>My Stuff</span>`,
		`<span>Sports</span>`,
		`<span>Events</span>`,
		`id="settings-menu-button"`,
		`class="settings-dropdown"`,
		`Refresh guide</button>`,
		`id="sports-topbar-tabs"`,
		`id="app-search-button"`,
		`data-view="search"`,
		`const searchHistoryKey = "silo.ramindex.dispatcharr.searchHistory.v1"`,
		`function renderSearchPage()`,
		`function renderSearchResults(query)`,
		`function renderOnLaterPage()`,
		`function groupedUpcomingAirings(programs, query)`,
		`function programIsGuidePlaceholder(program)`,
		`no games? today`,
		`function rememberSearch(value)`,
		`onLaterType`,
		`data-search-recent=`,
		`data-search-type=`,
		`data-search-channel=`,
		`data-search-category=`,
		`data-search-program-channel=`,
		`data-keyword-pass-add=`,
		`keywordPasses`,
		`allowRecordingsByDefault`,
		`Search movies, tv shows, channels and more`,
		`function renderSportsPage()`,
		`function renderSportsTopbarTabs()`,
		`function compareSportsEventsForTab(left, right)`,
		`return rightRecent - leftRecent;`,
		`sports-channel-logo`,
		`function renderEventsPage()`,
		`/dispatcharr/api/events`,
		`data-event-tab=`,
		`function renderMultiviewPage()`,
		`function addChannelToMultiview(channel)`,
		`function syncMultiviewAudio()`,
		`multiviewQuery`,
		`function multiviewCandidateChannels(limit)`,
		`id=\"multiview-picker\"`,
		`Search channels or programs`,
		`picker.outerHTML = renderMultiviewPicker()`,
		`data-multiview-channel=`,
		`data-player-action=\"add-multiview\"`,
		`/dispatcharr/api/sports`,
		`data-sports-tab=`,
		`sportsFavoriteTeams`,
		`const isAdminRoute = path.endsWith("/dispatcharr/admin")`,
		`if (state.view === "admin" && !isAdminRoute) state.view = "home"`,
		`delimiter: "pipe"`,
		`if (!settings.delimiter) settings.delimiter = "pipe"`,
		`function renderVirtualCategoryGuide(channels)`,
		`function renderVirtualCategoryViewToggle()`,
		`function renderVirtualCategoryChannelList(channels)`,
		`function renderVirtualCategoryContent(channels)`,
		`function setVirtualCategoryView(view)`,
		`renderVirtualCategoryGuide(channels)`,
		`function categoryTileHTML(category)`,
		`.tile strong { display: -webkit-box;`,
		`-webkit-line-clamp: 2`,
		`data-virtual-category-view=\"guide\"`,
		`data-virtual-category-view=\"list\"`,
		`No channels in this virtual group yet.`,
		`function isRewindableChannel(channel)`,
		`video.controls = rewindable`,
		`isLive: !rewindable`,
		`data-silo-theme="midnight-cinema"`,
		`function applySiloTheme()`,
		`--silo-bg`,
		`const appCacheKey = "silo.ramindex.dispatcharr.appSnapshot.v1." + localCacheSuffix`,
		`function readLocalAppCache()`,
		`function writeLocalAppCache(payload)`,
		`await hydrateApp(cached, { localCache: true })`,
		`Showing saved guide. Refresh failed.`,
		`function rebuildProgramIndex()`,
		`.overflow-tooltip`,
		`data-overflow-description=\"true\"`,
		`data-overflow-tooltip=\"`,
		`function descriptionOverflows(target)`,
		`function showOverflowTooltip(target, event)`,
		`if (!descriptionOverflows(target)) return;`,
		`.logo-fallback`,
		`function channelLogoFallback(channel)`,
		`onerror=\"this.hidden = true; this.nextElementSibling.hidden = false;\"`,
		`<span class=\"epg-channel-title\">`,
		`title=\"" + escapeHTML(channelName) + "\"`,
		`data-channel-name=\"`,
		`content: attr(data-channel-name)`,
		`.epg-channel:hover::after`,
		`.epg-channel:focus-visible::after`,
		`function renderEPGGapCell(channel, startUnix, endUnix, windowInfo)`,
		`class=\"epg-cell program epg-gap\"`,
		`program" + (isLive ? " live" : "")`,
		`if (start > cursor) cells.push(renderEPGGapCell(channel, cursor, start, windowInfo));`,
		`customGroupChannelID`,
		`role=\"combobox\"`,
		`role=\"listbox\"`,
		`data-custom-group-channel-option=`,
		`function selectCustomGroupChannel(channelID)`,
		`function tickGuideAutoRefresh()`,
		`state.guideAutoTimer = setInterval(tickGuideAutoRefresh, 60000);`,
		`now - state.guideLastAutoFetchAt < 5 * 60 * 1000`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected app page to include virtual folder drilldown marker %q", want)
		}
	}
	if strings.Contains(body, `id=\"custom-group-channel\"><option`) {
		t.Fatalf("expected custom group channel picker not to render a native select")
	}
	if !strings.Contains(body, `data-sports-refresh`) {
		t.Fatalf("expected sports scores to have a dedicated refresh action")
	}
	if strings.Contains(body, `<span>Multiview</span>`) || strings.Contains(body, `sports-channel-multiview`) {
		t.Fatalf("expected multiview controls to be hidden from navigation and sports cards")
	}
	if strings.Contains(body, `postJSON("/dispatcharr/api/sports/favorites"`) {
		t.Fatalf("expected sports favorite teams to save through user profile preferences")
	}
	if strings.Contains(body, `colorClass(`) {
		t.Fatalf("expected guide colors to be semantic, not rotated by position")
	}
	if !strings.Contains(body, `const recent = recentChannels(5);`) {
		t.Fatalf("expected home guide to be based on up to 5 continue-watching channels")
	}
	if !strings.Contains(body, `return pool.filter(channelHasCurrentGuide).slice(0, 5);`) {
		t.Fatalf("expected home guide preview to be capped at 5 channels")
	}
	if !strings.Contains(body, `const watched = recent.length ? recent : visibleChannels(false).slice(0, 5);`) ||
		!strings.Contains(body, `+ (favorites.length ? sectionHeader("Favorites") + favoriteHomeCards(favorites) : "")`) ||
		!strings.Contains(body, `+ sectionHeaderWithActions("TV Guide", guideFreshnessHTML())`) ||
		!strings.Contains(body, `+ renderHomeGuide(homeGuideChannels(watched), "No current guide data for recently watched channels.", { hideFreshness: true })`) ||
		!strings.Contains(body, `+ categoryGrid();`) {
		t.Fatalf("expected home page order to be continue watching, favorites, guide grid, then group sections")
	}
	virtualHeaderIndex := strings.Index(body, `byId("view").innerHTML = virtualFolderHeader(path, featured)`)
	virtualChildrenIndex := strings.Index(body, `+ (children.length ? "<div class=\"category-grid\">`)
	virtualContentIndex := strings.Index(body, `+ renderVirtualCategoryContent(channels)`)
	if virtualHeaderIndex < 0 || virtualChildrenIndex < 0 || virtualContentIndex < 0 {
		t.Fatalf("expected virtual category drilldown to render breadcrumbs, subfolders, and switchable channel content")
	}
	if !(virtualHeaderIndex < virtualChildrenIndex && virtualChildrenIndex < virtualContentIndex) {
		t.Fatalf("expected virtual category drilldown order to be breadcrumbs, subfolders, then channel content")
	}
	if strings.Contains(body, `+ (children.length ? sectionHeader("Virtual Groups")`) || strings.Contains(body, `+ (children.length ? sectionHeader("Virtual Categories")`) {
		t.Fatalf("expected virtual child groups to render without a duplicate section heading")
	}
	if strings.Contains(body, "Saved on this device. Silo profile sync is unavailable here.") {
		t.Fatalf("expected local-only profile save message to use the standard warning")
	}
	if strings.Contains(body, `data-title=\"" + escapeHTML(programTitle) + "\"`) {
		t.Fatalf("expected guide program cells not to expose hover title popups")
	}
}

func TestManifestDeclaresSportsAPIRoutes(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		HTTPRoutes []struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"http_routes"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	seen := map[string]bool{}
	for _, route := range manifest.HTTPRoutes {
		seen[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /dispatcharr/api/sports",
		"POST /dispatcharr/api/sports/favorites",
		"GET /dispatcharr/api/events",
	} {
		if !seen[route] {
			t.Fatalf("manifest does not declare %s", route)
		}
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
		`.shell.is-admin .rail { display: none; }`,
		`.shell.is-admin .main { display: grid; grid-template-rows: auto minmax(0, 1fr); min-height: 0; padding: 0; }`,
		`.admin-topbar`,
		`justify-content: flex-start`,
		`<div class="admin-topbar">`,
		`<nav id="admin-tabs" class="admin-tabs" aria-label="Live TV admin sections"></nav>`,
		`<div id="admin-actions" class="admin-actions"></div>`,
		`const adminSettingsKey = "adminCategorySettings"`,
		`adminTab: "settings"`,
		`function defaultAdminCategorySettings()`,
		`function renderAdminPage()`,
		`function renderAdminTopbarTabs()`,
		`function renderAdminTopbarActions()`,
		`function renderAdminSettingsTab()`,
		`function renderAdminIntegrationsTab()`,
		`Connection Status`,
		`function adminStatusPanel()`,
		`admin-status-strip`,
		`Presentation Overrides`,
		`function renderAdminCategoryAliasSettings()`,
		`function renderAdminECMSettings()`,
		`function adminECMURL()`,
		`ecmEnabled: false`,
		`state.adminCategorySettings.ecmEnabled = state.adminCategorySettings.ecmEnabled === true`,
		`return adminSettings().ecmEnabled === true && !!adminECMURL();`,
		`Group method`,
		`virtual-label-control`,
		`placeholder=\"Groups\"`,
		`Alternative group name`,
		`Also show as`,
		`alias-builder`,
		`alias-table`,
		`alias-table-row`,
		`Normal`,
		`By delimiter`,
		`Enable ECM`,
		`ECM URL`,
		`ecm-url-row`,
		`.settings-row.ecm-url-row input`,
		`data-admin-tab=\"settings\"`,
		`data-admin-tab=\"integrations\"`,
		`data-admin-tab=\"manager\"`,
		`data-admin-ecm-field=\"enabled\"`,
		`data-admin-ecm-field=\"url\"`,
		`byId("view").innerHTML = state.adminTab === "manager" ? renderExternalChannelManager()`,
		`data-admin-category-field=\"mode\"`,
		`data-admin-alias-action=\"add\"`,
		`data-admin-alias-action=\"remove\"`,
		`data-admin-settings-action=\"save\"`,
		`data-admin-settings-action=\"discard\"`,
		`renderAdminTopbarActions();`,
		`function renderExternalChannelManager()`,
		`classList.toggle("is-admin-manager"`,
		`class=\"external-manager-surface\"`,
		`class=\"external-manager-frame\"`,
		`Unsaved changes.`,
		`Save`,
		`Discard`,
		`function effectiveChannel(channel)`,
		`/dispatcharr/api/admin-settings`,
		`state.adminCategorySettings = await loadAdminCategorySettings().catch(function()`,
		`const adminSettingsToken = "`,
		`x-dispatcharr-admin-token`,
		`row.keywords.join("\n")`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected admin page to include category mapping marker %q", want)
		}
	}
	if strings.Contains(body, `row.keywords.join("\\n")`) {
		t.Fatal("expected event keyword textareas to render real line breaks, not escaped newline text")
	}
	if strings.Contains(body, `class="nav admin-nav"`) || strings.Contains(body, `function renderAdminSidebarTabs()`) {
		t.Fatal("expected admin tabs to render in the topbar, not the sidebar")
	}
	if strings.Contains(body, `<div class=\"settings-card\"><div class=\"external-manager-head\"`) {
		t.Fatal("expected ECM iframe to render as a full action-area surface, not inside a settings card")
	}
	if strings.Contains(body, `external-manager-toolbar`) || strings.Contains(body, `Open in new window`) {
		t.Fatal("expected ECM iframe to render without a floating open-in-new-window overlay")
	}
	if strings.Contains(body, `https://`+`ecm.ramindex.org`) {
		t.Fatal("expected admin page not to include a hardcoded ECM URL")
	}
	for _, removed := range []string{
		`Admin-only status panel. No usernames, passwords, or API keys are shown.`,
		`<div class=\"settings-card\"><h2>Preview</h2>`,
		`function adminCategoryPreview()`,
		`Group Renames`,
		`data-admin-rename-action`,
		`data-admin-rename-field`,
		`function renderAdminCategoryRenameSettings()`,
	} {
		if strings.Contains(body, removed) {
			t.Fatalf("expected admin page to omit removed settings clutter %q", removed)
		}
	}
	for _, hidden := range []string{`<span>Home</span>`, `<span>My Stuff</span>`, `<span>Guide</span>`, `aria-label="Live TV sections"`} {
		if strings.Contains(body, hidden) {
			t.Fatalf("expected admin page shell to hide user nav marker %q", hidden)
		}
	}
}

func TestDelimiterVirtualFoldersApplyToSourceGroups(t *testing.T) {
	t.Parallel()

	script := extractPlayerScript(t)
	context := map[string]any{
		"state": map[string]any{
			"app": map[string]any{
				"channels": []map[string]any{
					{"id": "channel:world-cup", "name": "World Cup Feed", "categoryId": "cat:world-cup", "categoryName": "* World Cup"},
					{"id": "channel:admin-favorites", "name": "Admin Favorite", "categoryId": "cat:admin-favorites", "categoryName": "* Admin Favorites"},
					{"id": "channel:argentina-sports", "name": "Argentina Sports", "categoryId": "cat:argentina-sports", "categoryName": "* International | Argentina | Sports"},
					{"id": "channel:world-cup-replay", "name": "World Cup Replay", "categoryId": "cat:world-cup-replays", "categoryName": "World Cup Replays"},
				},
				"categories": []map[string]any{
					{"id": "cat:world-cup", "name": "* World Cup"},
					{"id": "cat:admin-favorites", "name": "* Admin Favorites"},
					{"id": "cat:argentina-sports", "name": "* International | Argentina | Sports"},
					{"id": "cat:world-cup-replays", "name": "World Cup Replays"},
				},
			},
			"adminCategorySettings": map[string]any{
				"mode":      "delimiter",
				"delimiter": "pipe",
				"categoryAliases": []map[string]any{
					{"sourcePath": "International | Argentina | Sports", "aliasPath": "Sports | Argentina"},
					{"sourcePath": "International | Argentina | Sports", "aliasPath": "World Cup | Argentina"},
				},
			},
		},
	}

	result := runVirtualAliasScript(t, script, context)
	if !result.SourcePath {
		t.Fatalf("expected source path to remain visible: %+v", result)
	}
	if !result.AliasPath || !result.SecondAliasPath {
		t.Fatalf("expected Silo admin alias paths to be present: %+v", result)
	}
	if result.SourceCount != 1 || result.AliasCount != 1 || result.SecondAliasCount != 1 {
		t.Fatalf("expected source and alias counts to point at the same channel: %+v", result)
	}
	if result.ObjectParsedMode != "delimiter" {
		t.Fatalf("expected admin settings JSON object to preserve mode: %+v", result)
	}
	if result.StringParsedMode != "delimiter" {
		t.Fatalf("expected admin settings JSON string to preserve mode: %+v", result)
	}
	if !result.FeaturedSection || !result.FeaturedCategory {
		t.Fatalf("expected starred source category to render in featured section: %+v", result)
	}
	if !result.FeaturedRenamedSection {
		t.Fatalf("expected featured section label to follow the virtual label suffix: %+v", result)
	}
	if !result.GuideRenamedAllOption {
		t.Fatalf("expected guide filter all option to follow the virtual label suffix: %+v", result)
	}
	if !result.FeaturedAlphabetical {
		t.Fatalf("expected featured categories to render alphabetically by display name: %+v", result)
	}
	if result.FeaturedMarkerVisible {
		t.Fatalf("expected starred source category marker to be hidden: %+v", result)
	}
	if !result.FeaturedVirtualCategory {
		t.Fatalf("expected starred delimiter category to open the featured breadcrumb view: %+v", result)
	}
	if result.FeaturedSourceCategory {
		t.Fatalf("expected starred delimiter category to stop linking to the source-card view: %+v", result)
	}
	if !result.FeaturedBreadcrumbRoot || !result.FeaturedBreadcrumbPath || !result.FeaturedGuide {
		t.Fatalf("expected starred delimiter category to render featured breadcrumbs and guide: %+v", result)
	}
	if result.FeaturedGuideHeading || result.VirtualGuideHeading {
		t.Fatalf("expected virtual drilldown guide views to omit the redundant TV Guide heading: %+v", result)
	}
	if !result.FeaturedViewToggle || !result.FeaturedListView {
		t.Fatalf("expected featured virtual category to toggle between guide and channel list views: %+v", result)
	}
	if !result.SimpleFeaturedCategory || !result.SimpleFeaturedGuide || !result.SimpleFeaturedViewToggle || result.SimpleFeaturedSourcePage {
		t.Fatalf("expected simple starred groups to use the featured drilldown guide/list view: %+v", result)
	}
	if !result.VirtualBreadcrumbRoot {
		t.Fatalf("expected normal virtual group breadcrumb root to remain virtual groups: %+v", result)
	}
	if result.FeaturedBackButton || result.VirtualBackButton {
		t.Fatalf("expected virtual drilldowns to omit the redundant Back button: %+v", result)
	}
	if result.ChannelCategoryName != "International | Argentina | Sports" {
		t.Fatalf("expected channel category display name to hide marker: %+v", result)
	}
	if !result.ReplayRewindable || result.NormalRewindable {
		t.Fatalf("expected only World Cup Replays channels to be rewindable: %+v", result)
	}
	if !result.ReplayPlayerClass || !result.ReplayPlayerControls || !result.ReplayPlayerTag {
		t.Fatalf("expected World Cup Replays player to expose replay controls: %+v", result)
	}
	if !result.EPGOverlapResolved {
		t.Fatalf("expected overlapping EPG programs to render without overlapping cells: %+v", result)
	}
	if !result.GuideStartsAtCurrentSlot {
		t.Fatalf("expected guide window to start at the current half-hour slot: %+v", result)
	}
	if !result.ProgramSearchMatchesEPG {
		t.Fatalf("expected global search to match channels by EPG program title: %+v", result)
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
		`function setChannelFavorite(channelID, enabled)`,
		`const isFavorite = setChannelFavorite(id, !favoriteMap()[id]);`,
		`data-favorite-move=\"up\"`,
		`data-favorite-move=\"down\"`,
		`clip-path: inset(0);`,
		`.epg-cell .epg-play { position: absolute; inset: 0;`,
		`max-width: 100%; overflow: hidden; white-space: nowrap;`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected app page to include UI marker %q", want)
		}
	}
	helperIndex := strings.Index(body, `function setChannelFavorite(channelID, enabled)`)
	if helperIndex == -1 {
		t.Fatalf("expected app page to include channel favorite helper")
	}
	helperBody := body[helperIndex:]
	saveIndex := strings.Index(helperBody, `savePrefs();`)
	cacheIndex := strings.Index(helperBody, `postJSON("/dispatcharr/api/favorites"`)
	if saveIndex == -1 || cacheIndex == -1 {
		t.Fatalf("expected channel favorite helper to save Silo profile preferences and sync plugin cache")
	}
	if saveIndex > cacheIndex {
		t.Fatalf("expected channel favorite helper to save Silo profile preferences before syncing plugin cache")
	}
}

type virtualAliasResult struct {
	SourcePath               bool   `json:"sourcePath"`
	AliasPath                bool   `json:"aliasPath"`
	SecondAliasPath          bool   `json:"secondAliasPath"`
	SourceCount              int    `json:"sourceCount"`
	AliasCount               int    `json:"aliasCount"`
	SecondAliasCount         int    `json:"secondAliasCount"`
	ObjectParsedMode         string `json:"objectParsedMode"`
	StringParsedMode         string `json:"stringParsedMode"`
	FeaturedSection          bool   `json:"featuredSection"`
	FeaturedRenamedSection   bool   `json:"featuredRenamedSection"`
	GuideRenamedAllOption    bool   `json:"guideRenamedAllOption"`
	FeaturedCategory         bool   `json:"featuredCategory"`
	FeaturedAlphabetical     bool   `json:"featuredAlphabetical"`
	FeaturedVirtualCategory  bool   `json:"featuredVirtualCategory"`
	FeaturedSourceCategory   bool   `json:"featuredSourceCategory"`
	FeaturedMarkerVisible    bool   `json:"featuredMarkerVisible"`
	FeaturedBreadcrumbRoot   bool   `json:"featuredBreadcrumbRoot"`
	FeaturedBreadcrumbPath   bool   `json:"featuredBreadcrumbPath"`
	FeaturedGuide            bool   `json:"featuredGuide"`
	FeaturedGuideHeading     bool   `json:"featuredGuideHeading"`
	FeaturedViewToggle       bool   `json:"featuredViewToggle"`
	FeaturedListView         bool   `json:"featuredListView"`
	FeaturedBackButton       bool   `json:"featuredBackButton"`
	SimpleFeaturedCategory   bool   `json:"simpleFeaturedCategory"`
	SimpleFeaturedGuide      bool   `json:"simpleFeaturedGuide"`
	SimpleFeaturedViewToggle bool   `json:"simpleFeaturedViewToggle"`
	SimpleFeaturedSourcePage bool   `json:"simpleFeaturedSourcePage"`
	VirtualBreadcrumbRoot    bool   `json:"virtualBreadcrumbRoot"`
	VirtualGuideHeading      bool   `json:"virtualGuideHeading"`
	VirtualBackButton        bool   `json:"virtualBackButton"`
	ChannelCategoryName      string `json:"channelCategoryName"`
	ReplayRewindable         bool   `json:"replayRewindable"`
	NormalRewindable         bool   `json:"normalRewindable"`
	ReplayPlayerClass        bool   `json:"replayPlayerClass"`
	ReplayPlayerControls     bool   `json:"replayPlayerControls"`
	ReplayPlayerTag          bool   `json:"replayPlayerTag"`
	EPGOverlapResolved       bool   `json:"epgOverlapResolved"`
	GuideStartsAtCurrentSlot bool   `json:"guideStartsAtCurrentSlot"`
	ProgramSearchMatchesEPG  bool   `json:"programSearchMatchesEpg"`
}

func extractPlayerScript(t *testing.T) string {
	t.Helper()

	script := playerAppJavaScript()
	if script == "" {
		t.Fatal("expected embedded app script")
	}
	return script
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
function makeElement() {
  const attributes = {};
  return {
    innerHTML: "",
    textContent: "",
    value: "",
    style: {},
    classList: { add: () => {}, remove: () => {}, toggle: () => {} },
    setAttribute: (name, value) => { attributes[name] = String(value); },
    getAttribute: (name) => attributes[name] || null,
    removeAttribute: (name) => { delete attributes[name]; },
    focus: () => {},
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener: () => {},
    play: () => Promise.resolve(),
    pause: () => {},
  };
}
const sandbox = {
  window: { location: { pathname: "/api/v1/plugins/14/dispatcharr/admin", search: "" }, innerHeight: 800, scrollY: 0, addEventListener: () => {} },
  document: { documentElement: { dataset: {} }, elements: {}, fullscreenElement: null, querySelectorAll: () => [], querySelector: () => makeElement(), getElementById: function(id) { this.elements[id] = this.elements[id] || makeElement(); return this.elements[id]; }, addEventListener: () => {} },
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
const epgWindow = guideWindow();
state.app.programs = [
  { id: "overlap-a", channelId: "channel:argentina-sports", title: "First overlapping program with a very long title", startUnix: epgWindow.start, endUnix: epgWindow.start + 3600 },
  { id: "overlap-b", channelId: "channel:argentina-sports", title: "Second overlapping program", startUnix: epgWindow.start + 1800, endUnix: epgWindow.start + 5400 }
];
rebuildProgramIndex();
JSON.stringify((function() {
  const all = virtualCategoriesFromPaths("", function() { return true; }, true);
  const source = all.find(function(item) { return item.name === "International / Argentina / Sports"; });
  const alias = all.find(function(item) { return item.name === "Sports / Argentina"; });
  const secondAlias = all.find(function(item) { return item.name === "World Cup / Argentina"; });
  const channelsInSource = effectiveChannels(false).filter(function(channel) {
    return virtualPathsForChannel(channel).indexOf("International / Argentina / Sports") !== -1;
  });
  const channelsInAlias = effectiveChannels(false).filter(function(channel) {
    return virtualPathsForChannel(channel).indexOf("Sports / Argentina") !== -1;
  });
  const channelsInSecondAlias = effectiveChannels(false).filter(function(channel) {
    return virtualPathsForChannel(channel).indexOf("World Cup / Argentina") !== -1;
  });
  const grid = categoryGrid();
  state.adminCategorySettings.virtualGroupLabel = "Things";
  const renamedGrid = categoryGrid();
  renderGuidePage();
  const renamedGuideView = document.elements.view ? document.elements.view.innerHTML : "";
  state.adminCategorySettings.virtualGroupLabel = "Groups";
  const channel = channelByID("channel:argentina-sports");
  state.category = "featured:International / Argentina / Sports";
  renderLivePage();
  const featuredView = document.elements.view ? document.elements.view.innerHTML : "";
  state.category = "featured:Admin Favorites";
  renderLivePage();
  const simpleFeaturedView = document.elements.view ? document.elements.view.innerHTML : "";
  state.category = "featured:International / Argentina / Sports";
  state.virtualCategoryView = "list";
  renderLivePage();
  const featuredListView = document.elements.view ? document.elements.view.innerHTML : "";
  state.virtualCategoryView = "guide";
  state.category = "virtual:International / Argentina / Sports";
  renderLivePage();
  const virtualView = document.elements.view ? document.elements.view.innerHTML : "";
  const replayChannel = channelByID("channel:world-cup-replay");
  state.currentChannel = replayChannel;
  renderPlayerPage();
	const replayPlayerView = document.elements.view ? document.elements.view.innerHTML : "";
	const epgHTML = renderEPGCells(channel, 0);
	const epgProgramCells = epgHTML.split('style="left: calc(').slice(1).map(function(part) {
		const pieces = part.split(' * var(--epg-slot)); width: calc(');
		const widthPart = pieces[1] ? pieces[1].split(' * var(--epg-slot) - 0.0625rem);')[0] : "";
		return { left: Number(pieces[0]), width: Number(widthPart) };
	});
	const epgOverlapResolved = epgProgramCells.length >= 2 && epgProgramCells[1].left + 0.001 >= epgProgramCells[0].left + epgProgramCells[0].width;
const guideStartsAtCurrentSlot = guideWindow().start === Math.floor(Math.floor(Date.now() / 1000) / 1800) * 1800;
	state.view = "home";
	state.category = "";
	state.query = "Second overlapping";
	const programSearchMatchesEPG = visibleChannels(false).some(function(item) { return item.id === "channel:argentina-sports"; });
	return {
    sourcePath: !!source,
    aliasPath: !!alias,
    secondAliasPath: !!secondAlias,
    sourceCount: channelsInSource.length,
    aliasCount: channelsInAlias.length,
    secondAliasCount: channelsInSecondAlias.length,
    objectParsedMode: readAdminSettingsValue({ mode: "delimiter", delimiter: "pipe" }).mode,
    stringParsedMode: readAdminSettingsValue(JSON.stringify({ mode: "delimiter", delimiter: "pipe" })).mode,
    featuredSection: grid.indexOf(">Featured Groups<") !== -1,
    featuredRenamedSection: renamedGrid.indexOf(">Featured Things<") !== -1 && renamedGrid.indexOf(">Featured Groups<") === -1,
    guideRenamedAllOption: renamedGuideView.indexOf(">All things</option>") !== -1 && renamedGuideView.indexOf(">All groups</option>") === -1,
    featuredCategory: grid.indexOf("International | Argentina | Sports") !== -1,
    featuredAlphabetical: grid.indexOf(">Admin Favorites</strong>") !== -1 && grid.indexOf(">World Cup</strong>") !== -1 && grid.indexOf(">Admin Favorites</strong>") < grid.indexOf(">World Cup</strong>"),
    featuredVirtualCategory: grid.indexOf('data-category="featured:International / Argentina / Sports"') !== -1,
    featuredSourceCategory: grid.indexOf('data-category="source:cat:argentina-sports"') !== -1,
    featuredMarkerVisible: grid.indexOf("* International") !== -1,
    featuredBreadcrumbRoot: featuredView.indexOf(">Featured Groups</button>") !== -1,
    featuredBreadcrumbPath: featuredView.indexOf(">International</button>") !== -1 && featuredView.indexOf(">Argentina</button>") !== -1 && featuredView.indexOf(">Sports</button>") !== -1,
    featuredGuide: featuredView.indexOf('data-channel="channel:argentina-sports"') !== -1,
    featuredGuideHeading: featuredView.indexOf(">TV Guide<") !== -1,
    featuredViewToggle: featuredView.indexOf('data-virtual-category-view="guide"') !== -1 && featuredView.indexOf('data-virtual-category-view="list"') !== -1,
    featuredListView: featuredListView.indexOf(">Channels<") !== -1 && featuredListView.indexOf('class="virtual-channel-button" data-channel="channel:argentina-sports"') !== -1 && featuredListView.indexOf(">TV Guide<") === -1,
    featuredBackButton: featuredView.indexOf(">Back</button>") !== -1,
    simpleFeaturedCategory: grid.indexOf('data-category="featured:Admin Favorites"') !== -1,
    simpleFeaturedGuide: simpleFeaturedView.indexOf(">Featured Groups</button>") !== -1 && simpleFeaturedView.indexOf(">Admin Favorites</button>") !== -1 && simpleFeaturedView.indexOf('data-channel="channel:admin-favorites"') !== -1,
    simpleFeaturedViewToggle: simpleFeaturedView.indexOf('data-virtual-category-view="guide"') !== -1 && simpleFeaturedView.indexOf('data-virtual-category-view="list"') !== -1,
    simpleFeaturedSourcePage: simpleFeaturedView.indexOf(">Featured Groups<") !== -1 && simpleFeaturedView.indexOf(">Virtual Groups<") !== -1 && simpleFeaturedView.indexOf(">Admin Favorites<") !== -1,
    virtualBreadcrumbRoot: virtualView.indexOf(">Virtual Groups</button>") !== -1,
    virtualGuideHeading: virtualView.indexOf(">TV Guide<") !== -1,
    virtualBackButton: virtualView.indexOf(">Back</button>") !== -1,
    channelCategoryName: channel ? channel.categoryName : "",
    replayRewindable: isRewindableChannel(replayChannel),
    normalRewindable: isRewindableChannel(channel),
		replayPlayerClass: replayPlayerView.indexOf('class="playback-shell is-replay"') !== -1,
		replayPlayerControls: replayPlayerView.indexOf('controls></video>') !== -1,
		replayPlayerTag: replayPlayerView.indexOf(">Replay</span>") !== -1,
		epgOverlapResolved: epgOverlapResolved,
		guideStartsAtCurrentSlot: guideStartsAtCurrentSlot,
		programSearchMatchesEpg: programSearchMatchesEPG
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

func TestDvrEnabledForSourceAllowsDispatcharrDirectModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceMode model.SourceMode
		want       bool
	}{
		{name: "direct login", sourceMode: model.SourceModeDirectLogin, want: true},
		{name: "api key", sourceMode: model.SourceModeAPIKey, want: true},
		{name: "xtream", sourceMode: model.SourceModeXtream, want: false},
		{name: "m3u xmltv", sourceMode: model.SourceModeM3UXMLTV, want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := dvrEnabledForSource(tt.sourceMode); got != tt.want {
				t.Fatalf("dvrEnabledForSource(%q) = %t, want %t", tt.sourceMode, got, tt.want)
			}
		})
	}
}

func TestHTTPRoutesServerScheduleRecordingReportsDispatcharrPermission(t *testing.T) {
	t.Parallel()

	const channelUUID = "dispatcharr-channel-1"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/accounts/token/":
			_, _ = w.Write([]byte(`{"access":"token","refresh":"refresh"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/channels/channels/":
			_, _ = w.Write([]byte(`[{"id":4131,"uuid":"` + channelUUID + `","name":"News HD","effective_name":"News HD","effective_tvg_id":"news.hd"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/channels/recordings/":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"detail":"You do not have permission to perform this action."}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
		}
	}))
	defer upstream.Close()

	channel := model.Channel{
		ID:        model.StableChannelID(model.SourceModeDirectLogin, model.ChannelIdentity{UpstreamID: channelUUID, GuideID: "news.hd", Name: "News HD", StreamURL: upstream.URL + "/proxy/ts/stream/" + channelUUID}),
		Name:      "News HD",
		GuideID:   "news.hd",
		StreamURL: upstream.URL + "/proxy/ts/stream/" + channelUUID,
	}
	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeDirectLogin),
			Channels: []model.Channel{channel},
		},
	})
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeDirectLogin,
			DispatcharrURL:  upstream.URL,
			DispatcharrUser: "demo",
			DispatcharrPass: "secret",
			ChannelRefreshH: config.DefaultChannelRefreshHours,
			EPGRefreshH:     config.DefaultEPGRefreshHours,
		}
	})

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: http.MethodPost,
		Path:   "/dispatcharr/api/recordings",
		Body:   []byte(fmt.Sprintf(`{"channelId":%q,"title":"News","startUnix":1900000000,"endUnix":1900003600}`, channel.ID)),
	})
	if err != nil {
		t.Fatalf("schedule route: %v", err)
	}
	if response.GetStatusCode() != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", response.GetStatusCode(), response.GetBody())
	}
	if !strings.Contains(string(response.GetBody()), "admin account or API key") {
		t.Fatalf("expected actionable permission message, got %q", response.GetBody())
	}
}

func TestScheduleRecordingErrorResponseMapsDispatcharrAuthFailures(t *testing.T) {
	t.Parallel()

	for _, message := range []string{
		"unexpected status 401: {\"detail\":\"Authentication credentials were not provided.\"}",
		"unexpected status 403: {\"detail\":\"You do not have permission to perform this action.\"}",
		"unauthorized",
		"permission denied",
	} {
		response := scheduleRecordingErrorResponse(errors.New(message))
		if response.GetStatusCode() != http.StatusForbidden {
			t.Fatalf("expected auth failure %q to map to 403, got %d", message, response.GetStatusCode())
		}
		if !strings.Contains(string(response.GetBody()), "admin account or API key") {
			t.Fatalf("expected actionable auth message for %q, got %q", message, response.GetBody())
		}
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

func TestHTTPRoutesServerAppRouteRefreshesStalePersistedSnapshotForCurrentSettings(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		ConfigKey: config.CatalogCacheKey(config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: "https://old.example.com", XtreamUsername: "demo"}),
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{{ID: "xtream:old", Name: "Old Channel"}},
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

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/dispatcharr/api/app"})
	if err != nil {
		t.Fatalf("app route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	if syncer.calls != 1 {
		t.Fatalf("expected stale persisted snapshot to refresh, got %d calls", syncer.calls)
	}
	var payload AppPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal app payload: %v", err)
	}
	if len(payload.Channels) != 1 || payload.Channels[0].ID != "dispatcharr:news" {
		t.Fatalf("expected current settings payload, got %+v", payload.Channels)
	}
}

func TestHTTPRoutesServerAppRouteClearsStalePersistedSnapshotWhenCurrentSettingsInvalid(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		ConfigKey: config.CatalogCacheKey(config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: "https://old.example.com", XtreamUsername: "demo"}),
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{{ID: "xtream:old", Name: "Old Channel"}},
		},
	})
	syncer := &stubCatalogSyncer{store: store}
	server := NewHTTPRoutesServerWithSyncer(store, func() config.Settings {
		return config.Settings{
			SourceMode:      config.SourceModeDirectLogin,
			DispatcharrURL:  "https://dispatcharr.example.com",
			DispatcharrUser: "demo",
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
	if syncer.calls != 0 {
		t.Fatalf("expected invalid settings to skip sync, got %d calls", syncer.calls)
	}
	var payload AppPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal app payload: %v", err)
	}
	if len(payload.Channels) != 0 {
		t.Fatalf("expected stale channels to be cleared, got %+v", payload.Channels)
	}
}

func TestHTTPRoutesServerRefreshRouteStartsBackgroundCatalogSync(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeDirectLogin),
			Channels: []model.Channel{{ID: "dispatcharr:old", Name: "Old Channel"}},
		},
	})
	store.ReplacePrograms([]model.Program{
		{ID: "program:old-1", ChannelID: "dispatcharr:old", Title: "Old Morning"},
		{ID: "program:old-2", ChannelID: "dispatcharr:old", Title: "Old Evening"},
	}, 100)
	block := make(chan struct{})
	done := make(chan struct{}, 1)
	syncer := &stubCatalogSyncer{store: store, block: block, done: done}
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
	if response.GetStatusCode() != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", response.GetStatusCode())
	}
	var payload AppPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal app payload: %v", err)
	}
	if len(payload.Channels) != 1 || payload.Channels[0].ID != "dispatcharr:old" {
		t.Fatalf("expected current channel payload while sync runs, got %+v", payload.Channels)
	}
	if payload.Status.EPGStatus != "loading" {
		t.Fatalf("expected loading EPG status while sync runs, got %+v", payload.Status)
	}
	if len(payload.Programs) != 2 {
		t.Fatalf("expected current guide payload while sync runs, got %+v", payload.Programs)
	}

	close(block)
	waitForStubSync(t, done)
	if syncer.forceCallCount() != 1 {
		t.Fatalf("expected refresh route to force guide purge sync, got %d force calls", syncer.forceCallCount())
	}
	if syncer.callCount() != 1 {
		t.Fatalf("expected refresh to force one sync, got %d calls", syncer.callCount())
	}
	current := store.Current()
	if len(current.Catalog.Channels) != 1 || current.Catalog.Channels[0].ID != "dispatcharr:news" {
		t.Fatalf("expected refreshed channel payload, got %+v", current.Catalog.Channels)
	}
	if current.Health.EPGStatus != "ok" || current.Health.EPGProgramCount != 1 {
		t.Fatalf("expected refreshed guide health, got %+v", current.Health)
	}
	if len(current.Catalog.Programs) != 1 || current.Catalog.Programs[0].ID != "program:1" {
		t.Fatalf("expected refreshed guide programs, got %+v", current.Catalog.Programs)
	}
}

func TestHTTPRoutesServerGuidePingRefreshesWhenAnyCheckedChannelIsMissingGuide(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	now := time.Now().Unix()
	settings := config.Settings{
		SourceMode:      config.SourceModeDirectLogin,
		DispatcharrURL:  "https://dispatcharr.example.com",
		DispatcharrUser: "demo",
		DispatcharrPass: "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     24,
	}
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{
		Source: model.LiveTVSource(model.SourceModeDirectLogin),
		Channels: []model.Channel{
			{ID: "dispatcharr:news", Name: "News HD"},
			{ID: "dispatcharr:sports", Name: "Sports HD"},
		},
	}, ConfigKey: config.CatalogCacheKey(settings)})
	store.ReplacePrograms([]model.Program{{
		ID:        "program:news",
		ChannelID: "dispatcharr:news",
		Title:     "Current News",
		StartUnix: now - 60,
		EndUnix:   now + 1800,
	}}, now)
	done := make(chan struct{}, 1)
	syncer := &stubCatalogSyncer{store: store, done: done}
	server := NewHTTPRoutesServerWithSyncer(store, func() config.Settings { return settings }, syncer)

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: http.MethodPost,
		Path:   "/dispatcharr/api/guide/ping",
		Body:   []byte(`{"channelIds":["dispatcharr:news","dispatcharr:sports"]}`),
	})
	if err != nil {
		t.Fatalf("guide ping: %v", err)
	}
	if response.GetStatusCode() != http.StatusAccepted {
		t.Fatalf("expected partial guide to start refresh, got %d", response.GetStatusCode())
	}
	var payload GuidePingPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal guide ping payload: %v", err)
	}
	if payload.Status != "refreshing" || payload.CurrentPrograms != 1 {
		t.Fatalf("expected one covered channel and refreshing status, got %+v", payload)
	}
	waitForStubSync(t, done)
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
	store      *cache.Store
	calls      int
	forceCalls int
	mu         sync.Mutex
	block      <-chan struct{}
	done       chan<- struct{}
}

func (s *stubCatalogSyncer) ForceSyncNow(ctx context.Context, settings config.Settings, nowUnix int64) error {
	s.mu.Lock()
	s.forceCalls++
	s.mu.Unlock()
	return s.SyncNow(ctx, settings, nowUnix)
}

func (s *stubCatalogSyncer) RefreshGuideOnlyNow(ctx context.Context, settings config.Settings, nowUnix int64) error {
	return s.SyncNow(ctx, settings, nowUnix)
}

func (s *stubCatalogSyncer) SyncNow(_ context.Context, settings config.Settings, nowUnix int64) error {
	if s.block != nil {
		<-s.block
	}
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	s.store.Replace(cache.Snapshot{
		ConfigKey: config.CatalogCacheKey(settings),
		Catalog: model.CatalogState{
			Source:   model.LiveTVSource(model.SourceModeDirectLogin),
			Channels: []model.Channel{{ID: "dispatcharr:news", Name: "News HD"}},
			Programs: []model.Program{{ID: "program:1", ChannelID: "dispatcharr:news", Title: "Morning News", StartUnix: 100, EndUnix: 200}},
		},
		Health: model.SyncHealth{LastSuccessUnix: nowUnix},
	})
	if s.done != nil {
		select {
		case s.done <- struct{}{}:
		default:
		}
	}
	return nil
}

func (s *stubCatalogSyncer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubCatalogSyncer) forceCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forceCalls
}

func waitForStubSync(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for background refresh")
	}
}

func TestHTTPRoutesServerPreferencesRoutePersistsFullPayload(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/preferences",
		Body:   []byte(`{"favorites":{"channel:1":true},"favoriteOrder":["channel:1","channel:3"],"autoFavorites":{"channel:2":true},"hiddenCategories":{"sports":true},"sportsFavoriteTeams":{"mlb:cin":true},"keywordPasses":[{"id":"keyword:world-cup","keyword":"World Cup","createdAt":1234}],"recentChannels":["channel:1"],"continueWatching":{"channel:1":{"plays":3}},"playback":{"streamMode":"redirect","outputFormat":"hls"},"categoryParsing":{"enabled":true,"mode":"delimiter","delimiter":"pipe","regex":"","output":""},"customGroups":[{"id":"group:spanish","name":"Spanish","order":10}],"customGroupMemberships":{"group:spanish":["channel:1","channel:2"]}}`),
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
	if len(prefs.FavoriteOrder) != 2 || prefs.FavoriteOrder[0] != "channel:1" || prefs.FavoriteOrder[1] != "channel:3" {
		t.Fatalf("expected favorite order to persist: %+v", prefs.FavoriteOrder)
	}
	if !prefs.SportsFavoriteTeams["mlb:cin"] {
		t.Fatalf("expected sports favorite team to persist: %+v", prefs.SportsFavoriteTeams)
	}
	if len(prefs.KeywordPasses) != 1 || prefs.KeywordPasses[0].Keyword != "World Cup" {
		t.Fatalf("expected keyword passes to persist: %+v", prefs.KeywordPasses)
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
		Body:    []byte(`{"mode":"delimiter","delimiter":"pipe","virtualGroupLabel":" Virtual Categories ","allowRecordingsByDefault":false,"categoryRenames":[{"sourcePath":" International | Arabic | Sports ","displayName":" International Sports "},{"sourcePath":"International | Arabic | Sports","displayName":"Duplicate Ignored"},{"sourcePath":"","displayName":"Nowhere"},{"sourcePath":"International | TV","displayName":""}],"categoryAliases":[{"sourcePath":" International | Arabic | Sports ","aliasPath":" Sports | Arabic "},{"sourcePath":"International | Arabic | Sports","aliasPath":"Sports | Arabic"},{"sourcePath":"International | Arabic | Sports","aliasPath":"World Cup | Arabic"},{"sourcePath":"","aliasPath":"Nowhere"},{"sourcePath":"International | Arabic | Sports","aliasPath":""}]}`),
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
	if payload["mode"] != "delimiter" || payload["delimiter"] != "pipe" {
		t.Fatalf("expected admin settings to persist: %+v", payload)
	}
	if payload["virtualGroupLabel"] != "Virtual Categories" {
		t.Fatalf("expected virtual group label to persist: %+v", payload)
	}
	if payload["allowRecordingsByDefault"] != false {
		t.Fatalf("expected admin recording default to persist: %+v", payload)
	}
	renames, ok := payload["categoryRenames"].([]any)
	if !ok || len(renames) != 1 {
		t.Fatalf("expected one normalized category rename, got %+v", payload["categoryRenames"])
	}
	firstRename, _ := renames[0].(map[string]any)
	if firstRename["sourcePath"] != "International | Arabic | Sports" || firstRename["displayName"] != "International Sports" {
		t.Fatalf("expected category rename to be trimmed and preserved, got %+v", firstRename)
	}
	aliases, ok := payload["categoryAliases"].([]any)
	if !ok || len(aliases) != 2 {
		t.Fatalf("expected two normalized category aliases, got %+v", payload["categoryAliases"])
	}
	firstAlias, _ := aliases[0].(map[string]any)
	secondAlias, _ := aliases[1].(map[string]any)
	if firstAlias["sourcePath"] != "International | Arabic | Sports" || firstAlias["aliasPath"] != "Sports | Arabic" {
		t.Fatalf("expected first category alias to be trimmed and preserved, got %+v", firstAlias)
	}
	if secondAlias["sourcePath"] != "International | Arabic | Sports" || secondAlias["aliasPath"] != "World Cup | Arabic" {
		t.Fatalf("expected second category alias to preserve another display path, got %+v", secondAlias)
	}
	if persisted["mode"] != "delimiter" || persisted["delimiter"] != "pipe" {
		t.Fatalf("expected admin settings to write through to host config: %+v", persisted)
	}
	if persisted["virtualGroupLabel"] != "Virtual Categories" {
		t.Fatalf("expected virtual group label to write through to host config: %+v", persisted)
	}
	if persisted["allowRecordingsByDefault"] != false {
		t.Fatalf("expected admin recording default to write through to host config: %+v", persisted)
	}
	persistedRenames, ok := persisted["categoryRenames"].([]map[string]string)
	if !ok || len(persistedRenames) != 1 {
		t.Fatalf("expected category renames to write through to host config: %+v", persisted)
	}
	persistedAliases, ok := persisted["categoryAliases"].([]map[string]string)
	if !ok || len(persistedAliases) != 2 {
		t.Fatalf("expected category aliases to write through to host config: %+v", persisted)
	}
}

func TestHTTPRoutesServerAdminSettingsRouteReturnsSavedPayloadWhenHostPersistFails(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	server.adminPersister = func(context.Context, map[string]any) error {
		return fmt.Errorf("host timeout")
	}
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method:  "POST",
		Path:    "/dispatcharr/api/admin-settings",
		Headers: map[string]string{"x-dispatcharr-admin-token": server.adminToken},
		Body:    []byte(`{"mode":"delimiter","delimiter":"pipe"}`),
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200 when host persistence fails, got %d", response.GetStatusCode())
	}
	if !server.store.HasAdminSettings() {
		t.Fatal("expected admin settings to be saved in plugin store")
	}
}

func TestHTTPRoutesServerAdminSettingsRoutePersistsPayloadToFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "category-settings.json")
	server := NewHTTPRoutesServerWithSyncerAndAdminSettingsFile(cache.NewStore(), nil, nil, path)
	server.adminPersister = func(context.Context, map[string]any) error {
		return nil
	}
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method:  "POST",
		Path:    "/dispatcharr/api/admin-settings",
		Headers: map[string]string{"x-dispatcharr-admin-token": server.adminToken},
		Body:    []byte(`{"mode":"admin_delimiter","delimiter":"dash","ecmEnabled":false,"ecmURL":" https://ecm.example.test/manage ","categoryAliases":[{"sourcePath":"International | Argentina | Sports","aliasPath":"Sports | Argentina"}],"groupAliases":[{"from":"International | Argentina | Sports"}]}`),
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read admin settings file: %v", err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode admin settings file: %v", err)
	}
	if saved["mode"] != "delimiter" || saved["delimiter"] != "dash" {
		t.Fatalf("expected normalized admin settings file, got %+v", saved)
	}
	if saved["ecmEnabled"] != false || saved["ecmURL"] != "https://ecm.example.test/manage" {
		t.Fatalf("expected normalized ECM settings file, got %+v", saved)
	}
	if aliases, ok := saved["categoryAliases"].([]any); !ok || len(aliases) != 1 {
		t.Fatalf("expected normalized category aliases in settings file, got %+v", saved["categoryAliases"])
	}
	if _, ok := saved["groupAliases"]; ok {
		t.Fatalf("expected stale remapping keys to be stripped: %+v", saved)
	}

	nextServer := NewHTTPRoutesServerWithSyncerAndAdminSettingsFile(cache.NewStore(), nil, nil, path)
	response, err = nextServer.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method:  "GET",
		Path:    "/dispatcharr/api/admin-settings",
		Headers: map[string]string{"x-dispatcharr-admin-token": nextServer.adminToken},
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	var loaded map[string]any
	if err := json.Unmarshal(response.GetBody(), &loaded); err != nil {
		t.Fatalf("decode loaded admin settings: %v", err)
	}
	if loaded["mode"] != "delimiter" || loaded["delimiter"] != "dash" {
		t.Fatalf("expected admin settings to load from file: %+v", loaded)
	}
	if loaded["ecmEnabled"] != false || loaded["ecmURL"] != "https://ecm.example.test/manage" {
		t.Fatalf("expected ECM settings to load from file: %+v", loaded)
	}
	if aliases, ok := loaded["categoryAliases"].([]any); !ok || len(aliases) != 1 {
		t.Fatalf("expected category aliases to load from file: %+v", loaded["categoryAliases"])
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

func TestHTTPRoutesServerAdminSettingsRouteAllowsUserRead(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServerWithSettings(cache.NewStore(), func() config.Settings {
		return config.Settings{AdminSettings: json.RawMessage(`{"mode":"delimiter","delimiter":"pipe"}`)}
	})
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "GET",
		Path:   "/dispatcharr/api/admin-settings",
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200 for user admin settings read, got %d", response.GetStatusCode())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal admin settings: %v", err)
	}
	if payload["mode"] != "delimiter" || payload["delimiter"] != "pipe" {
		t.Fatalf("expected configured admin settings: %+v", payload)
	}
}

func TestHTTPRoutesServerAdminSettingsRouteRequiresAdminPageTokenForPost(t *testing.T) {
	t.Parallel()

	response, err := NewHTTPRoutesServer(cache.NewStore()).Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: http.MethodPost,
		Path:   "/dispatcharr/api/admin-settings",
		Body:   []byte(`{"mode":"delimiter","delimiter":"pipe"}`),
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	if response.GetStatusCode() != 403 {
		t.Fatalf("expected 403 without admin settings token, got %d", response.GetStatusCode())
	}
}

func TestHTTPRoutesServerAdminSettingsRouteAcceptsQueryToken(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	query, _ := structpb.NewStruct(map[string]any{"admin_token": server.adminToken})
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "GET",
		Path:   "/dispatcharr/api/admin-settings",
		Query:  query,
	})
	if err != nil {
		t.Fatalf("admin settings route: %v", err)
	}
	if response.GetStatusCode() != 200 {
		t.Fatalf("expected 200 with admin settings query token, got %d", response.GetStatusCode())
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
	if !strings.Contains(string(response.GetBody()), `aria-label="Live TV sections"`) {
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
