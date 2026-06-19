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
	Status       HealthPayload        `json:"status"`
	Source       model.Source         `json:"source"`
	Channels     []model.Channel      `json:"channels"`
	Programs     []model.Program      `json:"programs"`
	Categories   []model.Category     `json:"categories"`
	VOD          ContentPayload       `json:"vod"`
	Series       ContentPayload       `json:"series"`
	Preferences  cache.Preferences    `json:"preferences"`
	Sessions     []cache.WatchSession `json:"sessions"`
	Capabilities AppCapabilities      `json:"capabilities"`
}

type toggleRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Hidden  bool   `json:"hidden"`
}

type watchRequest struct {
	SessionID string `json:"sessionId"`
	ItemKind  string `json:"itemKind"`
	ItemID    string `json:"itemId"`
	ItemName  string `json:"itemName"`
	Reason    string `json:"reason"`
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
	case "/dispatcharr/api/watch/start":
		return s.handleWatchStart(request)
	case "/dispatcharr/api/watch/heartbeat":
		return s.handleWatchHeartbeat(request)
	case "/dispatcharr/api/watch/stop":
		return s.handleWatchStop(request)
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
		Programs:     snapshot.Catalog.Programs,
		Categories:   liveCategories(snapshot),
		VOD:          s.vodPayload(),
		Series:       s.seriesPayload(),
		Preferences:  preferences,
		Sessions:     s.store.ActiveSessions(),
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

func (s *HTTPRoutesServer) handleWatchStart(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	var payload watchRequest
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid watch payload"), nil
	}
	if strings.TrimSpace(payload.ItemID) == "" {
		return textResponse(http.StatusBadRequest, "missing itemId"), nil
	}
	if strings.TrimSpace(payload.ItemKind) == "" {
		payload.ItemKind = "channel"
	}
	session, preferences := s.store.StartWatch(payload.ItemKind, payload.ItemID, payload.ItemName)
	return s.respondJSON(http.StatusOK, map[string]any{"session": session, "preferences": preferences})
}

func (s *HTTPRoutesServer) handleWatchHeartbeat(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	var payload watchRequest
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid watch payload"), nil
	}
	session, ok := s.store.HeartbeatWatch(payload.SessionID)
	if !ok {
		return textResponse(http.StatusNotFound, "watch session not found"), nil
	}
	return s.respondJSON(http.StatusOK, map[string]any{"session": session})
}

