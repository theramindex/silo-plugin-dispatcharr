package plugin

import (
	"context"
	"encoding/json"
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
	if !payload.Capabilities.LiveTV || payload.Capabilities.NativeLiveTVExport {
		t.Fatalf("unexpected capabilities: %+v", payload.Capabilities)
	}
	if len(payload.Categories) != 1 || len(payload.Channels) != 1 {
		t.Fatalf("unexpected app payload: %+v", payload)
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

func TestHTTPRoutesServerPreferencesRoutePersistsFullPayload(t *testing.T) {
	t.Parallel()

	server := NewHTTPRoutesServer(cache.NewStore())
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: "POST",
		Path:   "/dispatcharr/api/preferences",
		Body:   []byte(`{"favorites":{"channel:1":true},"autoFavorites":{"channel:2":true},"hiddenCategories":{"sports":true},"recentChannels":["channel:1"],"continueWatching":{"channel:1":{"plays":3}},"playback":{"streamMode":"redirect","outputFormat":"hls"}}`),
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
	if !strings.Contains(string(response.GetBody()), "<video") {
		t.Fatalf("expected player html body")
	}
	if !strings.Contains(string(response.GetBody()), `href="/" aria-label="Back to Silo"`) {
		t.Fatalf("expected back to Silo link")
	}
}
