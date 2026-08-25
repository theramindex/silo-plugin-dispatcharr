package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

type staticSportsProvider struct {
	events []SportsEvent
	err    error
}

type deadlineSportsProvider struct {
	maxRemaining time.Duration
}

type blockingSportsProvider struct {
	events  []SportsEvent
	started chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int32
}

func (p *blockingSportsProvider) Events(ctx context.Context, _ time.Time) ([]SportsEvent, error) {
	p.calls.Add(1)
	p.once.Do(func() { close(p.started) })
	select {
	case <-p.release:
		return cloneSportsEvents(p.events), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*blockingSportsProvider) Source() string {
	return "blocking-test"
}

func (p *deadlineSportsProvider) Events(ctx context.Context, _ time.Time) ([]SportsEvent, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("sports provider context has no deadline")
	}
	p.maxRemaining = time.Until(deadline)
	return nil, errors.New("provider unavailable")
}

func (*deadlineSportsProvider) Source() string {
	return "deadline-test"
}

func (p staticSportsProvider) Events(context.Context, time.Time) ([]SportsEvent, error) {
	return cloneSportsEvents(p.events), p.err
}

func (p staticSportsProvider) Source() string {
	return "test"
}

func TestHTTPRoutesServerSportsMatchesChannelsAndFavoriteTeams(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{
				{ID: "ch:fs1", Name: "FOX Sports 1", CategoryID: "world", CategoryName: "World Cup"},
				{ID: "ch:fs1", Name: "FOX Sports 1", CategoryID: "world", CategoryName: "World Cup"},
				{ID: "ch:arg", Name: "Argentina Deportes", CategoryID: "arg", CategoryName: "Sports | Argentina"},
				{ID: "ch:news", Name: "News Now", CategoryID: "news", CategoryName: "News"},
			},
			Programs: []model.Program{
				{ID: "p:1", ChannelID: "ch:fs1", Title: "Argentina vs Brazil", StartUnix: 1700000000, EndUnix: 1700007200},
				{ID: "p:2", ChannelID: "ch:news", Title: "Morning News", StartUnix: 1700000000, EndUnix: 1700007200},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{
					{ID: "world", Name: "World Cup", Kind: "live"},
					{ID: "arg", Name: "Sports | Argentina", Kind: "live"},
					{ID: "news", Name: "News", Kind: "live"},
				},
			},
		},
	})
	server := NewHTTPRoutesServer(store)
	server.sportsProvider = staticSportsProvider{events: []SportsEvent{{
		ID:         "event:1",
		LeagueID:   "world-cup",
		LeagueName: "World Cup",
		Name:       "Argentina vs Brazil",
		ShortName:  "ARG vs BRA",
		Status:     "pre",
		StatusText: "Tonight",
		StartUnix:  1700000000,
		Home:       SportsTeam{ID: "team:arg", Name: "Argentina", Abbreviation: "ARG"},
		Away:       SportsTeam{ID: "team:bra", Name: "Brazil", Abbreviation: "BRA"},
	}}}

	payload := fetchSportsPayload(t, server)
	if payload.Source != "test" || len(payload.Events) != 1 {
		t.Fatalf("unexpected sports payload: %+v", payload)
	}
	event := payload.Events[0]
	if event.Home.Favorite || event.Away.Favorite {
		t.Fatalf("teams should not start as favorites: %+v", event)
	}
	assertSportsMatch(t, event.Channels, "ch:fs1")
	assertSportsMatch(t, event.Channels, "ch:arg")
	assertNoSportsMatch(t, event.Channels, "ch:news")
	matchCount := 0
	for _, match := range event.Channels {
		if match.ID == "ch:fs1" {
			matchCount++
		}
	}
	if matchCount != 1 {
		t.Fatalf("expected duplicate channel IDs to collapse, got %+v", event.Channels)
	}

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: http.MethodPost,
		Path:   "/dispatcharr/api/sports/favorites",
		Body:   []byte(`{"teamId":"team:arg","enabled":true}`),
	})
	if err != nil {
		t.Fatalf("favorite route: %v", err)
	}
	if response.GetStatusCode() != http.StatusGone {
		t.Fatalf("expected 410, got %d: %s", response.GetStatusCode(), string(response.GetBody()))
	}

	payload = fetchSportsPayload(t, server)
	if payload.Events[0].Home.Favorite || payload.Events[0].Away.Favorite {
		t.Fatalf("sports payload must remain user-neutral: %+v", payload.Events[0])
	}
}

func TestSportsPayloadFallsBackToGuideMatchups(t *testing.T) {
	t.Parallel()

	now := time.Now()
	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{
		Source: model.LiveTVSource(model.SourceModeXtream),
		Channels: []model.Channel{
			{ID: "ch:wnba-1", Name: "WNBA League Pass", CategoryID: "sports", CategoryName: "US TV | Sports"},
			{ID: "ch:wnba-2", Name: "ESPN", CategoryID: "sports", CategoryName: "US TV | Sports"},
			{ID: "ch:news", Name: "News", CategoryID: "news", CategoryName: "News"},
		},
		Programs: []model.Program{
			{ID: "p:wnba-1", ChannelID: "ch:wnba-1", Title: "WNBA Basketball: Indiana Fever vs Las Vegas Aces", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(90 * time.Minute).Unix()},
			{ID: "p:wnba-2", ChannelID: "ch:wnba-2", Title: "WNBA Basketball: Indiana Fever vs Las Vegas Aces", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(90 * time.Minute).Unix()},
			{ID: "p:news", ChannelID: "ch:news", Title: "Morning News", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(30 * time.Minute).Unix()},
		},
		Content: model.ContentState{LiveCategories: []model.Category{
			{ID: "sports", Name: "US TV | Sports", Kind: "live"},
			{ID: "news", Name: "News", Kind: "live"},
		}},
	}})
	server := NewHTTPRoutesServer(store)
	server.sportsProvider = staticSportsProvider{}

	payload := server.sportsPayload(context.Background(), false)
	if payload.Source != "EPG fallback" || len(payload.Events) != 1 {
		t.Fatalf("expected one EPG fallback event, got %+v", payload)
	}
	event := payload.Events[0]
	if event.LeagueID != "wnba" || event.Away.Name != "Indiana Fever" || event.Home.Name != "Las Vegas Aces" || !event.Live {
		t.Fatalf("unexpected fallback event: %+v", event)
	}
	assertSportsMatch(t, event.Channels, "ch:wnba-1")
	assertSportsMatch(t, event.Channels, "ch:wnba-2")
	assertNoSportsMatch(t, event.Channels, "ch:news")

	server = NewHTTPRoutesServer(store)
	server.sportsProvider = staticSportsProvider{events: []SportsEvent{{
		ID:         "sportarr:wnba-game",
		LeagueID:   "wnba",
		LeagueName: "WNBA",
		Name:       "Indiana Fever at Las Vegas Aces",
		ShortName:  "IND vs LV",
		StartUnix:  now.Add(-30 * time.Minute).Unix(),
		Live:       true,
		Status:     "live",
		Away:       SportsTeam{ID: "team:indiana", Name: "Indiana Fever", Abbreviation: "IND"},
		Home:       SportsTeam{ID: "team:las-vegas", Name: "Las Vegas Aces", Abbreviation: "LV"},
	}}}
	payload = server.sportsPayload(context.Background(), false)
	if payload.Source != "test + EPG" || len(payload.Events) != 1 || payload.Events[0].ID != "sportarr:wnba-game" {
		t.Fatalf("expected EPG to enrich the Sportarr event without duplicating it, got %+v", payload)
	}
	assertSportsMatch(t, payload.Events[0].Channels, "ch:wnba-1")
	assertSportsMatch(t, payload.Events[0].Channels, "ch:wnba-2")
}