func (s *HTTPRoutesServer) handleWatchStop(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	var payload watchRequest
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid watch payload"), nil
	}
	session, ok := s.store.StopWatch(payload.SessionID, payload.Reason)
	if !ok {
		return textResponse(http.StatusNotFound, "watch session not found"), nil
	}
	return s.respondJSON(http.StatusOK, map[string]any{"session": session})
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
        --bg: #171717;
        --rail: #1d1d1f;
        --rail-2: #222225;
        --panel: #2b2b2d;
        --panel-2: #353536;
        --line: #3e3e40;
        --text: #f5f3ef;
        --muted: #a7a5a0;
        --accent: #ff2f7d;
        --green: #173f31;
        --purple: #3b2147;
        --red: #4a211e;
        --blue: #1d3347;
        --warn: #f4c95f;
      }
      * { box-sizing: border-box; }
      body { margin: 0; min-height: 100vh; overflow: hidden; background: var(--bg); color: var(--text); font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
      button, input, select { font: inherit; }
      button { cursor: pointer; }
      .shell { display: grid; grid-template-columns: 19.5rem minmax(0, 1fr); height: 100vh; }
      .rail { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--line); background: linear-gradient(135deg, #19191a, #201e20); padding: 1rem; }
      .brand { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.25rem; }
      .brand h1 { margin: 0; font-size: 1.55rem; font-weight: 900; letter-spacing: 0; }
      .back { color: var(--muted); text-decoration: none; border: 1px solid var(--line); border-radius: 999px; padding: 0.42rem 0.65rem; font-size: 0.8rem; font-weight: 700; }
      .back:hover { color: var(--text); background: var(--panel); }
      .source { display: flex; align-items: center; gap: 0.6rem; margin: 0 0 0.7rem; color: var(--text); font-weight: 800; }
      .source-icon { width: 1.45rem; height: 1.45rem; border-radius: 999px; display: inline-grid; place-items: center; background: var(--accent); }
      .nav { display: grid; gap: 0.28rem; margin-bottom: 1rem; }
      .nav button { width: 100%; border: 0; border-radius: 0.65rem; background: transparent; color: var(--muted); display: flex; align-items: center; gap: 0.65rem; padding: 0.7rem 0.72rem; text-align: left; font-weight: 750; }
      .nav button.active, .nav button:hover { background: #2a292b; color: var(--text); }
      .nav small { margin-left: auto; color: var(--muted); }
      .rail-search { border: 1px solid var(--line); border-radius: 999px; background: #242426; color: var(--text); padding: 0.65rem 0.85rem; width: 100%; margin-bottom: 0.85rem; }
      .channel-list { overflow: auto; min-height: 0; padding-right: 0.2rem; }
      .channel-row { width: 100%; border: 0; border-radius: 0.75rem; background: transparent; color: var(--text); display: grid; grid-template-columns: 3.1rem minmax(0, 1fr) 1.8rem; align-items: center; gap: 0.65rem; padding: 0.45rem; text-align: left; }
      .channel-row:hover, .channel-row.active { background: #2a292b; }
      .logo { width: 3rem; height: 2.05rem; object-fit: contain; border-radius: 0.5rem; background: #121213; }
      .channel-row strong, .tile strong, .program strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .muted { color: var(--muted); }
      .star { color: var(--warn); font-size: 1rem; }
      .main { min-width: 0; overflow: auto; padding: 1rem 1.25rem 2rem; }
      .topbar { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 0.85rem; position: sticky; top: 0; z-index: 5; background: linear-gradient(180deg, var(--bg) 70%, rgba(23,23,23,0)); padding-bottom: 0.65rem; }
      .title { display: flex; align-items: center; gap: 0.55rem; min-width: 0; }
      .title h2 { margin: 0; font-size: 1.35rem; }
      .status { color: var(--muted); font-size: 0.82rem; white-space: nowrap; }
      .search { border: 1px solid var(--line); border-radius: 999px; background: #242426; color: var(--text); padding: 0.62rem 0.85rem; min-width: min(24rem, 40vw); }
      .section-title { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin: 1rem 0 0.55rem; color: var(--muted); font-size: 0.95rem; font-weight: 850; }
      .row-scroll { display: flex; gap: 0.6rem; overflow-x: auto; padding-bottom: 0.3rem; }
      .continue-card { flex: 0 0 15.5rem; border: 0; border-radius: 0.7rem; background: transparent; color: var(--text); text-align: left; }
      .poster-box { height: 8.7rem; border-radius: 0.65rem; background: #b19398; display: grid; place-items: center; overflow: hidden; margin-bottom: 0.45rem; }
      .poster-box img { width: 100%; height: 100%; object-fit: contain; }
      .poster-box span { font-size: 2.8rem; font-weight: 950; }
      .progress { height: 0.22rem; border-radius: 999px; background: rgba(255,255,255,0.14); overflow: hidden; margin: -0.75rem 0.85rem 0.6rem; position: relative; }
      .progress i { display: block; height: 100%; width: 62%; background: white; border-radius: inherit; }
      .guide-strip { display: grid; grid-auto-flow: column; grid-auto-columns: minmax(13rem, 1fr); gap: 0.35rem; overflow-x: auto; }
      .program { min-height: 3.05rem; border: 0; border-radius: 0.55rem; color: var(--text); text-align: left; padding: 0.48rem 0.65rem; background: var(--purple); }
      .program.green { background: var(--green); }
      .program.red { background: var(--red); }
      .program.blue { background: var(--blue); }
      .program.gray { background: var(--panel); }
      .program time { color: var(--muted); font-size: 0.78rem; display: block; }
      .category-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(8.7rem, 1fr)); gap: 0.5rem; }
      .tile { border: 0; border-radius: 0.65rem; background: var(--panel); color: var(--text); min-height: 3.85rem; text-align: left; padding: 0.75rem 0.85rem; font-weight: 850; }
      .tile:hover, .tile.active { background: var(--panel-2); }
      .guide-page { min-width: 54rem; }
      .guide-tools { display: grid; grid-template-columns: 10rem 1fr minmax(12rem, 22rem); align-items: center; gap: 0.8rem; margin-bottom: 0.7rem; }
      .select { border: 1px solid var(--line); border-radius: 999px; color: var(--text); background: var(--panel); padding: 0.55rem 0.75rem; }
      .time-head { display: grid; grid-template-columns: 9rem repeat(5, minmax(9.5rem, 1fr)); gap: 0.25rem; color: var(--muted); font-weight: 850; margin-bottom: 0.35rem; }
      .epg-row { display: grid; grid-template-columns: 9rem repeat(5, minmax(9.5rem, 1fr)); gap: 0.25rem; margin-bottom: 0.35rem; }
      .epg-channel { border-radius: 0.55rem; background: #b9969b; color: white; display: grid; grid-template-columns: 3.7rem minmax(0, 1fr); align-items: center; gap: 0.45rem; padding: 0.35rem; min-height: 3.25rem; }
      .epg-cell { border: 0; border-radius: 0.55rem; text-align: left; color: var(--text); background: var(--panel); padding: 0.5rem 0.65rem; min-width: 0; }
      .player-view { display: grid; grid-template-columns: minmax(0, 1fr) 22rem; gap: 1rem; align-items: start; }
      video { width: 100%; aspect-ratio: 16 / 9; background: #050505; border: 1px solid var(--line); border-radius: 0.75rem; }
      .now-card, .settings-card { border: 1px solid var(--line); background: var(--rail-2); border-radius: 0.8rem; padding: 0.85rem; }
      .settings-list { display: grid; gap: 0.55rem; }
      .settings-list label { display: flex; align-items: center; justify-content: space-between; gap: 1rem; background: var(--panel); border-radius: 0.65rem; padding: 0.7rem; }
      .empty { color: var(--muted); padding: 1rem 0; }
      .hide { display: none !important; }
      @media (max-width: 900px) {
        body { overflow: auto; }
        .shell { display: block; height: auto; }
        .rail { min-height: auto; border-right: 0; border-bottom: 1px solid var(--line); }
        .channel-list { max-height: 18rem; }
        .topbar { position: static; }
        .search { min-width: 0; width: 100%; }
        .player-view { grid-template-columns: 1fr; }
      }
    </style>
  </head>
  <body>
    <div class="shell">
      <aside class="rail">
        <div class="brand">
          <h1>IPTV</h1>
          <a class="back" href="/" aria-label="Back to Silo">&lt;- Silo</a>
        </div>
        <div class="source"><span class="source-icon">~</span><span id="source-name">Dispatcharr</span></div>
        <nav class="nav" aria-label="Dispatcharr views">
          <button class="active" data-view="home">Home</button>
          <button data-view="favorites">Favorites <small id="favorite-count">0</small></button>
          <button data-view="live">Live TV</button>
          <button data-view="guide">TV Guide</button>
          <button data-view="settings">Settings</button>
        </nav>
        <input id="rail-search" class="rail-search" placeholder="Search channels">
        <div id="channel-list" class="channel-list">Loading channels...</div>
      </aside>
      <main class="main">
        <div class="topbar">
          <div class="title"><span class="source-icon">~</span><h2 id="page-title">Home</h2><span id="health" class="status">Loading...</span></div>
          <input id="global-search" class="search" placeholder="Search by program or channel">
        </div>
        <div id="view"></div>
      </main>
    </div>
    <script>
      const path = window.location.pathname;
      const base = path.endsWith("/dispatcharr/player") ? path.slice(0, -"/dispatcharr/player".length) : (path.endsWith("/dispatcharr") ? path.slice(0, -"/dispatcharr".length) : "");
      const prefsKey = "silo.ramindex.dispatcharr.preferences.v1";
      const state = { app: null, view: "home", category: "", query: "", hls: null, currentChannel: null, currentSession: null, heartbeat: null };

      function route(url) { return base + url; }
      function byId(id) { return document.getElementById(id); }
      function items(value) { return Array.isArray(value) ? value : []; }
      function lower(value) { return String(value || "").toLowerCase(); }
      function escapeHTML(value) {
        return String(value || "").replace(/[&<>"']/g, function(ch) {
          return ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[ch];
        });
      }
      function defaultPrefs() {
        return { favorites: {}, autoFavorites: {}, hiddenCategories: {}, recentChannels: [], continueWatching: {}, playback: { backendProxySupported: false, streamMode: "redirect", outputFormat: "hls" } };
      }
      function prefs() { return state.app && state.app.preferences ? state.app.preferences : defaultPrefs(); }
      function favoriteMap() { return prefs().favorites || {}; }
      function autoFavoriteMap() { return prefs().autoFavorites || {}; }
      function hiddenMap() { return prefs().hiddenCategories || {}; }
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
      function readLocalPrefs() {
        try { return Object.assign(defaultPrefs(), JSON.parse(localStorage.getItem(prefsKey) || "{}")); }
        catch (_) { return defaultPrefs(); }
      }
      function savePrefs() {
        if (!state.app || !state.app.preferences) return;
        localStorage.setItem(prefsKey, JSON.stringify(state.app.preferences));
        postJSON("/dispatcharr/api/preferences", state.app.preferences).catch(function() {});
      }
      async function getJSON(url) {
        const response = await fetch(route(url), { credentials: "include" });
        if (!response.ok) throw new Error("request failed");
        return response.json();
      }
      async function postJSON(url, body) {
        const response = await fetch(route(url), { method: "POST", credentials: "include", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
        if (!response.ok) throw new Error("request failed");
        return response.json();
      }
      function channelByID(id) {
        return items(state.app.channels).find(function(channel) { return channel.id === id; }) || null;
      }
      function visibleChannels(ignoreQuery) {
        const hidden = hiddenMap();
        return items(state.app.channels).filter(function(channel) {
          if (channel.categoryId && hidden[channel.categoryId]) return false;
          if (state.category && channel.categoryId !== state.category) return false;
          if (!ignoreQuery && state.query && lower([channel.name, channel.categoryName, channel.number].join(" ")).indexOf(lower(state.query)) === -1) return false;
          if (state.view === "favorites" && !favoriteMap()[channel.id] && !autoFavoriteMap()[channel.id]) return false;
          return true;
        });
      }
      function programsFor(channelID) {
        const now = Math.floor(Date.now() / 1000);
        return items(state.app.programs).filter(function(program) {
          return (!channelID || program.channelId === channelID) && (!program.endUnix || program.endUnix >= now - 3600);
        }).sort(function(a, b) { return (a.startUnix || 0) - (b.startUnix || 0); });
      }
      function timeLabel(unix) {
        if (!unix) return "";
        return new Date(unix * 1000).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
      }
      function setView(view) {
        state.view = view;
        if (view !== "live") state.category = state.category;
        render();
      }
      function setCategory(id) {
        state.category = id || "";
        state.view = id ? "live" : "home";
        render();
      }
      async function loadApp() {
        state.app = await getJSON("/dispatcharr/api/app");
        state.app.preferences = mergePrefs(state.app.preferences, readLocalPrefs());
        state.app.programs = items(state.app.programs);
        savePrefs();
        byId("source-name").textContent = state.app.source && state.app.source.name ? state.app.source.name : "Dispatcharr";
        byId("health").textContent = state.app.status.status + " / " + state.app.status.channelCount + " channels";
        render();
      }
      function renderRail() {
        document.querySelectorAll(".nav button").forEach(function(button) {
          button.classList.toggle("active", button.dataset.view === state.view);
        });
        byId("favorite-count").textContent = Object.keys(favoriteMap()).length + Object.keys(autoFavoriteMap()).length;
        const root = byId("channel-list");
        root.innerHTML = "";
        const channels = visibleChannels(false).slice(0, 140);
        if (!channels.length) {
          root.innerHTML = "<div class=\"empty\">No channels match.</div>";
          return;
        }
        channels.forEach(function(channel) {
          const button = document.createElement("button");
          button.className = "channel-row" + (state.currentChannel && state.currentChannel.id === channel.id ? " active" : "");
          button.innerHTML = logoHTML(channel) + "<span><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><small class=\"muted\">" + escapeHTML(channel.categoryName || channel.categoryId || "Live TV") + "</small></span><span class=\"star\">" + (favoriteMap()[channel.id] ? "*" : (autoFavoriteMap()[channel.id] ? "+" : "")) + "</span>";
          button.onclick = function() { playChannel(channel); };
          root.appendChild(button);
        });
      }
      function logoHTML(channel) {
        if (channel.logoUrl) return "<img class=\"logo\" src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">";
        return "<div class=\"logo\" aria-hidden=\"true\"></div>";
      }
      function render() {
        if (!state.app) return;
        renderRail();
        byId("page-title").textContent = state.view === "home" ? "Home" : state.view === "live" ? (categoryName(state.category) || "Live TV") : state.view === "guide" ? "TV Guide" : state.view === "favorites" ? "Favorites" : "Settings";
        if (state.view === "guide") renderGuidePage();
        else if (state.view === "live" || state.view === "favorites") renderLivePage();
        else if (state.view === "settings") renderSettings();
        else renderHome();
      }
      function renderHome() {
        const root = byId("view");
        const recent = items(prefs().recentChannels).map(channelByID).filter(Boolean);
        root.innerHTML = sectionHeader("Continue watching") + rowCards(recent.length ? recent : visibleChannels(false).slice(0, 6)) + sectionHeader("TV Guide") + "<div id=\"strip\" class=\"guide-strip\"></div>" + sectionHeader("Categories") + categoryGrid();
        renderProgramStrip();
      }
      function sectionHeader(title) {
        return "<div class=\"section-title\"><span>" + escapeHTML(title) + "</span></div>";
      }
      function rowCards(channels) {
        if (!channels.length) return "<div class=\"empty\">No channels yet.</div>";
        return "<div class=\"row-scroll\">" + channels.map(function(channel) {
          return "<button class=\"continue-card\" data-channel=\"" + escapeHTML(channel.id) + "\"><div class=\"poster-box\">" + (channel.logoUrl ? "<img src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">" : "<span>" + escapeHTML((channel.name || "TV").slice(0, 5)) + "</span>") + "</div><div class=\"progress\"><i></i></div><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><div class=\"muted\">" + escapeHTML(channel.categoryName || "Live TV") + "</div></button>";
        }).join("") + "</div>";
      }
      function categoryGrid() {
        const categories = items(state.app.categories);
        if (!categories.length) return "<div class=\"empty\">No categories yet.</div>";
        return "<div class=\"category-grid\">" + categories.map(function(category) {
          return "<button class=\"tile" + (state.category === category.id ? " active" : "") + "\" data-category=\"" + escapeHTML(category.id) + "\"><strong>" + escapeHTML(category.name || category.id) + "</strong></button>";
        }).join("") + "</div>";
      }
      function renderProgramStrip() {
        const root = byId("strip");
        if (!root) return;
        const programs = programsFor("").slice(0, 18);
        root.innerHTML = programs.length ? programs.map(function(program, index) {
          const channel = channelByID(program.channelId) || {};
          return "<button class=\"program " + colorClass(index) + "\" data-channel=\"" + escapeHTML(program.channelId) + "\"><time>" + escapeHTML(timeLabel(program.startUnix)) + "</time><strong>" + escapeHTML(program.title || "Data not available") + "</strong><span class=\"muted\">" + escapeHTML(channel.name || "") + "</span></button>";
        }).join("") : "<div class=\"empty\">No guide data available.</div>";
      }
      function renderLivePage() {
        const channels = visibleChannels(false);
        byId("view").innerHTML = sectionHeader("Categories") + categoryGrid() + sectionHeader(state.view === "favorites" ? "Favorite channels" : "Channels") + rowCards(channels.slice(0, 24)) + playerBlock();
      }
      function playerBlock() {
        return "<div class=\"section-title\"><span>Player</span></div><div class=\"player-view\"><video id=\"player\" controls autoplay playsinline></video><aside class=\"now-card\"><h2 id=\"now-title\">" + escapeHTML(state.currentChannel ? state.currentChannel.name : "Choose a channel") + "</h2><p id=\"now-meta\" class=\"muted\">" + escapeHTML(state.currentChannel ? (state.currentChannel.categoryName || "Live TV") : "Live TV") + "</p><div id=\"now-guide\"></div></aside></div>";
      }
      function renderGuidePage() {
        const categories = items(state.app.categories);
        byId("view").innerHTML = "<div class=\"guide-page\"><div class=\"guide-tools\"><a class=\"back\" href=\"/\" aria-label=\"Back to Silo\">&lt;- Silo</a><select id=\"category-select\" class=\"select\"><option value=\"\">All categories</option>" + categories.map(function(category) { return "<option value=\"" + escapeHTML(category.id) + "\"" + (state.category === category.id ? " selected" : "") + ">" + escapeHTML(category.name || category.id) + "</option>"; }).join("") + "</select><input id=\"guide-search\" class=\"search\" placeholder=\"Search by program name\"></div><div class=\"time-head\"><span>Today</span><span>Now</span><span>+30m</span><span>+1h</span><span>+1h30m</span><span>+2h</span></div><div id=\"epg\"></div></div>";
        byId("category-select").onchange = function(event) { state.category = event.target.value; renderGuidePage(); };
        byId("guide-search").oninput = function(event) { state.query = event.target.value; renderGuidePage(); };
        renderEPG();
      }
      function renderEPG() {
        const root = byId("epg");
        const channels = visibleChannels(true).slice(0, 60);
        root.innerHTML = channels.map(function(channel, channelIndex) {
          const programs = programsFor(channel.id).filter(function(program) { return !state.query || lower(program.title).indexOf(lower(state.query)) !== -1; }).slice(0, 5);
          while (programs.length < 5) programs.push({ title: "Data not available", startUnix: 0 });
          return "<div class=\"epg-row\"><button class=\"epg-channel\" data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<strong>" + escapeHTML(channel.name || "Untitled") + "</strong></button>" + programs.map(function(program, index) {
            return "<button class=\"epg-cell program " + colorClass(index + channelIndex) + "\" data-channel=\"" + escapeHTML(channel.id) + "\"><time>" + escapeHTML(timeLabel(program.startUnix)) + "</time><strong>" + escapeHTML(program.title || "Data not available") + "</strong></button>";
          }).join("") + "</div>";
        }).join("");
      }
      function renderSettings() {
        byId("view").innerHTML = "<div class=\"settings-card\"><h2>Hidden categories</h2><div id=\"settings-list\" class=\"settings-list\"></div></div>";
        const root = byId("settings-list");
        root.innerHTML = items(state.app.categories).map(function(category) {
          return "<label><span>" + escapeHTML(category.name || category.id) + "</span><input type=\"checkbox\" data-hide=\"" + escapeHTML(category.id) + "\"" + (hiddenMap()[category.id] ? " checked" : "") + "></label>";
        }).join("");
      }
      function categoryName(id) {
        const category = items(state.app.categories).find(function(item) { return item.id === id; });
        return category ? category.name : "";
      }
      function colorClass(index) {
        return ["purple", "green", "red", "gray", "blue"][index % 5];
      }
      function setVideoSource(url) {
        const video = byId("player");
        if (!video) return;
        if (state.hls) { state.hls.destroy(); state.hls = null; }
        if (window.Hls && Hls.isSupported() && url.indexOf(".m3u8") !== -1) {
          state.hls = new Hls();
          state.hls.loadSource(url);
          state.hls.attachMedia(video);
        } else {
          video.src = url;
        }
        video.play().catch(function() {});
      }
      async function playChannel(channel) {
        state.currentChannel = channel;
        if (state.view !== "live" && state.view !== "favorites") state.view = "live";
        render();
        setVideoSource(route("/dispatcharr/stream?channel_id=" + encodeURIComponent(channel.id)));
        startWatch(channel);
        const guide = await getJSON("/dispatcharr/api/guide?channel_id=" + encodeURIComponent(channel.id)).catch(function() { return { programs: [] }; });
        const nowGuide = byId("now-guide");
        if (nowGuide) nowGuide.innerHTML = items(guide.programs).slice(0, 6).map(function(program) { return "<div class=\"program\"><time>" + escapeHTML(timeLabel(program.startUnix)) + "</time><strong>" + escapeHTML(program.title || "Untitled") + "</strong></div>"; }).join("") || "<div class=\"empty\">No guide entries.</div>";
      }
      function startWatch(channel) {
        if (state.currentSession) postJSON("/dispatcharr/api/watch/stop", { sessionId: state.currentSession.id, reason: "switch_channel" }).catch(function() {});
        postJSON("/dispatcharr/api/watch/start", { itemKind: "channel", itemId: channel.id, itemName: channel.name }).then(function(payload) {
          state.currentSession = payload.session;
          state.app.preferences = mergePrefs(payload.preferences, readLocalPrefs());
          savePrefs();
          if (state.heartbeat) clearInterval(state.heartbeat);
          state.heartbeat = setInterval(function() {
            if (state.currentSession) postJSON("/dispatcharr/api/watch/heartbeat", { sessionId: state.currentSession.id }).catch(function() {});
          }, 30000);
          renderRail();
        }).catch(function() {});
      }
      document.addEventListener("click", function(event) {
        const channelTarget = event.target.closest("[data-channel]");
        if (channelTarget) {
          const channel = channelByID(channelTarget.getAttribute("data-channel"));
          if (channel) playChannel(channel);
        }
        const categoryTarget = event.target.closest("[data-category]");
        if (categoryTarget) setCategory(categoryTarget.getAttribute("data-category"));
      });
      document.addEventListener("change", function(event) {
        const id = event.target.getAttribute("data-hide");
        if (!id) return;
        if (event.target.checked) state.app.preferences.hiddenCategories[id] = true;
        else delete state.app.preferences.hiddenCategories[id];
        savePrefs();
        postJSON("/dispatcharr/api/hidden-categories", { id: id, hidden: event.target.checked }).catch(function() {});
        render();
      });
      document.querySelectorAll(".nav button").forEach(function(button) {
        button.onclick = function() { setView(button.dataset.view); };
      });
      byId("rail-search").oninput = function(event) { state.query = event.target.value; render(); };
      byId("global-search").oninput = function(event) { state.query = event.target.value; render(); };
      window.addEventListener("beforeunload", function() {
        if (state.currentSession) navigator.sendBeacon(route("/dispatcharr/api/watch/stop"), JSON.stringify({ sessionId: state.currentSession.id, reason: "page_unload" }));
      });
      loadApp().catch(function() {
        byId("channel-list").textContent = "Unable to load Dispatcharr app data.";
        byId("health").textContent = "error";
      });
    </script>
  </body>
</html>`
