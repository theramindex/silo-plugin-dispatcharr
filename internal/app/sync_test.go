package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/dispatcharr"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

func TestSyncStoresChannelsAndPrograms(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := NewService(Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) XtreamClient {
			return &stubXtreamClient{
				streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}},
				epg:     xtream.ShortEPGResponse{EPGListings: []xtream.EPGListing{{ID: "epg-1", Title: "Morning News", StartTimestamp: "1700000000", StopTimestamp: "1700003600"}}},
			}
		},
	})

	err := service.SyncNow(context.Background(), config.Settings{
		SourceMode:      config.SourceModeXtream,
		XtreamBaseURL:   "https://dispatcharr.example.com",
		XtreamUsername:  "demo",
		XtreamPassword:  "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     6,
	}, 200)
	if err != nil {
		t.Fatalf("expected sync success, got %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(snapshot.Catalog.Channels))
	}
	if len(snapshot.Catalog.Programs) != 1 {
		t.Fatalf("expected 1 program, got %d", len(snapshot.Catalog.Programs))
	}
	if snapshot.Health.LastSuccessUnix != 200 {
		t.Fatalf("expected sync success timestamp, got %d", snapshot.Health.LastSuccessUnix)
	}
}

func TestSyncKeepsStaleSnapshotOnFailure(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{})

	service := NewService(Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) XtreamClient {
			return &stubXtreamClient{streamsErr: context.DeadlineExceeded}
		},
	})

	err := service.SyncNow(context.Background(), config.Settings{
		SourceMode:      config.SourceModeXtream,
		XtreamBaseURL:   "https://dispatcharr.example.com",
		XtreamUsername:  "demo",
		XtreamPassword:  "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     6,
	}, 300)
	if err == nil {
		t.Fatal("expected sync error")
	}

	snapshot := store.Current()
	if snapshot.Health.LastFailureUnix != 300 {
		t.Fatalf("expected failure timestamp, got %d", snapshot.Health.LastFailureUnix)
	}
}

func TestSyncDispatcharrRESTBuildsCatalog(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := NewService(Dependencies{
		Store: store,
		DispatcharrFactory: func(config.Settings) DispatcharrClient {
			return &stubDispatcharrClient{
				channels: []dispatcharr.Channel{{
					ID:                     "1",
					UUID:                   "11111111-1111-1111-1111-111111111111",
					Name:                   "Provider Name",
					EffectiveName:          "News HD",
					EffectiveChannelNumber: "5.1",
					EffectiveTVGID:         "news.hd",
					EffectiveGroupID:       "10",
				}},
				groups: []dispatcharr.ChannelGroup{{ID: "10", Name: "Local"}},
				programs: []dispatcharr.Program{{
					ID:          "epg-1",
					TVGID:       "news.hd",
					Title:       "Morning News",
					Description: "Top headlines.",
					StartTime:   "2026-06-18T12:00:00Z",
					EndTime:     "2026-06-18T13:00:00Z",
				}},
				vodCategories: []dispatcharr.VODCategory{{ID: "movies", Name: "Movies", CategoryType: "movie"}, {ID: "shows", Name: "Shows", CategoryType: "series"}},
				movies:        []dispatcharr.Movie{{UUID: "22222222-2222-2222-2222-222222222222", Name: "Movie One", CategoryID: "movies"}},
				series:        []dispatcharr.Series{{UUID: "33333333-3333-3333-3333-333333333333", Name: "Series One", CategoryID: "shows"}},
			}
		},
	})

	err := service.SyncNow(context.Background(), config.Settings{
		SourceMode:      config.SourceModeDirectLogin,
		DispatcharrURL:  "https://dispatcharr.example.com",
		DispatcharrUser: "demo",
		DispatcharrPass: "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     24,
	}, 500)
	if err != nil {
		t.Fatalf("expected dispatcharr sync success, got %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 || snapshot.Catalog.Channels[0].Name != "News HD" {
		t.Fatalf("unexpected dispatcharr channels: %+v", snapshot.Catalog.Channels)
	}
	if len(snapshot.Catalog.Programs) != 1 || snapshot.Catalog.Programs[0].ChannelID != snapshot.Catalog.Channels[0].ID {
		t.Fatalf("unexpected dispatcharr programs: %+v", snapshot.Catalog.Programs)
	}
	if len(snapshot.Catalog.Content.VODItems) != 1 || len(snapshot.Catalog.Content.SeriesItems) != 1 {
		t.Fatalf("unexpected dispatcharr content: %+v", snapshot.Catalog.Content)
	}
}

func TestSyncDirectLoginFallsBackToXtream(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := NewService(Dependencies{
		Store: store,
		DispatcharrFactory: func(config.Settings) DispatcharrClient {
			return &stubDispatcharrClient{channelsErr: errors.New("dispatcharr login status 405")}
		},
		XtreamFactory: func(baseURL, username, password string) XtreamClient {
			if baseURL != "https://dispatcharr.example.com" || username != "demo" || password != "secret" {
				t.Fatalf("unexpected fallback credentials: %q %q", baseURL, username)
			}
			return &stubXtreamClient{
				streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}},
				epg:     xtream.ShortEPGResponse{EPGListings: []xtream.EPGListing{{ID: "epg-1", Title: "Morning News", StartTimestamp: "1700000000", StopTimestamp: "1700003600"}}},
			}
		},
	})

	err := service.SyncNow(context.Background(), config.Settings{
		SourceMode:      config.SourceModeDirectLogin,
		DispatcharrURL:  "https://dispatcharr.example.com",
		DispatcharrUser: "demo",
		DispatcharrPass: "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     24,
	}, 600)
	if err != nil {
		t.Fatalf("expected direct login fallback sync success, got %v", err)
	}

	snapshot := store.Current()
	if snapshot.Catalog.Source.Mode != model.SourceModeDirectLogin {
		t.Fatalf("expected direct login source mode, got %q", snapshot.Catalog.Source.Mode)
	}
	if len(snapshot.Catalog.Channels) != 1 || len(snapshot.Catalog.Programs) != 1 {
		t.Fatalf("unexpected fallback snapshot: %+v", snapshot)
	}
	if snapshot.Health.LastFailureUnix != 0 || snapshot.Health.LastError != "" {
		t.Fatalf("expected fallback success to clear transient REST failure, got %+v", snapshot.Health)
	}
	if snapshot.Health.LastSuccessUnix != 600 {
		t.Fatalf("expected fallback success timestamp, got %d", snapshot.Health.LastSuccessUnix)
	}
}