func TestNormalizeSportsEventFreshnessExpiresStaleLiveGames(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		event     SportsEvent
		completed bool
	}{
		{
			name: "recent live event remains live",
			event: SportsEvent{
				Live:       true,
				Status:     "live",
				StatusText: "Top 4th",
				StartUnix:  now.Add(-2 * time.Hour).Unix(),
				EndUnix:    now.Add(time.Hour).Unix(),
			},
		},
		{
			name: "past end time expires",
			event: SportsEvent{
				Live:       true,
				Status:     "live",
				StatusText: "Top 3rd",
				StartUnix:  now.Add(-6 * time.Hour).Unix(),
				EndUnix:    now.Add(-3 * time.Hour).Unix(),
			},
			completed: true,
		},
		{
			name: "missing end time eventually expires",
			event: SportsEvent{
				Live:       true,
				Status:     "live",
				StatusText: "Live",
				StartUnix:  now.Add(-19 * time.Hour).Unix(),
			},
			completed: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeSportsEventFreshness(test.event, now)
			if test.completed {
				if got.Live || !got.Completed || got.Status != "final" || got.StatusText != "Final" {
					t.Fatalf("expected stale event to become final, got %+v", got)
				}
				return
			}
			if !got.Live || got.Completed {
				t.Fatalf("expected current event to remain live, got %+v", got)
			}
		})
	}
}

func TestNormalizeSportsEventsAddsGameThumbsIdentityFallbacks(t *testing.T) {
	t.Parallel()

	events := normalizeSportsEvents([]SportsEvent{
		{
			LeagueID: "nba", LeagueName: "NBA", Name: "NBA Playback: Heat vs. Pistons",
			Away: SportsTeam{Name: "Heat"}, Home: SportsTeam{Name: "Pistons"},
		},
		{
			LeagueID: "sports", LeagueName: "Sports", Name: "NWSL Soccer: Teal Rising Cup: Kansas City Current vs. Palmeiras",
			Away: SportsTeam{Name: "Kansas City Current"}, Home: SportsTeam{Name: "Palmeiras"},
		},
		{
			LeagueID: "nfl", LeagueName: "NFL", Name: "Existing artwork",
			LeagueLogoURL: "https://images.example/nfl.png",
			Away:          SportsTeam{Name: "Cowboys", LogoURL: "https://images.example/cowboys.png"},
			Home:          SportsTeam{Name: "Cardinals"},
		},
		{
			LeagueID: "sports", LeagueName: "Sports", Name: "Unknown local competition",
			Away: SportsTeam{Name: "Alpha"}, Home: SportsTeam{Name: "Beta"},
		},
		{
			LeagueID: "bra.1", LeagueName: "Brazil Serie A", Name: "São Paulo vs. Grêmio",
			Away: SportsTeam{Name: "São Paulo"}, Home: SportsTeam{Name: "Grêmio"},
		},
	})

	if got := events[0].LeagueLogoURL; got != "https://game-thumbs.swvn.io/nba/leaguelogo.png" {
		t.Fatalf("expected NBA league fallback, got %q", got)
	}
	if got := events[0].Away.LogoURL; got != "https://game-thumbs.swvn.io/nba/heat/teamlogo.png" {
		t.Fatalf("expected Heat logo fallback, got %q", got)
	}
	if got := events[0].Home.LogoURL; got != "https://game-thumbs.swvn.io/nba/pistons/teamlogo.png" {
		t.Fatalf("expected Pistons logo fallback, got %q", got)
	}
	if got := events[1].LeagueLogoURL; got != "https://game-thumbs.swvn.io/usa.nwsl/leaguelogo.png" {
		t.Fatalf("expected NWSL league fallback, got %q", got)
	}
	if got := events[1].Away.LogoURL; got != "https://game-thumbs.swvn.io/usa.nwsl/kansas-city-current/teamlogo.png" {
		t.Fatalf("expected Kansas City Current logo fallback, got %q", got)
	}
	if got := events[1].Home.LogoURL; got != "https://game-thumbs.swvn.io/bra.1/palmeiras/teamlogo.png" {
		t.Fatalf("expected Palmeiras cross-league fallback, got %q", got)
	}
	if events[2].LeagueLogoURL != "https://images.example/nfl.png" || events[2].Away.LogoURL != "https://images.example/cowboys.png" {
		t.Fatalf("expected upstream artwork to win, got %+v", events[2])
	}
	if events[3].LeagueLogoURL != "" || events[3].Away.LogoURL != "" || events[3].Home.LogoURL != "" {
		t.Fatalf("expected unknown competition to retain honest empty artwork, got %+v", events[3])
	}
	if got := events[4].Away.LogoURL; got != "https://game-thumbs.swvn.io/bra.1/sao-paulo/teamlogo.png" {
		t.Fatalf("expected diacritics removed from São Paulo fallback, got %q", got)
	}
	if got := events[4].Home.LogoURL; got != "https://game-thumbs.swvn.io/bra.1/gremio/teamlogo.png" {
		t.Fatalf("expected diacritics removed from Grêmio fallback, got %q", got)
	}
}

