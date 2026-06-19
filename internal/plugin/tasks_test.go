package plugin

import (
	"context"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/app"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
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
