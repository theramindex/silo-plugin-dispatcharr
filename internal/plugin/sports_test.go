package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

type staticSportsProvider struct {
	events []SportsEvent
	err    error
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

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Method: http.MethodPost,
		Path:   "/dispatcharr/api/sports/favorites",
		Body:   []byte(`{"teamId":"team:arg","enabled":true}`),
	})
	if err != nil {
		t.Fatalf("favorite route: %v", err)
	}
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.GetStatusCode(), string(response.GetBody()))
	}
	var preferences cache.Preferences
	if err := json.Unmarshal(response.GetBody(), &preferences); err != nil {
		t.Fatalf("unmarshal preferences: %v", err)
	}
	if !preferences.SportsFavoriteTeams["team:arg"] {
		t.Fatalf("favorite team was not persisted: %+v", preferences.SportsFavoriteTeams)
	}

	payload = fetchSportsPayload(t, server)
	if !payload.Events[0].Home.Favorite || payload.Events[0].Away.Favorite {
		t.Fatalf("favorite flags not applied to sports payload: %+v", payload.Events[0])
	}
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
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: "/dispatcharr/api/sports"})
	if err != nil {
		t.Fatalf("sports route: %v", err)
	}
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	var payload SportsPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Error == "" || len(payload.Events) != 1 || payload.Events[0].ID != "event:cached" {
		t.Fatalf("expected stale cached event with error, got %+v", payload)
	}
}

func fetchSportsPayload(t *testing.T, server *HTTPRoutesServer) SportsPayload {
	t.Helper()
	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: "/dispatcharr/api/sports"})
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