func TestGuideSportsMatchupParsesQualifiedBroadcastTitles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		title     string
		wantAway  string
		wantHome  string
		wantMatch bool
	}{
		{
			name:      "college bowl prefix containing at",
			title:     "CFP Quarterfinal at the Capital One Orange Bowl : Oregon vs. Texas Tech",
			wantAway:  "Oregon",
			wantHome:  "Texas Tech",
			wantMatch: true,
		},
		{
			name:      "cricket suffix after matchup",
			title:     "Cricket Highlights : Bangladesh vs Australia: 2nd ODI",
			wantAway:  "Bangladesh",
			wantHome:  "Australia",
			wantMatch: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			away, home, matched := guideSportsMatchup(test.title)
			if matched != test.wantMatch || away != test.wantAway || home != test.wantHome {
				t.Fatalf("guideSportsMatchup(%q) = %q, %q, %v; want %q, %q, %v", test.title, away, home, matched, test.wantAway, test.wantHome, test.wantMatch)
			}
		})
	}
}

func TestSportsEventsFromGuideCleansPromoMetadataAndRejectsNonSportsShows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 25, 23, 0, 0, 0, time.UTC)
	programs := []model.Program{
		{ID: "program:hunt", ChannelID: "channel:sports", Title: "Bob Redfern's Outdoor Magazine : Pheasant Hunt at Heartland Lodge", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(30 * time.Minute).Unix()},
		{ID: "program:soccer", ChannelID: "channel:sports", Title: "Fútbol UEFA Champions League : Sabah FK vs. Hapoel Beer Sheva ᴸᶦᵛᵉ", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(30 * time.Minute).Unix()},
		{ID: "program:morning-show", ChannelID: "channel:sports", Title: "Good Morning Arizona at 9am ᴺᵉʷ", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(30 * time.Minute).Unix()},
		{ID: "program:next-game", ChannelID: "channel:sports", Title: "Next Game: Boston Red Sox @ Miami Marlins on 2026-08-25 at 06:40PM EDT", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(30 * time.Minute).Unix()},
		{ID: "program:dated-team", ChannelID: "channel:sports", Title: "(CA) (CBC 07) | CEBL: Calgary at Winnipeg (2026-07-12 15:30:00)", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(30 * time.Minute).Unix()},
		{ID: "program:cfp", ChannelID: "channel:sports", Title: "CFP Quarterfinal at the Rose Bowl : Alabama vs. Indiana", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(30 * time.Minute).Unix()},
	}
	events := sportsEventsFromGuide(cache.Snapshot{Catalog: model.CatalogState{
		Channels: []model.Channel{{ID: "channel:sports", Name: "Sports Network", CategoryID: "sports", CategoryName: "Sports"}},
		Programs: programs,
		Content:  model.ContentState{LiveCategories: []model.Category{{ID: "sports", Name: "Sports", Kind: "live"}}},
	}}, now)
	byName := map[string]SportsEvent{}
	for _, event := range events {
		byName[event.Name] = event
	}

	if _, found := byName[programs[0].Title]; found {
		t.Fatalf("expected a hunt at a location to be excluded from sports matchups: %+v", byName[programs[0].Title])
	}
	if _, found := byName[programs[2].Title]; found {
		t.Fatalf("expected a morning news show to be excluded from sports matchups: %+v", byName[programs[2].Title])
	}

	soccer, found := byName["Fútbol UEFA Champions League : Sabah FK vs. Hapoel Beer Sheva"]
	if !found || soccer.Away.Name != "Sabah FK" || soccer.Home.Name != "Hapoel Beer Sheva" {
		t.Fatalf("expected live annotation removed from the event and team names, got %+v", soccer)
	}

	nextGame := byName[programs[3].Title]
	if nextGame.Away.Name != "Boston Red Sox" || nextGame.Home.Name != "Miami Marlins" || nextGame.Live || nextGame.Status != "scheduled" || nextGame.StatusText != "Upcoming" {
		t.Fatalf("expected Next Game promo to parse as an upcoming matchup, got %+v", nextGame)
	}

	dated := byName[programs[4].Title]
	if dated.Away.Name != "Calgary" || dated.Home.Name != "Winnipeg" {
		t.Fatalf("expected trailing date metadata removed from team names, got %+v", dated)
	}

	cfp := byName[programs[5].Title]
	if cfp.LeagueID != "college-football" || cfp.LeagueName != "College Football" || cfp.SportName != "Football" {
		t.Fatalf("expected CFP event classified as college football, got %+v", cfp)
	}
}

func TestSportsEventsFromGuideClassifiesRacesAndBroadcastState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	programs := []model.Program{
		{
			ID: "program:formula-e", ChannelID: "channel:sports",
			Title: "Formule : Formule E v Miami", StartUnix: now.Add(-30 * time.Minute).Unix(), EndUnix: now.Add(90 * time.Minute).Unix(),
		},
		{
			ID: "program:nfl-replay", ChannelID: "channel:sports",
			Title: "NFL Football : San Francisco 49ers at Los Angeles Chargers", Summary: "A rebroadcast of the complete game.",
			StartUnix: now.Add(-time.Hour).Unix(), EndUnix: now.Add(time.Hour).Unix(),
		},
		{
			ID: "program:cricket-highlights", ChannelID: "channel:sports",
			Title: "Cricket Highlights : Bangladesh vs Australia: 2nd ODI", StartUnix: now.Add(-time.Hour).Unix(), EndUnix: now.Add(time.Hour).Unix(),
		},
	}
	events := sportsEventsFromGuide(cache.Snapshot{Catalog: model.CatalogState{
		Channels: []model.Channel{{ID: "channel:sports", Name: "Sports Network", CategoryID: "sports", CategoryName: "Sports"}},
		Programs: programs,
		Content:  model.ContentState{LiveCategories: []model.Category{{ID: "sports", Name: "Sports", Kind: "live"}}},
	}}, now)
	byName := map[string]SportsEvent{}
	for _, event := range events {
		byName[event.Name] = event
	}

	race := byName[programs[0].Title]
	if race.EventType != "race" || race.LeagueID != "formula-e" || race.Away.Name != "Formula E" || race.Home.Name != "Miami" {
		t.Fatalf("expected Formula E race identity without a fake matchup, got %+v", race)
	}
	if !race.Live || race.Status != "airing" || race.StatusText != "On now" {
		t.Fatalf("expected unverified EPG race to be airing rather than live, got %+v", race)
	}

	replay := byName[programs[1].Title]
	if !replay.Live || replay.Status != "replay" || replay.StatusText != "Replay" {
		t.Fatalf("expected explicit EPG rebroadcast classification, got %+v", replay)
	}

	highlights := byName[programs[2].Title]
	if !highlights.Live || highlights.Status != "highlights" || highlights.StatusText != "Highlights" {
		t.Fatalf("expected highlights classification, got %+v", highlights)
	}
}

