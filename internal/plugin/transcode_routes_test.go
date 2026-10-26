package plugin

import (
	"context"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/transcode"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestTranscodeRoutesRequireIdentityAndCatalogChannel(t *testing.T) {
	server := NewHTTPRoutesServer(cache.NewStore())
	query, _ := structpb.NewStruct(map[string]any{"session_id": "unknown", "name": "index.m3u8"})
	cases := []struct {
		path, method string
		body         string
		headers      map[string]string
		want         int
	}{
		{"/dispatcharr/api/transcode/start", "POST", `{"channelId":"one"}`, nil, 401},
		{"/dispatcharr/transcode/media", "GET", "", nil, 401},
		{"/dispatcharr/api/transcode/start", "POST", `{"channelId":"one"}`, map[string]string{"X-Silo-User-Id": "spoofed"}, 401},
		{"/dispatcharr/api/transcode/start", "GET", "", map[string]string{"X-Silo-User-Id": "1"}, 405},
		{"/dispatcharr/api/transcode/start", "POST", `{"url":"http://localhost/private"}`, map[string]string{"X-Silo-User-Id": "1"}, 400},
		{"/dispatcharr/api/transcode/start", "POST", `{"channelId":"missing"}`, map[string]string{"X-Silo-User-Id": "1"}, 404},
		{"/dispatcharr/transcode/media", "GET", "", map[string]string{"X-Silo-User-Id": "1"}, 404},
	}
	for _, tc := range cases {
		response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Path: tc.path, Method: tc.method, Headers: tc.headers, Body: []byte(tc.body), Query: query})
		if err != nil || int(response.GetStatusCode()) != tc.want {
			t.Fatalf("%s %s got %d, want %d (err %v)", tc.method, tc.path, response.GetStatusCode(), tc.want, err)
		}
	}
}

// This boundary owns URL rewriting: a codec test cannot detect an HLS segment
// escaping Silo's installation-specific plugin prefix.
func TestConversionPlaylistKeepsSegmentsInsideSiloPluginRoute(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "encoder")
	script := "#!/bin/sh\nfor arg; do out=\"$arg\"; done\ndir=$(dirname \"$out\")\nprintf video > \"$dir/segment-000000000.ts\"\nprintf '#EXTM3U\\n#EXT-X-TARGETDURATION:2\\n#EXTINF:2,\\nsegment-000000000.ts\\n' > \"$out\"\nexec sleep 120\n"
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	manager := transcode.New(transcode.Config{FFmpegPath: binary, Device: binary, Root: dir})
	defer manager.Close()
	session, err := manager.Start("1:", "channel", "https://example.com/live.ts", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		status, err := manager.Status(session.ID, "1:")
		if err != nil {
			t.Fatal(err)
		}
		if status.State == "ready" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	server := NewHTTPRoutesServer(cache.NewStore())
	server.transcoder = manager
	base, _ := url.Parse("https://silo.example/api/v2/plugin-content/plugins/14/dispatcharr/transcode/media?session_id=" + session.ID + "&name=index.m3u8")
	get := func(user string, u *url.URL) *pluginv1.HandleHTTPResponse {
		t.Helper()
		values := map[string]any{}
		for k := range u.Query() {
			values[k] = u.Query().Get(k)
		}
		query, _ := structpb.NewStruct(values)
		response, err := server.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Path: "/dispatcharr/transcode/media", Method: "GET", Headers: map[string]string{"X-Silo-User-Id": user}, Query: query})
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	playlist := get("1", base)
	if playlist.GetStatusCode() != 200 {
		t.Fatalf("playlist: %d %s", playlist.GetStatusCode(), playlist.GetBody())
	}
	var segment *url.URL
	for _, line := range strings.Split(string(playlist.GetBody()), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			ref, err := url.Parse(line)
			if err != nil {
				t.Fatal(err)
			}
			segment = base.ResolveReference(ref)
			break
		}
	}
	if segment == nil || segment.Path != base.Path {
		t.Fatalf("segment escaped installation path: %v", segment)
	}
	response := get("1", segment)
	if response.GetStatusCode() != 200 || string(response.GetBody()) != "video" {
		t.Fatalf("segment: %d %s", response.GetStatusCode(), response.GetBody())
	}
	if response = get("2", segment); response.GetStatusCode() != 404 {
		t.Fatalf("other viewer read segment: %d", response.GetStatusCode())
	}
}
