package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestSyncXtreamUsesCustomXMLTVGuide(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	service := NewService(Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) XtreamClient {
			return &stubXtreamClient{
				streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}},
				epgErr:  errors.New("short epg should not be called when custom xmltv has programs"),
			}
		},
		FetchURL: func(_ context.Context, rawURL string) ([]byte, error) {
			if rawURL != "https://dispatcharr.example.com/xmltv.xml" {
				return nil, errors.New("unexpected xmltv url")
			}
			return []byte(`<?xml version="1.0"?><tv><programme start="20260619070000 +0000" stop="20260619080000 +0000" channel="news.hd"><title>Morning News</title><desc>Top headlines.</desc></programme></tv>`), nil
		},
	})

	err := service.SyncNow(context.Background(), config.Settings{
		SourceMode:      config.SourceModeXtream,
		XtreamBaseURL:   "https://dispatcharr.example.com",
		XtreamUsername:  "demo",
		XtreamPassword:  "secret",
		EPGXMLURL:       "https://dispatcharr.example.com/xmltv.xml",
		ChannelRefreshH: 24,
		EPGRefreshH:     6,
	}, 250)
	if err != nil {
		t.Fatalf("expected sync success, got %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Programs) != 1 || snapshot.Catalog.Programs[0].Title != "Morning News" {
		t.Fatalf("expected custom xmltv program, got %+v", snapshot.Catalog.Programs)
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
					EffectiveChannelNumber: "12",
					EffectiveTVGID:         "news.hd",
					EffectiveGroupID:       "10",
					EffectiveLogoID:        "99",
				}, {
					ID:                     "2",
					UUID:                   "44444444-4444-4444-4444-444444444444",
					Name:                   "Provider Two",
					EffectiveName:          "Local Five",
					EffectiveChannelNumber: "5.1",
					EffectiveTVGID:         "local.five",
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
				searchPrograms: []dispatcharr.ProgramSearchResult{{
					Program: dispatcharr.Program{
						ID:          "epg-2",
						Title:       "International Desk",
						Description: "Global headlines.",
						StartTime:   "2026-06-18T13:00:00Z",
						EndTime:     "2026-06-18T14:00:00Z",
					},
					Channels: []dispatcharr.ProgramChannel{{ID: "2"}},
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
	if len(snapshot.Catalog.Channels) != 2 || snapshot.Catalog.Channels[0].Name != "Local Five" || snapshot.Catalog.Channels[1].Name != "News HD" {
		t.Fatalf("unexpected dispatcharr channels: %+v", snapshot.Catalog.Channels)
	}
	if snapshot.Catalog.Channels[1].LogoURL != "https://dispatcharr.example.com/api/channels/logos/99/cache/" {
		t.Fatalf("expected logo cache url from effective logo id, got %q", snapshot.Catalog.Channels[1].LogoURL)
	}
	if len(snapshot.Catalog.Programs) != 2 || snapshot.Catalog.Programs[0].ChannelID != snapshot.Catalog.Channels[1].ID || snapshot.Catalog.Programs[1].ChannelID != snapshot.Catalog.Channels[0].ID {
		t.Fatalf("unexpected dispatcharr programs: %+v", snapshot.Catalog.Programs)
	}
	if len(snapshot.Catalog.Content.VODItems) != 1 || len(snapshot.Catalog.Content.SeriesItems) != 1 {
		t.Fatalf("unexpected dispatcharr content: %+v", snapshot.Catalog.Content)
	}
}

func TestSyncDirectLoginDoesNotFallbackToXtream(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	xtreamCalls := 0
	service := NewService(Dependencies{
		Store: store,
		DispatcharrFactory: func(config.Settings) DispatcharrClient {
			return &stubDispatcharrClient{channelsErr: errors.New("dispatcharr login status 405")}
		},
		XtreamFactory: func(baseURL, username, password string) XtreamClient {
			xtreamCalls++
			return &stubXtreamClient{}
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
	if err == nil {
		t.Fatal("expected direct login REST sync failure")
	}
	if xtreamCalls != 0 {
		t.Fatalf("expected no xtream fallback calls, got %d", xtreamCalls)
	}
	snapshot := store.Current()
	if snapshot.Health.LastFailureUnix != 600 || snapshot.Health.LastError == "" {
		t.Fatalf("expected direct failure to be recorded, got %+v", snapshot.Health)
	}
	if snapshot.Health.LastSuccessUnix != 0 {
		t.Fatalf("expected no direct success timestamp, got %d", snapshot.Health.LastSuccessUnix)
	}
}

func TestSyncDispatcharrSkipsVODWithTightDeadline(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	client := &stubDispatcharrClient{
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
			ID:        "epg-1",
			TVGID:     "news.hd",
			Title:     "Morning News",
			StartTime: "2026-06-18T12:00:00Z",
			EndTime:   "2026-06-18T13:00:00Z",
		}},
	}
	service := NewService(Dependencies{
		Store: store,
		DispatcharrFactory: func(config.Settings) DispatcharrClient {
			return client
		},
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	err := service.SyncNow(ctx, config.Settings{
		SourceMode:      config.SourceModeDirectLogin,
		DispatcharrURL:  "https://dispatcharr.example.com",
		DispatcharrUser: "demo",
		DispatcharrPass: "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     24,
	}, 650)
	if err != nil {
		t.Fatalf("expected tight-deadline dispatcharr sync success, got %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 1 || len(snapshot.Catalog.Programs) != 1 {
		t.Fatalf("expected live catalog under tight deadline, got %+v", snapshot.Catalog)
	}
	if client.vodCalls != 0 {
		t.Fatalf("expected no VOD calls under tight deadline, got %d", client.vodCalls)
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

func TestRefreshEPGStoresXMLTVPrograms(t *testing.T) {
	t.Parallel()

	xmltvDoc := `<?xml version="1.0"?><tv><programme start="20260619070000 +0000" stop="20260619080000 +0000" channel="2"><title>Morning News</title><desc>Top headlines.</desc></programme><programme start="20260619080000 +0000" stop="20260619090000 +0000" channel="missing"><title>Ignored</title></programme></tv>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xmltv.php" {
			t.Fatalf("unexpected epg path %q", r.URL.Path)
		}
		if r.URL.Query().Get("username") != "demo" || r.URL.Query().Get("password") != "secret" {
			t.Fatal("missing epg credentials")
		}
		w.Header().Set("content-type", "application/xml")
		_, _ = w.Write([]byte(xmltvDoc))
	}))
	defer server.Close()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{
		Source:   model.LiveTVSource(model.SourceModeDirectLogin),
		Channels: []model.Channel{{ID: "xtream:1590", Name: "WCBS CBS", GuideID: "2"}},
		Health:   model.SyncHealth{LastSuccessUnix: 100},
	}})
	service := NewService(Dependencies{Store: store})

	if err := service.refreshEPG(context.Background(), config.Settings{SourceMode: config.SourceModeDirectLogin, DispatcharrURL: server.URL, DispatcharrUser: "demo", DispatcharrPass: "secret"}, 800); err != nil {
		t.Fatalf("refresh epg: %v", err)
	}

	snapshot := store.Current()
	if len(snapshot.Catalog.Programs) != 1 {
		t.Fatalf("expected 1 matched program, got %+v", snapshot.Catalog.Programs)
	}
	if snapshot.Catalog.Programs[0].ChannelID != "xtream:1590" || snapshot.Catalog.Programs[0].Title != "Morning News" {
		t.Fatalf("unexpected program mapping: %+v", snapshot.Catalog.Programs[0])
	}
	if snapshot.Health.EPGStatus != "ok" || snapshot.Health.EPGProgramCount != 1 || snapshot.Health.EPGLastSuccessUnix != 800 {
		t.Fatalf("unexpected epg health: %+v", snapshot.Health)
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
	testErr        error
	channels       []dispatcharr.Channel
	channelsErr    error
	groups         []dispatcharr.ChannelGroup
	programs       []dispatcharr.Program
	searchPrograms []dispatcharr.ProgramSearchResult
	vodCategories  []dispatcharr.VODCategory
	movies         []dispatcharr.Movie
	series         []dispatcharr.Series
	vodCalls       int
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
func (s *stubDispatcharrClient) SearchPrograms(context.Context, time.Time, time.Time) ([]dispatcharr.ProgramSearchResult, error) {
	return s.searchPrograms, nil
}
func (s *stubDispatcharrClient) VODCategories(context.Context) ([]dispatcharr.VODCategory, error) {
	s.vodCalls++
	return s.vodCategories, nil
}
func (s *stubDispatcharrClient) Movies(context.Context) ([]dispatcharr.Movie, error) {
	s.vodCalls++
	return s.movies, nil
}
func (s *stubDispatcharrClient) Series(context.Context) ([]dispatcharr.Series, error) {
	s.vodCalls++
	return s.series, nil
}
func (s *stubDispatcharrClient) LiveStreamURL(channelUUID string) string {
	return "https://dispatcharr.example.com/proxy/ts/stream/" + channelUUID
}
func (s *stubDispatcharrClient) LogoCacheURL(logoID string) string {
	return "https://dispatcharr.example.com/api/channels/logos/" + logoID + "/cache/"
}
func (s *stubDispatcharrClient) MovieStreamURL(movieUUID string) string {
	return "https://dispatcharr.example.com/proxy/vod/movie/" + movieUUID
}
func (s *stubDispatcharrClient) SeriesStreamURL(seriesUUID string) string {
	return "https://dispatcharr.example.com/proxy/vod/series/" + seriesUUID
}
func (s *stubDispatcharrClient) AbsoluteURL(raw string) string { return raw }