func TestMergeSportsGuideEventsMarksLaterCompletedMatchupAiringAsReplay(t *testing.T) {
	t.Parallel()

	providerStart := time.Date(2026, time.August, 23, 20, 0, 0, 0, time.UTC).Unix()
	guideStart := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC).Unix()
	provider := SportsEvent{
		ID: "sportarr:nfl-game", LeagueID: "nfl", Name: "San Francisco 49ers at Los Angeles Chargers",
		Away: SportsTeam{Name: "San Francisco 49ers"}, Home: SportsTeam{Name: "Los Angeles Chargers"},
		StartUnix: providerStart, Completed: true, Status: "completed", StatusText: "Final",
	}
	guide := SportsEvent{
		ID: "epg:nfl-rebroadcast", LeagueID: "nfl", Name: "NFL Football : San Francisco 49ers at Los Angeles Chargers",
		Away: SportsTeam{Name: "San Francisco 49ers"}, Home: SportsTeam{Name: "Los Angeles Chargers"},
		StartUnix: guideStart, Live: true, Status: "airing", StatusText: "On now",
	}

	merged := mergeSportsGuideEvents([]SportsEvent{provider}, []SportsEvent{guide})
	if len(merged) != 2 {
		t.Fatalf("expected completed event and later guide airing to remain separate, got %+v", merged)
	}
	if merged[1].Status != "replay" || merged[1].StatusText != "Replay" || !merged[1].Live {
		t.Fatalf("expected later same-matchup airing to be identified as replay, got %+v", merged[1])
	}
}

func TestSportsEventsFromGuideCapsFallbackSlate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	programs := make([]model.Program, 0, 300)
	for index := 0; index < 300; index++ {
		programs = append(programs, model.Program{
			ID:        fmt.Sprintf("p:%d", index),
			ChannelID: "ch:sports",
			Title:     fmt.Sprintf("WNBA: Team %03d vs Club %03d", index, index),
			StartUnix: now.Add(time.Duration(index%48) * time.Hour).Unix(),
			EndUnix:   now.Add(time.Duration(index%48+2) * time.Hour).Unix(),
		})
	}
	events := sportsEventsFromGuide(cache.Snapshot{Catalog: model.CatalogState{
		Channels: []model.Channel{{ID: "ch:sports", Name: "WNBA League Pass", CategoryID: "sports"}},
		Programs: programs,
		Content:  model.ContentState{LiveCategories: []model.Category{{ID: "sports", Name: "Sports", Kind: "live"}}},
	}}, now)
	if len(events) != 250 {
		t.Fatalf("expected bounded EPG fallback slate, got %d events", len(events))
	}
}

func TestSportsPayloadBoundsProviderWorkBelowPluginRouteDeadline(t *testing.T) {
	t.Parallel()

	provider := &deadlineSportsProvider{}
	server := NewHTTPRoutesServer(cache.NewStore())
	server.sportsProvider = provider
	payload := server.sportsPayload(context.Background(), false)
	if provider.maxRemaining <= 0 || provider.maxRemaining > sportsProviderFetchTimeout {
		t.Fatalf("expected bounded provider deadline, got %s", provider.maxRemaining)
	}
	if payload.Error == "" || payload.Source != "deadline-test" {
		t.Fatalf("expected provider error payload, got %+v", payload)
	}
}

func TestSportsRouteReturnsWhilePreparedPayloadBuildRuns(t *testing.T) {
	t.Parallel()

	provider := &blockingSportsProvider{
		events:  []SportsEvent{{ID: "event:ready", LeagueID: "nfl", LeagueName: "NFL", Name: "Jets at Giants", StartUnix: 1700000000}},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server := NewHTTPRoutesServer(cache.NewStore())
	server.sportsProvider = provider

	startedAt := time.Now()
	payload := fetchSportsPayloadOnce(t, server, false)
	if elapsed := time.Since(startedAt); elapsed > 250*time.Millisecond {
		t.Fatalf("sports route waited for background build: %s", elapsed)
	}
	if !payload.Refreshing || payload.Source != "blocking-test" {
		t.Fatalf("expected refreshing placeholder, got %+v", payload)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("sports payload build did not start")
	}
	close(provider.release)

	ready := fetchSportsPayload(t, server)
	if ready.Refreshing || len(ready.Events) != 1 || ready.Events[0].ID != "event:ready" {
		t.Fatalf("expected prepared sports payload, got %+v", ready)
	}
	if calls := provider.calls.Load(); calls != 1 {
		t.Fatalf("expected one background provider call, got %d", calls)
	}
}

func TestSportsRouteReturnsGuideFallbackWhileProviderBuildRuns(t *testing.T) {
	t.Parallel()

	now := time.Now()
	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Channels: []model.Channel{{ID: "channel:wnba", Name: "WNBA League Pass", CategoryID: "sports", CategoryName: "US Sports | WNBA"}},
			Programs: []model.Program{{ID: "program:wnba", ChannelID: "channel:wnba", Title: "Indiana Fever vs Las Vegas Aces", StartUnix: now.Add(-time.Hour).Unix(), EndUnix: now.Add(time.Hour).Unix()}},
		},
		Health: model.SyncHealth{EPGLastSuccessUnix: now.Add(-time.Minute).Unix()},
	})
	provider := &blockingSportsProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server := NewHTTPRoutesServer(store)
	server.sportsProvider = provider

	payload := fetchSportsPayloadOnce(t, server, false)
	if !payload.Refreshing || payload.Source != "EPG fallback" || len(payload.Events) != 1 {
		t.Fatalf("expected immediate EPG fallback while provider builds, got %+v", payload)
	}
	if len(payload.Events[0].Channels) != 1 || payload.Events[0].Channels[0].ID != "channel:wnba" {
		t.Fatalf("expected guide channel match in fallback, got %+v", payload.Events[0].Channels)
	}
	close(provider.release)
}

