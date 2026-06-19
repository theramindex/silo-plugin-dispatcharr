package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

type HTTPRoutesServer struct {
	pluginv1.UnimplementedHttpRoutesServer
	store            *cache.Store
	settingsProvider func() config.Settings
}

func NewHTTPRoutesServer(store *cache.Store) *HTTPRoutesServer {
	return &HTTPRoutesServer{store: store}
}

func NewHTTPRoutesServerWithSettings(store *cache.Store, settingsProvider func() config.Settings) *HTTPRoutesServer {
	return &HTTPRoutesServer{store: store, settingsProvider: settingsProvider}
}

type ChannelsPayload struct {
	SourceName string           `json:"sourceName"`
	Channels   []model.Channel  `json:"channels"`
	Categories []model.Category `json:"categories"`
}

type GuidePayload struct {
	Programs []model.Program `json:"programs"`
}

type ContentPayload struct {
	Available  bool             `json:"available"`
	Reason     string           `json:"reason,omitempty"`
	Categories []model.Category `json:"categories"`
	Items      any              `json:"items"`
}

type AppCapabilities struct {
	LiveTV                bool   `json:"liveTv"`
	Guide                 bool   `json:"guide"`
	VOD                   bool   `json:"vod"`
	Series                bool   `json:"series"`
	Favorites             bool   `json:"favorites"`
	HiddenCategories      bool   `json:"hiddenCategories"`
	BackendProxySupported bool   `json:"backendProxySupported"`
	StreamMode            string `json:"streamMode"`
	NativeLiveTVExport    bool   `json:"nativeLiveTvExport"`
}

type AppPayload struct {
	Status       HealthPayload     `json:"status"`
	Source       model.Source      `json:"source"`
	Channels     []model.Channel   `json:"channels"`
	Categories   []model.Category  `json:"categories"`
	VOD          ContentPayload    `json:"vod"`
	Series       ContentPayload    `json:"series"`
	Preferences  cache.Preferences `json:"preferences"`
	Capabilities AppCapabilities   `json:"capabilities"`
}

type toggleRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Hidden  bool   `json:"hidden"`
}

func (s *HTTPRoutesServer) Handle(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	switch request.GetPath() {
	case "/dispatcharr", "/dispatcharr/player":
		return htmlResponse(http.StatusOK, playerPageHTML), nil
	case "/dispatcharr/status", "/dispatcharr/api/status":
		return s.respondJSON(http.StatusOK, BuildHealthPayload(s.store.Current()))
	case "/dispatcharr/api/app":
		return s.respondJSON(http.StatusOK, s.buildAppPayload())
	case "/dispatcharr/channels", "/dispatcharr/api/channels":
		return s.respondJSON(http.StatusOK, s.channelsPayload())
	case "/dispatcharr/guide", "/dispatcharr/api/guide":
		channelID := queryValue(request, "channel_id")
		programs := programsForChannel(s.store.Current().Catalog.Programs, channelID)
		sort.Slice(programs, func(i, j int) bool {
			return programs[i].StartUnix < programs[j].StartUnix
		})
		return s.respondJSON(http.StatusOK, GuidePayload{Programs: programs})
	case "/dispatcharr/api/categories":
		return s.respondJSON(http.StatusOK, s.categoriesPayload())
	case "/dispatcharr/api/vod":
		return s.respondJSON(http.StatusOK, s.vodPayload())
	case "/dispatcharr/api/series":
		return s.respondJSON(http.StatusOK, s.seriesPayload())
	case "/dispatcharr/api/preferences":
		return s.handlePreferences(request)
	case "/dispatcharr/api/favorites":
		return s.handleFavorite(request)
	case "/dispatcharr/api/hidden-categories":
		return s.handleHiddenCategory(request)
	case "/dispatcharr/api/playback":
		return s.handlePlaybackSettings(request)
	case "/dispatcharr/stream":
		channelID := queryValue(request, "channel_id")
		if strings.TrimSpace(channelID) == "" {
			return textResponse(http.StatusBadRequest, "missing channel_id query parameter"), nil
		}
		streamURL, err := s.resolveStreamURL(channelID)
		if err != nil {
			return textResponse(http.StatusNotFound, err.Error()), nil
		}
		return redirectResponse(streamURL), nil
	case "/dispatcharr/vod/stream":
		itemID := queryValue(request, "item_id")
		if strings.TrimSpace(itemID) == "" {
			return textResponse(http.StatusBadRequest, "missing item_id query parameter"), nil
		}
		streamURL, err := s.resolveVODStreamURL(ctx, itemID)
		if err != nil {
			return textResponse(http.StatusNotFound, err.Error()), nil
		}
		return redirectResponse(streamURL), nil
	default:
		return textResponse(http.StatusNotFound, "route not found"), nil
	}
}

