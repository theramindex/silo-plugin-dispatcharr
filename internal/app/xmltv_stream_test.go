package app

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestXtreamCustomXMLTVIsStreamedThroughOpenURL(t *testing.T) {
	t.Parallel()

	guide := `<tv><channel id="news.hd"><display-name>News HD</display-name></channel>` +
		`<programme channel="news.hd" start="20260101000000 +0000" stop="20260101010000 +0000"><title>Morning News</title></programme></tv>`
	body := &trackingBody{Reader: strings.NewReader(guide)}
	store := cache.NewStore()
	service := NewService(Dependencies{
		Store: store,
		XtreamFactory: func(string, string, string) XtreamClient {
			return &stubXtreamClient{streams: []xtream.LiveStream{{Num: 1, Name: "News HD", StreamID: 1001, EPGChannelID: "news.hd"}}}
		},
		FetchURL: func(context.Context, string) ([]byte, error) {
			return nil, errors.New("guide must not be buffered via FetchURL")
		},
		OpenURL: func(_ context.Context, rawURL string) (io.ReadCloser, error) {
			if rawURL != "https://guide.example/epg.xml" {
				return nil, errors.New("unexpected url " + rawURL)
			}
			return body, nil
		},
	})
	settings := xtreamTestSettings()
	settings.EPGXMLURL = "https://guide.example/epg.xml"
	if err := service.SyncNow(context.Background(), settings, 200); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !body.closed {
		t.Fatal("expected streamed body to be closed")
	}
	if got := len(store.Current().Catalog.Programs); got != 1 {
		t.Fatalf("expected 1 streamed program, got %d", got)
	}
}

func TestFetchXMLTVFallsBackToInjectedFetchURL(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{FetchURL: func(context.Context, string) ([]byte, error) {
		return []byte(`<tv><programme channel="a" start="1" stop="2"><title>x</title></programme></tv>`), nil
	}})
	doc, err := service.fetchXMLTV(context.Background(), "https://guide.example/epg.xml", "epg", nil)
	if err != nil || len(doc.Programmes) != 1 {
		t.Fatalf("expected fallback parse, got %+v %v", doc, err)
	}
}

func TestFetchXMLTVPrefixesParseErrors(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{FetchURL: func(context.Context, string) ([]byte, error) {
		return []byte(`<html></html>`), nil
	}})
	_, err := service.fetchXMLTV(context.Background(), "https://guide.example/epg.xml", "epg", nil)
	if err == nil || !strings.HasPrefix(err.Error(), "parse epg xmltv:") {
		t.Fatalf("expected parse error prefix, got %v", err)
	}
}
