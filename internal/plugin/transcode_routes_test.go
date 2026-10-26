package plugin

import (
	"context"
	"testing"

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