func (s *HTTPRoutesServer) buildAppPayload() AppPayload {
	snapshot := s.store.Current()
	preferences := s.store.Preferences()
	return AppPayload{
		Status:       BuildHealthPayload(snapshot),
		Source:       snapshot.Catalog.Source,
		Channels:     snapshot.Catalog.Channels,
		Categories:   liveCategories(snapshot),
		VOD:          s.vodPayload(),
		Series:       s.seriesPayload(),
		Preferences:  preferences,
		Capabilities: appCapabilities(preferences),
	}
}

func (s *HTTPRoutesServer) channelsPayload() ChannelsPayload {
	snapshot := s.store.Current()
	return ChannelsPayload{
		SourceName: snapshot.Catalog.Source.Name,
		Channels:   snapshot.Catalog.Channels,
		Categories: liveCategories(snapshot),
	}
}

func (s *HTTPRoutesServer) categoriesPayload() map[string][]model.Category {
	snapshot := s.store.Current()
	return map[string][]model.Category{
		"live":   liveCategories(snapshot),
		"vod":    snapshot.Catalog.Content.VODCategories,
		"series": snapshot.Catalog.Content.SeriesCategories,
	}
}

func (s *HTTPRoutesServer) vodPayload() ContentPayload {
	snapshot := s.store.Current()
	if snapshot.Catalog.Source.Mode == model.SourceModeM3UXMLTV {
		return ContentPayload{Available: false, Reason: "M3U/XMLTV mode only exposes Live TV and guide data.", Items: []model.VODItem{}}
	}
	return ContentPayload{
		Available:  len(snapshot.Catalog.Content.VODItems) > 0,
		Categories: snapshot.Catalog.Content.VODCategories,
		Items:      snapshot.Catalog.Content.VODItems,
	}
}

func (s *HTTPRoutesServer) seriesPayload() ContentPayload {
	snapshot := s.store.Current()
	if snapshot.Catalog.Source.Mode == model.SourceModeM3UXMLTV {
		return ContentPayload{Available: false, Reason: "M3U/XMLTV mode only exposes Live TV and guide data.", Items: []model.SeriesItem{}}
	}
	return ContentPayload{
		Available:  len(snapshot.Catalog.Content.SeriesItems) > 0,
		Categories: snapshot.Catalog.Content.SeriesCategories,
		Items:      snapshot.Catalog.Content.SeriesItems,
	}
}

func (s *HTTPRoutesServer) handlePreferences(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return s.respondJSON(http.StatusOK, s.store.Preferences())
	}
	var preferences cache.Preferences
	if err := json.Unmarshal(request.GetBody(), &preferences); err != nil {
		return textResponse(http.StatusBadRequest, "invalid preferences payload"), nil
	}
	return s.respondJSON(http.StatusOK, s.store.SetPreferences(preferences))
}

func (s *HTTPRoutesServer) handleFavorite(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return s.respondJSON(http.StatusOK, s.store.Preferences().Favorites)
	}
	var payload toggleRequest
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid favorite payload"), nil
	}
	if strings.TrimSpace(payload.ID) == "" {
		return textResponse(http.StatusBadRequest, "missing id"), nil
	}
	return s.respondJSON(http.StatusOK, s.store.SetFavorite(payload.ID, payload.Enabled))
}

