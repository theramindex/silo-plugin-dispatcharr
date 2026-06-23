package plugin

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	sdkruntime "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/dispatcharr"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

//go:embed assets/hls.min.js assets/mpegts.min.js
var playerAssets embed.FS

type HTTPRoutesServer struct {
	pluginv1.UnimplementedHttpRoutesServer
	store            *cache.Store
	settingsProvider func() config.Settings
	adminPersister   func(context.Context, map[string]any) error
	adminStorage     adminSettingsStorage
	adminToken       string
	syncer           catalogSyncer
	hydrateMu        sync.Mutex
}

type catalogSyncer interface {
	SyncNow(ctx context.Context, settings config.Settings, nowUnix int64) error
}

func NewHTTPRoutesServer(store *cache.Store) *HTTPRoutesServer {
	return newHTTPRoutesServer(store, nil, nil)
}

func NewHTTPRoutesServerWithSettings(store *cache.Store, settingsProvider func() config.Settings) *HTTPRoutesServer {
	return newHTTPRoutesServer(store, settingsProvider, nil)
}

func NewHTTPRoutesServerWithSyncer(store *cache.Store, settingsProvider func() config.Settings, syncer catalogSyncer) *HTTPRoutesServer {
	return newHTTPRoutesServer(store, settingsProvider, syncer)
}

func NewHTTPRoutesServerWithSyncerAndAdminSettingsFile(store *cache.Store, settingsProvider func() config.Settings, syncer catalogSyncer, path string) *HTTPRoutesServer {
	server := newHTTPRoutesServer(store, settingsProvider, syncer)
	server.adminStorage = NewFileAdminSettingsStorage(path)
	return server
}