func TestSportsRouteDoesNotWaitForCatalogHydration(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	store := cache.NewStore()
	syncer := &stubCatalogSyncer{store: store, block: block}
	settings := config.Settings{
		SourceMode:        config.SourceModeAPIKey,
		DispatcharrURL:    "https://dispatcharr.example.com",
		DispatcharrAPIKey: "secret",
	}
	server := NewHTTPRoutesServerWithSyncer(store, func() config.Settings { return settings }, syncer)

	startedAt := time.Now()
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: "/dispatcharr/api/sports"})
	if err != nil {
		close(block)
		t.Fatalf("sports route: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 250*time.Millisecond {
		close(block)
		t.Fatalf("sports route waited for catalog hydration: %s", elapsed)
	}
	if response.GetStatusCode() != http.StatusOK {
		close(block)
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	close(block)
}

func TestMatchSportsChannelsDoesNotUseLeagueOnlyGroups(t *testing.T) {
	t.Parallel()

	snapshot := cache.Snapshot{
		Catalog: model.CatalogState{
			Channels: []model.Channel{
				{ID: "ch:ari", Name: "Arizona Team Feed", CategoryID: "ari", CategoryName: "US Sports | NFL Teams | Arizona Cardinals"},
				{ID: "ch:lac", Name: "Los Angeles Team Feed", CategoryID: "lac", CategoryName: "US Sports | NFL Teams | Los Angeles Chargers"},
				{ID: "ch:atl", Name: "Atlanta Falcons", CategoryID: "atl", CategoryName: "US Sports | NFL Teams | Atlanta Falcons"},
				{ID: "ch:nfl", Name: "NFL Network", CategoryID: "nfl", CategoryName: "US Sports | NFL Teams"},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{
					{ID: "ari", Name: "US Sports | NFL Teams | Arizona Cardinals", Kind: "live"},
					{ID: "lac", Name: "US Sports | NFL Teams | Los Angeles Chargers", Kind: "live"},
					{ID: "atl", Name: "US Sports | NFL Teams | Atlanta Falcons", Kind: "live"},
					{ID: "nfl", Name: "US Sports | NFL Teams", Kind: "live"},
				},
			},
		},
	}
	event := SportsEvent{
		ID:         "event:nfl",
		LeagueID:   "nfl",
		LeagueName: "NFL",
		Name:       "Arizona Cardinals at Los Angeles Chargers",
		ShortName:  "ARI @ LAC",
		StartUnix:  1700000000,
		Home:       SportsTeam{ID: "team:lac", Name: "Los Angeles Chargers", Abbreviation: "LAC"},
		Away:       SportsTeam{ID: "team:ari", Name: "Arizona Cardinals", Abbreviation: "ARI"},
	}

	matches := matchSportsChannels(event, snapshot)
	assertSportsMatch(t, matches, "ch:ari")
	assertSportsMatch(t, matches, "ch:lac")
	assertNoSportsMatch(t, matches, "ch:atl")
	assertNoSportsMatch(t, matches, "ch:nfl")
}

func TestMatchSportsChannelsRejectsWeakGuideOnlyMatches(t *testing.T) {
	t.Parallel()

	snapshot := cache.Snapshot{
		Catalog: model.CatalogState{
			Channels: []model.Channel{
				{ID: "ch:sport", Name: "Sport1", CategoryID: "sports", CategoryName: "International Sports | Germany"},
				{ID: "ch:fox", Name: "FOX 7", CategoryID: "fox", CategoryName: "US | Locals | FOX"},
				{ID: "ch:starz", Name: "Starz Encore Westerns", CategoryID: "movies", CategoryName: "US | Movies"},
			},
			Programs: []model.Program{
				{ID: "p:sport", ChannelID: "ch:sport", Title: "Ecuador vs Mexico", StartUnix: 1700000000, EndUnix: 1700007200},
				{ID: "p:fox", ChannelID: "ch:fox", Title: "FIFA World Cup 2026: Ecuador vs. Mexico", StartUnix: 1700000000, EndUnix: 1700007200},
				{ID: "p:starz", ChannelID: "ch:starz", Title: "Western Movie", Summary: "A classic adventure near Mexico.", StartUnix: 1700000000, EndUnix: 1700007200},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{
					{ID: "sports", Name: "International Sports | Germany", Kind: "live"},
					{ID: "fox", Name: "US | Locals | FOX", Kind: "live"},
					{ID: "movies", Name: "US | Movies", Kind: "live"},
				},
			},
		},
	}
	event := SportsEvent{
		ID:         "event:world-cup",
		LeagueID:   "world-cup",
		LeagueName: "World Cup",
		Name:       "Ecuador vs Mexico",
		ShortName:  "ECU @ MEX",
		StartUnix:  1700000000,
		Home:       SportsTeam{ID: "team:mex", Name: "Mexico", Abbreviation: "MEX"},
		Away:       SportsTeam{ID: "team:ecu", Name: "Ecuador", Abbreviation: "ECU"},
	}

	matches := matchSportsChannels(event, snapshot)
	assertSportsMatch(t, matches, "ch:sport")
	assertSportsMatch(t, matches, "ch:fox")
	assertNoSportsMatch(t, matches, "ch:starz")
}

func TestMatchSportsChannelsRejectsPositiveButLowConfidenceMatch(t *testing.T) {
	t.Parallel()

	snapshot := cache.Snapshot{Catalog: model.CatalogState{
		Channels: []model.Channel{{
			ID: "ch:event-title", Name: "World Cup Quarterfinal", CategoryID: "general", CategoryName: "General TV",
		}},
		Content: model.ContentState{LiveCategories: []model.Category{{ID: "general", Name: "General TV", Kind: "live"}}},
	}}
	event := SportsEvent{
		ID:         "event:world-cup",
		LeagueID:   "world-cup",
		LeagueName: "World Cup",
		Name:       "World Cup Quarterfinal",
		Home:       SportsTeam{Name: "Mexico"},
		Away:       SportsTeam{Name: "Ecuador"},
	}

	assertNoSportsMatch(t, matchSportsChannels(event, snapshot), "ch:event-title")
}

func TestMatchSportsChannelsRejectsPartialSingleWordTeamNames(t *testing.T) {
	t.Parallel()

	snapshot := cache.Snapshot{
		Catalog: model.CatalogState{
			Channels: []model.Channel{
				{ID: "ch:new-england", Name: "New England Revolution", CategoryID: "mls", CategoryName: "US Sports | MLS Teams"},
				{ID: "ch:england", Name: "England Sports", CategoryID: "world", CategoryName: "International Sports | England"},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{
					{ID: "mls", Name: "US Sports | MLS Teams", Kind: "live"},
					{ID: "world", Name: "International Sports | England", Kind: "live"},
				},
			},
		},
	}
	event := SportsEvent{
		ID:         "event:world-cup",
		LeagueID:   "world-cup",
		LeagueName: "World Cup",
		Name:       "England vs Norway",
		ShortName:  "ENG vs NOR",
		StartUnix:  1700000000,
		Home:       SportsTeam{ID: "team:eng", Name: "England", Abbreviation: "ENG"},
		Away:       SportsTeam{ID: "team:nor", Name: "Norway", Abbreviation: "NOR"},
	}

	matches := matchSportsChannels(event, snapshot)
	assertSportsMatch(t, matches, "ch:england")
	assertNoSportsMatch(t, matches, "ch:new-england")
}

func TestMatchSportsChannelsRejectsAbbreviationOutsideSportsContext(t *testing.T) {
	t.Parallel()

	snapshot := cache.Snapshot{
		Catalog: model.CatalogState{
			Channels: []model.Channel{
				{ID: "ch:music", Name: "MC Dance EDM", CategoryID: "music", CategoryName: "International TV | Latino | Music"},
				{ID: "ch:edm", Name: "EDM Team Feed", CategoryID: "nhl", CategoryName: "US Sports | NHL Teams"},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{
					{ID: "music", Name: "International TV | Latino | Music", Kind: "live"},
					{ID: "nhl", Name: "US Sports | NHL Teams", Kind: "live"},
				},
			},
		},
	}
	event := SportsEvent{
		ID:         "event:nhl",
		LeagueID:   "nhl",
		LeagueName: "NHL",
		Name:       "Winnipeg Jets at Edmonton Oilers",
		ShortName:  "WPG @ EDM",
		StartUnix:  1700000000,
		Home:       SportsTeam{ID: "team:edm", Name: "Edmonton Oilers", Abbreviation: "EDM"},
		Away:       SportsTeam{ID: "team:wpg", Name: "Winnipeg Jets", Abbreviation: "WPG"},
	}

	matches := matchSportsChannels(event, snapshot)
	assertSportsMatch(t, matches, "ch:edm")
	assertNoSportsMatch(t, matches, "ch:music")
}

func TestMatchSportsChannelsRejectsAbbreviationOutsideEventLeague(t *testing.T) {
	t.Parallel()

	snapshot := cache.Snapshot{
		Catalog: model.CatalogState{
			Channels: []model.Channel{
				{ID: "ch:big-ten", Name: "Big Ten Network", CategoryID: "sports", CategoryName: "US TV | Sports"},
				{ID: "ch:ten", Name: "TEN Team Feed", CategoryID: "nfl", CategoryName: "US Sports | NFL Teams"},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{
					{ID: "sports", Name: "US TV | Sports", Kind: "live"},
					{ID: "nfl", Name: "US Sports | NFL Teams", Kind: "live"},
				},
			},
		},
	}
	event := SportsEvent{
		ID:         "event:nfl",
		LeagueID:   "nfl",
		LeagueName: "NFL",
		Name:       "Tennessee Titans at New York Jets",
		ShortName:  "TEN @ NYJ",
		StartUnix:  1700000000,
		Home:       SportsTeam{ID: "team:nyj", Name: "New York Jets", Abbreviation: "NYJ"},
		Away:       SportsTeam{ID: "team:ten", Name: "Tennessee Titans", Abbreviation: "TEN"},
	}

	matches := matchSportsChannels(event, snapshot)
	assertSportsMatch(t, matches, "ch:ten")
	assertNoSportsMatch(t, matches, "ch:big-ten")
}

func TestHTTPRoutesServerSportsUsesStaleCacheOnProviderError(t *testing.T) {
	t.Parallel()

	store := cache.NewStore()
	server := NewHTTPRoutesServer(store)
	server.sportsProvider = staticSportsProvider{events: []SportsEvent{{ID: "event:cached", LeagueID: "nfl", LeagueName: "NFL", Name: "Jets at Giants", StartUnix: 1700000000}}}
	first := fetchSportsPayload(t, server)
	if len(first.Events) != 1 {
		t.Fatalf("expected cached event seed, got %+v", first)
	}
	server.sportsProvider = staticSportsProvider{err: errors.New("provider down")}
	server.sportsCache.ExpiresAfter = time.Now().Add(-time.Second)
	server.sportsPrepared.ExpiresAfter = time.Now().Add(-time.Second)
	payload := fetchSportsPayload(t, server)
	if payload.Error == "" || len(payload.Events) != 1 || payload.Events[0].ID != "event:cached" {
		t.Fatalf("expected stale cached event with error, got %+v", payload)
	}
}

func TestSportarrSportsEventMapsCanonicalFields(t *testing.T) {
	t.Parallel()

	event := sportarrEvent{
		ID:                "event-uuid",
		ShortID:           "ev-401",
		Name:              "Panama vs Croatia",
		EventType:         "group_stage",
		LeagueID:          "league-uuid",
		LeagueName:        "World Cup",
		SeasonName:        "2026",
		Round:             "Group A",
		VenueName:         "MetLife Stadium",
		ScheduledStart:    "2026-06-26T22:35:00Z",
		ScheduledEnd:      "2026-06-27T00:35:00Z",
		BroadcastTimezone: "America/New_York",
		Status:            "in_progress",
		HomeTeamID:        "panama-id",
		HomeTeamName:      "Panama",
		AwayTeamID:        "croatia-id",
		AwayTeamName:      "Croatia",
		HomeScore:         sportarrString("1"),
		AwayScore:         sportarrString("2"),
	}
	converted := event.sportsEvent()
	expected := time.Date(2026, 6, 26, 22, 35, 0, 0, time.UTC).Unix()
	if converted.StartUnix != expected {
		t.Fatalf("expected parsed start %d, got %d", expected, converted.StartUnix)
	}
	if converted.ID != "sportarr:ev-401" || converted.ProviderID != "event-uuid" || converted.LeagueName != "World Cup" || converted.Home.Name != "Panama" || converted.Away.Name != "Croatia" {
		t.Fatalf("unexpected Sportarr mapping: %+v", converted)
	}
	if !converted.Live || converted.Completed || converted.StatusText != "Live" || converted.HomeScore != "1" || converted.AwayScore != "2" {
		t.Fatalf("unexpected Sportarr live state: %+v", converted)
	}
	if converted.EventType != "group_stage" || converted.Round != "Group A" || converted.Venue != "MetLife Stadium" || converted.BroadcastTimezone != "America/New_York" {
		t.Fatalf("expected canonical Sportarr metadata, got %+v", converted)
	}
}

func TestSportarrSportsProviderLoadsPaginatedPublicEvents(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path != "/events" {
			http.NotFound(response, request)
			return
		}
		if request.URL.Query().Get("page_size") != "100" || request.URL.Query().Get("from") == "" || request.URL.Query().Get("to") == "" {
			t.Errorf("unexpected Sportarr query: %s", request.URL.RawQuery)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.URL.Query().Get("page") == "2" {
			_, _ = response.Write([]byte(`{"items":[{"id":"uuid-2","shortId":"ev-2","name":"Team C vs Team D","leagueId":"league-2","leagueName":"League Two","scheduledStart":null,"status":"scheduled","homeTeamId":"team-c","homeTeamName":"Team C","awayTeamId":"team-d","awayTeamName":"Team D"}],"total":2,"page":2,"pageSize":100,"totalPages":2}`))
			return
		}
		_, _ = response.Write([]byte(`{"items":[{"id":"uuid-1","shortId":"ev-1","name":"Team A vs Team B","leagueId":"league-1","leagueName":"League One","scheduledStart":"2026-07-13T20:00:00Z","status":"completed","homeTeamId":"team-a","homeTeamName":"Team A","awayTeamId":"team-b","awayTeamName":"Team B","homeScore":3,"awayScore":"2"}],"total":-2,"page":1,"pageSize":100,"totalPages":2}`))
	})
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	provider := newSportarrSportsProvider(testServer.Client())
	provider.baseURL = testServer.URL
	events, err := provider.Events(context.Background(), time.Date(2026, 7, 13, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Sportarr events: %v", err)
	}
	if provider.Source() != "sportarr" || len(events) != 2 {
		t.Fatalf("expected two Sportarr events, got %+v", events)
	}
	if !events[0].Completed || events[0].HomeScore != "3" || events[0].AwayScore != "2" {
		t.Fatalf("expected numeric and string scores to normalize, got %+v", events[0])
	}
	if events[1].StartUnix != 0 {
		t.Fatalf("unknown Sportarr start should stay 0, got %d", events[1].StartUnix)
	}
}

func TestSportarrSportsProviderCoalescesConcurrentTeamRequests(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	testServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"team-id","name":"Team Name","abbreviation":"TM"}`))
	}))
	defer testServer.Close()

	provider := newSportarrSportsProvider(testServer.Client())
	provider.baseURL = testServer.URL
	var wait sync.WaitGroup
	errs := make(chan error, 12)
	for index := 0; index < 12; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := provider.team(context.Background(), "team-id")
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("team metadata request: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one coalesced metadata request, got %d", calls.Load())
	}
}

func TestSportarrSportsProviderCoalescesAndCachesEventImages(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	testServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"images":[{"image_type":"backdrop","url":"https://images.example/event.jpg","is_primary":true}]}`))
	}))
	defer testServer.Close()

	provider := newSportarrSportsProvider(testServer.Client())
	provider.baseURL = testServer.URL
	var wait sync.WaitGroup
	errs := make(chan error, 12)
	for index := 0; index < 12; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			imageURL, err := provider.eventImage(context.Background(), "event-id")
			if imageURL != "https://images.example/event.jpg" && err == nil {
				err = fmt.Errorf("unexpected image URL %q", imageURL)
			}
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("event image request: %v", err)
		}
	}
	if _, err := provider.eventImage(context.Background(), "event-id"); err != nil {
		t.Fatalf("cached event image request: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one coalesced and cached image request, got %d", calls.Load())
	}
}

