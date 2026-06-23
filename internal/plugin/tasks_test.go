package plugin

import (
	"context"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/app"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

func TestScheduledTaskServerRunsSyncTask(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := app.NewService(app.Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) app.XtreamClient {
			return &scheduledStubClient{
				streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}},
				epg:     xtream.ShortEPGResponse{EPGListings: []xtream.EPGListing{{ID: "epg-1", Title: "Morning News", StartTimestamp: "1700000000", StopTimestamp: "1700003600"}}},
			}
		},
	})
	server := NewScheduledTaskServer(service, config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: "https://dispatcharr.example.com", XtreamUsername: "demo", XtreamPassword: "secret", ChannelRefreshH: 24, EPGRefreshH: 6})

	response, err := server.Run(context.Background(), &pluginv1.RunScheduledTaskRequest{TaskKey: SyncTaskKey})
	if err != nil {
		t.Fatalf("run task: %v", err)
	}
	if response.GetOutput().AsMap()["status"] != "ok" {
		t.Fatalf("unexpected task output: %+v", response.GetOutput().AsMap())
	}
	if response.GetOutput().AsMap()["task"] != "catalog" {
		t.Fatalf("expected catalog task output, got %+v", response.GetOutput().AsMap())
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 || len(snapshot.Catalog.Programs) != 1 {
		t.Fatalf("expected sync to populate channels and programs, got %+v", snapshot.Catalog)
	}
}

func TestScheduledTaskServerRunsSiloNamespacedSyncTask(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := app.NewService(app.Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) app.XtreamClient {
			return &scheduledStubClient{
				streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}},
				epg:     xtream.ShortEPGResponse{EPGListings: []xtream.EPGListing{{ID: "epg-1", Title: "Morning News", StartTimestamp: "1700000000", StopTimestamp: "1700003600"}}},
			}
		},
	})
	server := NewScheduledTaskServer(service, config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: "https://dispatcharr.example.com", XtreamUsername: "demo", XtreamPassword: "secret", ChannelRefreshH: 24, EPGRefreshH: 6})

	if _, err := server.Run(context.Background(), &pluginv1.RunScheduledTaskRequest{TaskKey: "plugin:14:dispatcharr-sync"}); err != nil {
		t.Fatalf("run namespaced task: %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 || len(snapshot.Catalog.Programs) != 1 {
		t.Fatalf("expected namespaced task to populate channels and programs, got %+v", snapshot.Catalog)
	}
}

func TestScheduledTaskServerRunsChannelRefreshTask(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := app.NewService(app.Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) app.XtreamClient {
			return &scheduledStubClient{
				streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}},
				epg:     xtream.ShortEPGResponse{EPGListings: []xtream.EPGListing{{ID: "epg-1", Title: "Morning News", StartTimestamp: "1700000000", StopTimestamp: "1700003600"}}},
			}
		},
	})
	server := NewScheduledTaskServer(service, config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: "https://dispatcharr.example.com", XtreamUsername: "demo", XtreamPassword: "secret", ChannelRefreshH: 24, EPGRefreshH: 6})

	if _, err := server.Run(context.Background(), &pluginv1.RunScheduledTaskRequest{TaskKey: ChannelRefreshTaskKey}); err != nil {
		t.Fatalf("run channel refresh task: %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 {
		t.Fatalf("expected channel refresh to populate channels, got %+v", snapshot.Catalog)
	}
}

func TestScheduledTaskServerRunsEPGRefreshTask(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{
		Source:   model.LiveTVSource(model.SourceModeM3UXMLTV),
		Channels: []model.Channel{{ID: "m3u:news-hd", Name: "News HD", GuideID: "news.hd"}},
		Health:   model.SyncHealth{LastSuccessUnix: 100},
	}})
	service := app.NewService(app.Dependencies{
		Store: store,
		FetchURL: func(_ context.Context, rawURL string) ([]byte, error) {
			if rawURL != "https://dispatcharr.example.com/guide.xml" {
				t.Fatalf("unexpected epg url %q", rawURL)
			}
			return []byte("<?xml version=\"1.0\"?><tv><programme start=\"20260619070000 +0000\" stop=\"20260619080000 +0000\" channel=\"news.hd\"><title>Morning News</title></programme></tv>"), nil
		},
	})
	server := NewScheduledTaskServer(service, config.Settings{SourceMode: config.SourceModeM3UXMLTV, M3UURL: "https://dispatcharr.example.com/playlist.m3u", EPGXMLURL: "https://dispatcharr.example.com/guide.xml", ChannelRefreshH: 24, EPGRefreshH: 6})

	response, err := server.Run(context.Background(), &pluginv1.RunScheduledTaskRequest{TaskKey: "plugin:14:" + EPGRefreshTaskKey})
	if err != nil {
		t.Fatalf("run epg refresh task: %v", err)
	}
	if response.GetOutput().AsMap()["task"] != "epg" {
		t.Fatalf("expected epg task output, got %+v", response.GetOutput().AsMap())
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Programs) != 1 || snapshot.Catalog.Programs[0].Title != "Morning News" {
		t.Fatalf("expected epg refresh to populate guide programs, got %+v", snapshot.Catalog.Programs)
	}
	if snapshot.Health.EPGStatus != "ok" {
		t.Fatalf("expected epg health to be ok, got %+v", snapshot.Health)
	}
}

type scheduledStubClient struct {
	streams []xtream.LiveStream
	epg     xtream.ShortEPGResponse
}

func (s *scheduledStubClient) TestConnection(context.Context) error { return nil }
func (s *scheduledStubClient) LiveStreams(context.Context) ([]xtream.LiveStream, error) {
	return s.streams, nil
}
func (s *scheduledStubClient) ShortEPG(context.Context, int64) (xtream.ShortEPGResponse, error) {
	return s.epg, nil
}
func (s *scheduledStubClient) ResolveLiveStreamURL(streamID int64) string {
	return "https://dispatcharr.example.com/live/demo/secret/" + "1001.m3u8"
}