func newHTTPRoutesServer(store *cache.Store, settingsProvider func() config.Settings, syncer catalogSyncer) *HTTPRoutesServer {
	return &HTTPRoutesServer{store: store, settingsProvider: settingsProvider, syncer: syncer, adminToken: newAdminToken()}
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

type RecordingsPayload struct {
	Available bool              `json:"available"`
	Reason    string            `json:"reason,omitempty"`
	Items     []json.RawMessage `json:"items"`
}

type scheduleRecordingRequest struct {
	ChannelID   string `json:"channelId"`
	ProgramID   string `json:"programId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	StartUnix   int64  `json:"startUnix"`
	EndUnix     int64  `json:"endUnix"`
}

type AppCapabilities struct {
	LiveTV                bool   `json:"liveTv"`
	Guide                 bool   `json:"guide"`
	VOD                   bool   `json:"vod"`
	Series                bool   `json:"series"`
	Recordings            bool   `json:"recordings"`
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
	case "/dispatcharr", "/dispatcharr/player", "/dispatcharr/admin":
		return htmlResponse(http.StatusOK, s.playerPageHTML(request)), nil
	case "/dispatcharr/assets/hls.min.js", "/assets/hls.min.js":
		return assetResponse("assets/hls.min.js")
	case "/dispatcharr/assets/mpegts.min.js", "/assets/mpegts.min.js":
		return assetResponse("assets/mpegts.min.js")
	case "/dispatcharr/status", "/dispatcharr/api/status":
		return s.respondJSON(http.StatusOK, BuildHealthPayload(s.store.Current()))
	case "/dispatcharr/api/refresh":
		return s.handleRefresh(ctx, request)
	case "/dispatcharr/api/app":
		s.ensureCatalogHydrated(ctx)
		return s.respondJSON(http.StatusOK, s.buildAppPayload())
	case "/dispatcharr/channels", "/dispatcharr/api/channels":
		s.ensureCatalogHydrated(ctx)
		return s.respondJSON(http.StatusOK, s.channelsPayload())
	case "/dispatcharr/guide", "/dispatcharr/api/guide":
		s.ensureCatalogHydrated(ctx)
		channelID := queryValue(request, "channel_id")
		programs := programsForChannel(s.store.Current().Catalog.Programs, channelID)
		sort.Slice(programs, func(i, j int) bool {
			return programs[i].StartUnix < programs[j].StartUnix
		})
		return s.respondJSON(http.StatusOK, GuidePayload{Programs: programs})
	case "/dispatcharr/api/categories":
		s.ensureCatalogHydrated(ctx)
		return s.respondJSON(http.StatusOK, s.categoriesPayload())
	case "/dispatcharr/api/vod":
		return s.respondJSON(http.StatusOK, s.vodPayload())
	case "/dispatcharr/api/series":
		return s.respondJSON(http.StatusOK, s.seriesPayload())
	case "/dispatcharr/api/recordings":
		if request.GetMethod() == http.MethodPost {
			return s.handleScheduleRecording(ctx, request)
		}
		return s.handleRecordings(ctx)
	case "/dispatcharr/api/preferences":
		return s.handlePreferences(request)
	case "/dispatcharr/api/admin-settings":
		return s.handleAdminSettings(ctx, request)
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
		s.ensureCatalogHydrated(ctx)
		channelID := queryValue(request, "channel_id")
		if strings.TrimSpace(channelID) == "" {
			return textResponse(http.StatusBadRequest, "missing channel_id query parameter"), nil
		}
		streamURL, err := s.resolveStreamURL(channelID)
		if err != nil {
			return textResponse(http.StatusNotFound, err.Error()), nil
		}
		streamURL = appendPlaybackQuery(streamURL, request)
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

func (s *HTTPRoutesServer) ensureCatalogHydrated(ctx context.Context) {
	if len(s.store.Current().Catalog.Channels) > 0 || s.syncer == nil || s.settingsProvider == nil {
		return
	}

	s.hydrateMu.Lock()
	defer s.hydrateMu.Unlock()

	if len(s.store.Current().Catalog.Channels) > 0 {
		return
	}

	settings := s.settingsProvider()
	if err := settings.Validate(); err != nil {
		s.store.RecordFailure(time.Now().Unix(), err.Error())
		return
	}
	if err := s.syncer.SyncNow(ctx, settings, time.Now().Unix()); err != nil {
		s.store.RecordFailure(time.Now().Unix(), err.Error())
	}
}

func (s *HTTPRoutesServer) handleRefresh(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return textResponse(http.StatusMethodNotAllowed, "refresh requires POST"), nil
	}
	if s.syncer == nil || s.settingsProvider == nil {
		return textResponse(http.StatusServiceUnavailable, "catalog sync is not available"), nil
	}

	s.hydrateMu.Lock()
	defer s.hydrateMu.Unlock()

	settings := s.settingsProvider()
	if err := settings.Validate(); err != nil {
		s.store.RecordFailure(time.Now().Unix(), err.Error())
		return textResponse(http.StatusBadRequest, err.Error()), nil
	}
	if err := s.syncer.SyncNow(ctx, settings, time.Now().Unix()); err != nil {
		s.store.RecordFailure(time.Now().Unix(), err.Error())
		return textResponse(http.StatusBadGateway, err.Error()), nil
	}
	return s.respondJSON(http.StatusOK, s.buildAppPayload())
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
		Capabilities: appCapabilities(snapshot.Catalog.Source.Mode, preferences),
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

func (s *HTTPRoutesServer) handleRecordings(ctx context.Context) (*pluginv1.HandleHTTPResponse, error) {
	if !dvrEnabledForSource(s.store.Current().Catalog.Source.Mode) {
		return s.respondJSON(http.StatusOK, RecordingsPayload{Available: false, Reason: "Recordings require Dispatcharr Direct Connect.", Items: []json.RawMessage{}})
	}
	client, err := s.dispatcharrClient()
	if err != nil {
		return s.respondJSON(http.StatusOK, RecordingsPayload{Available: false, Reason: err.Error(), Items: []json.RawMessage{}})
	}
	recordings, err := client.Recordings(ctx)
	if err != nil {
		return s.respondJSON(http.StatusBadGateway, RecordingsPayload{Available: false, Reason: err.Error(), Items: []json.RawMessage{}})
	}
	return s.respondJSON(http.StatusOK, RecordingsPayload{Available: true, Items: enrichRecordings(client, recordings)})
}

func (s *HTTPRoutesServer) handleScheduleRecording(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if !dvrEnabledForSource(s.store.Current().Catalog.Source.Mode) {
		return textResponse(http.StatusConflict, "recordings require Dispatcharr Direct Connect"), nil
	}
	var payload scheduleRecordingRequest
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid recording payload"), nil
	}
	if strings.TrimSpace(payload.ChannelID) == "" {
		return textResponse(http.StatusBadRequest, "missing channel id"), nil
	}
	if payload.EndUnix <= payload.StartUnix || payload.EndUnix <= 0 {
		return textResponse(http.StatusBadRequest, "invalid recording window"), nil
	}
	if payload.StartUnix <= 0 {
		return textResponse(http.StatusBadRequest, "missing recording start"), nil
	}
	channel, ok := s.channelByID(payload.ChannelID)
	if !ok {
		return textResponse(http.StatusNotFound, "channel not found"), nil
	}
	client, err := s.dispatcharrClient()
	if err != nil {
		return s.respondJSON(http.StatusOK, RecordingsPayload{Available: false, Reason: err.Error(), Items: []json.RawMessage{}})
	}
	dispatcharrChannelID, err := s.dispatcharrChannelID(ctx, client, channel)
	if err != nil {
		return textResponse(http.StatusBadGateway, err.Error()), nil
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = channel.Name
	}
	start := time.Unix(payload.StartUnix, 0).UTC()
	end := time.Unix(payload.EndUnix, 0).UTC()
	recording, err := client.CreateRecording(ctx, map[string]any{
		"channel":    dispatcharrChannelID,
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
		"custom_properties": map[string]any{
			"program": map[string]any{
				"id":          strings.TrimSpace(payload.ProgramID),
				"title":       title,
				"description": strings.TrimSpace(payload.Description),
				"start_time":  start.Format(time.RFC3339),
				"end_time":    end.Format(time.RFC3339),
				"tvg_id":      strings.TrimSpace(channel.GuideID),
			},
			"channel_name": channel.Name,
			"source":       "silo.ramindex.dispatcharr",
		},
	})
	if err != nil {
		return textResponse(http.StatusBadGateway, err.Error()), nil
	}
	return s.respondJSON(http.StatusOK, map[string]any{"ok": true, "recording": enrichRecording(client, recording)})
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

func (s *HTTPRoutesServer) handleAdminSettings(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() == http.MethodPost && !s.adminSettingsAuthorized(request) {
		return textResponse(http.StatusForbidden, "admin settings token is required"), nil
	}
	if request.GetMethod() != http.MethodPost {
		if s.adminStorage != nil {
			saved, ok, err := s.adminStorage.Load()
			if err != nil {
				log.Printf("dispatcharr: load admin settings file failed: %v", err)
				return textResponse(http.StatusInternalServerError, "could not load admin settings"), nil
			}
			if ok {
				s.store.SetAdminSettings(saved)
				return s.respondJSON(http.StatusOK, json.RawMessage(saved))
			}
		}
		if s.store.HasAdminSettings() {
			return s.respondJSON(http.StatusOK, json.RawMessage(s.store.AdminSettings()))
		}
		if s.settingsProvider != nil {
			if configured := s.settingsProvider().AdminSettings; len(configured) > 0 {
				return s.respondJSON(http.StatusOK, json.RawMessage(configured))
			}
		}
		return s.respondJSON(http.StatusOK, json.RawMessage(s.store.AdminSettings()))
	}
	var payload map[string]any
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid admin settings payload"), nil
	}
	normalized := normalizeAdminSettingsPayload(payload)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return textResponse(http.StatusBadRequest, "invalid admin settings payload"), nil
	}
	if s.adminStorage != nil {
		if err := s.adminStorage.Save(encoded); err != nil {
			log.Printf("dispatcharr: save admin settings file failed: %v", err)
			return textResponse(http.StatusInternalServerError, "could not save admin settings"), nil
		}
	}
	saved := s.store.SetAdminSettings(encoded)
	s.persistAdminSettingsAsync(normalized)
	return s.respondJSON(http.StatusOK, json.RawMessage(saved))
}

func (s *HTTPRoutesServer) adminSettingsAuthorized(request *pluginv1.HandleHTTPRequest) bool {
	if s.adminToken == "" {
		return false
	}
	if headerValue(request.GetHeaders(), "x-dispatcharr-admin-token") == s.adminToken {
		return true
	}
	return queryValue(request, "admin_token") == s.adminToken
}

func headerValue(headers map[string]string, key string) string {
	for name, value := range headers {
		if strings.EqualFold(name, key) {
			return value
		}
	}
	return ""
}

func (s *HTTPRoutesServer) persistAdminSettings(ctx context.Context, payload map[string]any) error {
	if s.adminPersister != nil {
		return s.adminPersister(ctx, payload)
	}
	host := sdkruntime.Host()
	if host == nil {
		return fmt.Errorf("runtime host unavailable")
	}
	return host.SetGlobalConfigEntry(ctx, "category_settings", payload)
}

func (s *HTTPRoutesServer) persistAdminSettingsAsync(payload map[string]any) {
	if s.adminPersister != nil {
		if err := s.persistAdminSettings(context.Background(), payload); err != nil {
			log.Printf("dispatcharr: persist admin settings failed: %v", err)
		}
		return
	}
	copyPayload := make(map[string]any, len(payload))
	for key, value := range payload {
		copyPayload[key] = value
	}
	go func() {
		persistCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := s.persistAdminSettings(persistCtx, copyPayload); err != nil {
			log.Printf("dispatcharr: persist admin settings failed: %v", err)
		}
	}()
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

func assetResponse(path string) (*pluginv1.HandleHTTPResponse, error) {
	payload, err := playerAssets.ReadFile(path)
	if err != nil {
		return textResponse(http.StatusNotFound, "asset not found"), nil
	}
	return &pluginv1.HandleHTTPResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"cache-control": "public, max-age=31536000, immutable",
			"content-type":  "application/javascript; charset=utf-8",
		},
		Body: payload,
	}, nil
}

func (s *HTTPRoutesServer) playerPageHTML(request *pluginv1.HandleHTTPRequest) string {
	body := strings.Replace(playerPageHTMLTemplate, "__PLAYER_LIBRARIES__", playerLibrariesHTML(), 1)
	body = strings.Replace(body, "__SILO_THEME__", html.EscapeString(sanitizeThemeSlug(queryValue(request, "theme"))), 1)
	if request.GetPath() == "/dispatcharr/admin" {
		body = strings.Replace(body, "__ADMIN_SETTINGS_TOKEN__", html.EscapeString(s.adminToken), 1)
	} else {
		body = strings.Replace(body, "__ADMIN_SETTINGS_TOKEN__", "", 1)
	}
	if request.GetPath() == "/dispatcharr/admin" {
		body = removeTemplateBlock(body, "<!-- USER_NAV_START -->", "<!-- USER_NAV_END -->")
		body = removeTemplateBlock(body, "<!-- USER_TOPBAR_START -->", "<!-- USER_TOPBAR_END -->")
		body = strings.Replace(body, "__APP_TITLE__", "Live TV Admin", 2)
		return strings.Replace(body, "__ROUTE_CLASS__", "is-admin", 1)
	}
	body = strings.Replace(body, "__APP_TITLE__", "Live TV", 2)
	return strings.Replace(body, "__ROUTE_CLASS__", "", 1)
}

func newAdminToken() string {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(token[:])
}

func removeTemplateBlock(body string, startMarker string, endMarker string) string {
	start := strings.Index(body, startMarker)
	if start < 0 {
		return body
	}
	end := strings.Index(body[start:], endMarker)
	if end < 0 {
		return body
	}
	end += start + len(endMarker)
	return body[:start] + body[end:]
}

func sanitizeThemeSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return -1
	}, value)
}

func playerLibrariesHTML() string {
	var builder strings.Builder
	for _, path := range []string{"assets/hls.min.js", "assets/mpegts.min.js"} {
		payload, err := playerAssets.ReadFile(path)
		if err != nil {
			continue
		}
		builder.WriteString("<script>")
		builder.WriteString(strings.ReplaceAll(string(payload), "</script", "<\\/script"))
		builder.WriteString("</script>\n")
	}
	return builder.String()
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

func (s *HTTPRoutesServer) dispatcharrClient() (*dispatcharr.Client, error) {
	if s.settingsProvider == nil {
		return nil, fmt.Errorf("dispatcharr settings are unavailable")
	}
	settings := s.settingsProvider()
	switch settings.SourceMode {
	case config.SourceModeDirectLogin:
		if strings.TrimSpace(settings.DispatcharrURL) == "" || strings.TrimSpace(settings.DispatcharrUser) == "" || strings.TrimSpace(settings.DispatcharrPass) == "" {
			return nil, fmt.Errorf("dispatcharr direct login settings are incomplete")
		}
		return dispatcharr.NewLoginClient(settings.DispatcharrURL, settings.DispatcharrUser, settings.DispatcharrPass), nil
	case config.SourceModeAPIKey:
		if strings.TrimSpace(settings.DispatcharrURL) == "" || strings.TrimSpace(settings.DispatcharrAPIKey) == "" {
			return nil, fmt.Errorf("dispatcharr api key settings are incomplete")
		}
		return dispatcharr.NewAPIKeyClient(settings.DispatcharrURL, settings.DispatcharrAPIKey), nil
	default:
		return nil, fmt.Errorf("recordings require Dispatcharr direct or API key mode")
	}
}

func (s *HTTPRoutesServer) channelByID(channelID string) (model.Channel, bool) {
	snapshot := s.store.Current()
	for _, channel := range snapshot.Catalog.Channels {
		if channel.ID == channelID {
			return channel, true
		}
	}
	return model.Channel{}, false
}

func (s *HTTPRoutesServer) dispatcharrChannelID(ctx context.Context, client *dispatcharr.Client, channel model.Channel) (int, error) {
	upstreamChannels, err := client.Channels(ctx)
	if err != nil {
		return 0, fmt.Errorf("load Dispatcharr channels: %w", err)
	}
	streamUUID := dispatcharrStreamUUID(channel.StreamURL)
	for _, upstream := range upstreamChannels {
		if streamUUID != "" && strings.EqualFold(upstream.UUID.String(), streamUUID) {
			return strconv.Atoi(upstream.ID.String())
		}
	}
	for _, upstream := range upstreamChannels {
		if channel.GuideID != "" && strings.EqualFold(upstream.EffectiveTVGID.String(), channel.GuideID) {
			return strconv.Atoi(upstream.ID.String())
		}
		if channel.GuideID != "" && strings.EqualFold(upstream.TVGID.String(), channel.GuideID) {
			return strconv.Atoi(upstream.ID.String())
		}
	}
	for _, upstream := range upstreamChannels {
		name := strings.TrimSpace(channel.Name)
		if name != "" && strings.EqualFold(upstream.EffectiveName.String(), name) {
			return strconv.Atoi(upstream.ID.String())
		}
		if name != "" && strings.EqualFold(upstream.Name.String(), name) {
			return strconv.Atoi(upstream.ID.String())
		}
	}
	return 0, fmt.Errorf("unable to match %q to a Dispatcharr channel", channel.Name)
}

func dispatcharrStreamUUID(streamURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(streamURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 1 {
		return ""
	}
	for index := 0; index < len(parts)-1; index++ {
		if parts[index] == "stream" {
			return parts[index+1]
		}
	}
	return ""
}

func enrichRecordings(client *dispatcharr.Client, recordings []json.RawMessage) []json.RawMessage {
	enriched := make([]json.RawMessage, 0, len(recordings))
	for _, recording := range recordings {
		enriched = append(enriched, enrichRecording(client, recording))
	}
	return enriched
}

func enrichRecording(client *dispatcharr.Client, recording json.RawMessage) json.RawMessage {
	var object map[string]any
	if err := json.Unmarshal(recording, &object); err != nil {
		return recording
	}
	id := fmt.Sprint(object["id"])
	playbackURL := recordingPlaybackURL(client, id, object)
	object["_silo"] = map[string]any{
		"playback_url":   playbackURL,
		"playback_owner": "dispatcharr",
	}
	out, err := json.Marshal(object)
	if err != nil {
		return recording
	}
	return out
}

func recordingPlaybackURL(client *dispatcharr.Client, id string, object map[string]any) string {
	custom, _ := object["custom_properties"].(map[string]any)
	if raw, ok := custom["output_file_url"].(string); ok && strings.TrimSpace(raw) != "" {
		return client.AbsoluteURL(raw)
	}
	if raw, ok := custom["file_url"].(string); ok && strings.TrimSpace(raw) != "" {
		return client.AbsoluteURL(raw)
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(id) == "<nil>" {
		return ""
	}
	return client.AbsoluteURL("/api/channels/recordings/" + strings.TrimSpace(id) + "/file/")
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

func appCapabilities(sourceMode model.SourceMode, preferences cache.Preferences) AppCapabilities {
	return AppCapabilities{
		LiveTV:                true,
		Guide:                 true,
		VOD:                   true,
		Series:                true,
		Recordings:            dvrEnabledForSource(sourceMode),
		Favorites:             true,
		HiddenCategories:      true,
		BackendProxySupported: preferences.Playback.BackendProxySupported,
		StreamMode:            preferences.Playback.StreamMode,
		NativeLiveTVExport:    false,
	}
}

func dvrEnabledForSource(sourceMode model.SourceMode) bool {
	return sourceMode == model.SourceModeDirectLogin
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

func appendPlaybackQuery(streamURL string, request *pluginv1.HandleHTTPRequest) string {
	parsed, err := url.Parse(streamURL)
	if err != nil {
		return streamURL
	}
	values := parsed.Query()
	for _, key := range []string{"output_profile", "output_format", "output"} {
		value := strings.TrimSpace(queryValue(request, key))
		if value != "" {
			values.Set(key, value)
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
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

const playerPageHTMLTemplate = `<!doctype html>
<html lang="en" data-silo-theme="__SILO_THEME__">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>__APP_TITLE__</title>
    __PLAYER_LIBRARIES__
    <style>
      :root {
        color-scheme: dark;
        --fallback-bg: #171717;
        --fallback-rail: #1d1d1f;
        --fallback-rail-2: #222225;
        --fallback-panel: #2b2b2d;
        --fallback-panel-2: #353536;
        --fallback-line: #3e3e40;
        --fallback-text: #f5f3ef;
        --fallback-muted: #a7a5a0;
        --fallback-accent: #ff2f7d;
        --fallback-green: #173f31;
        --fallback-purple: #3b2147;
        --fallback-red: #4a211e;
        --fallback-blue: #1d3347;
        --fallback-warn: #f4c95f;
        --bg: var(--silo-bg, var(--silo-background, var(--fallback-bg)));
        --rail: var(--silo-sidebar, var(--silo-surface, var(--fallback-rail)));
        --rail-2: var(--silo-surface-2, var(--silo-card, var(--fallback-rail-2)));
        --panel: var(--silo-card, var(--silo-surface, var(--fallback-panel)));
        --panel-2: var(--silo-card-hover, var(--silo-surface-hover, var(--fallback-panel-2)));
        --line: var(--silo-border, var(--silo-line, var(--fallback-line)));
        --text: var(--silo-text, var(--silo-foreground, var(--fallback-text)));
        --muted: var(--silo-muted, var(--silo-muted-foreground, var(--fallback-muted)));
        --accent: var(--silo-accent, var(--silo-primary, var(--fallback-accent)));
        --green: var(--silo-success-surface, var(--fallback-green));
        --purple: var(--silo-info-surface, var(--fallback-purple));
        --red: var(--silo-danger-surface, var(--fallback-red));
        --blue: var(--silo-selection-surface, var(--fallback-blue));
        --warn: var(--silo-warning, var(--fallback-warn));
        --on-accent: var(--silo-on-accent, var(--silo-primary-foreground, #ffffff));
      }
      :root[data-silo-theme="midnight-cinema"] {
        --fallback-bg: #171717;
        --fallback-rail: #1d1d1f;
        --fallback-rail-2: #222225;
        --fallback-panel: #2b2b2d;
        --fallback-panel-2: #353536;
        --fallback-line: #3e3e40;
        --fallback-text: #f5f3ef;
        --fallback-muted: #a7a5a0;
        --fallback-accent: #ff2f7d;
      }
      :root[data-silo-theme="light"], :root[data-silo-theme="daylight"] {
        color-scheme: light;
        --fallback-bg: #f7f5f1;
        --fallback-rail: #ebe7df;
        --fallback-rail-2: #ffffff;
        --fallback-panel: #ffffff;
        --fallback-panel-2: #eee9df;
        --fallback-line: #d9d2c6;
        --fallback-text: #191714;
        --fallback-muted: #6e675e;
        --fallback-accent: #b52462;
        --fallback-green: #dcefe5;
        --fallback-purple: #eadff4;
        --fallback-red: #f2dedb;
        --fallback-blue: #dce9f3;
        --fallback-warn: #9b7418;
      }
      * { box-sizing: border-box; }
      body { margin: 0; min-height: 100vh; overflow: hidden; background: var(--bg); color: var(--text); font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
      button, input, select { font: inherit; }
      button { cursor: pointer; }
      .shell { display: grid; grid-template-columns: 17rem minmax(0, 1fr); height: 100vh; }
      .shell.is-player { grid-template-columns: minmax(0, 1fr); background: #050505; }
      .rail { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--line); background: linear-gradient(135deg, #19191a, #201e20); padding: 1rem; }
      .shell.is-player .rail { display: none; }
      .brand { display: flex; align-items: center; gap: 0.65rem; margin-bottom: 1.25rem; }
      .brand h1 { margin: 0; font-size: 1.55rem; font-weight: 900; letter-spacing: 0; }
      .back { width: 2.25rem; height: 2.25rem; color: var(--muted); text-decoration: none; border: 1px solid var(--line); border-radius: 999px; display: inline-grid; place-items: center; flex: 0 0 auto; }
      .back:hover { color: var(--text); background: var(--panel); }
      .back svg { width: 1.05rem; height: 1.05rem; display: block; }
      .source-icon { width: 1.45rem; height: 1.45rem; border-radius: 999px; display: inline-grid; place-items: center; background: var(--accent); color: var(--on-accent); }
      .nav { display: grid; gap: 0.28rem; margin-bottom: 1rem; }
      .nav button { width: 100%; border: 0; border-radius: 0.65rem; background: transparent; color: var(--muted); display: flex; align-items: center; gap: 0.65rem; padding: 0.7rem 0.72rem; text-align: left; font-weight: 750; }
      .nav button.active, .nav button:hover { background: var(--panel-2); color: var(--text); }
      .nav svg { width: 1.15rem; height: 1.15rem; flex: 0 0 auto; stroke-width: 1.9; }
      .nav small { margin-left: auto; color: var(--muted); }
      .channel-row { width: 100%; border: 0; border-radius: 0.75rem; background: transparent; color: var(--text); display: grid; grid-template-columns: 3.1rem minmax(0, 1fr) 1.8rem; align-items: center; gap: 0.65rem; padding: 0.45rem; text-align: left; }
      .channel-row:hover, .channel-row.active { background: var(--panel-2); }
      .logo { width: 3rem; height: 2.05rem; object-fit: contain; border-radius: 0.5rem; background: var(--rail); }
      .channel-row strong, .tile strong, .program strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .muted { color: var(--muted); }
      .star { color: var(--warn); font-size: 1rem; }
      .main { min-width: 0; overflow: auto; padding: 1rem 1.25rem 2rem; }
      .shell.is-player .main { padding: 0; overflow: hidden; background: #050505; }
      .topbar { display: flex; align-items: center; justify-content: flex-end; gap: 0.65rem; margin-bottom: 0.85rem; position: sticky; top: 0; z-index: 5; background: linear-gradient(180deg, var(--bg) 70%, color-mix(in srgb, var(--bg) 0%, transparent)); padding-bottom: 0.65rem; }
      .shell.is-player .topbar, .shell.is-guide .topbar { display: none; }
      .shell.is-admin .nav, .shell.is-admin .topbar { display: none; }
      .title { display: flex; align-items: center; gap: 0.55rem; min-width: 0; }
      .title h2 { margin: 0; font-size: 1.35rem; }
      .status { color: var(--muted); font-size: 0.82rem; white-space: nowrap; }
      .search { border: 1px solid var(--line); border-radius: 999px; background: var(--rail-2); color: var(--text); padding: 0.62rem 0.85rem; min-width: min(24rem, 40vw); }
      .topbar .search { width: min(32rem, 100%); }
      .refresh-button { width: 2.45rem; height: 2.45rem; border: 1px solid var(--line); border-radius: 999px; background: var(--panel); color: var(--text); display: inline-grid; place-items: center; flex: 0 0 auto; }
      .refresh-button:hover { background: var(--panel-2); }
      .refresh-button:disabled { cursor: default; opacity: 0.7; }
      .refresh-button svg { width: 1.1rem; height: 1.1rem; display: block; }
      .refresh-button.is-loading svg { animation: spin 880ms linear infinite; }
      .section-title { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin: 1rem 0 0.55rem; color: var(--muted); font-size: 0.95rem; font-weight: 850; }
      .breadcrumbs { display: flex; align-items: center; gap: 0.4rem; min-width: 0; color: var(--muted); }
      .breadcrumbs button { border: 0; background: transparent; color: var(--text); padding: 0.2rem 0; font: inherit; font-weight: 850; }
      .breadcrumbs button:hover { color: white; text-decoration: underline; }
      .breadcrumbs .sep { color: var(--muted); }
      .chip { border: 1px solid var(--line); border-radius: 999px; background: var(--panel); color: var(--text); padding: 0.42rem 0.72rem; font-size: 0.82rem; font-weight: 820; }
      .chip:hover { background: var(--panel-2); }
      .row-scroll { display: flex; gap: 0.6rem; overflow-x: auto; padding-bottom: 0.3rem; }
      .continue-card { flex: 0 0 15.5rem; border: 0; border-radius: 0.7rem; background: transparent; color: var(--text); text-align: left; }
      .poster-box { height: 8.7rem; border-radius: 0.65rem; background: #b19398; display: grid; place-items: center; overflow: hidden; margin-bottom: 0.45rem; }
      .poster-box img { width: 100%; height: 100%; object-fit: contain; }
      .poster-box span { font-size: 2.8rem; font-weight: 950; }
      .progress { height: 0.22rem; border-radius: 999px; background: rgba(255,255,255,0.14); overflow: hidden; margin: -0.75rem 0.85rem 0.6rem; position: relative; }
      .progress i { display: block; height: 100%; width: 62%; background: white; border-radius: inherit; }
      .home-guide { overflow-x: auto; padding-bottom: 0.15rem; }
      .home-guide .guide-page { min-width: 0; }
      .program { min-height: 3.05rem; border: 0; border-radius: 0.55rem; color: var(--text); text-align: left; padding: 0.48rem 0.65rem; background: var(--purple); }
      .program.green { background: var(--green); }
      .program.red { background: var(--red); }
      .program.blue { background: var(--blue); }
      .program.gray { background: var(--panel); }
      .program time { color: var(--muted); font-size: 0.78rem; display: block; }
      .category-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(8.7rem, 1fr)); gap: 0.5rem; }
      .tile { border: 0; border-radius: 0.65rem; background: var(--panel); color: var(--text); min-height: 3.85rem; text-align: left; padding: 0.75rem 0.85rem; font-weight: 850; }
      .tile:hover, .tile.active { background: var(--panel-2); }
      .tile span { display: block; margin-top: 0.25rem; color: var(--muted); font-size: 0.76rem; font-weight: 760; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .guide-page { min-width: 0; }
      .guide-tools { display: flex; align-items: center; gap: 0.65rem; margin-bottom: 0.7rem; }
      .guide-tools .select { flex: 0 1 26rem; min-width: min(24rem, 45vw); }
      .guide-tools .search { flex: 1 1 28rem; min-width: 16rem; margin-left: auto; }
      .select { border: 1px solid var(--line); border-radius: 999px; color: var(--text); background: var(--panel); padding: 0.55rem 0.75rem; }
      .guide-scroll { --epg-logo-col: 6.8rem; --epg-slot: 12rem; --epg-row-h: 3.7rem; overflow-x: auto; overflow-y: visible; padding-bottom: 0.45rem; }
      .guide-timeline { width: calc(var(--epg-logo-col) + var(--epg-width)); min-width: calc(var(--epg-logo-col) + var(--epg-width)); }
      .time-head { display: grid; grid-template-columns: var(--epg-logo-col) repeat(var(--epg-slots), var(--epg-slot)); gap: 0.25rem; color: var(--muted); font-weight: 850; margin-bottom: 0.35rem; }
      .time-head span { min-width: 0; padding: 0 0.25rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
      .time-head span:first-child { position: sticky; left: 0; z-index: 3; color: var(--text); font-size: 1.15rem; background: var(--bg); }
      .epg-row { display: grid; grid-template-columns: var(--epg-logo-col) var(--epg-width); gap: 0.25rem; height: var(--epg-row-h); margin-bottom: 0.35rem; overflow: visible; }
      .epg-channel { position: sticky; left: 0; z-index: 2; border: 0; border-radius: 0.55rem; background: var(--bg); color: white; display: grid; place-items: center; padding: 0; height: var(--epg-row-h); min-height: var(--epg-row-h); overflow: hidden; }
      .epg-channel .logo { width: 5.7rem; height: 3.25rem; border-radius: 0; background: transparent; }
      .epg-programs { position: relative; height: var(--epg-row-h); min-width: 0; overflow: hidden; }
      .epg-cell { position: absolute; top: 0; height: var(--epg-row-h); min-height: 0; border: 0; border-radius: 0.55rem; text-align: left; color: var(--text); background: var(--panel); padding: 0.48rem 0.7rem; min-width: 0; overflow: hidden; white-space: nowrap; }
      .epg-cell time, .epg-cell strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .epg-cell .epg-play { position: absolute; inset: 0; z-index: 1; border: 0; border-radius: inherit; background: transparent; color: inherit; text-align: left; padding: 0.48rem 0.7rem; display: grid; align-content: center; min-width: 0; }
      .epg-cell .epg-schedule { position: absolute; right: 0.4rem; top: 50%; z-index: 2; transform: translateY(-50%); width: 1.8rem; height: 1.8rem; border: 1px solid rgba(255,255,255,0.22); border-radius: 999px; color: white; background: rgba(0,0,0,0.34); display: inline-grid; place-items: center; opacity: 0; transition: opacity 140ms ease, background 140ms ease; }
      .epg-cell .epg-schedule svg { width: 1rem; height: 1rem; }
      .epg-cell:hover .epg-schedule, .epg-cell .epg-schedule:focus-visible { opacity: 1; }
      .epg-cell .epg-schedule:hover { background: rgba(255,47,125,0.86); }
      .player-view { display: grid; grid-template-columns: minmax(0, 1fr) 22rem; gap: 1rem; align-items: start; }
      video { width: 100%; aspect-ratio: 16 / 9; background: #050505; border: 1px solid var(--line); border-radius: 0.75rem; }
      .playback-shell { position: relative; min-height: 100vh; overflow: hidden; background: #050505; display: grid; place-items: center; }
      .playback-stage { position: relative; width: min(100%, calc(100vh * 16 / 9)); max-height: 100vh; aspect-ratio: 16 / 9; overflow: hidden; background: #050505; }
      .playback-video { position: absolute; inset: 0; width: 100%; height: 100%; aspect-ratio: auto; object-fit: cover; border: 0; border-radius: 0; background: #050505; }
      .playback-scrim { pointer-events: none; position: absolute; inset: 0; background: linear-gradient(180deg, rgba(0,0,0,0.82) 0%, rgba(0,0,0,0.26) 16%, rgba(0,0,0,0.02) 46%, rgba(0,0,0,0.24) 64%, rgba(0,0,0,0.92) 100%); transition: opacity 220ms ease; }
      .player-top { position: absolute; inset: 1.25rem 1.25rem auto; display: flex; align-items: center; justify-content: space-between; gap: 1rem; z-index: 2; }
      .player-top, .player-bottom { transition: opacity 220ms ease, transform 220ms ease; }
      .playback-shell.is-idle { cursor: none; }
      .playback-shell.is-idle .playback-scrim { opacity: 0; }
      .playback-shell.is-idle .player-top { opacity: 0; pointer-events: none; transform: translateY(-0.5rem); }
      .playback-shell.is-idle .player-bottom { opacity: 0; pointer-events: none; transform: translateY(0.75rem); }
      .player-top-actions, .player-bottom-actions { display: flex; align-items: center; gap: 0.55rem; }
      .player-audio, .player-volume, .player-more { position: relative; }
      .player-icon, .player-chip { border: 1px solid rgba(255,255,255,0.12); background: rgba(30,30,31,0.72); color: white; box-shadow: 0 0.35rem 1.4rem rgba(0,0,0,0.2); backdrop-filter: blur(18px); }
      .player-icon { width: 2.65rem; height: 2.65rem; border-radius: 999px; display: inline-grid; place-items: center; font-size: 1.15rem; }
      .player-chip { min-height: 2.4rem; border-radius: 999px; padding: 0 0.82rem; font-weight: 850; display: inline-flex; align-items: center; gap: 0.35rem; }
      .player-icon svg, .player-chip svg, .menu-icon svg { width: 1.15rem; height: 1.15rem; display: block; stroke-width: 1.85; }
      .player-chip svg:last-child { width: 0.95rem; height: 0.95rem; opacity: 0.72; }
      .player-icon:hover, .player-chip:hover { background: rgba(52,52,54,0.86); }
      .player-icon.active, .player-icon.favorite.active { color: white; background: rgba(255,47,116,0.9); border-color: rgba(255,255,255,0.28); }
      .player-exit { border: 0; background: transparent; color: white; display: inline-flex; align-items: center; gap: 0.55rem; padding: 0; font-weight: 850; text-shadow: 0 0.15rem 0.85rem rgba(0,0,0,0.72); }
      .player-exit .player-icon { box-shadow: none; }
      .player-exit span { font-size: 0.92rem; }
      .player-center-button { position: absolute; left: 50%; top: 50%; z-index: 2; transform: translate(-50%, -50%) scale(1); width: 4.8rem; height: 4.8rem; border-radius: 999px; border: 1px solid rgba(255,255,255,0.2); background: rgba(245,245,245,0.94); color: #050505; display: inline-grid; place-items: center; box-shadow: 0 1rem 2.5rem rgba(0,0,0,0.34); transition: opacity 180ms ease, transform 180ms ease; }
      .player-center-button svg { width: 2rem; height: 2rem; display: block; }
      .player-center-button.hidden { opacity: 0; pointer-events: none; transform: translate(-50%, -50%) scale(0.94); }
      .player-center-button.loading svg { animation: spin 880ms linear infinite; }
      @keyframes spin { to { transform: rotate(360deg); } }
      .player-menu { position: absolute; top: calc(100% + 0.45rem); right: 0; display: none; min-width: 13rem; max-width: min(18rem, 70vw); padding: 0.35rem; border: 1px solid rgba(255,255,255,0.14); border-radius: 0.8rem; background: rgba(20,20,21,0.94); box-shadow: 0 1rem 2rem rgba(0,0,0,0.36); backdrop-filter: blur(18px); }
      .player-menu.open { display: grid; gap: 0.2rem; }
      .player-menu button { border: 0; border-radius: 0.55rem; background: transparent; color: rgba(255,255,255,0.82); padding: 0.55rem 0.65rem; text-align: left; font-weight: 750; }
      .player-menu button:hover, .player-menu button.active { background: rgba(255,255,255,0.1); color: white; }
      .volume-popover { position: absolute; top: calc(100% + 0.45rem); right: 0; display: none; width: 12rem; padding: 0.75rem 0.8rem; border: 1px solid rgba(255,255,255,0.14); border-radius: 0.8rem; background: rgba(20,20,21,0.94); box-shadow: 0 1rem 2rem rgba(0,0,0,0.36); backdrop-filter: blur(18px); }
      .volume-popover.open { display: grid; grid-template-columns: 2rem minmax(0, 1fr) 2.5rem; gap: 0.6rem; align-items: center; }
      .volume-popover input { width: 100%; accent-color: white; }
      .volume-value { color: rgba(255,255,255,0.74); font-size: 0.78rem; font-weight: 850; text-align: right; }
      .player-bottom { position: absolute; inset: auto 0 0; z-index: 2; padding: 0 1.25rem 1.15rem; background: linear-gradient(180deg, transparent, rgba(0,0,0,0.54) 32%, rgba(0,0,0,0.86)); }
      .player-meta { min-width: 0; max-width: min(45rem, 64vw); text-shadow: 0 0.15rem 1rem rgba(0,0,0,0.65); }
      .player-logo { width: 3.6rem; height: 2.45rem; object-fit: contain; border-radius: 0.55rem; background: rgba(255,255,255,0.82); margin-bottom: 0.45rem; padding: 0.18rem; }
      .player-logo-fallback { display: inline-grid; place-items: center; color: white; background: #b19398; font-weight: 950; }
      .player-kicker { font-size: 0.82rem; font-weight: 900; color: rgba(255,255,255,0.82); }
      .player-title { margin: 0.18rem 0 0.18rem; font-size: clamp(1.45rem, 2.25vw, 2.35rem); line-height: 1.02; letter-spacing: 0; }
      .player-description { margin: 0; color: rgba(255,255,255,0.78); font-size: 0.92rem; max-width: 42rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .player-tags { display: flex; gap: 0.35rem; margin-top: 0.55rem; flex-wrap: wrap; }
      .player-tag { border-radius: 0.25rem; background: rgba(255,255,255,0.12); color: white; font-size: 0.67rem; padding: 0.18rem 0.33rem; font-weight: 850; }
      .timeline { display: grid; grid-template-columns: 4.2rem minmax(0,1fr) 11rem; gap: 0.65rem; align-items: center; color: white; font-size: 0.76rem; font-weight: 850; margin-top: 0.9rem; }
      .timeline-bar { position: relative; height: 0.48rem; border-radius: 999px; background: rgba(255,255,255,0.16); overflow: visible; }
      .timeline-fill { position: absolute; inset: 0 auto 0 0; width: 41%; border-radius: inherit; background: rgba(255,255,255,0.92); }
      .timeline-knob { position: absolute; left: 41%; top: 50%; width: 0.85rem; height: 0.85rem; margin-left: -0.42rem; margin-top: -0.42rem; border-radius: 999px; background: white; }
      .live-dot { display: inline-block; width: 0.45rem; height: 0.45rem; border-radius: 999px; background: #ff334d; margin-right: 0.3rem; vertical-align: middle; }
      .player-toast { position: absolute; right: 1.25rem; top: 4.55rem; z-index: 3; opacity: 0; transform: translateY(-0.4rem); pointer-events: none; transition: opacity 160ms ease, transform 160ms ease; max-width: min(24rem, calc(100vw - 2.5rem)); padding: 0.65rem 0.85rem; border: 1px solid rgba(255,255,255,0.14); border-radius: 999px; background: rgba(20,20,21,0.9); color: white; font-size: 0.82rem; font-weight: 800; box-shadow: 0 1rem 2rem rgba(0,0,0,0.36); backdrop-filter: blur(18px); }
      .player-toast.show { opacity: 1; transform: translateY(0); }
      .app-toast { position: fixed; left: 50%; bottom: 1.5rem; z-index: 80; transform: translateX(-50%) translateY(0.6rem); opacity: 0; pointer-events: none; border: 1px solid rgba(255,255,255,0.14); border-radius: 999px; background: rgba(20,20,21,0.92); color: white; padding: 0.7rem 1rem; font-weight: 850; box-shadow: 0 0.75rem 2rem rgba(0,0,0,0.35); transition: opacity 160ms ease, transform 160ms ease; }
      .app-toast.show { opacity: 1; transform: translateX(-50%) translateY(0); }
      .player-guide-button.active { background: rgba(255,255,255,0.18); }
      .player-guide-panel { position: absolute; top: 5rem; right: 1.25rem; bottom: 8.5rem; z-index: 3; display: none; width: min(30rem, calc(100vw - 3rem)); overflow: hidden; border: 1px solid rgba(255,255,255,0.13); border-radius: 1rem; background: rgba(17,17,18,0.88); box-shadow: 0 1.2rem 2.4rem rgba(0,0,0,0.42); backdrop-filter: blur(22px); }
      .player-guide-panel.open { display: grid; grid-template-rows: auto minmax(0, 1fr); }
      .player-guide-head { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.85rem 0.95rem; border-bottom: 1px solid rgba(255,255,255,0.08); color: white; }
      .player-guide-head strong { display: block; font-size: 0.92rem; }
      .player-guide-head span { display: block; color: rgba(255,255,255,0.58); font-size: 0.75rem; font-weight: 750; }
      .player-guide-list { overflow: auto; padding: 0.45rem; }
      .player-guide-row { width: 100%; border: 0; border-radius: 0.75rem; background: transparent; color: white; display: grid; grid-template-columns: 3.6rem minmax(0, 1fr); gap: 0.7rem; align-items: center; padding: 0.5rem; text-align: left; }
      .player-guide-row:hover, .player-guide-row.active { background: rgba(255,255,255,0.1); }
      .player-guide-row .logo { width: 3.2rem; height: 2.15rem; border-radius: 0.42rem; }
      .player-guide-row strong, .player-guide-row small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .player-guide-row strong { font-size: 0.86rem; }
      .player-guide-row small { margin-top: 0.12rem; color: rgba(255,255,255,0.62); font-size: 0.72rem; font-weight: 750; }
      .player-more-menu { position: absolute; top: calc(100% + 0.45rem); right: 0; z-index: 4; display: none; width: min(25rem, 84vw); overflow: hidden; border: 1px solid rgba(255,255,255,0.14); border-radius: 1rem; background: rgba(20,20,21,0.94); box-shadow: 0 1.2rem 2.4rem rgba(0,0,0,0.42); backdrop-filter: blur(22px); }
      .player-more-menu.open { display: block; }
      .player-more-kicker { padding: 0.8rem 0.95rem 0.35rem; color: rgba(255,255,255,0.56); font-size: 0.75rem; font-weight: 850; }
      .player-more-menu button { width: 100%; border: 0; background: transparent; color: white; display: grid; grid-template-columns: 2rem minmax(0, 1fr); gap: 0.65rem; align-items: center; padding: 0.72rem 0.95rem; text-align: left; font-weight: 820; }
      .menu-icon { width: 2rem; height: 2rem; display: inline-grid; place-items: center; color: rgba(255,255,255,0.78); }
      .player-more-menu button:hover { background: rgba(255,255,255,0.1); }
      .player-more-menu button small { display: block; margin-top: 0.12rem; color: rgba(255,255,255,0.58); font-size: 0.72rem; font-weight: 720; }
      .player-more-separator { height: 1px; background: rgba(255,255,255,0.08); margin: 0.35rem 0; }
      .player-bottom-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 1.2rem; align-items: end; }
      .player-bottom-actions { align-self: end; padding-bottom: 1.25rem; }
      .now-card, .settings-card { border: 1px solid var(--line); background: var(--rail-2); border-radius: 0.8rem; padding: 0.85rem; }
      .recording-toolbar { display: flex; justify-content: flex-end; margin-bottom: 0.75rem; }
      .recording-refresh { border: 1px solid var(--line); border-radius: 999px; color: var(--text); background: var(--panel); padding: 0.55rem 0.8rem; font-weight: 820; }
      .recording-refresh:hover { background: var(--panel-2); }
      .recording-list { display: grid; gap: 0.55rem; }
      .recording-card { border: 1px solid var(--line); border-radius: 0.75rem; background: var(--panel); color: var(--text); padding: 0.75rem 0.85rem; display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 1rem; }
      .recording-card strong, .recording-card span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .recording-meta { margin-top: 0.22rem; color: var(--muted); font-size: 0.82rem; }
      .recording-actions { display: flex; align-items: center; gap: 0.5rem; }
      .recording-action { border: 1px solid var(--line); border-radius: 999px; color: var(--text); background: var(--rail-2); padding: 0.42rem 0.62rem; font-weight: 850; display: inline-flex; align-items: center; gap: 0.38rem; white-space: nowrap; }
      .recording-action svg { width: 0.95rem; height: 0.95rem; }
      .recording-action:hover { background: rgba(255,47,125,0.18); }
      .recording-badge { border-radius: 999px; background: rgba(255,255,255,0.1); color: var(--text); padding: 0.28rem 0.52rem; font-size: 0.73rem; font-weight: 850; text-transform: capitalize; white-space: nowrap; }
      .recording-badge.recording { background: rgba(255,47,125,0.22); color: #ffd7e5; }
      .recording-badge.completed { background: rgba(35,101,74,0.42); color: #d4ffe9; }
      .recording-badge.upcoming { background: rgba(74,54,110,0.5); color: #eee4ff; }
      .recording-badge.interrupted, .recording-badge.failed { background: rgba(99,40,35,0.5); color: #ffe0dc; }
      .settings-stack { display: grid; gap: 0.85rem; }
      .settings-card h2 { margin: 0 0 0.7rem; font-size: 1.05rem; }
      .settings-list { display: grid; gap: 0.55rem; }
      .settings-list label, .settings-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; background: var(--panel); border-radius: 0.65rem; padding: 0.7rem; }
      .settings-row input, .settings-row select { min-width: 12rem; border: 1px solid var(--line); border-radius: 0.55rem; background: var(--rail-2); color: var(--text); padding: 0.45rem 0.55rem; }
      .settings-row button, .settings-actions button { border: 1px solid var(--line); border-radius: 999px; background: var(--panel); color: var(--text); padding: 0.45rem 0.7rem; font-weight: 820; }
      .settings-row button:hover, .settings-actions button:hover { background: var(--panel-2); }
      .settings-row button:disabled, .settings-actions button:disabled { cursor: default; opacity: 0.48; }
      .settings-actions { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.65rem; }
      .settings-preview { margin-top: 0.65rem; display: grid; gap: 0.35rem; color: var(--muted); font-size: 0.82rem; }
      .settings-note { color: var(--muted); font-size: 0.82rem; line-height: 1.35; }
      .settings-warning { color: #ffd37a; }
      .empty { color: var(--muted); padding: 1rem 0; }
      .hide { display: none !important; }
      @media (max-width: 900px) {
        body { overflow: auto; }
        .shell { display: block; height: auto; }
        .rail { min-height: auto; border-right: 0; border-bottom: 1px solid var(--line); }
        .topbar { position: static; }
        .search { min-width: 0; width: 100%; }
        .guide-tools { flex-wrap: wrap; }
        .guide-tools .select, .guide-tools .search { flex: 1 1 100%; min-width: 0; margin-left: 0; }
        .guide-tools .refresh-button { order: 3; }
        .player-view { grid-template-columns: 1fr; }
        .shell.is-player { display: block; }
        .shell.is-player .rail { display: none; }
        .playback-shell { min-height: min(100vh, calc(100vw * 9 / 16)); }
        .playback-stage { width: 100%; max-height: none; }
        .player-meta { max-width: calc(100vw - 2rem); }
        .timeline { grid-template-columns: 3.4rem minmax(0,1fr); }
        .timeline span:last-child { display: none; }
      }
    </style>
  </head>
  <body>
    <div class="shell __ROUTE_CLASS__">
      <aside class="rail">
        <div class="brand">
          <a class="back" href="/" aria-label="Back to Silo"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5"/></svg></a>
          <h1>__APP_TITLE__</h1>
        </div>
        <!-- USER_NAV_START --><nav class="nav" aria-label="Dispatcharr views">
          <button class="active" data-view="home"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 10.75 12 4l8.25 6.75M6.25 9.25v9.5h11.5v-9.5M9.75 18.75v-5h4.5v5"/></svg><span>Home</span></button>
          <button data-view="favorites"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M21 8.25c0 6.25-9 11.25-9 11.25s-9-5-9-11.25A4.75 4.75 0 0 1 11.25 5 4.75 4.75 0 0 1 21 8.25Z"/></svg><span>Favorites</span> <small id="favorite-count">0</small></button>
          <button data-view="guide"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M4.5 6.75h15M4.5 12h15M4.5 17.25h15M8.25 4.5v15M15.75 4.5v15"/></svg><span>TV Guide</span></button>
          <button data-view="recordings"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 20.25a8.25 8.25 0 1 0 0-16.5 8.25 8.25 0 0 0 0 16.5Zm0-4a4.25 4.25 0 1 1 0-8.5 4.25 4.25 0 0 1 0 8.5Z"/></svg><span>Recordings</span></button>
          <button data-view="settings"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 8.25a3.75 3.75 0 1 1 0 7.5 3.75 3.75 0 0 1 0-7.5Z"/><path stroke-linecap="round" stroke-linejoin="round" d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 8.92 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9c.23.64.84 1 1.51 1H21a2 2 0 0 1 0 4h-.09A1.65 1.65 0 0 0 19.4 15Z"/></svg><span>Preferences</span></button>
        </nav><!-- USER_NAV_END -->
      </aside>
      <main class="main">
        <!-- USER_TOPBAR_START --><div class="topbar">
          <button id="guide-refresh" class="refresh-button" type="button" data-guide-refresh="true" aria-label="Refresh guide" title="Refresh guide">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M20 12a8 8 0 0 1-14.1 5.15M4 12A8 8 0 0 1 18.1 6.85"/><path stroke-linecap="round" stroke-linejoin="round" d="M6 17.25H3.75V19.5M18 6.75h2.25V4.5"/></svg>
          </button>
          <input id="global-search" class="search" placeholder="Search by program or channel">
        </div><!-- USER_TOPBAR_END -->
        <div id="view"><div class="empty">Loading Live TV...</div></div>
      </main>
    </div>
    <script>
      const path = window.location.pathname;
      const base = path.endsWith("/dispatcharr/player") ? path.slice(0, -"/dispatcharr/player".length) : (path.endsWith("/dispatcharr/admin") ? path.slice(0, -"/dispatcharr/admin".length) : (path.endsWith("/dispatcharr") ? path.slice(0, -"/dispatcharr".length) : ""));
      const isAdminRoute = path.endsWith("/dispatcharr/admin");
      const prefsKey = "silo.ramindex.dispatcharr.preferences.v1";
      const adminSettingsKey = "adminCategorySettings";
      const pluginInstallationID = (base.match(/\/api\/v1\/plugins\/(\d+)/) || [])[1] || "";
      const adminSettingsToken = "__ADMIN_SETTINGS_TOKEN__";
      const state = { app: null, view: isAdminRoute ? "admin" : "home", category: "", query: "", hls: null, tsPlayer: null, currentChannel: null, currentSession: null, heartbeat: null, muted: false, volume: 1, volumeMenuOpen: false, audioMenuOpen: false, moreMenuOpen: false, playerGuideOpen: false, selectedAudioTrack: 0, selectedTextTrack: -1, aspectMode: "fill", playerChromeIdle: false, playerChromeTimer: null, playerWaiting: false, recordings: null, recordingsLoading: false, guideChannels: [], guideRendered: 0, guideLoading: false, refreshing: false, selectedCustomGroup: "", customGroupQuery: "", adminCategorySettings: null, savedAdminCategorySettings: null, profileSaveStatus: "idle", profileSaveMessage: "", adminSaveStatus: "idle", adminSaveMessage: "" };

      function applySiloTheme() {
        const params = new URLSearchParams(window.location.search);
        const theme = String(params.get("theme") || document.documentElement.dataset.siloTheme || "").trim().toLowerCase().replace(/[^a-z0-9_-]/g, "");
        if (theme) document.documentElement.dataset.siloTheme = theme;
      }

      applySiloTheme();

      function route(url) { return base + url; }
      function routeHeaders(extra) {
        const headers = Object.assign({}, extra || {});
        if (isAdminRoute && adminSettingsToken) headers["x-dispatcharr-admin-token"] = adminSettingsToken;
        return headers;
      }
      function byId(id) { return document.getElementById(id); }
      function items(value) { return Array.isArray(value) ? value : []; }
      function lower(value) { return String(value || "").toLowerCase(); }
      function uniqueIDs(values) {
        const seen = {};
        const result = [];
        items(values).forEach(function(value) {
          value = String(value || "");
          if (!value || seen[value]) return;
          seen[value] = true;
          result.push(value);
        });
        return result;
      }
      function escapeHTML(value) {
        return String(value || "").replace(/[&<>"']/g, function(ch) {
          return ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[ch];
        });
      }
      function icon(name) {
        const icons = {
          "arrow-left": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M15.75 19.5 8.25 12l7.5-7.5'/></svg>",
          "chevron-down": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m6 9 6 6 6-6'/></svg>",
          "ellipsis": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm6 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm6 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z'/></svg>",
          "play": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M8 5.6v12.8c0 .55.6.9 1.08.62l10.1-6.4a.73.73 0 0 0 0-1.24L9.08 4.98A.72.72 0 0 0 8 5.6Z'/></svg>",
          "record": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M12 20.25a8.25 8.25 0 1 0 0-16.5 8.25 8.25 0 0 0 0 16.5Zm0-4a4.25 4.25 0 1 1 0-8.5 4.25 4.25 0 0 1 0 8.5Z'/></svg>",
          "pause": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M7.25 5.25h3.25v13.5H7.25zM13.5 5.25h3.25v13.5H13.5z'/></svg>",
          "loader": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' d='M12 3a9 9 0 1 1-8.3 5.5'/></svg>",
          "speaker": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M19.1 8.9a7 7 0 0 1 0 6.2M16.2 10.9a3 3 0 0 1 0 2.2M4.5 14.25h3l4.25 3.25V6.5L7.5 9.75h-3v4.5Z'/></svg>",
          "speaker-off": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m4.5 4.5 15 15M5 14.25h2.5l4.25 3.25v-5.75M11.75 8.7V6.5L8.8 8.75M16 10.8a3 3 0 0 1 .2 2.2'/></svg>",
          "airplay": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6.75 17.25h-1.5A2.25 2.25 0 0 1 3 15V6.75A2.25 2.25 0 0 1 5.25 4.5h13.5A2.25 2.25 0 0 1 21 6.75V15a2.25 2.25 0 0 1-2.25 2.25h-1.5M8.25 21h7.5L12 16.5 8.25 21Z'/></svg>",
          "guide": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 6.75h15M4.5 12h15M4.5 17.25h15M8.25 4.5v15M15.75 4.5v15'/></svg>",
          "fullscreen": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8.25 4.5H4.5v3.75M15.75 4.5h3.75v3.75M19.5 15.75v3.75h-3.75M4.5 15.75v3.75h3.75M9 9 4.5 4.5M15 9l4.5-4.5M15 15l4.5 4.5M9 15l-4.5 4.5'/></svg>",
          "fullscreen-exit": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 9h4.25V4.75M15.25 4.75V9h4.25M19.5 15h-4.25v4.25M8.75 19.25V15H4.5M8.75 9 4.5 4.75M15.25 9l4.25-4.25M15.25 15l4.25 4.25M8.75 15 4.5 19.25'/></svg>",
          "heart": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M21 8.25c0 6.25-9 11.25-9 11.25s-9-5-9-11.25A4.75 4.75 0 0 1 11.25 5 4.75 4.75 0 0 1 21 8.25Z'/></svg>",
          "heart-solid": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M12 21s-9-5.1-9-12.25A5.45 5.45 0 0 1 12 4.7a5.45 5.45 0 0 1 9 4.05C21 15.9 12 21 12 21Z'/></svg>",
          "pip": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 6.75A2.25 2.25 0 0 1 6.75 4.5h10.5a2.25 2.25 0 0 1 2.25 2.25v10.5a2.25 2.25 0 0 1-2.25 2.25H6.75a2.25 2.25 0 0 1-2.25-2.25V6.75Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M13.25 13.25h4.25v3.25h-4.25z'/></svg>",
          "captions": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 7.5A2.5 2.5 0 0 1 7 5h10a2.5 2.5 0 0 1 2.5 2.5v9A2.5 2.5 0 0 1 17 19H7a2.5 2.5 0 0 1-2.5-2.5v-9Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M8.25 10.5h3M8.25 14h2.25M13.5 14h2.25'/></svg>",
          "language": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M3.75 9h16.5M3.75 15h16.5M12 3c2.25 2.35 3.25 5.25 3.25 9S14.25 18.65 12 21c-2.25-2.35-3.25-5.25-3.25-9S9.75 5.35 12 3Z'/></svg>",
          "aspect": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 7.25A2.75 2.75 0 0 1 7.25 4.5h9.5a2.75 2.75 0 0 1 2.75 2.75v9.5a2.75 2.75 0 0 1-2.75 2.75h-9.5a2.75 2.75 0 0 1-2.75-2.75v-9.5Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M8 8h3M8 8v3M16 16h-3M16 16v-3'/></svg>",
          "search": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m20 20-4.5-4.5M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15Z'/></svg>",
          "copy": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8 8h9.25A1.75 1.75 0 0 1 19 9.75v9.5A1.75 1.75 0 0 1 17.25 21h-9.5A1.75 1.75 0 0 1 6 19.25V10'/><path stroke-linecap='round' stroke-linejoin='round' d='M5.75 16H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v.75'/></svg>",
          "external": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M13.5 4.5H19.5V10.5M19.25 4.75 11 13M10.5 6H6.75A2.25 2.25 0 0 0 4.5 8.25v9A2.25 2.25 0 0 0 6.75 19.5h9A2.25 2.25 0 0 0 18 17.25V13.5'/></svg>",
          "x": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6 6l12 12M18 6 6 18'/></svg>"
        };
        return icons[name] || "";
      }
      function menuIcon(name) { return "<span class=\"menu-icon\">" + icon(name) + "</span>"; }
      function defaultPrefs() {
        return { favorites: {}, favoriteOrder: [], autoFavorites: {}, hiddenCategories: {}, recentChannels: [], continueWatching: {}, playback: { backendProxySupported: false, streamMode: "redirect", outputFormat: "ts" }, categoryParsing: { enabled: false, mode: "off", delimiter: "pipe", regex: "", output: "" }, customGroups: [], customGroupMemberships: {} };
      }
      function prefs() { return state.app && state.app.preferences ? state.app.preferences : defaultPrefs(); }
      function defaultAdminCategorySettings() {
        return { mode: "normal", delimiter: "pipe" };
      }
      function cloneAdminCategorySettings(settings) {
        try { return JSON.parse(JSON.stringify(Object.assign(defaultAdminCategorySettings(), settings || {}))); }
        catch (_) { return defaultAdminCategorySettings(); }
      }
      function adminSettingsSignature(settings) {
        return JSON.stringify(cloneAdminCategorySettings(settings));
      }
      function adminSettingsDirty() {
        return adminSettingsSignature(state.adminCategorySettings) !== adminSettingsSignature(state.savedAdminCategorySettings);
      }
      function markAdminSettingsDraft() {
        if (state.adminSaveStatus !== "saving") {
          if (adminSettingsDirty()) {
            state.adminSaveStatus = "dirty";
            state.adminSaveMessage = "Unsaved changes.";
          } else {
            state.adminSaveStatus = "idle";
            state.adminSaveMessage = "";
          }
        }
      }
      function adminSettings() {
        return Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
      }
      function sourceMode() { return state.app && state.app.source ? String(state.app.source.mode || "") : ""; }
      function dvrEnabled() {
        return !!(state.app && state.app.capabilities && state.app.capabilities.recordings && sourceMode() === "direct_login");
      }
      function favoriteMap() { return prefs().favorites || {}; }
      function autoFavoriteMap() { return prefs().autoFavorites || {}; }
      function hiddenMap() { return prefs().hiddenCategories || {}; }
      function mergePrefs(remote, local) {
        remote = Object.assign(defaultPrefs(), remote || {});
        local = Object.assign(defaultPrefs(), local || {});
        return {
          favorites: Object.assign({}, remote.favorites, local.favorites),
          favoriteOrder: uniqueIDs(items(local.favoriteOrder).length ? items(local.favoriteOrder) : items(remote.favoriteOrder)),
          autoFavorites: Object.assign({}, remote.autoFavorites, local.autoFavorites),
          hiddenCategories: Object.assign({}, remote.hiddenCategories, local.hiddenCategories),
          recentChannels: uniqueIDs(items(remote.recentChannels).concat(items(local.recentChannels))).slice(0, 24),
          continueWatching: Object.assign({}, remote.continueWatching, local.continueWatching),
          playback: Object.assign({}, remote.playback, local.playback),
          categoryParsing: Object.assign({}, remote.categoryParsing, local.categoryParsing),
          customGroups: items(local.customGroups).length ? items(local.customGroups) : items(remote.customGroups),
          customGroupMemberships: Object.assign({}, remote.customGroupMemberships, local.customGroupMemberships)
        };
      }
      function normalizePreferences() {
        if (!state.app || !state.app.preferences) return;
        state.app.preferences = Object.assign(defaultPrefs(), state.app.preferences || {});
        state.app.preferences.categoryParsing = Object.assign(defaultPrefs().categoryParsing, state.app.preferences.categoryParsing || {});
        state.app.preferences.customGroups = items(state.app.preferences.customGroups);
        state.app.preferences.customGroupMemberships = state.app.preferences.customGroupMemberships || {};
        const valid = {};
        items(state.app.channels).forEach(function(channel) { valid[channel.id] = true; });
        const explicitFavorites = Object.keys(state.app.preferences.favorites || {}).filter(function(id) { return !!state.app.preferences.favorites[id] && !!valid[id]; });
        state.app.preferences.favoriteOrder = uniqueIDs(items(state.app.preferences.favoriteOrder).filter(function(id) { return !!state.app.preferences.favorites[id] && !!valid[id]; }).concat(explicitFavorites));
        const recent = uniqueIDs(items(state.app.preferences.recentChannels).filter(function(id) { return !!valid[id]; }));
        const watched = Object.keys(state.app.preferences.continueWatching || {}).sort(function(left, right) {
          const leftPlayed = Number((state.app.preferences.continueWatching[left] || {}).playedAt || 0);
          const rightPlayed = Number((state.app.preferences.continueWatching[right] || {}).playedAt || 0);
          return rightPlayed - leftPlayed;
        }).filter(function(id) { return !!valid[id]; });
        state.app.preferences.recentChannels = uniqueIDs(recent.concat(watched)).slice(0, 24);
        Object.keys(state.app.preferences.customGroupMemberships).forEach(function(groupID) {
          state.app.preferences.customGroupMemberships[groupID] = uniqueIDs(items(state.app.preferences.customGroupMemberships[groupID]).filter(function(id) { return !!valid[id]; }));
        });
      }
      function normalizeAdminCategorySettings() {
        state.adminCategorySettings = Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
        if (state.adminCategorySettings.mode === "custom" || state.adminCategorySettings.mode === "admin_delimiter") state.adminCategorySettings.mode = "delimiter";
        if (["normal", "delimiter"].indexOf(state.adminCategorySettings.mode) === -1) state.adminCategorySettings.mode = "normal";
        if (!state.adminCategorySettings.delimiter) state.adminCategorySettings.delimiter = "pipe";
        if (state.adminCategorySettings.delimiter !== "pipe" && state.adminCategorySettings.delimiter !== "dash") state.adminCategorySettings.delimiter = "pipe";
        delete state.adminCategorySettings.groupAliases;
        delete state.adminCategorySettings.adminGroups;
        delete state.adminCategorySettings.adminGroupMemberships;
        delete state.adminCategorySettings.presentationOverrides;
      }
      function recordWatchPreference(channel) {
        if (!state.app || !state.app.preferences || !channel) return;
        const id = String(channel.id || "");
        if (!id) return;
        const now = Math.floor(Date.now() / 1000);
        const existing = state.app.preferences.continueWatching[id] || {};
        const plays = Number(existing.plays || 0) + 1;
        state.app.preferences.recentChannels = uniqueIDs([id].concat(items(state.app.preferences.recentChannels))).slice(0, 24);
        state.app.preferences.continueWatching[id] = {
          itemKind: "channel",
          itemId: id,
          itemName: channel.name || id,
          playedAt: now,
          plays: plays
        };
        if (plays >= 3 && !favoriteMap()[id]) state.app.preferences.autoFavorites[id] = true;
        normalizePreferences();
        savePrefs();
      }
      function readLocalPrefs() {
        try { return Object.assign(defaultPrefs(), JSON.parse(localStorage.getItem(prefsKey) || "{}")); }
        catch (_) { return defaultPrefs(); }
      }
      function readSiloPrefsValue(value) {
        if (!value) return null;
        try { return Object.assign(defaultPrefs(), JSON.parse(value)); }
        catch (_) { return null; }
      }
      function readAdminSettingsValue(value) {
        if (!value) return defaultAdminCategorySettings();
        try {
          if (typeof value === "string") return Object.assign(defaultAdminCategorySettings(), JSON.parse(value));
          if (typeof value === "object") return Object.assign(defaultAdminCategorySettings(), value);
        }
        catch (_) { return defaultAdminCategorySettings(); }
        return defaultAdminCategorySettings();
      }
      async function loadPluginSettingsValues() {
        if (!pluginInstallationID) return null;
        const payload = await coreGetJSON("/api/v1/settings/plugins/" + encodeURIComponent(pluginInstallationID));
        return payload && payload.values ? payload.values : {};
      }
      async function loadUserPrefs() {
        const values = await loadPluginSettingsValues();
        return readSiloPrefsValue(values ? values.preferences : "");
      }
      async function loadAdminCategorySettings() {
        return readAdminSettingsValue(await getJSON(adminSettingsURL()));
      }
      function adminSettingsURL() {
        if (!adminSettingsToken) return "/dispatcharr/api/admin-settings";
        return "/dispatcharr/api/admin-settings?admin_token=" + encodeURIComponent(adminSettingsToken);
      }
      async function savePluginSettingValue(key, value) {
        if (!pluginInstallationID) throw new Error("plugin installation settings unavailable");
        const values = await loadPluginSettingsValues().catch(function() { return {}; }) || {};
        values[key] = value;
        await corePutNoContent("/api/v1/settings/plugins/" + encodeURIComponent(pluginInstallationID), { values: values });
      }
      function writeLocalPrefs() {
        try { localStorage.setItem(prefsKey, JSON.stringify(state.app.preferences)); } catch (_) {}
      }
      function savePrefs(options) {
        if (!state.app || !state.app.preferences) return;
        options = options || {};
        writeLocalPrefs();
        if (pluginInstallationID) {
          state.profileSaveStatus = "saving";
          state.profileSaveMessage = "";
          savePluginSettingValue("preferences", JSON.stringify(state.app.preferences)).then(function() {
            state.profileSaveStatus = "saved";
            state.profileSaveMessage = "Saved to your Silo profile.";
            if (state.view === "settings") renderSettings();
          }).catch(function(error) {
            state.profileSaveStatus = "error";
            state.profileSaveMessage = "Saved on this device, but not to your Silo profile.";
            if (!options.quiet) showAppToast(state.profileSaveMessage);
            if (state.view === "settings") renderSettings();
            try { console.warn("Dispatcharr profile preference save failed", error); } catch (_) {}
          });
        } else {
          state.profileSaveStatus = "local";
          state.profileSaveMessage = "Saved on this device, but not to your Silo profile.";
        }
        postJSON("/dispatcharr/api/preferences", state.app.preferences).catch(function() {});
      }
      function saveAdminCategorySettings() {
        state.adminCategorySettings = Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
        normalizeAdminCategorySettings();
        state.adminSaveStatus = "saving";
        state.adminSaveMessage = "Saving...";
        if (state.view === "admin") renderAdminPage();
        postJSON(adminSettingsURL(), state.adminCategorySettings).then(function(saved) {
          state.adminCategorySettings = readAdminSettingsValue(saved);
          normalizeAdminCategorySettings();
        }).then(function() {
          state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
          state.adminSaveStatus = "saved";
          state.adminSaveMessage = "Saved category mode.";
          if (state.view === "admin") renderAdminPage();
        }).catch(function(error) {
          state.adminSaveStatus = "error";
          state.adminSaveMessage = "Could not save category mode: " + readableError(error);
          if (state.view === "admin") renderAdminPage();
          try { console.warn("Dispatcharr admin category save failed", error); } catch (_) {}
        });
      }
      function discardAdminCategorySettings() {
        state.adminCategorySettings = cloneAdminCategorySettings(state.savedAdminCategorySettings);
        state.adminSaveStatus = "idle";
        state.adminSaveMessage = "";
        if (state.category.indexOf("virtual:") === 0 && !categoryName(state.category)) state.category = "";
        renderAdminPage();
      }
      async function getJSON(url) {
        const response = await fetch(route(url), { credentials: "include", headers: routeHeaders() });
        if (!response.ok) throw await requestError(response);
        return response.json();
      }
      async function postJSON(url, body) {
        const response = await fetch(route(url), { method: "POST", credentials: "include", headers: routeHeaders({ "content-type": "application/json" }), body: JSON.stringify(body) });
        if (!response.ok) throw await requestError(response);
        return response.json();
      }
      async function coreGetJSON(url) {
        const response = await fetch(url, { credentials: "include" });
        if (!response.ok) throw await requestError(response);
        return response.json();
      }
      async function corePutNoContent(url, body) {
        const response = await fetch(url, { method: "PUT", credentials: "include", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
        if (!response.ok) throw await requestError(response);
      }
      async function requestError(response) {
        const text = await response.text().catch(function() { return ""; });
        const detail = text ? ": " + text.slice(0, 240) : "";
        return new Error("request failed (" + response.status + ")" + detail);
      }
      function readableError(error) {
        return String(error && error.message ? error.message : error || "unknown error");
      }
      function channelByID(id) {
        const channel = rawChannelByID(id);
        return channel ? effectiveChannel(channel) : null;
      }
      function rawChannelByID(id) {
        return items(state.app.channels).find(function(channel) { return channel.id === id; }) || null;
      }
      function categoryStartsFeatured(name) {
        return String(name || "").trim().indexOf("*") === 0;
      }
      function categoryDisplayName(name) {
        name = String(name || "").trim();
        return categoryStartsFeatured(name) ? name.slice(1).trim() : name;
      }
      function effectiveChannel(channel) {
        if (!channel) return null;
        const copy = Object.assign({}, channel);
        const label = sourceCategoryLabel(channel);
        if (label) copy.categoryName = label;
        return copy;
      }
      function effectiveChannels(includeHidden) {
        return items(state.app.channels).map(function(channel, index) {
          const copy = effectiveChannel(channel);
          copy.sourceIndex = index;
          return copy;
        }).sort(function(left, right) {
          return (left.sourceIndex || 0) - (right.sourceIndex || 0);
        });
      }
      function sourceCategoryID(id) { return "source:" + String(id || ""); }
      function customCategoryID(id) { return "custom:" + String(id || ""); }
      function virtualCategoryID(path) { return "virtual:" + String(path || ""); }
      function virtualCategoryPath(id) { return String(id || "").indexOf("virtual:") === 0 ? String(id || "").slice("virtual:".length) : ""; }
      function categoryParsing() {
        const settings = adminSettings();
        const delimiterEnabled = settings.mode === "delimiter";
        return { enabled: delimiterEnabled, mode: delimiterEnabled ? "delimiter" : "off", delimiter: settings.delimiter || "pipe", regex: "", output: "" };
      }
      function customGroups() {
        return items(prefs().customGroups).slice().filter(function(group) {
          return group && group.id && group.name;
        }).sort(function(left, right) {
          return (Number(left.order || 0) - Number(right.order || 0)) || String(left.name || "").localeCompare(String(right.name || ""));
        });
      }
      function customMemberships(groupID) {
        return uniqueIDs(items((prefs().customGroupMemberships || {})[groupID]));
      }
      function sourceCategoryRawName(id) {
        const category = items(state.app.categories).find(function(item) { return item.id === id; });
        return category ? category.name : "";
      }
      function sourceCategoryName(id) {
        return categoryDisplayName(sourceCategoryRawName(id));
      }
      function sourceCategoryRawLabel(channel) {
        return channel.categoryName || sourceCategoryRawName(channel.categoryId) || "";
      }
      function sourceCategoryLabel(channel) {
        return categoryDisplayName(sourceCategoryRawLabel(channel));
      }
      function delimiterPattern() {
        return (adminSettings().delimiter || "pipe") === "pipe" ? /\s*\|\s*/ : /\s+-\s*/;
      }
      function parsedDelimitedPath(name) {
        name = String(name || "").trim();
        if (!name) return [];
        return name.split(delimiterPattern()).map(function(part) { return String(part || "").trim(); }).filter(Boolean);
      }
      function parsedCategoryPath(name) {
        const settings = categoryParsing();
        name = String(name || "").trim();
        if (!settings.enabled || !name) return [];
        let parts = [];
        if (settings.mode === "delimiter") {
          parts = parsedDelimitedPath(name);
        } else if (settings.mode === "regex" && settings.regex) {
          try {
            const pattern = new RegExp(settings.regex);
            const match = name.match(pattern);
            if (!match) return [];
            if (settings.output) parts = name.replace(pattern, settings.output).split("/");
            else parts = match.slice(1);
          } catch (_) {
            return [];
          }
        }
        parts = parts.map(function(part) { return String(part || "").trim(); }).filter(Boolean);
        return parts.length > 1 ? parts : [];
      }
      function virtualPathForChannel(channel) {
        const paths = virtualPathsForChannel(channel);
        return paths.length ? paths[0] : "";
      }
      function sourceVirtualPathForChannel(channel) {
        return parsedCategoryPath(sourceCategoryLabel(channel)).join(" / ");
      }
      function virtualPathsForChannel(channel) {
        const paths = [];
        const sourcePath = sourceVirtualPathForChannel(channel);
        if (sourcePath) paths.push(sourcePath);
        return uniqueIDs(paths);
      }
      function channelInSelectedCategory(channel, id) {
        if (!id) return true;
        if (id.indexOf("source:") === 0) return channel.categoryId === id.slice("source:".length);
        if (id.indexOf("custom:") === 0) return customMemberships(id.slice("custom:".length)).indexOf(channel.id) !== -1;
        if (id.indexOf("virtual:") === 0) {
          const selected = id.slice("virtual:".length);
          return virtualPathsForChannel(channel).some(function(path) {
            return path === selected || path.indexOf(selected + " / ") === 0;
          });
        }
        return channel.categoryId === id;
      }
      function visibleChannels(ignoreQuery) {
        const hidden = hiddenMap();
        const channels = effectiveChannels(false).filter(function(channel) {
          if (channel.categoryId && hidden[channel.categoryId]) return false;
          if (state.view !== "favorites" && state.category && !channelInSelectedCategory(channel, state.category)) return false;
          if (!ignoreQuery && state.query && !channelMatchesQuery(channel)) return false;
          if (state.view === "favorites" && !favoriteMap()[channel.id] && !autoFavoriteMap()[channel.id]) return false;
          return true;
        });
        return state.view === "favorites" ? orderedFavoriteChannels(channels) : channels;
      }
      function orderedFavoriteChannels(channels) {
        const byID = {};
        items(channels || effectiveChannels(false)).forEach(function(channel) { byID[channel.id] = channel; });
        const ordered = uniqueIDs(items(prefs().favoriteOrder)).map(function(id) { return byID[id]; }).filter(Boolean);
        const missing = items(channels || effectiveChannels(false)).filter(function(channel) {
          return (favoriteMap()[channel.id] || autoFavoriteMap()[channel.id]) && ordered.indexOf(channel) === -1;
        });
        return ordered.concat(missing);
      }
      function moveFavorite(channelID, direction) {
        const favorites = orderedFavoriteChannels(visibleChannels(true)).filter(function(channel) { return !!favoriteMap()[channel.id]; });
        const order = favorites.map(function(channel) { return channel.id; });
        const index = order.indexOf(channelID);
        if (index === -1) return;
        const target = direction === "up" ? index - 1 : index + 1;
        if (target < 0 || target >= order.length) return;
        const value = order[index];
        order[index] = order[target];
        order[target] = value;
        state.app.preferences.favoriteOrder = order;
        savePrefs();
        render();
      }
      function channelMatchesQuery(channel) {
        if (!state.query) return true;
        return lower([channel.name, channel.categoryName, channel.number].join(" ")).indexOf(lower(state.query)) !== -1;
      }
      function programMatchesQuery(program) {
        if (!state.query) return true;
        return lower([program.title, program.description].join(" ")).indexOf(lower(state.query)) !== -1;
      }
      function guideChannelMatchesQuery(channel) {
        if (!state.query || channelMatchesQuery(channel)) return true;
        return programsFor(channel.id).some(programMatchesQuery);
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
      function guideSlotStart() {
        const now = Math.floor(Date.now() / 1000);
        return Math.floor((now - 3600) / 1800) * 1800;
      }
      function guideSlots() {
        const start = guideSlotStart();
        const slots = [];
        for (let index = 0; index < 50; index++) slots.push(start + index * 1800);
        return slots;
      }
      function guideTimelineStyle(slots) {
        return "--epg-slots: " + slots.length + "; --epg-width: " + (slots.length * 12) + "rem;";
      }
      function guideWindow() {
        const start = guideSlotStart();
        return { start: start, end: start + (25 * 3600), slotCount: 50 };
      }
      function epgCellStyle(startUnix, endUnix, windowInfo) {
        const duration = windowInfo.end - windowInfo.start;
        const start = Math.max(startUnix || windowInfo.start, windowInfo.start);
        const end = Math.min(endUnix || start + 1800, windowInfo.end);
        const left = ((start - windowInfo.start) / duration) * 100;
        const width = Math.max(((end - start) / duration) * 100, 100 / windowInfo.slotCount * 0.66);
        return "left: " + left.toFixed(4) + "%; width: calc(" + width.toFixed(4) + "% - 0.25rem);";
      }
      function stopPlayback() {
        const video = byId("player");
        if (state.hls) { state.hls.destroy(); state.hls = null; }
        if (state.tsPlayer) { state.tsPlayer.destroy(); state.tsPlayer = null; }
        if (state.playerChromeTimer) {
          clearTimeout(state.playerChromeTimer);
          state.playerChromeTimer = null;
        }
        state.playerChromeIdle = false;
        if (video) {
          video.pause();
          video.removeAttribute("src");
          video.load();
        }
      }
      function stopCurrentWatch(reason) {
        if (!state.currentSession) return;
        postJSON("/dispatcharr/api/watch/stop", { sessionId: state.currentSession.id, reason: reason || "stop" }).catch(function() {});
        state.currentSession = null;
        if (state.heartbeat) {
          clearInterval(state.heartbeat);
          state.heartbeat = null;
        }
      }
      function setView(view) {
        if (view !== "player") {
          stopPlayback();
          if (state.view === "player") stopCurrentWatch("leave_player");
        }
        state.view = view;
        if (view === "favorites") state.category = "";
        render();
      }
      function setCategory(id) {
        state.category = id || "";
        state.view = id ? "live" : "home";
        render();
      }
      async function hydrateApp(payload) {
        state.app = payload;
        const values = await loadPluginSettingsValues().catch(function() { return null; });
        const siloPrefs = readSiloPrefsValue(values && values.preferences ? values.preferences : "");
        const localPrefs = readLocalPrefs();
        state.app.preferences = siloPrefs ? mergePrefs(siloPrefs, {}) : mergePrefs(state.app.preferences, localPrefs);
        const savedAdminSettings = readAdminSettingsValue(values && values[adminSettingsKey] ? values[adminSettingsKey] : "");
        state.adminCategorySettings = await loadAdminCategorySettings().catch(function() {
          return savedAdminSettings;
        });
        state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
        state.app.programs = items(state.app.programs);
        normalizePreferences();
        normalizeAdminCategorySettings();
        state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
        if (!isAdminRoute) savePrefs({ quiet: true });
      }
      async function loadApp() {
        await hydrateApp(await getJSON("/dispatcharr/api/app"));
        render();
      }
      async function refreshAppData() {
        if (state.refreshing) return;
        const buttons = Array.prototype.slice.call(document.querySelectorAll("[data-guide-refresh]"));
        state.refreshing = true;
        buttons.forEach(function(button) {
          button.classList.add("is-loading");
          button.disabled = true;
        });
        try {
          await hydrateApp(await postJSON("/dispatcharr/api/refresh", {}));
          state.recordings = null;
          render();
          showAppToast("Guide refreshed from Dispatcharr.");
        } catch (error) {
          showAppToast("Dispatcharr refresh failed.");
        } finally {
          state.refreshing = false;
          buttons.forEach(function(button) {
            button.classList.remove("is-loading");
            button.disabled = false;
          });
        }
      }
      function renderRail() {
        document.querySelectorAll(".nav button").forEach(function(button) {
          const unavailable = button.dataset.view === "recordings" && !dvrEnabled();
          button.hidden = unavailable;
          button.classList.toggle("active", !unavailable && button.dataset.view === state.view);
        });
        const favoriteCount = byId("favorite-count");
        if (favoriteCount) favoriteCount.textContent = Object.keys(favoriteMap()).length + Object.keys(autoFavoriteMap()).length;
      }
      function logoHTML(channel) {
        if (channel.logoUrl) return "<img class=\"logo\" src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">";
        return "<div class=\"logo\" aria-hidden=\"true\"></div>";
      }
      function render() {
        if (!state.app) return;
        if (state.view === "recordings" && !dvrEnabled()) state.view = "home";
        if (state.view === "admin" && !isAdminRoute) state.view = "home";
        document.querySelector(".shell").classList.toggle("is-player", state.view === "player");
        document.querySelector(".shell").classList.toggle("is-guide", state.view === "guide");
        renderRail();
        if (state.view === "guide") renderGuidePage();
        else if (state.view === "player") renderPlayerPage();
        else if (state.view === "live" || state.view === "favorites") renderLivePage();
        else if (state.view === "recordings") renderRecordingsPage();
        else if (state.view === "admin") renderAdminPage();
        else if (state.view === "settings") renderSettings();
        else renderHome();
      }
      function renderHome() {
        const root = byId("view");
        const recent = recentChannels(6);
        root.innerHTML = sectionHeader("Continue watching") + rowCards(recent.length ? recent : visibleChannels(false).slice(0, 6)) + sectionHeader("TV Guide") + renderHomeGuide(recent.slice(0, 5)) + sectionHeader("Categories") + categoryGrid();
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
      function favoriteCards(channels) {
        if (!channels.length) return "<div class=\"empty\">No favorite channels yet.</div>";
        return "<div class=\"row-scroll\">" + channels.map(function(channel, index) {
          const controls = favoriteMap()[channel.id] ? "<div class=\"settings-actions\"><button data-favorite-move=\"up\" data-channel-id=\"" + escapeHTML(channel.id) + "\"" + (index === 0 ? " disabled" : "") + ">Move up</button><button data-favorite-move=\"down\" data-channel-id=\"" + escapeHTML(channel.id) + "\"" + (index === channels.length - 1 ? " disabled" : "") + ">Move down</button></div>" : "";
          return "<div class=\"continue-card\"><button class=\"continue-card\" data-channel=\"" + escapeHTML(channel.id) + "\"><div class=\"poster-box\">" + (channel.logoUrl ? "<img src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">" : "<span>" + escapeHTML((channel.name || "TV").slice(0, 5)) + "</span>") + "</div><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><div class=\"muted\">" + escapeHTML(channel.categoryName || "Live TV") + "</div></button>" + controls + "</div>";
        }).join("") + "</div>";
      }
      function categoryGrid() {
        const hidden = hiddenMap();
        const sourceCategories = sourceCategoriesWithChannels(function(channel) {
          return !(channel.categoryId && hidden[channel.categoryId]);
        });
        const custom = customGroupCategories();
        const listing = adminListingCategories("");
        const featured = sourceCategories.filter(function(category) { return !!category.featured; });
        const featuredSourceIDs = {};
        featured.forEach(function(category) { featuredSourceIDs[category.sourceID] = true; });
        const regularListing = listing.filter(function(category) {
          return !(category.kind === "source" && featuredSourceIDs[category.sourceID]);
        });
        const sections = [];
        if (featured.length) sections.push(categoryGridSection("Featured", featured));
        if (custom.length) sections.push(categoryGridSection("My groups", custom));
        if (regularListing.length) sections.push(categoryGridSection(adminListingTitle(), regularListing));
        if (!listing.length && sourceCategories.length) sections.push(categoryGridSection("Source categories", sourceCategories));
        return sections.length ? sections.join("") : "<div class=\"empty\">No categories yet.</div>";
      }
      function categoryGridSection(title, categories) {
        return sectionHeader(title) + "<div class=\"category-grid\">" + categories.map(function(category) {
          return "<button class=\"tile" + (state.category === category.id ? " active" : "") + "\" data-category=\"" + escapeHTML(category.id) + "\"><strong>" + escapeHTML(category.name || category.id) + "</strong><span>" + escapeHTML(category.count ? category.count + " channels" : category.kind || "") + "</span></button>";
        }).join("") + "</div>";
      }
      function virtualFolderHeader(path) {
        const parts = path.split(" / ").filter(Boolean);
        const parentPath = parts.slice(0, -1).join(" / ");
        const backID = parentPath ? virtualCategoryID(parentPath) : "";
        return "<div class=\"section-title\">" + virtualFolderBreadcrumbs(path) + (path ? "<button class=\"chip\" data-category=\"" + escapeHTML(backID) + "\">Back</button>" : "") + "</div>";
      }
      function virtualFolderBreadcrumbs(path) {
        const parts = path.split(" / ").filter(Boolean);
        const crumbs = ["<button data-category=\"\">Virtual Categories</button>"];
        parts.forEach(function(part, index) {
          const crumbPath = parts.slice(0, index + 1).join(" / ");
          crumbs.push("<span class=\"sep\">/</span><button data-category=\"" + escapeHTML(virtualCategoryID(crumbPath)) + "\">" + escapeHTML(part) + "</button>");
        });
        return "<div class=\"breadcrumbs\" aria-label=\"Virtual folder breadcrumbs\">" + crumbs.join("") + "</div>";
      }
      function sourceCategoriesWithChannels(includeChannel) {
        const categoryCounts = {};
        effectiveChannels(false).forEach(function(channel) {
          if (includeChannel && !includeChannel(channel)) return;
          if (channel.categoryId) categoryCounts[channel.categoryId] = (categoryCounts[channel.categoryId] || 0) + 1;
        });
        return items(state.app.categories).filter(function(category) {
          return !!categoryCounts[category.id];
        }).map(function(category) {
          const rawName = category.name || category.id;
          return { id: sourceCategoryID(category.id), sourceID: category.id, name: categoryDisplayName(rawName), featured: categoryStartsFeatured(rawName), kind: "source", count: categoryCounts[category.id] || 0 };
        });
      }
      function customGroupCategories() {
        return customGroups().map(function(group) {
          return { id: customCategoryID(group.id), name: group.name, kind: "custom", count: customMemberships(group.id).filter(channelByID).length };
        }).filter(function(group) {
          return group.count > 0;
        });
      }
      function virtualGroupCategories(includeChannel) {
        return virtualCategoriesFromPaths("", includeChannel, true);
      }
      function virtualCategoriesFromPaths(parentPath, includeChannel, includeAllDescendants) {
        parentPath = String(parentPath || "");
        const groups = {};
        effectiveChannels(false).forEach(function(channel) {
          if (includeChannel && !includeChannel(channel)) return;
          virtualPathsForChannel(channel).forEach(function(path) {
            const parts = path.split(" / ").filter(Boolean);
            const parentParts = parentPath ? parentPath.split(" / ").filter(Boolean) : [];
            if (parts.length <= parentParts.length) return;
            for (let index = 0; index < parentParts.length; index++) {
              if (parts[index] !== parentParts[index]) return;
            }
            const limit = includeAllDescendants ? parts.length : parentParts.length + 1;
            for (let index = parentParts.length; index < limit; index++) {
              const childPath = parts.slice(0, index + 1).join(" / ");
              groups[childPath] = groups[childPath] || { id: virtualCategoryID(childPath), name: includeAllDescendants ? childPath : parts[index], kind: "virtual", count: 0, channelIDs: {} };
              groups[childPath].channelIDs[channel.id] = true;
            }
          });
        });
        return Object.keys(groups).sort().map(function(path) {
          const group = groups[path];
          group.count = Object.keys(group.channelIDs).length;
          delete group.channelIDs;
          return group;
        });
      }
      function sourceVirtualChildCategories(parentPath, includeChannel) {
        parentPath = String(parentPath || "");
        const children = {};
        effectiveChannels(false).forEach(function(channel) {
          if (includeChannel && !includeChannel(channel)) return;
          const parts = parsedCategoryPath(sourceCategoryLabel(channel));
          for (let index = 0; index < parts.length; index++) {
            const path = parts.slice(0, index + 1).join(" / ");
            const parentParts = parentPath ? parentPath.split(" / ").filter(Boolean) : [];
            if (parts.length <= parentParts.length) return;
            for (let parentIndex = 0; parentIndex < parentParts.length; parentIndex++) {
              if (parts[parentIndex] !== parentParts[parentIndex]) return;
            }
            if (index !== parentParts.length) continue;
            children[path] = children[path] || { id: virtualCategoryID(path), name: parts[parentParts.length], kind: "virtual", count: 0 };
            children[path].count++;
          }
        });
        return Object.keys(children).sort().map(function(path) { return children[path]; });
      }
      function virtualChildCategories(parentPath, includeChannel) {
        return sourceVirtualChildCategories(parentPath, includeChannel);
      }
      function allFilterCategories() {
        const hidden = hiddenMap();
        return customGroupCategories().concat(adminListingCategories("", function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); }));
      }
      function adminListingTitle() {
        const mode = adminSettings().mode || "normal";
        if (mode === "delimiter") return "Virtual Categories";
        return "Source categories";
      }
      function adminListingCategories(parentPath, includeChannel) {
        const hidden = hiddenMap();
        includeChannel = includeChannel || function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); };
        const mode = adminSettings().mode || "normal";
        if (mode === "delimiter") return parentPath ? sourceVirtualChildCategories(parentPath, includeChannel) : sourceVirtualChildCategories("", includeChannel);
        return sourceCategoriesWithChannels(includeChannel);
      }
      function virtualCategoriesActive() {
        const hidden = hiddenMap();
        return adminSettings().mode === "delimiter" && virtualGroupCategories(function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); }).length > 0;
      }
      function recentChannels(limit) {
        const seen = {};
        const channels = [];
        items(prefs().recentChannels).forEach(function(id) {
          if (seen[id]) return;
          const channel = channelByID(id);
          if (!channel) return;
          seen[id] = true;
          channels.push(channel);
        });
        return channels.slice(0, limit || channels.length);
      }
      function renderHomeGuide(channels, emptyMessage) {
        if (!channels.length) return "<div class=\"empty\">" + escapeHTML(emptyMessage || "No recently watched channels yet.") + "</div>";
        const slots = guideSlots();
        return "<div class=\"home-guide guide-scroll\"><div class=\"guide-page guide-timeline\" style=\"" + guideTimelineStyle(slots) + "\"><div class=\"time-head\"><span>Today</span>" + slots.map(function(slot) { return "<span>" + escapeHTML(timeLabel(slot)) + "</span>"; }).join("") + "</div>" + channels.map(function(channel, channelIndex) {
          return "<div class=\"epg-row\"><button class=\"epg-channel\" data-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"" + escapeHTML(channel.name || "Untitled") + "\">" + logoHTML(channel) + "</button><div class=\"epg-programs\">" + renderEPGCells(channel, channelIndex) + "</div></div>";
        }).join("") + "</div></div>";
      }
      function renderVirtualCategoryGuide(channels) {
        return sectionHeader("TV Guide") + renderHomeGuide(channels, "No channels in this virtual category yet.");
      }
      function renderLivePage() {
        const channels = visibleChannels(false);
        if (state.view === "favorites") {
          byId("view").innerHTML = sectionHeader("Favorite channels") + favoriteCards(channels.slice(0, 60));
          return;
        }
        if (state.category.indexOf("virtual:") === 0) {
          const path = virtualCategoryPath(state.category);
          const hidden = hiddenMap();
          const children = virtualChildCategories(path, function(channel) {
            return !(channel.categoryId && hidden[channel.categoryId]);
          });
          byId("view").innerHTML = virtualFolderHeader(path)
            + renderVirtualCategoryGuide(channels)
            + (children.length ? sectionHeader("Virtual Categories") + "<div class=\"category-grid\">" + children.map(function(category) {
              return "<button class=\"tile\" data-category=\"" + escapeHTML(category.id) + "\"><strong>" + escapeHTML(category.name || category.id) + "</strong><span>" + escapeHTML(category.count ? category.count + " channels" : category.kind || "") + "</span></button>";
            }).join("") + "</div>" : "");
          return;
        }
        byId("view").innerHTML = categoryGrid() + sectionHeader(categoryName(state.category) || "Channels") + rowCards(channels.slice(0, 24));
      }
      function recordingCustom(recording) {
        return recording && recording.custom_properties && typeof recording.custom_properties === "object" ? recording.custom_properties : {};
      }
      function recordingProgram(recording) {
        const custom = recordingCustom(recording);
        return custom.program && typeof custom.program === "object" ? custom.program : {};
      }
      function recordingStatus(recording) {
        const custom = recordingCustom(recording);
        const now = Date.now();
        const start = Date.parse(recording.start_time || custom.start_time || "");
        const end = Date.parse(recording.end_time || custom.end_time || "");
        if (custom.status) return String(custom.status);
        if (!Number.isNaN(start) && start > now) return "upcoming";
        if (!Number.isNaN(start) && !Number.isNaN(end) && start <= now && end >= now) return "recording";
        return "completed";
      }
      function recordingTitle(recording) {
        const custom = recordingCustom(recording);
        const program = recordingProgram(recording);
        return custom.title || program.title || custom.file_name || "Untitled recording";
      }
      function recordingChannelName(recording) {
        const custom = recordingCustom(recording);
        const program = recordingProgram(recording);
        return custom.channel_name || program.channel || program.channel_name || "Dispatcharr";
      }
      function recordingTimeLabel(value) {
        const date = new Date(value || "");
        if (Number.isNaN(date.getTime())) return "";
        return date.toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
      }
      function recordingWindow(recording) {
        const start = recordingTimeLabel(recording.start_time);
        const end = recordingTimeLabel(recording.end_time);
        if (start && end) return start + " - " + end;
        return start || end || "Time unavailable";
      }
      function normalizeRecordings(payload) {
        if (!payload || !payload.available) return [];
        return items(payload.items).slice().sort(function(a, b) {
          const aTime = Date.parse(a.start_time || "");
          const bTime = Date.parse(b.start_time || "");
          return (Number.isNaN(bTime) ? 0 : bTime) - (Number.isNaN(aTime) ? 0 : aTime);
        });
      }
      function recordingPlaybackURL(recording) {
        const silo = recording && recording._silo ? recording._silo : {};
        return silo.playback_url || "";
      }
      function recordingMatchesQuery(recording) {
        if (!state.query) return true;
        const haystack = [recordingTitle(recording), recordingChannelName(recording), recordingStatus(recording)].join(" ").toLowerCase();
        return haystack.indexOf(lower(state.query)) !== -1;
      }
      function renderRecordingCard(recording) {
        const status = recordingStatus(recording).toLowerCase();
        const playbackURL = recordingPlaybackURL(recording);
        const action = playbackURL ? "<button class=\"recording-action\" data-recording-playback=\"" + escapeHTML(playbackURL) + "\">" + icon("play") + "<span>Playback</span></button>" : "";
        return "<div class=\"recording-card\"><span><strong>" + escapeHTML(recordingTitle(recording)) + "</strong><span class=\"recording-meta\">" + escapeHTML(recordingChannelName(recording) + " - " + recordingWindow(recording)) + "</span></span><div class=\"recording-actions\">" + action + "<span class=\"recording-badge " + escapeHTML(status) + "\">" + escapeHTML(status.split("_").join(" ")) + "</span></div></div>";
      }
      function renderRecordingSection(title, recordings) {
        if (!recordings.length) return "";
        return sectionHeader(title) + "<div class=\"recording-list\">" + recordings.map(renderRecordingCard).join("") + "</div>";
      }
      function loadRecordings(force) {
        if (!dvrEnabled()) {
          state.recordings = { available: false, reason: "Recordings require Dispatcharr Direct Connect.", items: [] };
          return;
        }
        if (state.recordingsLoading || (state.recordings && !force)) return;
        state.recordingsLoading = true;
        getJSON("/dispatcharr/api/recordings").then(function(payload) {
          state.recordings = payload;
        }).catch(function(error) {
          state.recordings = { available: false, reason: "Unable to load Dispatcharr recordings.", items: [] };
        }).finally(function() {
          state.recordingsLoading = false;
          if (state.view === "recordings") render();
        });
      }
      function programByID(channelID, programID) {
        return programsFor(channelID).find(function(program) { return String(program.id || "") === String(programID || ""); }) || null;
      }
      function scheduleProgram(channelID, programID, button) {
        if (!dvrEnabled()) {
          showAppToast("Recordings require Dispatcharr Direct Connect.");
          return;
        }
        const channel = channelByID(channelID);
        const program = programByID(channelID, programID);
        if (!channel || !program) {
          showAppToast("Could not find that guide entry.");
          return;
        }
        if (button) button.disabled = true;
        postJSON("/dispatcharr/api/recordings", {
          channelId: channel.id,
          programId: program.id || "",
          title: program.title || channel.name || "Recording",
          description: program.description || "",
          startUnix: program.startUnix || 0,
          endUnix: program.endUnix || 0
        }).then(function() {
          state.recordings = null;
          loadRecordings(true);
          showAppToast("Recording scheduled in Dispatcharr.");
        }).catch(function() {
          showAppToast("Dispatcharr could not schedule that recording.");
        }).finally(function() {
          if (button) button.disabled = false;
        });
      }
      function renderRecordingsPage() {
        const root = byId("view");
        if (!state.recordings) {
          root.innerHTML = sectionHeader("Recordings") + "<div class=\"empty\">Loading Dispatcharr recordings...</div>";
          loadRecordings(false);
          return;
        }
        const toolbar = "<div class=\"recording-toolbar\"><button class=\"recording-refresh\" data-recordings-refresh=\"true\">Refresh recordings</button></div>";
        if (!state.recordings.available) {
          root.innerHTML = toolbar + sectionHeader("Recordings") + "<div class=\"empty\">" + escapeHTML(state.recordings.reason || "Recordings are not available for this connection mode.") + "</div>";
          return;
        }
        const recordings = normalizeRecordings(state.recordings).filter(recordingMatchesQuery);
        const active = recordings.filter(function(recording) { return recordingStatus(recording).toLowerCase() === "recording"; });
        const upcoming = recordings.filter(function(recording) { return recordingStatus(recording).toLowerCase() === "upcoming"; });
        const completed = recordings.filter(function(recording) {
          const status = recordingStatus(recording).toLowerCase();
          return status !== "recording" && status !== "upcoming";
        });
        root.innerHTML = toolbar
          + renderRecordingSection("Recording now", active)
          + renderRecordingSection("Upcoming", upcoming)
          + renderRecordingSection("Completed", completed.slice(0, 80))
          + (!recordings.length ? "<div class=\"empty\">No Dispatcharr recordings found.</div>" : "");
      }
      function currentProgram(channel) {
        if (!channel) return null;
        const now = Math.floor(Date.now() / 1000);
        return programsFor(channel.id).find(function(program) {
          return (!program.startUnix || program.startUnix <= now + 600) && (!program.endUnix || program.endUnix >= now);
        }) || programsFor(channel.id)[0] || null;
      }
      function playerLogoHTML(channel) {
        if (channel && channel.logoUrl) return "<img class=\"player-logo\" src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">";
        return "<div class=\"player-logo player-logo-fallback\">" + escapeHTML(((channel && channel.name) || "TV").slice(0, 5)) + "</div>";
      }
      function playerFavoriteButtonHTML(channel) {
        const isFavorite = !!(channel && favoriteMap()[channel.id]);
        return "<button id=\"player-favorite-button\" class=\"player-icon favorite" + (isFavorite ? " active" : "") + "\" data-player-action=\"favorite\" aria-label=\"" + (isFavorite ? "Remove channel from favorites" : "Favorite channel") + "\" aria-pressed=\"" + (isFavorite ? "true" : "false") + "\">" + icon(isFavorite ? "heart-solid" : "heart") + "</button>";
      }
      function renderPlayerPage() {
        const channel = state.currentChannel || visibleChannels(false)[0] || null;
        const program = currentProgram(channel) || {};
        const channelName = channel ? channel.name || "Untitled channel" : "Choose a channel";
        const categoryNameText = channel ? channel.categoryName || "Live TV" : "Live TV";
        const title = program.title || channelName;
        const description = program.description || categoryNameText;
        const start = timeLabel(program.startUnix) || "LIVE";
        const end = timeLabel(program.endUnix) || "Now";
        byId("view").innerHTML = "<section class=\"playback-shell\"><div class=\"playback-stage\"><video id=\"player\" class=\"playback-video\" autoplay playsinline></video><div class=\"playback-scrim\"></div><button id=\"player-center-button\" class=\"player-center-button hidden\" data-player-action=\"play-toggle\" aria-label=\"Play\">" + icon("play") + "</button><div class=\"player-top\"><button class=\"player-exit\" data-player-action=\"back\" aria-label=\"Back to Live TV browse\"><span class=\"player-icon\">" + icon("x") + "</span><span>Exit</span></button><div class=\"player-top-actions\"><div class=\"player-audio\"><button id=\"player-audio-button\" class=\"player-chip\" data-player-action=\"audio-menu\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("language") + "<span>Audio</span>" + icon("chevron-down") + "</button><div id=\"player-audio-menu\" class=\"player-menu\" role=\"menu\"></div></div><div class=\"player-volume\"><button id=\"player-volume-button\" class=\"player-icon\" data-player-action=\"volume-menu\" aria-label=\"Volume\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("speaker") + "</button><div id=\"player-volume-popover\" class=\"volume-popover\"><span>VOL</span><input id=\"player-volume-slider\" type=\"range\" min=\"0\" max=\"100\" step=\"1\" value=\"" + Math.round(state.volume * 100) + "\" aria-label=\"Volume\"><span id=\"player-volume-value\" class=\"volume-value\"></span></div></div><button class=\"player-icon\" data-player-action=\"cast\" aria-label=\"AirPlay or Cast\">" + icon("airplay") + "</button><button id=\"player-guide-button\" class=\"player-icon player-guide-button\" data-player-action=\"guide\" aria-label=\"Guide\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("guide") + "</button><button id=\"player-fullscreen-button\" class=\"player-icon\" data-player-action=\"fullscreen\" aria-label=\"Fullscreen\" aria-pressed=\"false\">" + icon("fullscreen") + "</button><div class=\"player-more\"><button id=\"player-more-button\" class=\"player-icon\" data-player-action=\"more\" aria-label=\"More\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("ellipsis") + "</button><div id=\"player-more-menu\" class=\"player-more-menu\"></div></div></div></div><div id=\"player-toast\" class=\"player-toast\" role=\"status\"></div><div id=\"player-guide-panel\" class=\"player-guide-panel\"></div><div class=\"player-bottom\"><div class=\"player-bottom-row\"><div class=\"player-meta\">" + playerLogoHTML(channel) + "<div class=\"player-kicker\">" + escapeHTML(channelName) + "</div><h2 class=\"player-title\">" + escapeHTML(title) + "</h2><p class=\"player-description\">" + escapeHTML(description) + "</p><div class=\"player-tags\"><span class=\"player-tag\">" + escapeHTML(categoryNameText) + "</span><span class=\"player-tag\">AV</span></div></div><div class=\"player-bottom-actions\">" + playerFavoriteButtonHTML(channel) + "<button class=\"player-icon\" data-player-action=\"pip\" aria-label=\"Picture in Picture\">" + icon("pip") + "</button><button id=\"player-subtitles-button\" class=\"player-icon\" data-player-action=\"subtitles\" aria-label=\"Subtitles\" aria-pressed=\"false\">" + icon("captions") + "</button><button id=\"player-language-button\" class=\"player-icon\" data-player-action=\"language-menu\" aria-label=\"Audio language\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("language") + "</button></div></div><div class=\"timeline\"><span>" + escapeHTML(start) + "</span><div class=\"timeline-bar\"><div class=\"timeline-fill\"></div><div class=\"timeline-knob\"></div></div><span><span class=\"live-dot\"></span>LIVE&nbsp;&nbsp;" + escapeHTML(end) + "</span></div></div></div></section>";
        updateAudioMenu();
        updateVolumeMenu();
        renderPlayerGuidePanel();
        renderPlayerMoreMenu();
        updateFullscreenButton();
        wakePlayerChrome(1800);
      }
      function hasOpenPlayerOverlay() {
        return state.audioMenuOpen || state.volumeMenuOpen || state.moreMenuOpen || state.playerGuideOpen;
      }
      function updatePlayerChrome() {
        const shell = document.querySelector(".playback-shell");
        if (!shell) return;
        shell.classList.toggle("is-idle", state.playerChromeIdle && !hasOpenPlayerOverlay());
      }
      function wakePlayerChrome(delay) {
        if (state.view !== "player") return;
        state.playerChromeIdle = false;
        updatePlayerChrome();
        if (state.playerChromeTimer) clearTimeout(state.playerChromeTimer);
        state.playerChromeTimer = setTimeout(function() {
          state.playerChromeIdle = true;
          updatePlayerChrome();
        }, delay || 2400);
      }
      function renderPlayerGuidePanel() {
        const panel = byId("player-guide-panel");
        const button = byId("player-guide-button");
        if (!panel) return;
        const channels = visibleChannels(true).slice(0, 42);
        panel.classList.toggle("open", state.playerGuideOpen);
        if (button) {
          button.classList.toggle("active", state.playerGuideOpen);
          button.setAttribute("aria-expanded", state.playerGuideOpen ? "true" : "false");
        }
        updatePlayerChrome();
        if (!state.playerGuideOpen) return;
        panel.innerHTML = "<div class=\"player-guide-head\"><div><strong>Channel Guide</strong><span>" + escapeHTML(categoryName(state.category) || "Live TV") + "</span></div><button class=\"player-icon\" data-player-action=\"guide-close\" aria-label=\"Close guide\">" + icon("x") + "</button></div><div class=\"player-guide-list\">" + channels.map(function(channel) {
          const program = currentProgram(channel) || {};
          const title = program.title || "Data not available";
          const time = timeLabel(program.startUnix) || "Live";
          return "<button class=\"player-guide-row" + (state.currentChannel && state.currentChannel.id === channel.id ? " active" : "") + "\" data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><small>" + escapeHTML(time + " - " + title) + "</small></span></button>";
        }).join("") + "</div>";
      }
      function currentStreamURL() {
        return state.currentChannel ? route("/dispatcharr/stream?channel_id=" + encodeURIComponent(state.currentChannel.id)) : "";
      }
      function browserStreamURL(channel) {
        return route("/dispatcharr/stream?channel_id=" + encodeURIComponent(channel.id) + "&output_profile=2");
      }
      function applyAspectMode() {
        const video = byId("player");
        if (video) video.style.objectFit = state.aspectMode === "fit" ? "contain" : "cover";
      }
      function renderPlayerMoreMenu() {
        const button = byId("player-more-button");
        const menu = byId("player-more-menu");
        if (!menu) return;
        if (button) button.setAttribute("aria-expanded", state.moreMenuOpen ? "true" : "false");
        menu.classList.toggle("open", state.moreMenuOpen);
        updatePlayerChrome();
        if (!state.moreMenuOpen) return;
        const recent = items(prefs().recentChannels).map(channelByID).filter(Boolean).filter(function(channel) {
          return !state.currentChannel || channel.id !== state.currentChannel.id;
        }).slice(0, 3);
        menu.innerHTML = "<div class=\"player-more-kicker\">Video settings & controls</div>"
          + "<button data-player-action=\"aspect\">" + menuIcon("aspect") + "<span>Aspect ratio<small>" + (state.aspectMode === "fit" ? "Fit to screen" : "Fill screen") + "</small></span></button>"
          + "<button data-player-action=\"fullscreen\">" + menuIcon(document.fullscreenElement ? "fullscreen-exit" : "fullscreen") + "<span>Fullscreen<small>" + (document.fullscreenElement ? "Exit player fullscreen" : "Fill the display") + "</small></span></button>"
          + "<button data-player-action=\"guide\">" + menuIcon("guide") + "<span>Channel guide<small>Browse channels without leaving playback</small></span></button>"
          + "<button data-player-action=\"search-channel\">" + menuIcon("search") + "<span>Search channel<small>Jump to the channel list search</small></span></button>"
          + (recent.length ? "<div class=\"player-more-separator\"></div><div class=\"player-more-kicker\">Channels history</div>" + recent.map(function(channel) { return "<button data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span>" + escapeHTML(channel.name || "Untitled") + "<small>" + escapeHTML(channel.categoryName || "Live TV") + "</small></span></button>"; }).join("") : "")
          + "<div class=\"player-more-separator\"></div><div class=\"player-more-kicker\">Video & audio casting</div>"
          + "<button data-player-action=\"cast\">" + menuIcon("airplay") + "<span>AirPlay or Cast<small>Use browser playback target picker</small></span></button>"
          + "<button data-player-action=\"copy-stream\">" + menuIcon("copy") + "<span>Copy stream URL<small>For an external player</small></span></button>"
          + "<button data-player-action=\"open-stream\">" + menuIcon("external") + "<span>Use external video player<small>Open the stream route in a new tab</small></span></button>";
      }
      function renderGuidePage() {
        const categories = allFilterCategories();
        const slots = guideSlots();
        byId("view").innerHTML = "<div class=\"guide-page\"><div class=\"guide-tools\"><select id=\"category-select\" class=\"select\"><option value=\"\">All categories</option>" + categories.map(function(category) { return "<option value=\"" + escapeHTML(category.id) + "\"" + (state.category === category.id ? " selected" : "") + ">" + escapeHTML(category.name || category.id) + "</option>"; }).join("") + "</select><button id=\"guide-inline-refresh\" class=\"refresh-button\" type=\"button\" data-guide-refresh=\"true\" aria-label=\"Refresh guide\" title=\"Refresh guide\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" aria-hidden=\"true\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M20 12a8 8 0 0 1-14.1 5.15M4 12A8 8 0 0 1 18.1 6.85\"/><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M6 17.25H3.75V19.5M18 6.75h2.25V4.5\"/></svg></button><input id=\"guide-search\" class=\"search\" placeholder=\"Search by program or channel\" value=\"" + escapeHTML(state.query) + "\"></div><div class=\"guide-scroll\"><div class=\"guide-timeline\" style=\"" + guideTimelineStyle(slots) + "\"><div class=\"time-head\"><span>Today</span>" + slots.map(function(slot) { return "<span>" + escapeHTML(timeLabel(slot)) + "</span>"; }).join("") + "</div><div id=\"epg\"></div></div></div></div>";
        byId("category-select").onchange = function(event) { state.category = event.target.value; renderGuidePage(); };
        byId("guide-search").oninput = function(event) { state.query = event.target.value; resetGuideRows(); renderEPG(); };
        resetGuideRows();
        renderEPG();
      }
      function guideBatchSize() { return 40; }
      function resetGuideRows() {
        state.guideChannels = visibleChannels(true).filter(guideChannelMatchesQuery);
        state.guideRendered = 0;
        state.guideLoading = false;
      }
      function renderEPGCells(channel, channelIndex) {
        const windowInfo = guideWindow();
        const windowStart = windowInfo.start;
        const windowEnd = windowInfo.end;
        const now = Math.floor(Date.now() / 1000);
        const channelMatched = channelMatchesQuery(channel);
        const programs = programsFor(channel.id).filter(function(program) {
          const start = program.startUnix || windowStart;
          const end = program.endUnix || start + 1800;
          const matchesQuery = channelMatched || programMatchesQuery(program);
          return matchesQuery && end > windowStart && start < windowEnd;
        });
        if (!programs.length) {
          return "<button class=\"epg-cell program gray\" data-channel=\"" + escapeHTML(channel.id) + "\" style=\"left: 0; width: calc(100% - 0.25rem);\"><time>" + escapeHTML(timeLabel(windowStart)) + "</time><strong>Data not available</strong></button>";
        }
        return programs.map(function(program, index) {
          const canSchedule = dvrEnabled() && (program.endUnix || 0) > now;
          return "<div class=\"epg-cell program " + colorClass(index + channelIndex) + "\" style=\"" + epgCellStyle(program.startUnix, program.endUnix, windowInfo) + "\"><button class=\"epg-play\" data-channel=\"" + escapeHTML(channel.id) + "\"><time>" + escapeHTML(timeLabel(program.startUnix)) + "</time><strong>" + escapeHTML(program.title || "Data not available") + "</strong></button>" + (canSchedule ? "<button class=\"epg-schedule\" data-schedule-channel=\"" + escapeHTML(channel.id) + "\" data-schedule-program=\"" + escapeHTML(program.id || "") + "\" aria-label=\"Schedule recording\">" + icon("record") + "</button>" : "") + "</div>";
        }).join("");
      }
      function renderEPGRow(channel, channelIndex) {
        return "<div class=\"epg-row\"><button class=\"epg-channel\" data-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"" + escapeHTML(channel.name || "Untitled") + "\">" + logoHTML(channel) + "</button><div class=\"epg-programs\">" + renderEPGCells(channel, channelIndex) + "</div></div>";
      }
      function renderEPG() {
        const root = byId("epg");
        root.innerHTML = "";
        appendGuideRows();
      }
      function appendGuideRows() {
        if (state.view !== "guide" || state.guideLoading) return;
        const root = byId("epg");
        if (!root) return;
        if (!state.guideChannels.length) {
          root.innerHTML = "<div class=\"empty\">No guide matches.</div>";
          return;
        }
        if (state.guideRendered >= state.guideChannels.length) return;
        state.guideLoading = true;
        const start = state.guideRendered;
        const end = Math.min(start + guideBatchSize(), state.guideChannels.length);
        const rows = state.guideChannels.slice(start, end).map(function(channel, offset) {
          return renderEPGRow(channel, start + offset);
        }).join("");
        root.insertAdjacentHTML("beforeend", rows);
        state.guideRendered = end;
        state.guideLoading = false;
        if (isNearGuideEnd()) appendGuideRows();
      }
      function isNearGuideEnd() {
        return window.innerHeight + window.scrollY > document.documentElement.scrollHeight - 900;
      }
      function renderSettings() {
        ensureSelectedCustomGroup();
        const showSourceCategorySettings = !virtualCategoriesActive();
        byId("view").innerHTML = "<div class=\"settings-stack\">"
          + "<div class=\"settings-card\"><h2>Custom groups</h2><div id=\"custom-group-settings\" class=\"settings-list\"></div></div>"
          + (showSourceCategorySettings ? "<div class=\"settings-card\"><h2>Hidden source categories</h2><div id=\"settings-list\" class=\"settings-list\"></div></div>" : "")
          + "</div>";
        renderCustomGroupSettings();
        if (!showSourceCategorySettings) return;
        const root = byId("settings-list");
        const categories = sourceCategoriesWithChannels();
        root.innerHTML = categories.map(function(category) {
          return "<label><span>" + escapeHTML(category.name || category.sourceID) + "</span><input type=\"checkbox\" data-hide=\"" + escapeHTML(category.sourceID) + "\"" + (hiddenMap()[category.sourceID] ? " checked" : "") + "></label>";
        }).join("") || "<div class=\"empty\">No source categories available for this connection.</div>";
      }
      function categoryName(id) {
        if (String(id || "").indexOf("source:") === 0) return sourceCategoryName(String(id || "").slice("source:".length));
        const category = allFilterCategories().find(function(item) { return item.id === id; });
        return category ? category.name : "";
      }
      function profileSaveStatusHTML() {
        if (!state.profileSaveMessage) return "";
        const warning = state.profileSaveStatus === "error" || state.profileSaveStatus === "local";
        return "<div class=\"settings-note" + (warning ? " settings-warning" : "") + "\">" + escapeHTML(state.profileSaveMessage) + "</div>";
      }
      function adminSaveStatusHTML() {
        if (!state.adminSaveMessage) return "";
        const warning = state.adminSaveStatus === "error" || state.adminSaveStatus === "dirty";
        return "<div class=\"settings-note" + (warning ? " settings-warning" : "") + "\">" + escapeHTML(state.adminSaveMessage) + "</div>";
      }
      function updateCategoryParsingField(field, target) {
        const settings = state.adminCategorySettings || defaultAdminCategorySettings();
        settings[field] = target.value;
        if (!settings.delimiter) settings.delimiter = "pipe";
        state.adminCategorySettings = settings;
        if (state.category.indexOf("virtual:") === 0 && !categoryName(state.category)) state.category = "";
        normalizeAdminCategorySettings();
        markAdminSettingsDraft();
        renderAdminPage();
      }
      function renderAdminPage() {
        normalizeAdminCategorySettings();
        const settings = adminSettings();
        byId("view").innerHTML = "<div class=\"settings-stack\">"
          + "<div class=\"settings-card\"><h2>Category method</h2><div id=\"admin-category-settings\" class=\"settings-list\"></div></div>"
          + "<div class=\"settings-card\"><h2>Preview</h2><div class=\"settings-preview\">" + adminCategoryPreview() + "</div></div>"
          + "</div>";
        renderAdminCategorySettings();
      }
      function renderAdminCategorySettings() {
        const settings = adminSettings();
        const root = byId("admin-category-settings");
        const dirty = adminSettingsDirty();
        const saving = state.adminSaveStatus === "saving";
        root.innerHTML = adminSaveStatusHTML()
          + "<div class=\"settings-row\"><span>Mode</span><select data-admin-category-field=\"mode\"><option value=\"normal\"" + (settings.mode === "normal" ? " selected" : "") + ">Normal</option><option value=\"delimiter\"" + (settings.mode === "delimiter" ? " selected" : "") + ">By delimiter</option></select></div>"
          + (settings.mode !== "normal" ? "<div class=\"settings-row\"><span>Delimiter</span><select data-admin-category-field=\"delimiter\"><option value=\"pipe\"" + (settings.delimiter === "pipe" ? " selected" : "") + ">Pipe: Sports | NHL Teams</option><option value=\"dash\"" + (settings.delimiter === "dash" ? " selected" : "") + ">Dash: Sports - NHL Teams</option></select></div>" : "")
          + (settings.mode === "normal" ? "<div class=\"settings-note\">Source categories are shown as provided, without remapping or resorting.</div>" : "")
          + (settings.mode === "delimiter" ? "<div class=\"settings-note\">Source category names are split into virtual folders using the selected delimiter.</div>" : "")
          + "<div class=\"settings-actions\"><button data-admin-settings-action=\"save\"" + ((!dirty || saving) ? " disabled" : "") + ">Save</button><button data-admin-settings-action=\"discard\"" + ((!dirty || saving) ? " disabled" : "") + ">Discard</button></div>";
      }
      function adminCategoryPreview() {
        const categories = adminListingCategories("").slice(0, 8);
        return categories.length ? categories.map(function(category) {
          return "<div>" + escapeHTML(category.name || category.id) + " · " + escapeHTML(String(category.count || 0)) + " channels</div>";
        }).join("") : "<div>No admin categories available yet.</div>";
      }
      function ensureSelectedCustomGroup() {
        const groups = customGroups();
        if (groups.some(function(group) { return group.id === state.selectedCustomGroup; })) return;
        state.selectedCustomGroup = groups.length ? groups[0].id : "";
      }
      function selectedCustomGroup() {
        ensureSelectedCustomGroup();
        return customGroups().find(function(group) { return group.id === state.selectedCustomGroup; }) || null;
      }
      function renderCustomGroupSettings() {
        const root = byId("custom-group-settings");
        const groups = customGroups();
        const selected = selectedCustomGroup();
        const memberships = selected ? customMemberships(selected.id) : [];
        const query = lower(state.customGroupQuery);
        const availableChannels = items(state.app.channels).filter(function(channel) {
          if (selected && memberships.indexOf(channel.id) !== -1) return false;
          if (!query) return true;
          return lower(channel.name || channel.id).indexOf(query) !== -1 || lower(sourceCategoryLabel(channel)).indexOf(query) !== -1;
        });
        root.innerHTML = "<div class=\"settings-row\"><span>New group</span><input id=\"custom-group-name\" placeholder=\"Spanish\"><button data-custom-group-action=\"create\">Create</button></div>"
          + (groups.length ? "<div class=\"settings-row\"><span>Edit group</span><select id=\"custom-group-select\">" + groups.map(function(group) { return "<option value=\"" + escapeHTML(group.id) + "\"" + (selected && selected.id === group.id ? " selected" : "") + ">" + escapeHTML(group.name) + "</option>"; }).join("") + "</select><button data-custom-group-action=\"delete\">Delete</button></div>" : "<div class=\"empty\">Create a group to build your own channel lineup.</div>")
          + (selected ? "<div class=\"settings-row\"><span>Find channel</span><input id=\"custom-group-channel-search\" placeholder=\"Name or category\" value=\"" + escapeHTML(state.customGroupQuery) + "\"></div>"
          + "<div class=\"settings-row\"><span>Add channel</span><select id=\"custom-group-channel\">" + availableChannels.slice(0, 80).map(function(channel) { return "<option value=\"" + escapeHTML(channel.id) + "\">" + escapeHTML(channel.name || channel.id) + "</option>"; }).join("") + "</select><button data-custom-group-action=\"add-channel\">Add</button></div>"
          + "<div class=\"settings-note\">" + escapeHTML(availableChannels.length ? "Showing " + Math.min(availableChannels.length, 80) + " of " + availableChannels.length + " matching channels." : "No matching channels.") + "</div>"
          + "<div class=\"settings-preview\">" + (memberships.length ? memberships.map(function(id) { const channel = channelByID(id); return "<div>" + escapeHTML((channel && channel.name) || id) + " <button data-custom-group-action=\"remove-channel\" data-channel-id=\"" + escapeHTML(id) + "\">Remove</button></div>"; }).join("") : "<div>No channels in this group yet.</div>") + "</div>" : "");
      }
      function slug(value) {
        return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 48) || "group";
      }
      function createCustomGroup(name) {
        name = String(name || "").trim();
        if (!name) return;
        const base = "group:" + slug(name);
        let id = base;
        let index = 2;
        while (customGroups().some(function(group) { return group.id === id; })) id = base + "-" + index++;
        state.app.preferences.customGroups.push({ id: id, name: name, order: customGroups().length + 1 });
        state.app.preferences.customGroupMemberships[id] = [];
        state.selectedCustomGroup = id;
        savePrefs();
        render();
      }
      function deleteSelectedCustomGroup() {
        const selected = selectedCustomGroup();
        if (!selected) return;
        state.app.preferences.customGroups = customGroups().filter(function(group) { return group.id !== selected.id; });
        delete state.app.preferences.customGroupMemberships[selected.id];
        if (state.category === customCategoryID(selected.id)) state.category = "";
        state.selectedCustomGroup = "";
        savePrefs();
        render();
      }
      function addChannelToSelectedGroup(channelID) {
        const selected = selectedCustomGroup();
        if (!selected || !channelID) return;
        state.app.preferences.customGroupMemberships[selected.id] = uniqueIDs(customMemberships(selected.id).concat([channelID]));
        savePrefs();
        render();
      }
      function removeChannelFromSelectedGroup(channelID) {
        const selected = selectedCustomGroup();
        if (!selected || !channelID) return;
        state.app.preferences.customGroupMemberships[selected.id] = customMemberships(selected.id).filter(function(id) { return id !== channelID; });
        savePrefs();
        render();
      }
      function colorClass(index) {
        return ["purple", "green", "red", "gray", "blue"][index % 5];
      }
      function audioTrackList() {
        const video = byId("player");
        if (!video || !video.audioTracks || typeof video.audioTracks.length !== "number") return [];
        const tracks = [];
        for (let index = 0; index < video.audioTracks.length; index++) tracks.push(video.audioTracks[index]);
        return tracks;
      }
      function audioTrackName(track, index) {
        return track && (track.label || track.language || track.kind || track.id) ? (track.label || track.language || track.kind || track.id) : "Audio " + (index + 1);
      }
      function textTrackList() {
        const video = byId("player");
        if (!video || !video.textTracks || typeof video.textTracks.length !== "number") return [];
        const tracks = [];
        for (let index = 0; index < video.textTracks.length; index++) {
          const track = video.textTracks[index];
          if (!track || (track.kind && ["subtitles", "captions"].indexOf(track.kind) === -1)) continue;
          tracks.push(track);
        }
        return tracks;
      }
      function textTrackName(track, index) {
        return track && (track.label || track.language || track.kind || track.id) ? (track.label || track.language || track.kind || track.id) : "Subtitles " + (index + 1);
      }
      function updateSubtitlesButton() {
        const button = byId("player-subtitles-button");
        if (!button) return;
        const tracks = textTrackList();
        const activeIndex = tracks.findIndex(function(track) { return track.mode === "showing"; });
        if (activeIndex >= 0) state.selectedTextTrack = activeIndex;
        button.classList.toggle("active", activeIndex >= 0);
        button.setAttribute("aria-pressed", activeIndex >= 0 ? "true" : "false");
        button.setAttribute("aria-label", activeIndex >= 0 ? "Subtitles: " + textTrackName(tracks[activeIndex], activeIndex) : "Subtitles");
      }
      function toggleSubtitles() {
        const tracks = textTrackList();
        closePlayerPopovers();
        if (!tracks.length) {
          showPlayerToast("No subtitles are available for this stream.");
          updateSubtitlesButton();
          return;
        }
        const activeIndex = tracks.findIndex(function(track) { return track.mode === "showing"; });
        const nextIndex = activeIndex >= 0 && activeIndex < tracks.length - 1 ? activeIndex + 1 : (activeIndex >= 0 ? -1 : Math.max(0, state.selectedTextTrack));
        tracks.forEach(function(track, index) {
          track.mode = index === nextIndex ? "showing" : "disabled";
        });
        state.selectedTextTrack = nextIndex;
        updateSubtitlesButton();
        showPlayerToast(nextIndex >= 0 ? "Subtitles: " + textTrackName(tracks[nextIndex], nextIndex) : "Subtitles off.");
      }
      function updateAudioMenu() {
        const button = byId("player-audio-button");
        const languageButton = byId("player-language-button");
        const menu = byId("player-audio-menu");
        if (!button || !menu) return;
        const tracks = audioTrackList();
        const activeIndex = tracks.findIndex(function(track) { return !!track.enabled; });
        state.selectedAudioTrack = activeIndex >= 0 ? activeIndex : state.selectedAudioTrack;
        const activeLabel = tracks.length ? audioTrackName(tracks[state.selectedAudioTrack] || tracks[0], state.selectedAudioTrack || 0) : "Default audio";
        button.innerHTML = icon("language") + "<span>" + escapeHTML(activeLabel) + "</span>" + icon("chevron-down");
        button.setAttribute("aria-expanded", state.audioMenuOpen ? "true" : "false");
        if (languageButton) {
          languageButton.classList.toggle("active", state.audioMenuOpen && tracks.length > 1);
          languageButton.setAttribute("aria-expanded", state.audioMenuOpen && tracks.length > 1 ? "true" : "false");
          languageButton.setAttribute("aria-label", tracks.length > 1 ? "Audio language: " + activeLabel : "Audio language");
        }
        menu.classList.toggle("open", state.audioMenuOpen);
        updatePlayerChrome();
        menu.innerHTML = tracks.length ? tracks.map(function(track, index) {
          return "<button type=\"button\" role=\"menuitem\" data-player-action=\"audio-track\" data-audio-index=\"" + index + "\" class=\"" + (index === state.selectedAudioTrack ? "active" : "") + "\">" + escapeHTML(audioTrackName(track, index)) + "</button>";
        }).join("") : "<button type=\"button\" role=\"menuitem\" class=\"active\" data-player-action=\"audio-track\" data-audio-index=\"0\">Default audio</button>";
      }
      function toggleLanguageMenu() {
        const tracks = audioTrackList();
        if (tracks.length <= 1) {
          closePlayerPopovers();
          showPlayerToast("No alternate audio languages are available for this stream.");
          return;
        }
        state.audioMenuOpen = !state.audioMenuOpen;
        closePlayerPopovers("audio");
        updateAudioMenu();
      }
      function selectAudioTrack(index) {
        const tracks = audioTrackList();
        if (!tracks.length) {
          state.selectedAudioTrack = 0;
          state.audioMenuOpen = false;
          updateAudioMenu();
          return;
        }
        tracks.forEach(function(track, trackIndex) { track.enabled = trackIndex === index; });
        state.selectedAudioTrack = index;
        state.audioMenuOpen = false;
        updateAudioMenu();
      }
      function volumeLabel() {
        if (state.muted || state.volume <= 0) return "0%";
        return Math.round(state.volume * 100) + "%";
      }
      function applyVolumeToVideo() {
        const video = byId("player");
        state.volume = Math.max(0, Math.min(1, Number(state.volume) || 0));
        state.muted = state.volume <= 0;
        if (video) {
          video.volume = state.volume;
          video.muted = state.muted;
        }
        updateVolumeMenu();
      }
      function updateVolumeMenu() {
        const button = byId("player-volume-button");
        const popover = byId("player-volume-popover");
        const slider = byId("player-volume-slider");
        const value = byId("player-volume-value");
        if (!button || !popover) return;
        button.innerHTML = icon(state.muted || state.volume <= 0 ? "speaker-off" : "speaker");
        button.setAttribute("aria-expanded", state.volumeMenuOpen ? "true" : "false");
        popover.classList.toggle("open", state.volumeMenuOpen);
        if (slider) slider.value = String(Math.round(state.volume * 100));
        if (value) value.textContent = volumeLabel();
        updatePlayerChrome();
      }
      function closePlayerPopovers(except) {
        if (except !== "audio") state.audioMenuOpen = false;
        if (except !== "volume") state.volumeMenuOpen = false;
        if (except !== "more") state.moreMenuOpen = false;
        updateAudioMenu();
        updateVolumeMenu();
        renderPlayerMoreMenu();
      }
      function showPlayerToast(message) {
        const toast = byId("player-toast");
        if (!toast) return;
        toast.textContent = message;
        toast.classList.add("show");
        clearTimeout(state.toastTimer);
        state.toastTimer = setTimeout(function() { toast.classList.remove("show"); }, 2400);
      }
      function showAppToast(message) {
        let toast = byId("app-toast");
        if (!toast) {
          toast = document.createElement("div");
          toast.id = "app-toast";
          toast.className = "app-toast";
          toast.setAttribute("role", "status");
          document.body.appendChild(toast);
        }
        toast.textContent = message;
        toast.classList.add("show");
        clearTimeout(state.appToastTimer);
        state.appToastTimer = setTimeout(function() { toast.classList.remove("show"); }, 2600);
      }
      async function openCastPicker() {
        const video = byId("player");
        if (!video) return;
        closePlayerPopovers();
        try {
          if (typeof video.webkitShowPlaybackTargetPicker === "function") {
            video.webkitShowPlaybackTargetPicker();
            return;
          }
          if (video.remote && typeof video.remote.prompt === "function") {
            await video.remote.prompt();
            return;
          }
          showPlayerToast("AirPlay or Cast is not available in this browser.");
        } catch (error) {
          showPlayerToast("No playback target selected.");
        }
      }
      async function togglePictureInPicture() {
        const video = byId("player");
        if (!video) return;
        closePlayerPopovers();
        if (!document.pictureInPictureEnabled || typeof video.requestPictureInPicture !== "function") {
          showPlayerToast("Picture in Picture is not available in this browser.");
          return;
        }
        try {
          if (document.pictureInPictureElement) await document.exitPictureInPicture();
          else await video.requestPictureInPicture();
        } catch (error) {
          showPlayerToast("Picture in Picture could not be opened.");
        }
      }
      function updateCenterPlayButton() {
        const video = byId("player");
        const button = byId("player-center-button");
        if (!video || !button) return;
        const loading = !!state.playerWaiting && !video.paused;
        const show = loading || video.paused;
        button.classList.toggle("hidden", !show);
        button.classList.toggle("loading", loading);
        button.innerHTML = loading ? icon("loader") : icon(video.paused ? "play" : "pause");
        button.setAttribute("aria-label", loading ? "Loading stream" : (video.paused ? "Play" : "Pause"));
        button.disabled = loading;
      }
      function togglePlayPause() {
        const video = byId("player");
        if (!video) return;
        closePlayerPopovers();
        if (video.paused) video.play().catch(function() { showPlayerToast("Playback could not be started."); });
        else video.pause();
        updateCenterPlayButton();
      }
      function fullscreenElement() {
        return document.fullscreenElement || document.webkitFullscreenElement || null;
      }
      function updateFullscreenButton() {
        const button = byId("player-fullscreen-button");
        if (!button) return;
        const active = !!fullscreenElement();
        button.innerHTML = icon(active ? "fullscreen-exit" : "fullscreen");
        button.classList.toggle("active", active);
        button.setAttribute("aria-pressed", active ? "true" : "false");
        button.setAttribute("aria-label", active ? "Exit fullscreen" : "Fullscreen");
        renderPlayerMoreMenu();
      }
      async function toggleFullscreen() {
        const shell = document.querySelector(".playback-shell");
        closePlayerPopovers();
        try {
          if (fullscreenElement()) {
            if (document.exitFullscreen) await document.exitFullscreen();
            else if (document.webkitExitFullscreen) document.webkitExitFullscreen();
          } else if (shell) {
            if (shell.requestFullscreen) await shell.requestFullscreen();
            else if (shell.webkitRequestFullscreen) shell.webkitRequestFullscreen();
            else showPlayerToast("Fullscreen is not available in this browser.");
          }
        } catch (error) {
          showPlayerToast("Fullscreen could not be changed.");
        }
        updateFullscreenButton();
      }
      function setVideoSource(url) {
        const video = byId("player");
        if (!video) return;
        applyVolumeToVideo();
        state.selectedAudioTrack = 0;
        state.selectedTextTrack = -1;
        state.audioMenuOpen = false;
        state.volumeMenuOpen = false;
        state.moreMenuOpen = false;
        updateAudioMenu();
        updateSubtitlesButton();
        updateVolumeMenu();
        renderPlayerMoreMenu();
        if (video.audioTracks && video.audioTracks.addEventListener) {
          video.audioTracks.addEventListener("addtrack", updateAudioMenu);
          video.audioTracks.addEventListener("removetrack", updateAudioMenu);
          video.audioTracks.addEventListener("change", updateAudioMenu);
        }
        video.addEventListener("loadedmetadata", updateAudioMenu, { once: true });
        video.addEventListener("loadedmetadata", updateSubtitlesButton, { once: true });
        video.addEventListener("waiting", function() { state.playerWaiting = true; updateCenterPlayButton(); });
        video.addEventListener("stalled", function() { state.playerWaiting = true; updateCenterPlayButton(); });
        video.addEventListener("canplay", function() { state.playerWaiting = false; updateCenterPlayButton(); });
        video.addEventListener("playing", function() { state.playerWaiting = false; updateCenterPlayButton(); });
        video.addEventListener("pause", updateCenterPlayButton);
        video.addEventListener("play", updateCenterPlayButton);
        video.addEventListener("error", function() { state.playerWaiting = false; updateCenterPlayButton(); });
        if (video.textTracks && video.textTracks.addEventListener) {
          video.textTracks.addEventListener("addtrack", updateSubtitlesButton);
          video.textTracks.addEventListener("removetrack", updateSubtitlesButton);
          video.textTracks.addEventListener("change", updateSubtitlesButton);
        }
        if (state.hls) { state.hls.destroy(); state.hls = null; }
        if (state.tsPlayer) { state.tsPlayer.destroy(); state.tsPlayer = null; }
        const isHLS = url.indexOf(".m3u8") !== -1;
        if (window.Hls && Hls.isSupported() && isHLS) {
          state.hls = new Hls();
          state.hls.loadSource(url);
          state.hls.attachMedia(video);
        } else if (window.mpegts && mpegts.isSupported() && !isHLS) {
          state.tsPlayer = mpegts.createPlayer({ type: "mpegts", isLive: true, url: url });
          state.tsPlayer.attachMediaElement(video);
          state.tsPlayer.load();
        } else {
          video.src = url;
        }
        setTimeout(updateAudioMenu, 500);
        setTimeout(updateAudioMenu, 1800);
        setTimeout(updateSubtitlesButton, 500);
        setTimeout(updateSubtitlesButton, 1800);
        updateCenterPlayButton();
        applyAspectMode();
        video.play().then(updateCenterPlayButton).catch(function() { updateCenterPlayButton(); });
      }
      async function playChannel(channel) {
        state.currentChannel = channel;
        state.view = "player";
        render();
        setVideoSource(browserStreamURL(channel));
        startWatch(channel);
        const guide = await getJSON("/dispatcharr/api/guide?channel_id=" + encodeURIComponent(channel.id)).catch(function() { return { programs: [] }; });
        const nowGuide = byId("now-guide");
        if (nowGuide) nowGuide.innerHTML = items(guide.programs).slice(0, 6).map(function(program) { return "<div class=\"program\"><time>" + escapeHTML(timeLabel(program.startUnix)) + "</time><strong>" + escapeHTML(program.title || "Untitled") + "</strong></div>"; }).join("") || "<div class=\"empty\">No guide entries.</div>";
      }
      function startWatch(channel) {
        if (state.currentSession) postJSON("/dispatcharr/api/watch/stop", { sessionId: state.currentSession.id, reason: "switch_channel" }).catch(function() {});
        recordWatchPreference(channel);
        postJSON("/dispatcharr/api/watch/start", { itemKind: "channel", itemId: channel.id, itemName: channel.name }).then(function(payload) {
          state.currentSession = payload.session;
          if (state.heartbeat) clearInterval(state.heartbeat);
          state.heartbeat = setInterval(function() {
            if (state.currentSession) postJSON("/dispatcharr/api/watch/heartbeat", { sessionId: state.currentSession.id }).catch(function() {});
          }, 30000);
          renderRail();
        }).catch(function() {});
      }
      function handlePlayerAction(action, button) {
        const video = byId("player");
        wakePlayerChrome();
        if (action === "back") {
          setView("live");
          return;
        }
        if (action === "guide") {
          state.playerGuideOpen = !state.playerGuideOpen;
          closePlayerPopovers();
          renderPlayerGuidePanel();
          return;
        }
        if (action === "guide-close") {
          state.playerGuideOpen = false;
          renderPlayerGuidePanel();
          return;
        }
        if (action === "cast") {
          closePlayerPopovers();
          openCastPicker();
          return;
        }
        if (action === "pip") {
          togglePictureInPicture();
          return;
        }
        if (action === "play-toggle") {
          togglePlayPause();
          return;
        }
        if (action === "fullscreen") {
          toggleFullscreen();
          return;
        }
        if (action === "subtitles") {
          toggleSubtitles();
          return;
        }
        if (action === "volume-menu") {
          state.volumeMenuOpen = !state.volumeMenuOpen;
          closePlayerPopovers("volume");
          updateVolumeMenu();
          return;
        }
        if (action === "audio-menu") {
          state.audioMenuOpen = !state.audioMenuOpen;
          closePlayerPopovers("audio");
          updateAudioMenu();
          return;
        }
        if (action === "language-menu") {
          toggleLanguageMenu();
          return;
        }
        if (action === "audio-track") {
          selectAudioTrack(Number(button && button.getAttribute("data-audio-index")) || 0);
          return;
        }
        if (action === "more") {
          state.moreMenuOpen = !state.moreMenuOpen;
          closePlayerPopovers("more");
          renderPlayerMoreMenu();
          return;
        }
        if (action === "aspect") {
          state.aspectMode = state.aspectMode === "fit" ? "fill" : "fit";
          applyAspectMode();
          renderPlayerMoreMenu();
          return;
        }
        if (action === "search-channel") {
          state.moreMenuOpen = false;
          renderPlayerMoreMenu();
          const search = byId("global-search");
          if (search) search.focus();
          return;
        }
        if (action === "copy-stream") {
          const url = currentStreamURL();
          if (url && navigator.clipboard) navigator.clipboard.writeText(new URL(url, window.location.href).href).then(function() { showPlayerToast("Stream URL copied."); }).catch(function() { showPlayerToast("Could not copy stream URL."); });
          else showPlayerToast("No stream URL available.");
          state.moreMenuOpen = false;
          renderPlayerMoreMenu();
          return;
        }
        if (action === "open-stream") {
          const url = currentStreamURL();
          if (url) window.open(url, "_blank", "noopener");
          state.moreMenuOpen = false;
          renderPlayerMoreMenu();
          return;
        }
        if (action === "favorite" && state.currentChannel) {
          const id = state.currentChannel.id;
          if (favoriteMap()[id]) {
            delete state.app.preferences.favorites[id];
            state.app.preferences.favoriteOrder = items(state.app.preferences.favoriteOrder).filter(function(item) { return item !== id; });
          } else {
            state.app.preferences.favorites[id] = true;
            state.app.preferences.favoriteOrder = uniqueIDs(items(state.app.preferences.favoriteOrder).concat([id]));
          }
          if (button) {
            const isFavorite = !!favoriteMap()[id];
            button.innerHTML = icon(isFavorite ? "heart-solid" : "heart");
            button.classList.toggle("active", isFavorite);
            button.setAttribute("aria-pressed", isFavorite ? "true" : "false");
            button.setAttribute("aria-label", isFavorite ? "Remove channel from favorites" : "Favorite channel");
          }
          savePrefs();
          postJSON("/dispatcharr/api/favorites", { id: id, enabled: !!favoriteMap()[id] }).catch(function() {});
          renderRail();
        }
      }
      document.addEventListener("click", function(event) {
        const playerTarget = event.target.closest("[data-player-action]");
        if (playerTarget) {
          event.preventDefault();
          handlePlayerAction(playerTarget.getAttribute("data-player-action"), playerTarget);
          return;
        }
        const recordingsRefresh = event.target.closest("[data-recordings-refresh]");
        if (recordingsRefresh) {
          event.preventDefault();
          state.recordings = null;
          loadRecordings(true);
          renderRecordingsPage();
          return;
        }
        const guideRefresh = event.target.closest("[data-guide-refresh]");
        if (guideRefresh) {
          event.preventDefault();
          refreshAppData();
          return;
        }
        const recordingPlayback = event.target.closest("[data-recording-playback]");
        if (recordingPlayback) {
          event.preventDefault();
          const url = recordingPlayback.getAttribute("data-recording-playback");
          if (url) window.open(url, "_blank", "noopener");
          return;
        }
        const scheduleTarget = event.target.closest("[data-schedule-channel]");
        if (scheduleTarget) {
          event.preventDefault();
          event.stopPropagation();
          scheduleProgram(scheduleTarget.getAttribute("data-schedule-channel"), scheduleTarget.getAttribute("data-schedule-program"), scheduleTarget);
          return;
        }
        const customGroupAction = event.target.closest("[data-custom-group-action]");
        if (customGroupAction) {
          event.preventDefault();
          const action = customGroupAction.getAttribute("data-custom-group-action");
          if (action === "create") createCustomGroup((byId("custom-group-name") || {}).value || "");
          if (action === "delete") deleteSelectedCustomGroup();
          if (action === "add-channel") addChannelToSelectedGroup((byId("custom-group-channel") || {}).value || "");
          if (action === "remove-channel") removeChannelFromSelectedGroup(customGroupAction.getAttribute("data-channel-id"));
          return;
        }
        const adminSettingsAction = event.target.closest("[data-admin-settings-action]");
        if (adminSettingsAction) {
          event.preventDefault();
          const action = adminSettingsAction.getAttribute("data-admin-settings-action");
          if (action === "save") saveAdminCategorySettings();
          if (action === "discard") discardAdminCategorySettings();
          return;
        }
        const favoriteMove = event.target.closest("[data-favorite-move]");
        if (favoriteMove) {
          event.preventDefault();
          moveFavorite(favoriteMove.getAttribute("data-channel-id"), favoriteMove.getAttribute("data-favorite-move"));
          return;
        }
        const channelTarget = event.target.closest("[data-channel]");
        if (channelTarget) {
          const channel = channelByID(channelTarget.getAttribute("data-channel"));
          if (channel) playChannel(channel);
        }
        const categoryTarget = event.target.closest("[data-category]");
        if (categoryTarget) setCategory(categoryTarget.getAttribute("data-category"));
      });
      document.addEventListener("fullscreenchange", updateFullscreenButton);
      document.addEventListener("webkitfullscreenchange", updateFullscreenButton);
      document.addEventListener("keydown", function(event) {
        if (state.view !== "player") return;
        const tag = event.target && event.target.tagName ? event.target.tagName.toLowerCase() : "";
        if (tag === "input" || tag === "textarea" || tag === "select") return;
        if (event.key === " " || event.key === "k" || event.key === "K") {
          event.preventDefault();
          togglePlayPause();
        }
        if (event.key === "f" || event.key === "F") {
          event.preventDefault();
          toggleFullscreen();
        }
      });
      ["mousemove", "mousedown", "touchstart", "keydown"].forEach(function(eventName) {
        document.addEventListener(eventName, function(event) {
          if (state.view !== "player") return;
          if (eventName === "mousemove" && event.movementX === 0 && event.movementY === 0) return;
          wakePlayerChrome();
        }, { passive: true });
      });
      document.addEventListener("change", function(event) {
        const adminField = event.target.getAttribute("data-admin-category-field");
        if (adminField) {
          updateCategoryParsingField(adminField, event.target);
          return;
        }
        if (event.target && event.target.id === "custom-group-select") {
          state.selectedCustomGroup = event.target.value;
          renderSettings();
          return;
        }
        const id = event.target.getAttribute("data-hide");
        if (!id) return;
        if (event.target.checked) state.app.preferences.hiddenCategories[id] = true;
        else delete state.app.preferences.hiddenCategories[id];
        savePrefs();
        postJSON("/dispatcharr/api/hidden-categories", { id: id, hidden: event.target.checked }).catch(function() {});
        render();
      });
      document.addEventListener("input", function(event) {
        if (event.target && event.target.id === "player-volume-slider") {
          state.volume = Number(event.target.value || 0) / 100;
          applyVolumeToVideo();
        }
        if (event.target && event.target.id === "custom-group-channel-search") {
          state.customGroupQuery = event.target.value || "";
          renderSettings();
          const input = byId("custom-group-channel-search");
          if (input) {
            input.focus();
            input.setSelectionRange(input.value.length, input.value.length);
          }
        }
      });
      document.querySelectorAll(".nav button").forEach(function(button) {
        button.onclick = function() { setView(button.dataset.view); };
      });
      const globalSearch = byId("global-search");
      if (globalSearch) globalSearch.oninput = function(event) { state.query = event.target.value; render(); };
      window.addEventListener("scroll", function() {
        if (state.view === "guide" && isNearGuideEnd()) appendGuideRows();
      }, { passive: true });
      window.addEventListener("beforeunload", function() {
        if (state.currentSession) navigator.sendBeacon(route("/dispatcharr/api/watch/stop"), JSON.stringify({ sessionId: state.currentSession.id, reason: "page_unload" }));
      });
      loadApp().catch(function() {
        byId("view").innerHTML = "<div class=\"empty\">Unable to load Live TV app data.</div>";
      });
    </script>
  </body>
</html>`