func TestSportarrSportsProviderBoundsMetadataCaches(t *testing.T) {
	t.Parallel()

	provider := newSportarrSportsProvider(nil)
	for index := 0; index < sportarrTeamCacheLimit+8; index++ {
		id := fmt.Sprintf("team-%d", index)
		provider.storeTeamLocked(id, sportarrTeamCacheEntry{Team: sportarrTeam{ID: id}, ExpiresAt: time.Now().Add(time.Duration(index) * time.Minute)})
	}
	for index := 0; index < sportarrLeagueCacheLimit+8; index++ {
		id := fmt.Sprintf("league-%d", index)
		provider.storeLeagueLocked(id, sportarrLeagueCacheEntry{League: sportarrLeague{ID: id}, ExpiresAt: time.Now().Add(time.Duration(index) * time.Minute)})
	}
	for index := 0; index < sportarrImageCacheLimit+8; index++ {
		id := fmt.Sprintf("event-%d", index)
		provider.storeImageLocked(id, sportarrImageCacheEntry{ImageURL: "https://images.example/" + id + ".jpg", ExpiresAt: time.Now().Add(time.Duration(index) * time.Minute)})
	}
	if len(provider.teams) != sportarrTeamCacheLimit || len(provider.leagues) != sportarrLeagueCacheLimit || len(provider.images) != sportarrImageCacheLimit {
		t.Fatalf("expected bounded metadata caches, got %d teams, %d leagues, and %d images", len(provider.teams), len(provider.leagues), len(provider.images))
	}
}