func (s *HTTPRoutesServer) handleHiddenCategory(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return s.respondJSON(http.StatusOK, s.store.Preferences().HiddenCategories)
	}
	var payload toggleRequest
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid hidden category payload"), nil
	}
	if strings.TrimSpace(payload.ID) == "" {
		return textResponse(http.StatusBadRequest, "missing id"), nil
	}
	return s.respondJSON(http.StatusOK, s.store.SetHiddenCategory(payload.ID, payload.Hidden))
}

func (s *HTTPRoutesServer) handlePlaybackSettings(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return s.respondJSON(http.StatusOK, s.store.Preferences().Playback)
	}
	var settings cache.PlaybackSettings
	if err := json.Unmarshal(request.GetBody(), &settings); err != nil {
		return textResponse(http.StatusBadRequest, "invalid playback settings payload"), nil
	}
	return s.respondJSON(http.StatusOK, s.store.SetPlaybackSettings(settings).Playback)
}

func (s *HTTPRoutesServer) respondJSON(status int, value any) (*pluginv1.HandleHTTPResponse, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &pluginv1.HandleHTTPResponse{
		StatusCode: int32(status),
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: payload,
	}, nil
}

func (s *HTTPRoutesServer) resolveStreamURL(channelID string) (string, error) {
	snapshot := s.store.Current()
	for _, channel := range snapshot.Catalog.Channels {
		if channel.ID != channelID {
			continue
		}
		if strings.TrimSpace(channel.StreamURL) != "" {
			return channel.StreamURL, nil
		}
		if strings.HasPrefix(channel.ID, "xtream:") && s.settingsProvider != nil {
			streamID, err := strconv.ParseInt(strings.TrimPrefix(channel.ID, "xtream:"), 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid xtream channel id")
			}
			settings := s.settingsProvider()
			baseURL, username, password := xtreamConnectionSettings(settings)
			client := xtream.NewClient(baseURL, username, password)
			streamURL := client.ResolveLiveStreamURL(streamID)
			if strings.TrimSpace(streamURL) == "" {
				return "", fmt.Errorf("unable to resolve stream url")
			}
			return streamURL, nil
		}
		return "", fmt.Errorf("stream url unavailable for channel")
	}
	return "", fmt.Errorf("channel not found")
}

func (s *HTTPRoutesServer) resolveVODStreamURL(_ context.Context, itemID string) (string, error) {
	snapshot := s.store.Current()
	for _, item := range snapshot.Catalog.Content.VODItems {
		if item.ID != itemID {
			continue
		}
		if strings.TrimSpace(item.StreamURL) != "" {
			return item.StreamURL, nil
		}
		if !strings.HasPrefix(item.ID, "vod:") || s.settingsProvider == nil {
			return "", fmt.Errorf("stream url unavailable for item")
		}
		streamID, err := strconv.ParseInt(strings.TrimPrefix(item.ID, "vod:"), 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid vod item id")
		}
		settings := s.settingsProvider()
		baseURL, username, password := xtreamConnectionSettings(settings)
		client := xtream.NewClient(baseURL, username, password)
		streamURL := client.ResolveVODStreamURL(streamID, item.Container)
		if strings.TrimSpace(streamURL) == "" {
			return "", fmt.Errorf("unable to resolve vod stream url")
		}
		return streamURL, nil
	}
	return "", fmt.Errorf("vod item not found")
}

func xtreamConnectionSettings(settings config.Settings) (string, string, string) {
	if settings.SourceMode == config.SourceModeDirectLogin {
		return settings.DispatcharrURL, settings.DispatcharrUser, settings.DispatcharrPass
	}
	return settings.XtreamBaseURL, settings.XtreamUsername, settings.XtreamPassword
}

func programsForChannel(programs []model.Program, channelID string) []model.Program {
	if strings.TrimSpace(channelID) == "" {
		return append([]model.Program(nil), programs...)
	}
	filtered := make([]model.Program, 0, len(programs))
	for _, program := range programs {
		if program.ChannelID == channelID {
			filtered = append(filtered, program)
		}
	}
	return filtered
}

