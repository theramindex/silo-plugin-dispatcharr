package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

func TestHTTPRoutesServerEventsDetectsGuidePrograms(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(24 * time.Hour).Unix()
	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source: model.LiveTVSource(model.SourceModeXtream),
			Channels: []model.Channel{
				{ID: "ch:abc", Name: "ABC East", CategoryID: "abc", CategoryName: "US | Locals | ABC"},
				{ID: "ch:news", Name: "News Now", CategoryID: "news", CategoryName: "News"},
			},
			Programs: []model.Program{
				{ID: "p:oscars", ChannelID: "ch:abc", Title: "The Oscars", Summary: "Academy Awards ceremony", StartUnix: start, EndUnix: start + 3*3600},
				{ID: "p:news", ChannelID: "ch:news", Title: "Evening News", StartUnix: start, EndUnix: start + 3600},
			},
			Content: model.ContentState{
				LiveCategories: []model.Category{
					{ID: "abc", Name: "US | Locals | ABC", Kind: "live"},
					{ID: "news", Name: "News", Kind: "live"},
				},
			},
		},
	})
	server := NewHTTPRoutesServer(store)

	payload := fetchEventsPayload(t, server)
	if payload.Source != "epg" || len(payload.Events) != 1 {
		t.Fatalf("unexpected events payload: %+v", payload)
	}
	event := payload.Events[0]
	if event.CategoryID != "awards" || event.Keyword == "" {
		t.Fatalf("expected awards event with matched keyword, got %+v", event)
	}
	assertBroadcastEventMatch(t, event.Channels, "ch:abc")
	assertNoBroadcastEventMatch(t, event.Channels, "ch:news")
}

func TestHTTPRoutesServerEventsUsesAdminKeywordRules(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(24 * time.Hour).Unix()
	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Channels: []model.Channel{{ID: "ch:local", Name: "Local 5", CategoryID: "local", CategoryName: "US | Locals"}},
			Programs: []model.Program{{
				ID:        "p:town",
				ChannelID: "ch:local",
				Title:     "City Council Special",
				StartUnix: start,
				EndUnix:   start + 3600,
			}},
			Content: model.ContentState{
				LiveCategories: []model.Category{{ID: "local", Name: "US | Locals", Kind: "live"}},
			},
		},
	})
	store.SetAdminSettings(json.RawMessage(`{"eventKeywords":[{"categoryId":"civic","categoryName":"Civic","keywords":["City Council Special"]}]}`))
	server := NewHTTPRoutesServer(store)

	payload := fetchEventsPayload(t, server)
	if len(payload.Events) != 1 {
		t.Fatalf("expected custom keyword event, got %+v", payload)
	}
	event := payload.Events[0]
	if event.CategoryID != "civic" || event.Keyword != "City Council Special" {
		t.Fatalf("expected custom civic event, got %+v", event)
	}
	assertBroadcastEventMatch(t, event.Channels, "ch:local")
}

func fetchEventsPayload(t *testing.T, server *HTTPRoutesServer) EventsPayload {
	t.Helper()

	response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: "/dispatcharr/api/events"})
	if err != nil {
		t.Fatalf("events route: %v", err)
	}
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.GetStatusCode(), string(response.GetBody()))
	}
	var payload EventsPayload
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}

func assertBroadcastEventMatch(t *testing.T, matches []SportsChannelMatch, channelID string) {
	t.Helper()

	for _, match := range matches {
		if match.ID == channelID {
			return
		}
	}
	t.Fatalf("expected %s in event matches: %+v", channelID, matches)
}

func assertNoBroadcastEventMatch(t *testing.T, matches []SportsChannelMatch, channelID string) {
	t.Helper()

	for _, match := range matches {
		if match.ID == channelID {
			t.Fatalf("did not expect %s in event matches: %+v", channelID, matches)
		}
	}
}