func TestSyncXtreamSkipsPerChannelEPGWithTightDeadline(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := NewService(Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) XtreamClient {
			return &stubXtreamClient{
				streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}},
				epgErr:  errors.New("short epg should not be called"),
			}
		},
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	err := service.SyncNow(ctx, config.Settings{
		SourceMode:      config.SourceModeXtream,
		XtreamBaseURL:   "https://dispatcharr.example.com",
		XtreamUsername:  "demo",
		XtreamPassword:  "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     24,
	}, 700)
	if err != nil {
		t.Fatalf("expected tight-deadline sync success, got %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 {
		t.Fatalf("expected channels under tight deadline, got %+v", snapshot.Catalog.Channels)
	}
	if len(snapshot.Catalog.Programs) != 0 {
		t.Fatalf("expected no eager EPG under tight deadline, got %+v", snapshot.Catalog.Programs)
	}
}

func TestSyncM3UXMLTVBuildsFallbackCatalog(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := NewService(Dependencies{Store: store, FetchURL: func(_ context.Context, rawURL string) ([]byte, error) {
		switch rawURL {
		case "https://dispatcharr.example.com/playlist.m3u":
			return []byte("#EXTM3U\n#EXTINF:-1 tvg-id=\"news.hd\",News HD\nhttps://dispatcharr.example.com/live/news-hd.m3u8\n"), nil
		case "https://dispatcharr.example.com/guide.xml":
			return []byte("<?xml version=\"1.0\"?><tv><channel id=\"news.hd\"><display-name>News HD</display-name></channel><programme start=\"20231114221320 +0000\" stop=\"20231114231320 +0000\" channel=\"news.hd\"><title>Morning News</title><desc>Top headlines.</desc></programme></tv>"), nil
		default:
			return nil, context.DeadlineExceeded
		}
	}})

	err := service.SyncNow(context.Background(), config.Settings{SourceMode: config.SourceModeM3UXMLTV, M3UURL: "https://dispatcharr.example.com/playlist.m3u", EPGXMLURL: "https://dispatcharr.example.com/guide.xml", ChannelRefreshH: 24, EPGRefreshH: 6}, 400)
	if err != nil {
		t.Fatalf("expected fallback sync success, got %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 || len(snapshot.Catalog.Programs) != 1 {
		t.Fatalf("unexpected fallback snapshot: %+v", snapshot)
	}
}

type stubDispatcharrClient struct {
	testErr       error
	channels      []dispatcharr.Channel
	channelsErr   error
	groups        []dispatcharr.ChannelGroup
	programs      []dispatcharr.Program
	vodCategories []dispatcharr.VODCategory
	movies        []dispatcharr.Movie
	series        []dispatcharr.Series
}

func (s *stubDispatcharrClient) TestConnection(context.Context) error { return s.testErr }
func (s *stubDispatcharrClient) Channels(context.Context) ([]dispatcharr.Channel, error) {
	if s.channelsErr != nil {
		return nil, s.channelsErr
	}
	return s.channels, nil
}
func (s *stubDispatcharrClient) ChannelGroups(context.Context) ([]dispatcharr.ChannelGroup, error) {
	return s.groups, nil
}
func (s *stubDispatcharrClient) Programs(context.Context) ([]dispatcharr.Program, error) {
	return s.programs, nil
}
func (s *stubDispatcharrClient) VODCategories(context.Context) ([]dispatcharr.VODCategory, error) {
	return s.vodCategories, nil
}
func (s *stubDispatcharrClient) Movies(context.Context) ([]dispatcharr.Movie, error) {
	return s.movies, nil
}
func (s *stubDispatcharrClient) Series(context.Context) ([]dispatcharr.Series, error) {
	return s.series, nil
}
func (s *stubDispatcharrClient) LiveStreamURL(channelUUID string) string {
	return "https://dispatcharr.example.com/proxy/ts/stream/" + channelUUID
}
func (s *stubDispatcharrClient) MovieStreamURL(movieUUID string) string {
	return "https://dispatcharr.example.com/proxy/vod/movie/" + movieUUID
}
func (s *stubDispatcharrClient) SeriesStreamURL(seriesUUID string) string {
	return "https://dispatcharr.example.com/proxy/vod/series/" + seriesUUID
}
func (s *stubDispatcharrClient) AbsoluteURL(raw string) string { return raw }
