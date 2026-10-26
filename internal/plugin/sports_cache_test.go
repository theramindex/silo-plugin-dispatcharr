package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestFootballStatsLoadDedupesWithoutHoldingTheLock(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		entered <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/scoreboard":
			_, _ = w.Write([]byte(`{"events":[{"id":"401860879","competitions":[` + statsFixtureCompetition + `]}]}`))
		case "/summary":
			_, _ = w.Write([]byte(`{"header":{"id":"401860879","competitions":[` + statsFixtureCompetition + `]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	stats := &footballStatsCache{baseURL: server.URL, client: server.Client()}

	var wg sync.WaitGroup
	results := make([]SportsGameStats, 6)
	for index := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[index] = stats.load(context.Background(), statsFixtureEvent())
		}()
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("stats fetch never started")
	}
	// The lock must stay free while ESPN is slow, so other games and the
	// remembered-fixture lookup are not blocked.
	if !stats.mu.TryLock() {
		t.Fatal("footballStatsCache.mu is held across the ESPN request")
	}
	stats.mu.Unlock()
	close(release)
	wg.Wait()
	if got := requests.Load(); got != 3 {
		t.Fatalf("concurrent pollers must share one load (2 scoreboards + 1 summary), got %d requests", got)
	}
	for _, result := range results {
		if result.SourceURL == "" || result.HomeScore != "46" {
			t.Fatalf("every poller gets the shared result: %+v", result)
		}
	}
}

func TestPreparedSportsEventCopiesOnlyTheRequestedEvent(t *testing.T) {
	t.Parallel()
	server := NewHTTPRoutesServer(cache.NewStore())
	spread := 3.5
	server.sportsPrepared = sportsPreparedCache{Ready: true, ExpiresAfter: time.Now().Add(time.Minute), Payload: SportsPayload{Events: []SportsEvent{
		{ID: "other"},
		{ID: "game", StableID: "sports-event:stable", Spread: &spread, Channels: []SportsChannelMatch{{ID: "channel"}}},
	}}}
	event, ok := server.preparedSportsEvent("sports-event:stable")
	if !ok || event.ID != "game" {
		t.Fatalf("lookup by stable ID failed: %+v %v", event, ok)
	}
	event.Channels[0].ID = "mutated"
	*event.Spread = 0
	if server.sportsPrepared.Payload.Events[1].Channels[0].ID != "channel" || *server.sportsPrepared.Payload.Events[1].Spread != 3.5 {
		t.Fatal("the returned event must be a copy")
	}
	if _, ok := server.preparedSportsEvent("missing"); ok {
		t.Fatal("unknown IDs must not resolve")
	}
}

func TestSportsNewsCachesFailuresBriefly(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	news := &sportsNewsCache{baseURL: server.URL + "/", standingsBaseURL: server.URL + "/", client: server.Client()}
	league, _ := espnLeagueFor("mlb")
	for range 3 {
		if _, err := news.news(context.Background(), "mlb", league, "", 5); err == nil {
			t.Fatal("upstream failure must surface")
		}
		if standings := news.standings(context.Background(), "mlb", league, false); !strings.Contains(standings.Message, "unavailable") {
			t.Fatalf("standings failure message = %q", standings.Message)
		}
		if summary := news.teamSummary(context.Background(), "mlb", league, "New York Yankees"); summary.Available || summary.Message == "" {
			t.Fatalf("team failure must be reported: %+v", summary)
		}
	}
	// One request each for news, standings and the team list; repeats inside
	// the negative TTL are served from the cache.
	if got := requests.Load(); got != 3 {
		t.Fatalf("failed lookups must be negatively cached, got %d upstream requests", got)
	}
	news.mu.Lock()
	for key, entry := range news.entries {
		if entry.err == nil {
			continue
		}
		if ttl := time.Until(entry.expires); ttl > sportsNewsNegativeTTL || ttl < sportsNewsNegativeTTL-5*time.Second {
			t.Fatalf("negative entry %q expires in %s", key, ttl)
		}
	}
	news.mu.Unlock()
}

func TestSportsNewsDedupesConcurrentLoads(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"articles":[{"id":1,"headline":"Yankees win","links":{"web":{"href":"https://www.espn.com/a"}}}]}`))
	}))
	defer server.Close()
	news := &sportsNewsCache{baseURL: server.URL + "/", client: server.Client()}
	league, _ := espnLeagueFor("mlb")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			articles, err := news.news(context.Background(), "mlb", league, "", 5)
			if err != nil || len(articles) != 1 {
				t.Errorf("shared load result: %v %+v", err, articles)
			}
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for requests.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if got := requests.Load(); got != 1 {
		t.Fatalf("concurrent news loads must share one request, got %d", got)
	}
}

func TestSportsFlightGroupSharesOneCall(t *testing.T) {
	t.Parallel()
	var group sportsFlightGroup
	var calls atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := group.do(context.Background(), "key", func() (any, error) {
				calls.Add(1)
				time.Sleep(20 * time.Millisecond)
				return "value", nil
			})
			if err != nil || value != "value" {
				t.Errorf("shared value = %v, %v", value, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if calls.Load() > 2 {
		t.Fatalf("concurrent callers should share a call, got %d", calls.Load())
	}
	if _, err := group.do(context.Background(), "key", func() (any, error) { calls.Add(1); return nil, nil }); err != nil || len(group.calls) != 0 {
		t.Fatal("finished flights must be forgotten")
	}
}

func TestSportsNewsRouteStopsWaitingWhenRequestIsCanceled(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	var requests atomic.Int32
	espn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		close(entered)
		<-release
		_, _ = w.Write([]byte(`{"articles":[]}`))
	}))
	defer espn.Close()
	defer releaseOnce.Do(func() { close(release) })
	server := NewHTTPRoutesServer(cache.NewStore())
	server.sportsNews.baseURL = espn.URL + "/"
	server.sportsNews.client = espn.Client()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *pluginv1.HandleHTTPResponse, 1)
	go func() {
		response, _ := server.Handle(ctx, &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: "/dispatcharr/api/sports/news", Query: func() *structpb.Struct { q, _ := structpb.NewStruct(map[string]any{"league": "nba"}); return q }()})
		done <- response
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("ESPN request did not start")
	}
	cancel()
	select {
	case response := <-done:
		if response.GetStatusCode() != http.StatusOK || !strings.Contains(string(response.GetBody()), "unavailable") {
			t.Fatalf("a canceled wait must return the normal unavailable response: %+v", response)
		}
	case <-time.After(time.Second):
		t.Fatal("Sports handler kept waiting for ESPN after its request was canceled")
	}
	releaseOnce.Do(func() { close(release) })
	query, _ := structpb.NewStruct(map[string]any{"league": "nba"})
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: "/dispatcharr/api/sports/news", Query: query})
	if err != nil || response.GetStatusCode() != http.StatusOK || strings.Contains(string(response.GetBody()), "unavailable") {
		t.Fatalf("the shared fill must still serve the next caller: %v %+v", err, response)
	}
	if requests.Load() != 1 {
		t.Fatalf("canceled caller caused a duplicate ESPN fetch: %d", requests.Load())
	}
}
