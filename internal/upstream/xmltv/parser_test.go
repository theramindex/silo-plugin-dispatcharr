package xmltv

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseXMLTVExtractsChannelsAndProgrammes(t *testing.T) {
	t.Parallel()

	data := readFixture(t, "sample.xml")
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse xmltv: %v", err)
	}
	if len(doc.Channels) != 1 || len(doc.Programmes) != 1 {
		t.Fatalf("unexpected parsed document: %+v", doc)
	}
	if doc.Channels[0].ID != "news.hd" || doc.Programmes[0].Channel != "news.hd" {
		t.Fatalf("unexpected parsed values: %+v %+v", doc.Channels[0], doc.Programmes[0])
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "xmltv", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestParseReaderStreamsFixtureLikeParse(t *testing.T) {
	t.Parallel()

	data := readFixture(t, "sample.xml")
	want, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := ParseReader(bytes.NewReader(data), ParseOptions{MaxBytes: int64(len(data))})
	if err != nil {
		t.Fatalf("parse reader: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("streamed document differs:\nwant %+v\ngot  %+v", want, got)
	}
}

func TestParseReaderFiltersProgrammesByChannel(t *testing.T) {
	t.Parallel()

	input := `<?xml version="1.0"?>
<tv generator-info-name="x">
  <channel id="a.us"><display-name>A</display-name></channel>
  <channel id="b.us"><display-name>B</display-name></channel>
  <unknown><nested>ignored</nested></unknown>
  <programme channel="a.us" start="20260101000000 +0000" stop="20260101010000 +0000"><title>A1</title><category>News</category><icon src="https://img/a"/></programme>
  <programme channel="b.us" start="20260101000000 +0000" stop="20260101010000 +0000"><title>B1</title></programme>
  <programme channel=" A.US " start="20260101010000 +0000" stop="20260101020000 +0000"><title>A2</title></programme>
</tv>`
	doc, err := ParseReader(strings.NewReader(input), ParseOptions{ChannelIDs: map[string]struct{}{"a.us": {}}})
	if err != nil {
		t.Fatalf("parse reader: %v", err)
	}
	if len(doc.Channels) != 2 {
		t.Fatalf("channels should not be filtered, got %d", len(doc.Channels))
	}
	if len(doc.Programmes) != 2 || doc.Programmes[0].Title != "A1" || doc.Programmes[1].Title != "A2" {
		t.Fatalf("unexpected programmes: %+v", doc.Programmes)
	}
	if doc.Programmes[0].Icon.Src != "https://img/a" || len(doc.Programmes[0].Categories) != 1 {
		t.Fatalf("programme fields not decoded: %+v", doc.Programmes[0])
	}
}

func TestParseReaderRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	input := `<tv>` + strings.Repeat(`<programme channel="a" start="1" stop="2"><title>x</title></programme>`, 100) + `</tv>`
	_, err := ParseReader(strings.NewReader(input), ParseOptions{MaxBytes: 128})
	if !errors.Is(err, ErrDocumentTooLarge) {
		t.Fatalf("expected ErrDocumentTooLarge, got %v", err)
	}
	if _, err := ParseReader(strings.NewReader(input), ParseOptions{MaxBytes: int64(len(input))}); err != nil {
		t.Fatalf("exact-size input should parse: %v", err)
	}
}

func TestParseReaderRejectsWrongRoot(t *testing.T) {
	t.Parallel()

	if _, err := ParseReader(strings.NewReader(`<html><channel id="x"/></html>`), ParseOptions{}); err == nil {
		t.Fatal("expected error for non-tv root")
	}
}
