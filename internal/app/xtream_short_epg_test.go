package app

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

type concurrentShortEPGClient struct {
	stubXtreamClient
	failIDs  map[int64]bool
	delay    time.Duration
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	calls    atomic.Int32
	mu       sync.Mutex
}

func (c *concurrentShortEPGClient) ShortEPG(ctx context.Context, streamID int64) (xtream.ShortEPGResponse, error) {
	c.calls.Add(1)
	current := c.inFlight.Add(1)
	defer c.inFlight.Add(-1)
	c.mu.Lock()
	if current > c.maxSeen.Load() {
		c.maxSeen.Store(current)
	}
	c.mu.Unlock()
	if c.delay > 0 {
		select {
		case <-time.After(c.delay):
		case <-ctx.Done():
			return xtream.ShortEPGResponse{}, ctx.Err()
		}
	}
	if c.failIDs[streamID] {
		return xtream.ShortEPGResponse{}, errors.New("upstream 500 for stream " + strconv.FormatInt(streamID, 10))
	}
	id := strconv.FormatInt(streamID, 10)
	return xtream.ShortEPGResponse{EPGListings: []xtream.EPGListing{{ID: "epg-" + id, Title: "Show " + id, StartTimestamp: "1700000000", StopTimestamp: "1700003600"}}}, nil
}

func xtreamStreams(count int) []xtream.LiveStream {
	streams := make([]xtream.LiveStream, 0, count)
	for i := 0; i < count; i++ {
		id := int64(1000 + i)
		streams = append(streams, xtream.LiveStream{Num: i + 1, Name: "Channel " + strconv.Itoa(i), StreamID: id, EPGChannelID: "ch" + strconv.Itoa(i)})
	}
	return streams
}

func xtreamTestSettings() config.Settings {
	return config.Settings{
		SourceMode:      config.SourceModeXtream,
		XtreamBaseURL:   "https://dispatcharr.example.com",
		XtreamUsername:  "demo",
		XtreamPassword:  "secret",
		ChannelRefreshH: 24,
		EPGRefreshH:     6,
	}
}

func TestXtreamShortEPGUsesBoundedPoolAndSkipsFailures(t *testing.T) {
	t.Parallel()

	client := &concurrentShortEPGClient{
		stubXtreamClient: stubXtreamClient{streams: xtreamStreams(40)},
		failIDs:          map[int64]bool{1003: true, 1017: true},
		delay:            5 * time.Millisecond,
	}
	store := cache.NewStore()
	service := NewService(Dependencies{Store: store, XtreamFactory: func(string, string, string) XtreamClient { return client }})

	if err := service.SyncNow(context.Background(), xtreamTestSettings(), 200); err != nil {
		t.Fatalf("expected sync success despite per-channel failures, got %v", err)
	}
	if got := client.calls.Load(); got != 40 {
		t.Fatalf("expected 40 short EPG calls, got %d", got)
	}
	if max := client.maxSeen.Load(); max > xtreamShortEPGWorkers || max < 2 {
		t.Fatalf("expected bounded parallelism (2..%d), got %d", xtreamShortEPGWorkers, max)
	}
	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 40 {
		t.Fatalf("expected all channels saved, got %d", len(snapshot.Catalog.Channels))
	}
	if len(snapshot.Catalog.Programs) != 38 {
		t.Fatalf("expected 38 programs (2 failed channels), got %d", len(snapshot.Catalog.Programs))
	}
}

func TestXtreamShortEPGAllFailuresStillSaveChannels(t *testing.T) {
	t.Parallel()

	client := &concurrentShortEPGClient{
		stubXtreamClient: stubXtreamClient{streams: xtreamStreams(3)},
		failIDs:          map[int64]bool{1000: true, 1001: true, 1002: true},
	}
	store := cache.NewStore()
	service := NewService(Dependencies{Store: store, XtreamFactory: func(string, string, string) XtreamClient { return client }})

	if err := service.SyncNow(context.Background(), xtreamTestSettings(), 200); err != nil {
		t.Fatalf("expected channel sync success, got %v", err)
	}
	snapshot := store.Current()
	if len(snapshot.Catalog.Channels) != 3 {
		t.Fatalf("expected channels saved, got %d", len(snapshot.Catalog.Channels))
	}
	if snapshot.Health.EPGStatus != "failed" || snapshot.Health.EPGLastError == "" {
		t.Fatalf("expected epg failure recorded, got %+v", snapshot.Health)
	}
	if snapshot.Health.LastError != "" {
		t.Fatalf("connection health should stay clean, got %q", snapshot.Health.LastError)
	}
}

func TestXtreamShortEPGRespectsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	client := &concurrentShortEPGClient{
		stubXtreamClient: stubXtreamClient{streams: xtreamStreams(200)},
		delay:            20 * time.Millisecond,
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	service := NewService(Dependencies{XtreamFactory: func(string, string, string) XtreamClient { return client }})

	err := service.SyncNow(ctx, xtreamTestSettings(), 200)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	if calls := client.calls.Load(); calls >= 200 {
		t.Fatalf("expected dispatch to stop after cancellation, got %d calls", calls)
	}
}
