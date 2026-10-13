package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"google.golang.org/protobuf/types/known/structpb"
)

func liveTVContractServer(mode model.SourceMode, channels []model.Channel, settings config.Settings) *HTTPRoutesServer {
	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{Source: model.LiveTVSource(mode), Channels: channels},
	})
	return NewHTTPRoutesServerWithSettings(store, func() config.Settings { return settings })
}

func liveTVContractGet(t *testing.T, server *HTTPRoutesServer, path string, query map[string]any) *pluginv1.HandleHTTPResponse {
	t.Helper()
	request := &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Path: path}
	if query != nil {
		parsed, err := structpb.NewStruct(query)
		if err != nil {
			t.Fatalf("new query struct: %v", err)
		}
		request.Query = parsed
	}
	response, err := server.Handle(context.Background(), request)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return response
}

func TestLiveTVContractInfo(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeDirectLogin, []model.Channel{
		{ID: "direct:1", Name: "News HD", StreamURL: "https://CDN.example/live/1.m3u8"},
		{ID: "direct:2", Name: "Sports", StreamURL: "https://cdn.example/live/2.m3u8"},
		{ID: "direct:3", Name: "No Stream"},
	}, config.Settings{DispatcharrURL: "http://dispatcharr.example:9191", XtreamBaseURL: "not a url", M3UURL: ""})

	response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/info", nil)
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.GetStatusCode(), response.GetBody())
	}
	var info struct {
		ContractVersion int      `json:"contractVersion"`
		Name            string   `json:"name"`
		StreamHosts     []string `json:"streamHosts"`
	}
	if err := json.Unmarshal(response.GetBody(), &info); err != nil {
		t.Fatalf("decode info: %v", err)
	}
	if info.ContractVersion != 1 || info.Name != "Dispatcharr" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if want := []string{"cdn.example", "dispatcharr.example:9191"}; !reflect.DeepEqual(info.StreamHosts, want) {
		t.Fatalf("streamHosts = %v, want %v", info.StreamHosts, want)
	}
}

func TestLiveTVContractInfoEmptyHostsIsArray(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeM3UXMLTV, nil, config.Settings{})
	response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/info", nil)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(response.GetBody(), &raw); err != nil {
		t.Fatalf("decode info: %v", err)
	}
	if string(raw["streamHosts"]) != "[]" {
		t.Fatalf("streamHosts must be [] when nothing is configured, got %s", raw["streamHosts"])
	}
}

func TestLiveTVContractChannelsMatchesFixtureShape(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeM3UXMLTV, []model.Channel{
		{ID: "m3u:news.hd", Number: "101", Name: "News HD", LogoURL: "http://dispatcharr.example:9191/logos/news.png", CategoryID: "7", CategoryName: "News"},
		{ID: "m3u:sports.one", Number: "201", Name: "Sports One", LogoURL: "data:image/png;base64,AAAA", CategoryID: "8", CategoryName: "Sports"},
	}, config.Settings{})

	response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/channels", nil)
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.GetStatusCode(), response.GetBody())
	}
	var got []map[string]any
	if err := json.Unmarshal(response.GetBody(), &got); err != nil {
		t.Fatalf("decode channels: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "livetv-v1", "fixtures", "channels.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture []map[string]any
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected channels in response")
	}
	for i, item := range got {
		for key := range fixture[0] {
			if _, ok := item[key]; !ok {
				t.Fatalf("channel %d is missing fixture key %q: %v", i, key, item)
			}
		}
	}
	if got[0]["id"] != "m3u:news.hd" || got[0]["number"] != "101" || got[0]["groupId"] != "7" || got[0]["group"] != "News" {
		t.Fatalf("unexpected first channel: %v", got[0])
	}
	if got[0]["logoUrl"] != "http://dispatcharr.example:9191/logos/news.png" {
		t.Fatalf("expected absolute logo to pass through, got %v", got[0]["logoUrl"])
	}
	if got[1]["logoUrl"] != "" {
		t.Fatalf("expected non-http logo to be dropped, got %v", got[1]["logoUrl"])
	}
}

func TestLiveTVContractChannelsEmptyLineupIsArray(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeM3UXMLTV, nil, config.Settings{})
	response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/channels", nil)
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.GetStatusCode())
	}
	if string(response.GetBody()) != "[]" {
		t.Fatalf("empty lineup must be [], got %s", response.GetBody())
	}
}

func decodeLiveTVStream(t *testing.T, response *pluginv1.HandleHTTPResponse) livetvStream {
	t.Helper()
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.GetStatusCode(), response.GetBody())
	}
	var stream livetvStream
	if err := json.Unmarshal(response.GetBody(), &stream); err != nil {
		t.Fatalf("decode stream: %v", err)
	}
	return stream
}

func decodeLiveTVCode(t *testing.T, response *pluginv1.HandleHTTPResponse) string {
	t.Helper()
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("decode error body %q: %v", response.GetBody(), err)
	}
	return payload.Code
}