func TestSportarrSportsProviderEnrichesMatchedEvents(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/teams/home-id":
			_, _ = response.Write([]byte(`{"id":"home-id","name":"New York Liberty","abbreviation":"NYL","logoUrl":"https://images.example/home.png","primaryColor":"#112233"}`))
		case "/teams/away-id":
			_, _ = response.Write([]byte(`{"id":"away-id","name":"Las Vegas Aces","abbreviation":"LVA","logoUrl":"https://images.example/away.png"}`))
		case "/leagues/league-id":
			_, _ = response.Write([]byte(`{"id":"league-id","name":"WNBA","sportName":"Basketball","description":"Professional women's basketball.","logoUrl":"https://images.example/league.png"}`))
		case "/api/v1/images/entity/event/event-id":
			if request.URL.Query().Get("completed_only") != "true" {
				t.Errorf("expected completed_only=true, got %q", request.URL.Query().Get("completed_only"))
			}
			_, _ = response.Write([]byte(`{"images":[{"image_type":"poster","url":"https://images.example/poster.jpg","is_primary":true,"priority":1},{"image_type":"backdrop","url":"https://images.example/backdrop.jpg","is_primary":true,"priority":2}]}`))
		default:
			http.NotFound(response, request)
		}
	})
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	provider := newSportarrSportsProvider(testServer.Client())
	provider.baseURL = testServer.URL
	events := []SportsEvent{{
		ProviderID: "event-id",
		LeagueID:   "league-id",
		Home:       SportsTeam{ID: "home-id", Name: "Liberty"},
		Away:       SportsTeam{ID: "away-id", Name: "Aces"},
		Channels:   []SportsChannelMatch{{ID: "channel-1"}},
	}}
	enriched := provider.EnrichEvents(context.Background(), events, 1)
	waitForSportarrEnrichment(t, provider)
	enriched = provider.EnrichEvents(context.Background(), events, 1)
	waitForSportarrEnrichment(t, provider)
	if enriched[0].SportName != "Basketball" || enriched[0].LeagueLogoURL == "" || enriched[0].LeagueDescription == "" {
		t.Fatalf("expected league enrichment, got %+v", enriched[0])
	}
	if enriched[0].Home.Abbreviation != "NYL" || enriched[0].Home.LogoURL == "" || enriched[0].Away.Abbreviation != "LVA" {
		t.Fatalf("expected team enrichment, got %+v", enriched[0])
	}
	if enriched[0].ImageURL != "https://images.example/backdrop.jpg" {
		t.Fatalf("expected canonical Sportarr event artwork, got %+v", enriched[0])
	}
}

