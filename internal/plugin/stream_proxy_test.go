package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStreamURLExposesSecrets(t *testing.T) {
	t.Parallel()
	xtream := config.Settings{SourceMode: config.SourceModeXtream, XtreamUsername: "demo", XtreamPassword: "p@ss word!"}
	m3u := config.Settings{SourceMode: config.SourceModeM3UXMLTV, M3UURL: "http://provider.example/get.php"}
	direct := config.Settings{SourceMode: config.SourceModeDirectLogin, DispatcharrPass: "hunter22", DispatcharrAPIKey: "abcdef123456"}
	tests := []struct {
		name     string
		url      string
		settings config.Settings
		want     bool
	}{
		{"dispatcharr proxy ts", "http://dispatcharr.lan:9191/proxy/ts/stream/2f1c9a3e-0000-4000-8000-000000000001", direct, false},
		{"public hls", "https://cdn.example.com/hls/news/index.m3u8", m3u, false},
		{"plain query", "https://cdn.example.com/live.m3u8?profile=2&output=hls", m3u, false},
		{"userinfo", "http://user:pass@provider.example/stream.ts", m3u, true},
		{"userinfo username only", "http://user@provider.example/stream.ts", m3u, true},
		{"xtream live path", "http://provider.example/live/u/p/1001.ts", m3u, true},
		{"xtream timeshift path", "http://provider.example/timeshift/u/p/60/2024-01-01:10-00/1001.ts", m3u, true},
		{"xtream style in m3u mode", "http://provider.example:8080/USER/PASS/12345", m3u, true},
		{"xtream style with ext", "http://provider.example:8080/USER/PASS/12345.ts", m3u, true},
		{"username password query", "http://provider.example/stream?username=a&password=b", m3u, true},
		{"token query", "https://cdn.example.com/live.m3u8?token=abc", m3u, true},
		{"uppercase api key", "https://cdn.example.com/live.m3u8?API-KEY=abc", m3u, true},
		{"apikey", "https://cdn.example.com/live.m3u8?apikey=abc", m3u, true},
		{"key", "https://cdn.example.com/live.m3u8?key=abc", m3u, true},
		{"auth", "https://cdn.example.com/live.m3u8?auth=abc", m3u, true},
		{"pass", "https://cdn.example.com/live.m3u8?pass=abc", m3u, true},
		{"signature", "https://cdn.example.com/live.m3u8?Signature=abc", m3u, true},
		{"sig", "https://cdn.example.com/live.m3u8?sig=abc", m3u, true},
		{"access token", "https://cdn.example.com/live.m3u8?access_token=abc", m3u, true},
		{"empty token value", "https://cdn.example.com/live.m3u8?token=", m3u, false},
		{"escaped configured password in path", "http://provider.example/play/demo/p%40ss%20word%21/stream", xtream, true},
		{"query-escaped configured password", "http://provider.example/play?x=p%40ss+word%21", xtream, true},
		{"configured password as segment", "http://dispatcharr.lan/proxy/hunter22/stream", direct, true},
		{"configured api key in query", "http://dispatcharr.lan/proxy/ts/stream/x?ak=abcdef123456", direct, true},
		{"unparseable", "http://[::1", m3u, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := streamURLExposesSecrets(test.url, test.settings); got != test.want {
				t.Fatalf("streamURLExposesSecrets(%q) = %v, want %v", test.url, got, test.want)
			}
		})
	}
}

func newStreamProxyTestServer(settings config.Settings) *HTTPRoutesServer {
	return NewHTTPRoutesServerWithSettings(cache.NewStore(), func() config.Settings { return settings })
}

func TestStreamProxyRejectsEndlessTSWithStructuredError(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		packet := make([]byte, 188)
		packet[0] = 0x47
		for {
			if _, err := w.Write(packet); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}))
	t.Cleanup(upstream.Close)
	server := newStreamProxyTestServer(config.Settings{SourceMode: config.SourceModeM3UXMLTV, M3UURL: upstream.URL + "/get.php"})

	started := time.Now()
	response := server.serveProviderStream(context.Background(), upstream.URL+"/USER/PASS/12345.ts", &pluginv1.HandleHTTPRequest{})
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("endless stream took %s to reject", elapsed)
	}
	assertStreamRequiresHLS(t, response)
}