func TestLiveTVContractStreamRedirect(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeM3UXMLTV, []model.Channel{
		{ID: "m3u:news.hd", Name: "News HD", StreamURL: "https://dispatcharr.example.com/live/news.m3u8"},
	}, config.Settings{})

	stream := decodeLiveTVStream(t, liveTVContractGet(t, server, "/silo-compat/livetv/v1/stream", map[string]any{"channel_id": "m3u:news.hd"}))
	want := livetvStream{URL: "https://dispatcharr.example.com/live/news.m3u8", Container: "hls", Mode: "redirect"}
	if stream != want {
		t.Fatalf("stream = %+v, want %+v", stream, want)
	}
}

func TestLiveTVContractStreamServerMode(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeXtream, []model.Channel{
		{ID: "xtream:42", Name: "News Tonight", StreamURL: "http://xtream.example/live/u/p/42.ts"},
	}, config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: "http://xtream.example", XtreamUsername: "u", XtreamPassword: "p"})

	stream := decodeLiveTVStream(t, liveTVContractGet(t, server, "/silo-compat/livetv/v1/stream", map[string]any{"channel_id": "xtream:42", "format": "ts"}))
	if stream.Mode != "server" || stream.Container != "ts" || stream.URL != "http://xtream.example/live/u/p/42.ts" {
		t.Fatalf("unexpected stream: %+v", stream)
	}
}

func TestLiveTVContractStreamDirectProxyIsRedirect(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeDirectLogin, []model.Channel{
		{ID: "direct:1", Name: "News HD", StreamURL: "http://dispatcharr.example:9191/proxy/ts/stream/abc"},
	}, config.Settings{DispatcharrURL: "http://dispatcharr.example:9191"})

	stream := decodeLiveTVStream(t, liveTVContractGet(t, server, "/silo-compat/livetv/v1/stream", map[string]any{"channel_id": "direct:1"}))
	if stream.Mode != "redirect" || stream.Container != "ts" {
		t.Fatalf("unexpected stream: %+v", stream)
	}
}

func TestLiveTVContractStreamFormatUnavailable(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeM3UXMLTV, []model.Channel{
		{ID: "m3u:news.hd", Name: "News HD", StreamURL: "https://dispatcharr.example.com/live/news.ts"},
	}, config.Settings{})

	response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/stream", map[string]any{"channel_id": "m3u:news.hd", "format": "hls"})
	if response.GetStatusCode() != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", response.GetStatusCode(), response.GetBody())
	}
	if code := decodeLiveTVCode(t, response); code != "format_unavailable" {
		t.Fatalf("code = %q, want format_unavailable", code)
	}
}

func TestLiveTVContractStreamUnknownChannel(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeM3UXMLTV, nil, config.Settings{})
	response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/stream", map[string]any{"channel_id": "m3u:missing"})
	if response.GetStatusCode() != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", response.GetStatusCode(), response.GetBody())
	}
	if code := decodeLiveTVCode(t, response); code != "channel_not_found" {
		t.Fatalf("code = %q, want channel_not_found", code)
	}
}

func TestLiveTVContractStreamRejectsBadRequests(t *testing.T) {
	t.Parallel()

	server := liveTVContractServer(model.SourceModeM3UXMLTV, []model.Channel{
		{ID: "m3u:news.hd", Name: "News HD", StreamURL: "https://dispatcharr.example.com/live/news.m3u8"},
	}, config.Settings{})
	for name, query := range map[string]map[string]any{
		"missing channel_id": nil,
		"blank channel_id":   {"channel_id": "  "},
		"unknown format":     {"channel_id": "m3u:news.hd", "format": "dash"},
	} {
		response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/stream", query)
		if response.GetStatusCode() != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, response.GetStatusCode(), response.GetBody())
		}
	}
}

func TestManifestDeclaresLiveTVContractRoutes(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		HTTPRoutes []struct {
			ID        string `json:"id"`
			Method    string `json:"method"`
			Path      string `json:"path"`
			Access    string `json:"access"`
			Navigable bool   `json:"navigable"`
		} `json:"http_routes"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	// info and stream are admin-only so Silo's plugin proxy never hands
	// credentialed stream URLs or provider hosts to ordinary users; Silo calls
	// them in-process. channels is how Silo discovers the provider.
	want := map[string]struct{ path, access string }{
		"livetv-v1-info":     {"/silo-compat/livetv/v1/info", "admin"},
		"livetv-v1-channels": {"/silo-compat/livetv/v1/channels", "authenticated"},
		"livetv-v1-programs": {"/silo-compat/livetv/v1/programs", "authenticated"},
		"livetv-v1-stream":   {"/silo-compat/livetv/v1/stream", "admin"},
	}
	found := map[string]bool{}
	for _, route := range manifest.HTTPRoutes {
		expected, ok := want[route.ID]
		if !ok {
			continue
		}
		found[route.ID] = true
		if route.Method != http.MethodGet || route.Path != expected.path || route.Access != expected.access || route.Navigable {
			t.Fatalf("route %s declared incorrectly: %+v", route.ID, route)
		}
	}
	for id := range want {
		if !found[id] {
			t.Fatalf("manifest does not declare %s", id)
		}
	}
}
