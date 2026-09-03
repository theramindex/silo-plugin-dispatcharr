package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/timeshift"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestTimeShiftMediaUsesManifestDeclaredRoutes(t *testing.T) {
	t.Parallel()
	server := NewHTTPRoutesServer(cache.NewStore())
	server.timeShift = timeshift.NewManager(t.TempDir())

	manifestQuery, _ := structpb.NewStruct(map[string]any{"buffer_id": "buffer", "lease": "lease"})
	manifest, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Path: "/dispatcharr/timeshift/manifest", Method: http.MethodGet, Query: manifestQuery,
	})
	if err != nil || manifest.GetStatusCode() != http.StatusTooEarly {
		t.Fatalf("expected declared manifest route to reach the buffer manager: status=%d err=%v", manifest.GetStatusCode(), err)
	}

	segmentQuery, _ := structpb.NewStruct(map[string]any{"buffer_id": "buffer", "lease": "lease", "sequence": "invalid"})
	segment, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Path: "/dispatcharr/timeshift/segment", Method: http.MethodGet, Query: segmentQuery,
	})
	if err != nil || segment.GetStatusCode() != http.StatusBadRequest {
		t.Fatalf("expected declared segment route to validate the sequence: status=%d err=%v", segment.GetStatusCode(), err)
	}
}

func TestTimeShiftRoutesShareBuffersAndGateAdminOperations(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer upstream.Close()

	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{
		Source:   model.LiveTVSource(model.SourceModeDirectLogin),
		Channels: []model.Channel{{ID: "channel-1", Name: "Channel 1", StreamURL: upstream.URL}},
	}})
	store.SetAdminSettings(json.RawMessage(`{"liveRewindEnabled":true,"liveRewindCacheGB":1,"liveRewindWindowMinutes":15,"liveRewindMinFreeGB":1,"liveRewindMaxChannels":2}`))
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings { return config.Settings{SourceMode: config.SourceModeDirectLogin} })
	server.timeShift = timeshift.NewManager(t.TempDir())

	start := func() map[string]any {
		response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
			Path: "/dispatcharr/api/timeshift/start", Method: http.MethodPost, Body: []byte(`{"channelId":"channel-1"}`),
		})
		if err != nil || response.GetStatusCode() != http.StatusAccepted {
			t.Fatalf("start rewind: status=%d err=%v body=%s", response.GetStatusCode(), err, response.GetBody())
		}
		var payload map[string]any
		if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
			t.Fatalf("decode start response: %v", err)
		}
		return payload
	}
	first := start()
	second := start()
	if first["bufferId"] != second["bufferId"] || first["leaseId"] == second["leaseId"] {
		t.Fatalf("expected shared buffer and unique leases: first=%v second=%v", first, second)
	}
	manifestPath, _ := first["manifestPath"].(string)
	if !strings.HasPrefix(manifestPath, "/dispatcharr/timeshift/manifest?buffer_id=") {
		t.Fatalf("expected start response to use the declared manifest route, got %q", manifestPath)
	}

	unauthorized, _ := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Path: "/dispatcharr/api/timeshift/admin-status", Method: http.MethodGet})
	if unauthorized.GetStatusCode() != http.StatusForbidden {
		t.Fatalf("expected admin status to require admin role, got %d", unauthorized.GetStatusCode())
	}
	authorized, _ := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Path: "/dispatcharr/api/timeshift/admin-status", Method: http.MethodGet, Headers: map[string]string{"x-silo-user-role": "admin"},
	})
	if authorized.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected admin status, got %d: %s", authorized.GetStatusCode(), authorized.GetBody())
	}
	var stats timeshift.Stats
	if err := json.Unmarshal(authorized.GetBody(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.ActiveBuffers != 1 || stats.ActiveLeases != 2 {
		t.Fatalf("expected shared runtime usage, got %+v", stats)
	}

	cleared, _ := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Path: "/dispatcharr/api/timeshift/clear", Method: http.MethodPost, Headers: map[string]string{"x-silo-user-role": "admin"},
	})
	if cleared.GetStatusCode() != http.StatusOK {
		t.Fatalf("clear rewind cache: %d %s", cleared.GetStatusCode(), cleared.GetBody())
	}
}

func TestTimeShiftStartRejectsHLSProxyStreams(t *testing.T) {
	t.Parallel()
	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{Channels: []model.Channel{
		{ID: "hls", StreamURL: "https://dispatcharr.example/proxy/hls/channel-1"},
	}}})
	store.SetAdminSettings(json.RawMessage(`{"liveRewindEnabled":true}`))
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings { return config.Settings{SourceMode: config.SourceModeDirectLogin} })
	server.timeShift = timeshift.NewManager(t.TempDir())
	response, _ := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Path: "/dispatcharr/api/timeshift/start", Method: http.MethodPost, Body: []byte(`{"channelId":"hls"}`),
	})
	if response.GetStatusCode() != http.StatusConflict {
		t.Fatalf("expected HLS rewind to stay unavailable, got %d %s", response.GetStatusCode(), response.GetBody())
	}
}

func TestTimeShiftStartFallsBackWhenDisabledOrNotDirect(t *testing.T) {
	t.Parallel()
	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{Channels: []model.Channel{{ID: "channel", StreamURL: "http://example.test/live.ts"}}}})
	store.SetAdminSettings(json.RawMessage(`{"liveRewindEnabled":false}`))
	server := NewHTTPRoutesServerWithSettings(store, func() config.Settings { return config.Settings{SourceMode: config.SourceModeXtream} })
	server.timeShift = timeshift.NewManager(t.TempDir())
	response, _ := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{
		Path: "/dispatcharr/api/timeshift/start", Method: http.MethodPost, Body: []byte(`{"channelId":"channel"}`),
	})
	if response.GetStatusCode() != http.StatusConflict {
		t.Fatalf("expected unavailable response, got %d", response.GetStatusCode())
	}
}
