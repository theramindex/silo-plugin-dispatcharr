package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const statsFixtureCompetition = `{"date":"2026-09-06T01:30Z","competitors":[{"homeAway":"home","score":"46","team":{"id":"21","location":"San Diego State"}},{"homeAway":"away","score":"20","team":{"id":"2502","location":"Portland State"}}],"status":{"type":{"state":"in","detail":"4:04 - 4th Quarter","completed":false}}}`

func statsFixtureEvent() SportsEvent {
	return SportsEvent{ID: "game", LeagueID: "college-football", StartUnix: time.Date(2026, 9, 6, 1, 30, 0, 0, time.UTC).Unix(), Home: SportsTeam{Name: "San Diego State"}, Away: SportsTeam{Name: "Portland State"}}
}

func TestCollegeFootballStatsMatchBothSchoolsAndDate(t *testing.T) {
	var competition espnStatsCompetition
	if err := json.Unmarshal([]byte(statsFixtureCompetition), &competition); err != nil {
		t.Fatal(err)
	}
	event := statsFixtureEvent()
	if !espnStatsMatches(event, competition) {
		t.Fatal("exact matchup with ESPN minute timestamp must match")
	}
	event.Away.Name = "Portland"
	if espnStatsMatches(event, competition) {
		t.Fatal("a different school must not match")
	}
	event = statsFixtureEvent()
	event.StartUnix -= 24 * 3600
	if espnStatsMatches(event, competition) {
		t.Fatal("another date must not match")
	}
	event = statsFixtureEvent()
	event.Home, event.Away = event.Away, event.Home
	if espnStatsMatches(event, competition) {
		t.Fatal("reversed teams must not swap the stats")
	}
}

func TestCollegeFootballStatsFetchAndCache(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/scoreboard":
			if r.URL.Query().Get("dates") != "20260905-20260906" {
				t.Error("wrong date window")
			}
			ioJSON := `{"events":[{"id":"401860879","competitions":[` + statsFixtureCompetition + `]}]}`
			_, _ = w.Write([]byte(ioJSON))
		case "/summary":
			if r.URL.Query().Get("event") != "401860879" {
				t.Error("wrong event")
			}
			_, _ = w.Write([]byte(`{"header":{"id":"401860879","competitions":[` + statsFixtureCompetition + `]},"boxscore":{"teams":[{"team":{"id":"2502","location":"Portland State"},"statistics":[{"name":"totalYards","displayValue":"370"},{"name":"turnovers","displayValue":"0"}]},{"team":{"id":"21","location":"San Diego State"},"statistics":[{"name":"totalYards","displayValue":"366"},{"name":"turnovers","displayValue":"3"}]}]},"drives":{"current":{"plays":[{"text":"Previous play"},{"text":"Four-yard rush"}]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cache := footballStatsCache{baseURL: server.URL, client: server.Client()}
	value := cache.load(context.Background(), statsFixtureEvent())
	if !value.Available || !value.Live || value.HomeScore != "46" || value.AwayScore != "20" || value.StatusText != "4:04 - 4th Quarter" || value.LastPlay != "Four-yard rush" {
		t.Fatalf("incorrect live stats: %+v", value)
	}
	if len(value.Rows) != 2 || value.Rows[0].Away != "370" || value.Rows[0].Home != "366" || value.Rows[1].Away != "0" {
		t.Fatalf("stats must retain orientation and zero values: %+v", value.Rows)
	}
	_ = cache.load(context.Background(), statsFixtureEvent())
	if requests != 2 {
		t.Fatalf("expected cached scoreboard and summary, got %d requests", requests)
	}
}

func TestCollegeFootballStatsRejectAmbiguousFixtures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "summary") {
			t.Error("must not fetch an ambiguous game")
		}
		_, _ = w.Write([]byte(`{"events":[{"id":"1","competitions":[` + statsFixtureCompetition + `]},{"id":"2","competitions":[` + statsFixtureCompetition + `]}]}`))
	}))
	defer server.Close()
	cache := footballStatsCache{baseURL: server.URL, client: server.Client()}
	if value := cache.load(context.Background(), statsFixtureEvent()); value.Available {
		t.Fatal("ambiguous stats must remain unavailable")
	}
}

func TestCollegeFootballStatsUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	cache := footballStatsCache{baseURL: server.URL, client: server.Client()}
	if value := cache.load(context.Background(), statsFixtureEvent()); value.Available || value.Message == "" {
		t.Fatal("failed source must report unavailable without invented statistics")
	}
}
