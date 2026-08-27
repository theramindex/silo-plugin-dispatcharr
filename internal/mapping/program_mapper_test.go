package mapping

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/dispatcharr"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xmltv"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

func TestMapXtreamProgramProducesStableProgramModel(t *testing.T) {
	t.Parallel()

	program := MapXtreamProgram("xtream:1001", xtream.EPGListing{ID: "epg-1", Title: "Morning News", Description: "Top headlines.", StartTimestamp: "1700000000", StopTimestamp: "1700003600"})
	if program.ID != "program:epg-1" {
		t.Fatalf("expected program id, got %q", program.ID)
	}
	if program.ChannelID != "xtream:1001" || program.Title != "Morning News" || program.Summary != "Top headlines." {
		t.Fatalf("unexpected program mapping: %+v", program)
	}
	if program.StartUnix != 1700000000 || program.EndUnix != 1700003600 {
		t.Fatalf("unexpected program timing: %+v", program)
	}
}

func TestMapDispatcharrProgramPreservesRichSearchCategories(t *testing.T) {
	t.Parallel()

	var upstream dispatcharr.Program
	if err := json.Unmarshal([]byte(`{"id":"epg-1","title":"WNBA Basketball: Indiana Fever at Chicago Sky","start_time":"2026-08-25T20:00:00Z","end_time":"2026-08-25T22:00:00Z","custom_properties":{"categories":["Sports event","Basketball"]}}`), &upstream); err != nil {
		t.Fatalf("decode Dispatcharr rich program: %v", err)
	}
	program := MapDispatcharrProgram("dispatcharr:espn", upstream)
	if len(program.Categories) != 2 || program.Categories[0] != "Sports event" || program.Categories[1] != "Basketball" {
		t.Fatalf("expected Dispatcharr sports categories to survive mapping, got %+v", program.Categories)
	}
}

func TestMapDispatcharrProgramPreservesProgramArtwork(t *testing.T) {
	t.Parallel()

	// Fixture mirrors Dispatcharr's ProgramSearchResultSerializer contract and
	// XMLTV custom_properties extraction (icon plus images[{url,type}]).
	payload, err := os.ReadFile("testdata/dispatcharr-program-search-artwork.json")
	if err != nil {
		t.Fatalf("read Dispatcharr program search fixture: %v", err)
	}
	var result dispatcharr.ProgramSearchResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode Dispatcharr artwork program: %v", err)
	}
	program := MapDispatcharrProgram("dispatcharr:music", result.Program)
	if program.ImageURL != "https://images.example/backdrop.jpg" {
		t.Fatalf("expected the landscape program image to survive mapping, got %q", program.ImageURL)
	}
}

func TestDispatcharrProgramArtworkURLPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		properties dispatcharr.ProgramCustomProperties
		want       string
	}{
		{name: "icon only", properties: dispatcharr.ProgramCustomProperties{Icon: "icon"}, want: "icon"},
		{name: "poster over icon", properties: dispatcharr.ProgramCustomProperties{Icon: "icon", Images: []dispatcharr.ProgramImage{{URL: "poster", Type: "poster"}}}, want: "poster"},
		{name: "backdrop over poster", properties: dispatcharr.ProgramCustomProperties{Images: []dispatcharr.ProgramImage{{URL: "poster", Type: "poster"}, {URL: "backdrop", Type: "backdrop"}}}, want: "backdrop"},
		{name: "landscape alias", properties: dispatcharr.ProgramCustomProperties{Images: []dispatcharr.ProgramImage{{URL: "wide", Type: "landscape"}}}, want: "wide"},
		{name: "blank ignored", properties: dispatcharr.ProgramCustomProperties{Icon: "icon", Images: []dispatcharr.ProgramImage{{URL: "", Type: "backdrop"}}}, want: "icon"},
		{name: "unknown fallback", properties: dispatcharr.ProgramCustomProperties{Images: []dispatcharr.ProgramImage{{URL: "generic", Type: "banner"}}}, want: "generic"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := dispatcharrProgramArtworkURL(test.properties); got != test.want {
				t.Fatalf("dispatcharrProgramArtworkURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMapXMLTVProgrammePreservesSportsCategories(t *testing.T) {
	t.Parallel()

	doc, err := xmltv.Parse([]byte(`<tv><programme channel="espn.us" start="20260825200000 +0000" stop="20260825230000 +0000"><title>College Football: Alabama at Indiana</title><category>Sports event</category><category>Football</category></programme></tv>`))
	if err != nil || len(doc.Programmes) != 1 {
		t.Fatalf("expected one parsed XMLTV programme, got %+v, %v", doc.Programmes, err)
	}
	program := MapXMLTVProgramme("m3u:espn", doc.Programmes[0])
	if len(program.Categories) != 2 || program.Categories[0] != "Sports event" || program.Categories[1] != "Football" {
		t.Fatalf("expected XMLTV sports categories to survive mapping, got %+v", program.Categories)
	}
}

func TestMapXMLTVProgrammePreservesProgramIcon(t *testing.T) {
	t.Parallel()

	doc, err := xmltv.Parse([]byte(`<tv><programme channel="music.us" start="20260825200000 +0000" stop="20260825230000 +0000"><title>Festival Special</title><icon src="https://images.example/festival.jpg" /></programme></tv>`))
	if err != nil || len(doc.Programmes) != 1 {
		t.Fatalf("expected one parsed XMLTV programme, got %+v, %v", doc.Programmes, err)
	}
	program := MapXMLTVProgramme("m3u:music", doc.Programmes[0])
	if program.ImageURL != "https://images.example/festival.jpg" {
		t.Fatalf("expected XMLTV programme artwork to survive mapping, got %q", program.ImageURL)
	}
}
