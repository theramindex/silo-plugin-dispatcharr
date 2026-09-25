package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestSelectSportsEventsKeepsNearbyGamesWithoutChannels(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 24, 20, 0, 0, 0, time.UTC)
	channel := []SportsChannelMatch{{ID: "channel:espn", Name: "ESPN", Confidence: "high", Score: 90}}
	lowConfidence := []SportsChannelMatch{{ID: "channel:guess", Name: "Guess", Confidence: "low", Score: 90}}
	events := []SportsEvent{
		{ID: "watchable", LeagueID: "cebl", StartUnix: now.Unix(), Channels: channel},
		{ID: "covered-league", LeagueID: "cebl", StartUnix: now.Add(3 * time.Hour).Unix(), Channels: lowConfidence},
		{ID: "news-league", LeagueID: "mlb", Live: true},
		{ID: "recent-final", LeagueID: "nfl", Completed: true, StartUnix: now.Add(-20 * time.Hour).Unix()},
		{ID: "old-final", LeagueID: "nfl", Completed: true, StartUnix: now.Add(-40 * time.Hour).Unix()},
		{ID: "far-future", LeagueID: "mlb", StartUnix: now.Add(72 * time.Hour).Unix()},
		{ID: "unknown-league", LeagueID: "obscure-league", StartUnix: now.Unix()},
		{ID: "generic", LeagueID: "sports", StartUnix: now.Unix()},
	}
	selected := selectSportsEvents(events, now)
	ids := []string{}
	for _, event := range selected {
		ids = append(ids, event.ID)
	}
	if got := strings.Join(ids, ","); got != "watchable,covered-league,news-league,recent-final" {
		t.Fatalf("unexpected selection %s", got)
	}
	if len(selected[1].Channels) != 0 {
		t.Fatal("low-confidence channel matches must not be offered as Watch options")
	}
}

func TestSelectSportsEventsCapsGamesWithoutChannels(t *testing.T) {
	t.Parallel()
	now := time.Now()
	events := make([]SportsEvent, sportsUnwatchableEventLimit+20)
	for index := range events {
		events[index] = SportsEvent{ID: "game", LeagueID: "mlb", StartUnix: now.Unix()}
	}
	if got := len(selectSportsEvents(events, now)); got != sportsUnwatchableEventLimit {
		t.Fatalf("expected %d games without channels, got %d", sportsUnwatchableEventLimit, got)
	}
}

func TestApplySportsTeamChannelsPutsPinnedChannelFirst(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 24, 20, 0, 0, 0, time.UTC)
	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{Channels: []model.Channel{{ID: "channel:yes", Name: "YES Network"}, {ID: "channel:espn", Name: "ESPN"}}}})
	pins := []sportsTeamChannelPin{{LeagueID: "mlb", TeamName: normalizeMatchText("New York Yankees"), ChannelID: "channel:yes"}}
	events := []SportsEvent{
		{ID: "home", LeagueID: "mlb", StartUnix: now.Unix(), Home: SportsTeam{Name: "New York Yankees"}, Away: SportsTeam{Name: "Tampa Bay Rays"}, Channels: []SportsChannelMatch{{ID: "channel:espn", Confidence: "medium"}, {ID: "channel:yes", Confidence: "low"}}},
		{ID: "other-league", LeagueID: "nfl", StartUnix: now.Unix(), Home: SportsTeam{Name: "New York Yankees"}},
		{ID: "final", LeagueID: "mlb", Completed: true, StartUnix: now.Unix(), Home: SportsTeam{Name: "New York Yankees"}},
	}
	result := applySportsTeamChannels(events, store.Current(), pins, now)
	if len(result[0].Channels) != 2 || result[0].Channels[0].ID != "channel:yes" || result[0].Channels[0].Confidence != "high" || result[0].Channels[1].ID != "channel:espn" {
		t.Fatalf("pinned channel must lead without duplicates: %+v", result[0].Channels)
	}
	if len(result[1].Channels) != 0 || len(result[2].Channels) != 0 {
		t.Fatal("pins apply only to the same league and to games that are not over")
	}
}

