package matching

import (
	"testing"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/m3u"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xmltv"
)

func TestMatchUsesTvgNameAndStripsQualitySuffix(t *testing.T) {
	t.Parallel()

	doc := xmltv.Document{Channels: []xmltv.Channel{{ID: "espn.us", DisplayNames: []string{"ESPN"}}}}
	match, ok := Match(m3u.Entry{Name: "ESPN HD", TvgName: "ESPN"}, doc)
	if !ok || match.ID != "espn.us" {
		t.Fatalf("expected tvg-name/quality-stripped match, got %+v ok=%t", match, ok)
	}
	match, ok = Match(m3u.Entry{Name: "ESPN 4K"}, doc)
	if !ok || match.ID != "espn.us" {
		t.Fatalf("expected trailing quality suffix to match display name, got %+v ok=%t", match, ok)
	}
}

func TestMatchGuideIDPreferred(t *testing.T) {
	t.Parallel()

	entry := m3u.Entry{GuideID: "news.hd", Name: "News HD"}
	doc := xmltv.Document{Channels: []xmltv.Channel{{ID: "news.hd", DisplayNames: []string{"News HD"}}}}
	match, ok := Match(entry, doc)
	if !ok {
		t.Fatal("expected match")
	}
	if match.ID != "news.hd" {
		t.Fatalf("expected guide id match, got %+v", match)
	}
}
