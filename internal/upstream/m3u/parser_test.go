package m3u

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePlaylistExtractsChannels(t *testing.T) {
	t.Parallel()

	data := readFixture(t, "sample.m3u")
	entries, err := Parse(data)
	if err != nil {
		t.Fatalf("parse playlist: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].GuideID != "news.hd" || entries[0].Name != "News HD" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
}

func TestParsePlaylistCapturesTvgName(t *testing.T) {
	t.Parallel()

	entries, err := Parse([]byte("#EXTM3U\n#EXTINF:-1 tvg-id=\"espn.us\" tvg-name=\"ESPN\",ESPN HD\nhttps://example.test/espn\n"))
	if err != nil {
		t.Fatalf("parse playlist: %v", err)
	}
	if len(entries) != 1 || entries[0].TvgName != "ESPN" || entries[0].Name != "ESPN HD" {
		t.Fatalf("expected tvg-name to be captured, got %+v", entries)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "m3u", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}