func TestStreamProxyRejectsOversizedVOD(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", fmt.Sprint(int64(4)<<30))
		_, _ = w.Write([]byte("ftypisom"))
	}))
	t.Cleanup(upstream.Close)
	server := newStreamProxyTestServer(config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: upstream.URL, XtreamUsername: "demo", XtreamPassword: "secret"})

	response := server.serveProviderStream(context.Background(), upstream.URL+"/movie/demo/secret/2001.mp4", &pluginv1.HandleHTTPRequest{})
	assertStreamRequiresHLS(t, response)
}

func assertStreamRequiresHLS(t *testing.T, response *pluginv1.HandleHTTPResponse) {
	t.Helper()
	if response.GetStatusCode() != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d %q", response.GetStatusCode(), response.GetBody())
	}
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.GetBody(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != "stream_requires_hls" || !strings.Contains(payload.Error, "HLS") || !strings.Contains(payload.Error, "Dispatcharr Direct") {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
	if strings.Contains(string(response.GetBody()), "secret") || strings.Contains(string(response.GetBody()), "PASS") {
		t.Fatalf("error payload leaked credentials: %q", response.GetBody())
	}
}

func TestStreamProxyRewritesTagURIsAndRelativeSegments(t *testing.T) {
	t.Parallel()
	server := newStreamProxyTestServer(config.Settings{})
	playlist := "#EXTM3U\n" +
		`#EXT-X-KEY:METHOD=AES-128,URI="key.bin?token=abc",IV=0x1` + "\n" +
		`#EXT-X-MAP:URI="/init.mp4"` + "\n" +
		"#EXTINF:2,\nseg-1.ts\n" +
		"#EXTINF:2,\nrtmp://provider.example/seg-2\n"
	rewritten, err := server.rewriteHLSPlaylist("http://provider.example/live/u/p/index.m3u8", "provider.example", []byte(playlist))
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	body := string(rewritten)
	if strings.Contains(body, "token=abc") || strings.Contains(body, "/live/u/p/") || strings.Contains(body, "rtmp://") {
		t.Fatalf("rewritten playlist leaked upstream URLs: %q", body)
	}
	if strings.Count(body, "/dispatcharr/stream/asset?ticket=") != 3 || !strings.Contains(body, "IV=0x1") {
		t.Fatalf("expected key, map, and segment to be rewritten: %q", body)
	}
	wantURLs := map[string]bool{
		"http://provider.example/live/u/p/key.bin?token=abc": false,
		"http://provider.example/init.mp4":                   false,
		"http://provider.example/live/u/p/seg-1.ts":          false,
	}
	for _, line := range strings.Split(body, "\n") {
		index := strings.Index(line, "ticket=")
		if index < 0 {
			continue
		}
		ticket := strings.TrimRight(line[index+len("ticket="):], `"`)
		ticket = strings.SplitN(ticket, `"`, 2)[0]
		target, host := server.streamTicketURL(ticket)
		if _, ok := wantURLs[target]; !ok || host != "provider.example" {
			t.Fatalf("ticket %q resolved to %q (host %q)", ticket, target, host)
		}
		wantURLs[target] = true
	}
	for target, seen := range wantURLs {
		if !seen {
			t.Fatalf("missing ticket for %s", target)
		}
	}
}

func TestStreamTicketsSlideReuseAndEvictByPlaylist(t *testing.T) {
	t.Parallel()
	server := newStreamProxyTestServer(config.Settings{})
	targets := func(prefix string, count int) []string {
		out := make([]string, count)
		for index := range out {
			out[index] = fmt.Sprintf("http://provider.example/%s/seg-%d.ts", prefix, index)
		}
		return out
	}

	// Re-fetching the same playlist reuses tickets instead of growing the store.
	first := server.issueStreamTickets("http://provider.example/a.m3u8", "", targets("a", 10))
	again := server.issueStreamTickets("http://provider.example/a.m3u8", "", targets("a", 10))
	if first[3] != again[3] || len(server.streamTickets) != 10 {
		t.Fatalf("expected reused tickets, got %d stored", len(server.streamTickets))
	}

	// Sliding TTL: any access keeps the whole playlist alive.
	server.streamTicketsMu.Lock()
	server.streamTickets[first[0]].group.lastAccess = time.Now().Add(-streamTicketIdleTTL + time.Minute)
	server.streamTicketsMu.Unlock()
	if target, _ := server.streamTicketURL(first[0]); target == "" {
		t.Fatal("expected ticket inside the sliding window to resolve")
	}
	if target, _ := server.streamTicketURL(first[9]); target == "" {
		t.Fatal("expected access to refresh every ticket in the playlist")
	}
	server.streamTicketsMu.Lock()
	server.streamTickets[first[0]].group.lastAccess = time.Now().Add(-streamTicketIdleTTL - time.Minute)
	server.streamTicketsMu.Unlock()
	if target, _ := server.streamTicketURL(first[5]); target != "" {
		t.Fatal("expected idle playlist to expire")
	}
	if len(server.streamTickets) != 0 {
		t.Fatalf("expected expired playlist to be dropped, %d left", len(server.streamTickets))
	}

	// A long VOD playlist larger than half the cap survives; the least
	// recently used playlist is evicted as a whole to make room.
	older := server.issueStreamTickets("http://provider.example/old.m3u8", "", targets("old", streamTicketLimit/2))
	vod := server.issueStreamTickets("http://provider.example/vod.m3u8", "", targets("vod", streamTicketLimit/2+100))
	if target, _ := server.streamTicketURL(older[0]); target != "" {
		t.Fatal("expected least recently used playlist to be evicted")
	}
	for _, index := range []int{0, len(vod) / 2, len(vod) - 1} {
		if target, _ := server.streamTicketURL(vod[index]); target == "" {
			t.Fatalf("current playlist lost ticket %d", index)
		}
	}

	// A single playlist over the cap never evicts its own tickets.
	huge := server.issueStreamTickets("http://provider.example/huge.m3u8", "", targets("huge", streamTicketLimit+10))
	if target, _ := server.streamTicketURL(huge[0]); target == "" {
		t.Fatal("playlist evicted its own first ticket")
	}
	if target, _ := server.streamTicketURL(huge[len(huge)-1]); target == "" {
		t.Fatal("playlist evicted its own last ticket")
	}
}

func TestStreamProxyBlocksPlaylistTargetsOnUnconfiguredPrivateHosts(t *testing.T) {
	t.Parallel()
	var internalHits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits.Add(1)
		_, _ = w.Write([]byte("INTERNAL"))
	}))
	t.Cleanup(internal.Close)
	// "localhost" is not configured; it resolves to loopback at dial time.
	internalURL := strings.Replace(internal.URL, "127.0.0.1", "localhost", 1) + "/admin"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/seg.ts") {
			w.Header().Set("Content-Type", "video/mp2t")
			_, _ = w.Write([]byte("TSSEG"))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:1,\nseg.ts\n#EXTINF:1,\n" + internalURL + "\n"))
	}))
	t.Cleanup(upstream.Close)
	// The configured source is on 127.0.0.1, which must stay reachable.
	server := newStreamProxyTestServer(config.Settings{SourceMode: config.SourceModeXtream, XtreamBaseURL: upstream.URL, XtreamUsername: "demo", XtreamPassword: "secret"})

	response := server.serveProviderStream(context.Background(), upstream.URL+"/live/demo/secret/1001.m3u8", &pluginv1.HandleHTTPRequest{})
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected proxied playlist, got %d %q", response.GetStatusCode(), response.GetBody())
	}
	var tickets []string
	for _, line := range strings.Split(string(response.GetBody()), "\n") {
		if index := strings.Index(line, "ticket="); index >= 0 {
			tickets = append(tickets, line[index+len("ticket="):])
		}
	}
	if len(tickets) != 2 {
		t.Fatalf("expected two tickets, got %q", response.GetBody())
	}
	fetch := func(ticket string) *pluginv1.HandleHTTPResponse {
		query, _ := structpb.NewStruct(map[string]any{"ticket": ticket})
		return server.handleStreamAsset(context.Background(), &pluginv1.HandleHTTPRequest{Method: http.MethodGet, Query: query})
	}
	if allowed := fetch(tickets[0]); allowed.GetStatusCode() != http.StatusOK || string(allowed.GetBody()) != "TSSEG" {
		t.Fatalf("expected configured host segment, got %d %q", allowed.GetStatusCode(), allowed.GetBody())
	}
	if blocked := fetch(tickets[1]); blocked.GetStatusCode() != http.StatusBadGateway {
		t.Fatalf("expected blocked internal target, got %d %q", blocked.GetStatusCode(), blocked.GetBody())
	}
	if internalHits.Load() != 0 {
		t.Fatal("proxy reached an unconfigured private host")
	}
}