func TestSportarrEnrichmentPreservesExistingIdentityFallbacks(t *testing.T) {
	t.Parallel()

	provider := newSportarrSportsProvider(nil)
	provider.leagues["nba"] = sportarrLeagueCacheEntry{League: sportarrLeague{ID: "nba", Name: "NBA"}}
	provider.teams["heat"] = sportarrTeamCacheEntry{Team: sportarrTeam{ID: "heat", Name: "Miami Heat"}}
	event := SportsEvent{
		LeagueID:          "nba",
		LeagueName:        "NBA",
		LeagueLogoURL:     "https://game-thumbs.swvn.io/nba/leaguelogo.png",
		LeagueDescription: "Existing description",
		SportName:         "Basketball",
		Away: SportsTeam{
			ID:             "heat",
			Name:           "Heat",
			LogoURL:        "https://game-thumbs.swvn.io/nba/heat/teamlogo.png",
			PrimaryColor:   "#aa0000",
			SecondaryColor: "#000000",
		},
	}

	got := provider.applyCachedDetails(event)
	if got.LeagueLogoURL != event.LeagueLogoURL || got.LeagueDescription != event.LeagueDescription || got.SportName != event.SportName {
		t.Fatalf("empty Sportarr league metadata replaced existing identity: %+v", got)
	}
	if got.Away.LogoURL != event.Away.LogoURL || got.Away.PrimaryColor != event.Away.PrimaryColor || got.Away.SecondaryColor != event.Away.SecondaryColor {
		t.Fatalf("empty Sportarr team metadata replaced existing identity: %+v", got.Away)
	}
}

func TestSportarrEnrichmentDoesNotBlockSportsResponse(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		time.Sleep(150 * time.Millisecond)
		response.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(request.URL.Path, "/images/entity/event/"):
			_, _ = response.Write([]byte(`{"images":[]}`))
		case strings.Contains(request.URL.Path, "/leagues/"):
			_, _ = response.Write([]byte(`{"id":"league-id","name":"League"}`))
		default:
			_, _ = response.Write([]byte(`{"id":"team-id","name":"Team"}`))
		}
	}))
	defer testServer.Close()

	provider := newSportarrSportsProvider(testServer.Client())
	provider.baseURL = testServer.URL
	events := []SportsEvent{{
		ProviderID: "event-id",
		LeagueID:   "league-id",
		Home:       SportsTeam{ID: "home-id"},
		Away:       SportsTeam{ID: "away-id"},
		Channels:   []SportsChannelMatch{{ID: "channel-id"}},
	}}
	started := time.Now()
	_ = provider.EnrichEvents(context.Background(), events, 1)
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("expected non-blocking enrichment, took %s", elapsed)
	}
	waitForSportarrEnrichment(t, provider)
}

func waitForSportarrEnrichment(t *testing.T, provider *sportarrSportsProvider) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		provider.metadataMu.Lock()
		running := provider.enriching
		provider.metadataMu.Unlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for Sportarr enrichment")
}

func TestSportarrSportsProviderRetriesTransientResponses(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	testServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Cache-Control") != "no-cache, no-store" || request.Header.Get("Pragma") != "no-cache" {
			t.Errorf("expected no-cache request headers")
		}
		if calls.Add(1) == 1 {
			response.Header().Set("Retry-After", "0")
			response.WriteHeader(http.StatusTooManyRequests)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"team-id","name":"Recovered Team"}`))
	}))
	defer testServer.Close()

	provider := newSportarrSportsProvider(testServer.Client())
	provider.baseURL = testServer.URL
	team, err := provider.team(context.Background(), "team-id")
	if err != nil {
		t.Fatalf("expected transient response to recover: %v", err)
	}
	if calls.Load() != 2 || team.Name != "Recovered Team" {
		t.Fatalf("expected one retry and recovered payload, got %d calls and %+v", calls.Load(), team)
	}
}

func TestWaitForSportarrRetryHonorsContextForLongRetryAfter(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := waitForSportarrRetry(ctx, "5", 0)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("expected context to bound long Retry-After, took %s", elapsed)
	}
}

func TestPickSportarrEventImagePrefersBackdropAndPriority(t *testing.T) {
	t.Parallel()

	images := []sportarrEntityImage{
		{ImageType: "poster", URL: "https://images.example/poster.jpg", IsPrimary: true, Priority: 100},
		{ImageType: "backdrop", URL: "https://images.example/lower.jpg", Priority: 999},
		{ImageType: "backdrop", URL: "https://images.example/best.jpg", IsPrimary: true, Priority: 2},
		{ImageType: "backdrop", URL: "javascript:alert(1)", IsPrimary: true, Priority: 999},
	}
	if got := pickSportarrEventImage(images); got != "https://images.example/best.jpg" {
		t.Fatalf("expected preferred backdrop, got %q", got)
	}
}

func fetchSportsPayload(t *testing.T, server *HTTPRoutesServer) SportsPayload {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		payload := fetchSportsPayloadOnce(t, server, false)
		if !payload.Refreshing {
			return payload
		}
		if time.Now().After(deadline) {
			t.Fatalf("sports payload did not finish preparing: %+v", payload)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func fetchSportsPayloadOnce(t *testing.T, server *HTTPRoutesServer, refresh bool) SportsPayload {
	t.Helper()
	path := "/dispatcharr/api/sports"
	if refresh {
		path += "?refresh=1"
	}
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: path})
	if err != nil {
		t.Fatalf("sports route: %v", err)
	}
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.GetStatusCode(), string(response.GetBody()))
	}
	var payload SportsPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}

func assertSportsMatch(t *testing.T, matches []SportsChannelMatch, channelID string) {
	t.Helper()
	for _, match := range matches {
		if match.ID == channelID {
			return
		}
	}
	t.Fatalf("expected %s in sports matches: %+v", channelID, matches)
}

func assertNoSportsMatch(t *testing.T, matches []SportsChannelMatch, channelID string) {
	t.Helper()
	for _, match := range matches {
		if match.ID == channelID {
			t.Fatalf("did not expect %s in sports matches: %+v", channelID, matches)
		}
	}
}
