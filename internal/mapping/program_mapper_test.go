package mapping

import (
	"encoding/json"
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