func liveCategories(snapshot cache.Snapshot) []model.Category {
	if len(snapshot.Catalog.Content.LiveCategories) > 0 {
		return snapshot.Catalog.Content.LiveCategories
	}
	seen := map[string]bool{}
	categories := make([]model.Category, 0)
	for _, channel := range snapshot.Catalog.Channels {
		if channel.CategoryID == "" || seen[channel.CategoryID] {
			continue
		}
		seen[channel.CategoryID] = true
		name := channel.CategoryName
		if name == "" {
			name = "Category " + channel.CategoryID
		}
		categories = append(categories, model.Category{ID: channel.CategoryID, Name: name, Kind: "live"})
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})
	return categories
}

func appCapabilities(preferences cache.Preferences) AppCapabilities {
	return AppCapabilities{
		LiveTV:                true,
		Guide:                 true,
		VOD:                   true,
		Series:                true,
		Favorites:             true,
		HiddenCategories:      true,
		BackendProxySupported: preferences.Playback.BackendProxySupported,
		StreamMode:            preferences.Playback.StreamMode,
		NativeLiveTVExport:    false,
	}
}

func queryValue(request *pluginv1.HandleHTTPRequest, key string) string {
	query := request.GetQuery()
	if query == nil {
		return ""
	}
	value := query.AsMap()[key]
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func redirectResponse(location string) *pluginv1.HandleHTTPResponse {
	return &pluginv1.HandleHTTPResponse{
		StatusCode: http.StatusFound,
		Headers: map[string]string{
			"location": location,
		},
	}
}

func htmlResponse(status int, body string) *pluginv1.HandleHTTPResponse {
	return &pluginv1.HandleHTTPResponse{
		StatusCode: int32(status),
		Headers: map[string]string{
			"content-type": "text/html; charset=utf-8",
		},
		Body: []byte(body),
	}
}

func textResponse(status int, message string) *pluginv1.HandleHTTPResponse {
	return &pluginv1.HandleHTTPResponse{
		StatusCode: int32(status),
		Headers: map[string]string{
			"content-type": "text/plain; charset=utf-8",
		},
		Body: []byte(message),
	}
}

const playerPageHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Dispatcharr IPTV</title>
    <script src="https://cdn.jsdelivr.net/npm/hls.js@1"></script>
    <style>
      :root {
        color-scheme: dark;
        --bg: #0b0d0f;
        --rail: #141414;
        --panel: #1b1d1d;
        --panel-2: #242827;
        --line: #363a38;
        --text: #f2f1ec;
        --muted: #a8aaa5;
        --soft: #d9d1bf;
        --accent: #ff3d7f;
        --accent-2: #2fbf9f;
        --warn: #f6c95f;
      }
      * { box-sizing: border-box; }
      body { margin: 0; min-height: 100vh; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: var(--bg); color: var(--text); }
      button, input { font: inherit; }
      main { display: grid; grid-template-columns: minmax(18rem, 22rem) minmax(0, 1fr); min-height: 100vh; }
      aside { border-right: 1px solid var(--line); background: var(--rail); padding: 1rem; overflow: auto; }
      section { padding: clamp(1rem, 2vw, 1.6rem); min-width: 0; }
      h1, h2, h3 { margin: 0; }
      h1 { font-size: 1.45rem; letter-spacing: 0; }
      h2 { font-size: 1rem; margin: 1rem 0 0.6rem; color: var(--muted); }
      .topbar { display: flex; justify-content: space-between; gap: 1rem; align-items: center; margin-bottom: 1rem; }
      .status { color: var(--muted); font-size: 0.82rem; }
      .tabs { display: flex; flex-wrap: wrap; gap: 0.45rem; margin: 0 0 1rem; align-items: center; }
      .tab, .channel, .card, .pill, .control { border: 1px solid var(--line); background: var(--panel); color: var(--text); border-radius: 0.5rem; }
      .tab { padding: 0.58rem 0.8rem; cursor: pointer; min-height: 2.35rem; }
      .tab.active { border-color: var(--accent); background: color-mix(in oklab, var(--accent) 18%, var(--panel)); color: var(--text); }
      .search { width: 100%; margin: 0.8rem 0; padding: 0.75rem 0.8rem; border-radius: 0.5rem; border: 1px solid var(--line); background: #0f1010; color: var(--text); }
      .filters { display: flex; gap: 0.45rem; margin-bottom: 0.8rem; overflow-x: auto; padding-bottom: 0.25rem; }
      .pill { padding: 0.45rem 0.7rem; cursor: pointer; font-size: 0.78rem; color: var(--muted); white-space: nowrap; }
      .pill.active { color: #04120d; background: var(--accent); border-color: var(--accent); }
      .channel { display: grid; grid-template-columns: 2.7rem minmax(0, 1fr) auto; gap: 0.75rem; width: 100%; text-align: left; align-items: center; margin: 0 0 0.5rem; padding: 0.7rem; cursor: pointer; }
      .channel:hover, .card:hover { border-color: #61665f; background: var(--panel-2); }
      .logo { width: 2.35rem; height: 2.35rem; object-fit: contain; border-radius: 0.4rem; background: #0b0d0f; }
      .channel strong, .card strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .muted { color: var(--muted); }
      .star { color: var(--warn); }
      .hero { display: grid; grid-template-columns: minmax(0, 1fr) minmax(14rem, 22rem); gap: 1rem; align-items: start; }
      video { width: 100%; aspect-ratio: 16 / 9; max-height: 64vh; background: #000; border-radius: 0.55rem; border: 1px solid var(--line); }
      .now-panel { border-left: 3px solid var(--accent); padding: 0.25rem 0 0.25rem 0.85rem; min-width: 0; }
      .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(10.5rem, 1fr)); gap: 0.8rem; }
      .card { padding: 0.7rem; cursor: pointer; min-height: 8rem; text-align: left; }
      .poster { width: 100%; aspect-ratio: 2 / 3; height: auto; object-fit: cover; border-radius: 0.45rem; background: #0b0d0f; margin-bottom: 0.55rem; }
      .guide { display: grid; gap: 0.45rem; margin-top: 0.8rem; }
      .program { border-left: 3px solid var(--accent-2); background: #181b1a; padding: 0.6rem 0.75rem; border-radius: 0.45rem; }
      .controls { display: grid; gap: 0.75rem; max-width: 44rem; }
      .control { padding: 0.8rem; }
      .unsupported { color: var(--warn); }
      .count { margin-left: auto; color: var(--muted); font-size: 0.82rem; }
      .empty { color: var(--muted); padding: 1rem 0; }
      @media (max-width: 900px) {
        main { grid-template-columns: 1fr; }
        aside { border-right: 0; border-bottom: 1px solid var(--line); max-height: 44vh; }
        .hero { grid-template-columns: 1fr; }
      }
    </style>
  </head>
  <body>
    <main>
      <aside>
        <div class="topbar">
          <h1>Dispatcharr IPTV</h1>
          <span id="health" class="status">Loading...</span>
        </div>
        <input id="search" class="search" placeholder="Search channels, movies, series">
        <div id="filters" class="filters"></div>
        <div id="channels">Loading channels...</div>
      </aside>
      <section>
        <div class="tabs">
          <button class="tab active" data-tab="live">Live TV</button>
          <button class="tab" data-tab="favorites">Favorites</button>
          <button class="tab" data-tab="guide">Guide</button>
          <button class="tab" data-tab="vod">Movies</button>
          <button class="tab" data-tab="series">Series</button>
          <button class="tab" data-tab="settings">Settings</button>
          <span id="count" class="count"></span>
        </div>
        <div class="hero">
          <video id="player" controls autoplay playsinline></video>
          <div class="now-panel">
            <h2>Now</h2>
            <h1 id="now">Live TV</h1>
            <div id="now-meta" class="muted"></div>
          </div>
        </div>
        <div id="content" aria-live="polite"></div>
      </section>
    </main>
    <script>
      const path = window.location.pathname;
      const base = path.endsWith("/dispatcharr/player") ? path.slice(0, -"/dispatcharr/player".length) : (path.endsWith("/dispatcharr") ? path.slice(0, -"/dispatcharr".length) : "");
      const prefsKey = "silo.dispatcharr.preferences.v1";
      const state = { app: null, tab: "live", category: "", query: "", hls: null, currentChannel: null };

      function route(url) { return base + url; }
      function byId(id) { return document.getElementById(id); }
      function lower(value) { return String(value || "").toLowerCase(); }
      function items(value) { return Array.isArray(value) ? value : []; }
      function favoriteMap() { return (state.app && state.app.preferences && state.app.preferences.favorites) || {}; }
      function autoFavoriteMap() { return (state.app && state.app.preferences && state.app.preferences.autoFavorites) || {}; }
      function hiddenMap() { return (state.app && state.app.preferences && state.app.preferences.hiddenCategories) || {}; }
      function defaultPrefs() {
        return { favorites: {}, autoFavorites: {}, hiddenCategories: {}, recentChannels: [], continueWatching: {}, playback: { backendProxySupported: false, streamMode: "redirect", outputFormat: "hls" } };
      }
      function readLocalPrefs() {
        try { return Object.assign(defaultPrefs(), JSON.parse(localStorage.getItem(prefsKey) || "{}")); }
        catch (_) { return defaultPrefs(); }
      }
      function mergePrefs(remote, local) {
        remote = Object.assign(defaultPrefs(), remote || {});
        local = Object.assign(defaultPrefs(), local || {});
        return {
          favorites: Object.assign({}, remote.favorites, local.favorites),
          autoFavorites: Object.assign({}, remote.autoFavorites, local.autoFavorites),
          hiddenCategories: Object.assign({}, remote.hiddenCategories, local.hiddenCategories),
          recentChannels: items(local.recentChannels).length ? local.recentChannels : items(remote.recentChannels),
          continueWatching: Object.assign({}, remote.continueWatching, local.continueWatching),
          playback: Object.assign({}, remote.playback, local.playback)
        };
      }
      function savePrefs() {
        if (!state.app || !state.app.preferences) return;
        localStorage.setItem(prefsKey, JSON.stringify(state.app.preferences));
        postJSON("/dispatcharr/api/preferences", state.app.preferences).catch(function() {});
      }
      function rememberChannel(channel) {
        const prefs = state.app.preferences || defaultPrefs();
        const recent = [channel.id].concat(items(prefs.recentChannels).filter(function(id) { return id !== channel.id; })).slice(0, 16);
        prefs.recentChannels = recent;
        const plays = (prefs.continueWatching[channel.id] && prefs.continueWatching[channel.id].plays || 0) + 1;
        prefs.continueWatching[channel.id] = { kind: "channel", name: channel.name, playedAt: Date.now(), plays: plays };
        if (plays >= 3 && !prefs.favorites[channel.id]) prefs.autoFavorites[channel.id] = true;
        state.app.preferences = prefs;
        savePrefs();
      }

      async function getJSON(url) {
        const response = await fetch(route(url), { credentials: "include" });
        if (!response.ok) throw new Error("request failed");
        return response.json();
      }

      async function postJSON(url, body) {
        const response = await fetch(route(url), {
          method: "POST",
          credentials: "include",
          headers: { "content-type": "application/json" },
          body: JSON.stringify(body)
        });
        if (!response.ok) throw new Error("request failed");
        return response.json();
      }

      async function loadApp() {
        state.app = await getJSON("/dispatcharr/api/app");
        state.app.preferences = mergePrefs(state.app.preferences, readLocalPrefs());
        savePrefs();
        byId("health").textContent = state.app.status.status + " / " + state.app.status.channelCount + " channels";
        render();
      }

      function filteredChannels() {
        const hidden = hiddenMap();
        return items(state.app.channels).filter(function(channel) {
          if (channel.categoryId && hidden[channel.categoryId]) return false;
          if (state.tab === "favorites" && !favoriteMap()[channel.id] && !autoFavoriteMap()[channel.id]) return false;
          if (state.category && channel.categoryId !== state.category) return false;
          if (state.query && lower(channel.name).indexOf(state.query) === -1) return false;
          return true;
        });
      }

      function renderFilters() {
        const root = byId("filters");
        root.innerHTML = "";
        const all = document.createElement("button");
        all.className = "pill" + (state.category === "" ? " active" : "");
        all.textContent = "All";
        all.onclick = function() { state.category = ""; render(); };
        root.appendChild(all);
        for (const category of items(state.app.categories)) {
          const btn = document.createElement("button");
          btn.className = "pill" + (state.category === category.id ? " active" : "");
          btn.textContent = category.name || category.id;
          btn.onclick = function() { state.category = category.id; render(); };
          root.appendChild(btn);
        }
      }

      function renderChannels() {
        const root = byId("channels");
        root.innerHTML = "";
        const channels = filteredChannels();
        byId("count").textContent = channels.length + " channels";
        if (channels.length === 0) {
          root.innerHTML = "<div class=\"empty\">No channels match.</div>";
          return;
        }
        for (const channel of channels) {
          const button = document.createElement("button");
          button.className = "channel";
          const img = document.createElement("img");
          img.className = "logo";
          img.alt = "";
          if (channel.logoUrl) img.src = channel.logoUrl;
          const label = document.createElement("div");
          label.innerHTML = "<strong>" + escapeHTML(channel.name || "Untitled") + "</strong><br><span class=\"muted\">" + escapeHTML(channel.categoryName || channel.categoryId || "Live") + "</span>";
          const fav = document.createElement("span");
          fav.className = favoriteMap()[channel.id] || autoFavoriteMap()[channel.id] ? "star" : "muted";
          fav.textContent = favoriteMap()[channel.id] ? "★" : (autoFavoriteMap()[channel.id] ? "◆" : "☆");
          fav.onclick = async function(event) {
            event.stopPropagation();
            const next = !favoriteMap()[channel.id];
            if (next) state.app.preferences.favorites[channel.id] = true;
            else delete state.app.preferences.favorites[channel.id];
            delete state.app.preferences.autoFavorites[channel.id];
            savePrefs();
            postJSON("/dispatcharr/api/favorites", { id: channel.id, enabled: next }).then(function(prefs) {
              state.app.preferences = mergePrefs(prefs, readLocalPrefs());
              renderChannels();
            }).catch(function() {});
            renderChannels();
          };
          button.onclick = async function() { await playChannel(channel); };
          button.appendChild(img);
          button.appendChild(label);
          button.appendChild(fav);
          root.appendChild(button);
        }
      }

      async function playChannel(channel) {
        state.currentChannel = channel;
        rememberChannel(channel);
        setVideoSource(route("/dispatcharr/stream?channel_id=" + encodeURIComponent(channel.id)));
        byId("now").textContent = channel.name || "Live TV";
        byId("now-meta").textContent = [channel.number, channel.categoryName || channel.categoryId].filter(Boolean).join(" / ");
        const guide = await getJSON("/dispatcharr/api/guide?channel_id=" + encodeURIComponent(channel.id)).catch(function() { return { programs: [] }; });
        renderGuide(guide.programs || []);
      }

      function setVideoSource(url) {
        const video = byId("player");
        if (state.hls) {
          state.hls.destroy();
          state.hls = null;
        }
        if (window.Hls && Hls.isSupported() && url.indexOf(".m3u8") !== -1) {
          state.hls = new Hls();
          state.hls.loadSource(url);
          state.hls.attachMedia(video);
        } else {
          video.src = url;
        }
        video.play().catch(function() {});
      }

      function renderGuide(programs) {
        const root = byId("content");
        root.innerHTML = "<h2>Guide</h2>";
        const list = document.createElement("div");
        list.className = "guide";
        for (const program of items(programs).slice(0, 18)) {
          const start = program.startUnix ? new Date(program.startUnix * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "--:--";
          const row = document.createElement("div");
          row.className = "program";
          row.innerHTML = "<strong>" + escapeHTML(start + " " + (program.title || "Untitled")) + "</strong><br><span class=\"muted\">" + escapeHTML(program.summary || "") + "</span>";
          list.appendChild(row);
        }
        if (!list.children.length) list.innerHTML = "<div class=\"empty\">No guide entries.</div>";
        root.appendChild(list);
      }

      function renderContentCards(kind) {
        const payload = state.app[kind] || {};
        const root = byId("content");
        root.innerHTML = "<h2>" + (kind === "vod" ? "Movies" : "Series") + "</h2>";
        if (payload.available === false && payload.reason) {
          root.innerHTML += "<p class=\"unsupported\">" + escapeHTML(payload.reason) + "</p>";
        }
        const grid = document.createElement("div");
        grid.className = "grid";
        for (const item of items(payload.items).filter(function(item) { return !state.query || lower(item.name).indexOf(state.query) !== -1; })) {
          const card = document.createElement("button");
          card.className = "card";
          const poster = item.posterUrl ? "<img class=\"poster\" src=\"" + escapeHTML(item.posterUrl) + "\" alt=\"\">" : "";
          card.innerHTML = poster + "<strong>" + escapeHTML(item.name || "Untitled") + "</strong><br><span class=\"muted\">" + escapeHTML(item.rating || item.releaseDate || "") + "</span>";
          if (kind === "vod") {
            card.onclick = function() {
              setVideoSource(route("/dispatcharr/vod/stream?item_id=" + encodeURIComponent(item.id)));
              byId("now").textContent = item.name || "Movie";
            };
          }
          grid.appendChild(card);
        }
        if (!grid.children.length) grid.innerHTML = "<div class=\"empty\">No " + (kind === "vod" ? "movies" : "series") + " available.</div>";
        root.appendChild(grid);
      }

      function renderSettings() {
        const prefs = state.app.preferences || {};
        const root = byId("content");
        root.innerHTML = "<h2>Settings</h2>";
        const controls = document.createElement("div");
        controls.className = "controls";
        controls.innerHTML = "<div class=\"control\"><strong>Stream mode</strong><br><span class=\"muted\">Direct redirect is active. Backend proxy/remux is not available through the current buffered HTTP-route SDK response.</span></div>";
        for (const category of items(state.app.categories)) {
          const row = document.createElement("label");
          row.className = "control";
          const checked = hiddenMap()[category.id] ? " checked" : "";
          row.innerHTML = "<input type=\"checkbox\"" + checked + "> Hide " + escapeHTML(category.name || category.id);
          row.querySelector("input").onchange = async function(event) {
            if (event.target.checked) state.app.preferences.hiddenCategories[category.id] = true;
            else delete state.app.preferences.hiddenCategories[category.id];
            savePrefs();
            postJSON("/dispatcharr/api/hidden-categories", { id: category.id, hidden: event.target.checked }).then(function(prefs) {
              state.app.preferences = mergePrefs(prefs, readLocalPrefs());
            }).catch(function() {});
            render();
          };
          controls.appendChild(row);
        }
        if (prefs.playback && prefs.playback.backendProxyRequested) {
          controls.innerHTML += "<p class=\"unsupported\">Backend proxy was requested, but this SDK build only supports redirect playback.</p>";
        }
        root.appendChild(controls);
      }

      function render() {
        if (!state.app) return;
        renderFilters();
        renderChannels();
        document.querySelectorAll(".tab").forEach(function(tab) {
          tab.classList.toggle("active", tab.dataset.tab === state.tab);
        });
        if (state.tab === "favorites") {
          byId("content").innerHTML = "<h2>Favorites</h2>";
          if (!filteredChannels().length) byId("content").innerHTML += "<div class=\"empty\">No favorites yet.</div>";
        }
        if (state.tab === "vod") renderContentCards("vod");
        else if (state.tab === "series") renderContentCards("series");
        else if (state.tab === "settings") renderSettings();
        else if (state.tab === "guide") {
          if (state.currentChannel) playChannel(state.currentChannel);
          else renderGuide([]);
        }
      }

      function escapeHTML(value) {
        return String(value || "").replace(/[&<>"']/g, function(ch) {
          return ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[ch];
        });
      }

      byId("search").oninput = function(event) {
        state.query = event.target.value;
        render();
      };
      document.querySelectorAll(".tab").forEach(function(tab) {
        tab.onclick = function() {
          state.tab = tab.dataset.tab;
          render();
        };
      });
      loadApp().then(function() {
        const first = filteredChannels()[0];
        if (first) playChannel(first);
      }).catch(function() {
        byId("channels").textContent = "Unable to load Dispatcharr app data.";
        byId("health").textContent = "error";
      });
    </script>
  </body>
</html>`