func TestSportsStandingsGroupsAndColumns(t *testing.T) {
	t.Parallel()
	espn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"MLB","children":[{"name":"American League","standings":{"entries":[
			{"team":{"displayName":"Cleveland Guardians","abbreviation":"CLE","logos":[{"href":"https://a.espncdn.com/cle.png"}]},"stats":[{"abbreviation":"W","displayValue":"83"},{"abbreviation":"L","displayValue":"76"},{"abbreviation":"PCT","displayValue":".522","value":0.522},{"abbreviation":"GB","displayValue":"13"},{"abbreviation":"STRK","displayValue":"W1"},{"abbreviation":"Last Ten","displayValue":"8-2"},{"abbreviation":"CLINCH","displayValue":"z"}]},
			{"team":{"displayName":"New York Yankees","abbreviation":"NYY"},"stats":[{"abbreviation":"W","displayValue":"91"},{"abbreviation":"L","displayValue":"67"},{"abbreviation":"PCT","displayValue":".576","value":0.576},{"abbreviation":"GB","displayValue":"-"},{"abbreviation":"STRK","displayValue":"L1"},{"abbreviation":"Last Ten","displayValue":"6-4"}]}]}}]}`))
	}))
	defer espn.Close()
	news := sportsNewsCache{standingsBaseURL: espn.URL + "/", client: espn.Client()}
	league, _ := espnLeagueFor("mlb")
	payload := news.standings(context.Background(), "mlb", league, false)
	if strings.Join(payload.Columns, ",") != "W,L,PCT,GB,STRK,L10" || len(payload.Groups) != 1 {
		t.Fatalf("unexpected standings shape %+v", payload)
	}
	rows := payload.Groups[0].Rows
	if rows[0].Team != "New York Yankees" || rows[1].Clinch != "z" || rows[1].Values[5] != "8-2" {
		t.Fatalf("standings must sort by percentage and keep clinch markers: %+v", rows)
	}
}

func TestApplyESPNCompetitionDetailBaseballSituationAndLeaders(t *testing.T) {
	t.Parallel()
	var competition espnStatsCompetition
	if err := json.Unmarshal([]byte(`{"status":{"type":{"state":"in"}},"situation":{"balls":3,"strikes":2,"outs":1,"onFirst":true,"onThird":true,"batter":{"athlete":{"displayName":"Elly De La Cruz","headshot":"https://a.espncdn.com/elly.png"},"summary":"1-3"},"pitcher":{"athlete":{"displayName":"Max Fried","headshot":{"href":"https://a.espncdn.com/fried.png"}}}},
		"competitors":[{"homeAway":"home","team":{"id":"10"}},{"homeAway":"away","team":{"id":"17"}}],
		"leaders":[{"displayName":"Hits","leaders":[{"displayValue":"3-4","athlete":{"displayName":"Aaron Judge"},"team":{"id":"10"}}]}]}`), &competition); err != nil {
		t.Fatal(err)
	}
	result := SportsGameStats{Live: true}
	applyESPNCompetitionDetail(&result, competition, "baseball/mlb")
	situation := result.Situation
	if situation == nil || situation.Balls != 3 || situation.Outs != 1 || !situation.OnFirst || situation.OnSecond || !situation.OnThird {
		t.Fatalf("unexpected situation %+v", situation)
	}
	if situation.Batter.Name != "Elly De La Cruz" || situation.Batter.Headshot == "" || situation.Pitcher.Headshot != "https://a.espncdn.com/fried.png" {
		t.Fatalf("batter and pitcher must keep names and headshots: %+v %+v", situation.Batter, situation.Pitcher)
	}
	if len(result.Leaders) != 1 || result.Leaders[0].Side != "home" || !result.Available {
		t.Fatalf("leaders must map to the home or away side: %+v", result.Leaders)
	}
}

func TestGuideNextGameListingsUseTheEmbeddedStart(t *testing.T) {
	t.Parallel()
	start, ok := guideSportsNextGameStart("Next Game: Baltimore Orioles @ New York Yankees on 2026-09-25 at 07:05PM EDT")
	if !ok || time.Unix(start, 0).UTC().Format(time.RFC3339) != "2026-09-25T23:05:00Z" {
		t.Fatalf("unexpected start %v %v", time.Unix(start, 0).UTC(), ok)
	}
	if _, ok := guideSportsNextGameStart("Next Game: Mets @ Nationals"); ok {
		t.Fatal("listings without a date must not produce a start time")
	}
	now := time.Date(2026, time.September, 25, 4, 0, 0, 0, time.UTC)
	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{
		Channels: []model.Channel{{ID: "channel:yankees", Name: "New York Yankees", CategoryName: "MLB"}},
		Programs: []model.Program{{ID: "p1", ChannelID: "channel:yankees", Title: "Next Game: Baltimore Orioles @ New York Yankees on 2026-09-25 at 07:05PM EDT", Categories: []string{"Sports", "Baseball"}, StartUnix: now.Unix(), EndUnix: now.Add(time.Hour).Unix()}},
	}})
	events, _ := sportsEventsFromGuideWithScoreHints(store.Current(), now)
	if len(events) != 1 || time.Unix(events[0].StartUnix, 0).UTC().Hour() != 23 || events[0].Home.Name != "New York Yankees" || strings.Contains(events[0].Name, "Next Game") {
		t.Fatalf("next-game filler must become a game at its real start: %+v", events)
	}
	provider := []SportsEvent{{ID: "sportarr:1", LeagueID: "mlb", StartUnix: now.Add(15*time.Hour + 5*time.Minute).Unix(), Home: SportsTeam{Name: "New York Yankees"}, Away: SportsTeam{Name: "Baltimore Orioles"}}}
	merged := mergeSportsGuideEvents(provider, events)
	if len(merged) != 1 || len(merged[0].Channels) != 1 || merged[0].Channels[0].ID != "channel:yankees" {
		t.Fatalf("the provider game must pick up the team channel: %+v", merged)
	}
}

func TestMergeSportsGuideEventsMarksNextDayRebroadcastsAsReplays(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 25, 4, 0, 0, 0, time.UTC)
	provider := []SportsEvent{{ID: "sportarr:1", LeagueID: "mlb", Completed: true, Status: "completed", StartUnix: now.Add(-5 * time.Hour).Unix(), Home: SportsTeam{Name: "New York Yankees"}, Away: SportsTeam{Name: "Tampa Bay Rays"}}}
	guide := []SportsEvent{{ID: "epg:1", LeagueID: "mlb", Status: "scheduled", StartUnix: now.Add(8 * time.Hour).Unix(), Home: SportsTeam{Name: "New York Yankees"}, Away: SportsTeam{Name: "Tampa Bay Rays"}}}
	merged := mergeSportsGuideEvents(provider, guide)
	if len(merged) != 2 || merged[1].Status != "replay" {
		t.Fatalf("a next-morning airing of a finished game must be a replay: %+v", merged)
	}
}

func TestSportsTeamFollowIDsCoverGuideOnlyIdentities(t *testing.T) {
	t.Parallel()
	team := SportsTeam{ID: "36248520-provider", Name: "Washington Nationals", Abbreviation: "WSH"}
	ids := strings.Join(sportsTeamFollowIDs(team), ",")
	if !strings.Contains(ids, "sports-team:6d655deb3b0a8d9e") {
		t.Fatalf("a follow saved from the guide-only team must still match the provider team, got %s", ids)
	}
	if sportsTeamFollowIDs(SportsTeam{}) != nil {
		t.Fatal("unnamed teams have no follow identities")
	}
}

func TestMatchESPNTeamPrefersExactNames(t *testing.T) {
	t.Parallel()
	teams := []espnTeam{
		{ID: "127", DisplayName: "Michigan State Spartans", ShortDisplayName: "Michigan St", Location: "Michigan State", Name: "Spartans"},
		{ID: "130", DisplayName: "Michigan Wolverines", ShortDisplayName: "Michigan", Location: "Michigan", Name: "Wolverines"},
		{ID: "10", DisplayName: "New York Yankees", Location: "New York", Name: "Yankees", Abbreviation: "NYY"},
	}
	for name, want := range map[string]string{"Michigan": "130", "Michigan State Spartans": "127", "New York Yankees": "10", "NYY": "10"} {
		team, ok := matchESPNTeam(teams, name)
		if !ok || team.ID != want {
			t.Fatalf("%s: got %q (%v), want %s", name, team.ID, ok, want)
		}
	}
	if _, ok := matchESPNTeam(teams, "Boston Red Sox"); ok {
		t.Fatal("unknown teams must not match")
	}
}

func TestHTTPRoutesServerSportsNewsAndTeam(t *testing.T) {
	t.Parallel()
	requests := []string{}
	espn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/baseball/mlb/teams"):
			_, _ = w.Write([]byte(`{"sports":[{"leagues":[{"teams":[{"team":{"id":"10","displayName":"New York Yankees","location":"New York","name":"Yankees","abbreviation":"NYY","color":"003087","logos":[{"href":"https://a.espncdn.com/i/teamlogos/mlb/500/nyy.png"}]}}]}]}]}`))
		case strings.HasSuffix(r.URL.Path, "/baseball/mlb/teams/10/schedule"):
			_, _ = w.Write([]byte(`{"events":[{"name":"Old game","date":"2020-01-01T00:00Z","competitions":[{}]},{"name":"Baltimore Orioles at New York Yankees","date":"2099-09-25T23:05Z","competitions":[{"competitors":[{"homeAway":"home","team":{"id":"10","displayName":"New York Yankees"}},{"homeAway":"away","team":{"id":"1","displayName":"Baltimore Orioles"}}],"broadcasts":[{"media":{"shortName":"YES"}}]}]}]}`))
		case strings.HasSuffix(r.URL.Path, "/baseball/mlb/teams/10"):
			_, _ = w.Write([]byte(`{"team":{"id":"10","displayName":"New York Yankees","standingSummary":"2nd in AL East","record":{"items":[{"summary":"91-67"}]},"nextEvent":[{"name":"Tampa Bay Rays at New York Yankees","date":"2026-09-24T23:05Z"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/baseball/mlb/news"):
			_, _ = w.Write([]byte(`{"articles":[{"id":1,"headline":"Yankees clinch","description":"Big night","published":"2026-09-24T20:00:00Z","images":[{"url":"https://a.espncdn.com/photo/1.jpg"}],"links":{"web":{"href":"https://www.espn.com/mlb/story/1"}},"categories":[{"type":"team","description":"New York Yankees"},{"type":"league","description":"MLB"}]},{"id":2,"headline":"No link"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer espn.Close()
	server := NewHTTPRoutesServer(cache.NewStore())
	server.sportsNews = sportsNewsCache{baseURL: espn.URL + "/", client: espn.Client()}

	get := func(path string, values map[string]any, target any) {
		query, _ := structpb.NewStruct(values)
		response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: "GET", Path: path, Query: query})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if err := json.Unmarshal(response.GetBody(), target); err != nil {
			t.Fatalf("%s: decode %v: %s", path, err, response.GetBody())
		}
	}
	var news SportsNewsPayload
	get("/dispatcharr/api/sports/news", map[string]any{"league": "mlb", "team": "New York Yankees"}, &news)
	if len(news.Articles) != 1 || news.Articles[0].Headline != "Yankees clinch" || news.Articles[0].Teams[0] != "New York Yankees" || news.LeagueName != "MLB" {
		t.Fatalf("unexpected news payload %+v", news)
	}
	if !strings.Contains(strings.Join(requests, " "), "team=10") {
		t.Fatalf("team news must use the ESPN team id, requests: %v", requests)
	}
	var team SportsTeamSummary
	get("/dispatcharr/api/sports/team", map[string]any{"league": "mlb", "name": "New York Yankees"}, &team)
	if !team.Available || team.Record != "91-67" || team.Standing != "2nd in AL East" || team.LogoURL == "" || team.NextEventUnix == 0 || len(team.Articles) != 1 {
		t.Fatalf("unexpected team summary %+v", team)
	}
	if len(team.Upcoming) != 1 || team.Upcoming[0].Opponent != "Baltimore Orioles" || !team.Upcoming[0].Home || team.Upcoming[0].Broadcast != "YES" {
		t.Fatalf("team schedule must list future games with opponent and network, got %+v", team.Upcoming)
	}
	var unsupported SportsNewsPayload
	get("/dispatcharr/api/sports/news", map[string]any{"league": "cebl"}, &unsupported)
	if len(unsupported.Articles) != 0 || unsupported.Message == "" {
		t.Fatalf("unsupported leagues must explain missing news, got %+v", unsupported)
	}
}
