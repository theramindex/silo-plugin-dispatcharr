package xmltv

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Document struct {
	XMLName    xml.Name    `xml:"tv"`
	Channels   []Channel   `xml:"channel"`
	Programmes []Programme `xml:"programme"`
}

type Channel struct {
	ID           string   `xml:"id,attr"`
	DisplayNames []string `xml:"display-name"`
}

type Programme struct {
	Channel    string   `xml:"channel,attr"`
	Start      string   `xml:"start,attr"`
	Stop       string   `xml:"stop,attr"`
	Title      string   `xml:"title"`
	SubTitle   string   `xml:"sub-title"`
	Desc       string   `xml:"desc"`
	Categories []string `xml:"category"`
	Icon       Icon     `xml:"icon"`
}

type Icon struct {
	Src string `xml:"src,attr"`
}

// ErrDocumentTooLarge is returned by ParseReader when the input exceeds
// ParseOptions.MaxBytes.
var ErrDocumentTooLarge = errors.New("xmltv document exceeds size limit")

// ParseOptions tunes streaming decoding.
type ParseOptions struct {
	// MaxBytes caps how many bytes are read from the reader. Zero means no cap.
	MaxBytes int64
	// ChannelIDs, when non-nil, keeps only programmes whose channel attribute
	// matches one of the IDs (compared case-insensitively after trimming).
	// Channel elements are always retained so guide matching still works.
	ChannelIDs map[string]struct{}
}

// Parse decodes an in-memory XMLTV document.
func Parse(data []byte) (Document, error) {
	return ParseReader(bytes.NewReader(data), ParseOptions{})
}

// ParseReader decodes XMLTV incrementally so only the retained channel and
// programme structs are held in memory, never the raw document body.
func ParseReader(reader io.Reader, options ParseOptions) (Document, error) {
	var counter *limitedReader
	if options.MaxBytes > 0 {
		counter = &limitedReader{reader: reader, remaining: options.MaxBytes}
		reader = counter
	}
	filter := normalizedChannelFilter(options.ChannelIDs)

	var doc Document
	decoder := xml.NewDecoder(reader)
	sawRoot := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if counter != nil && counter.exceeded {
				return Document{}, fmt.Errorf("%w (%d bytes)", ErrDocumentTooLarge, options.MaxBytes)
			}
			return Document{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if !sawRoot {
			if start.Name.Local != "tv" {
				return Document{}, fmt.Errorf("expected element type <tv> but have <%s>", start.Name.Local)
			}
			sawRoot = true
			doc.XMLName = start.Name
			continue
		}
		switch start.Name.Local {
		case "channel":
			var channel Channel
			if err := decoder.DecodeElement(&channel, &start); err != nil {
				return Document{}, decodeError(err, counter, options.MaxBytes)
			}
			doc.Channels = append(doc.Channels, channel)
		case "programme":
			if filter != nil && !filter[normalizeChannelID(attrValue(start, "channel"))] {
				if err := decoder.Skip(); err != nil {
					return Document{}, decodeError(err, counter, options.MaxBytes)
				}
				continue
			}
			var programme Programme
			if err := decoder.DecodeElement(&programme, &start); err != nil {
				return Document{}, decodeError(err, counter, options.MaxBytes)
			}
			doc.Programmes = append(doc.Programmes, programme)
		default:
			// Unknown elements under <tv> are skipped without buffering.
			if err := decoder.Skip(); err != nil {
				return Document{}, decodeError(err, counter, options.MaxBytes)
			}
		}
	}
	if counter != nil && counter.exceeded {
		return Document{}, fmt.Errorf("%w (%d bytes)", ErrDocumentTooLarge, options.MaxBytes)
	}
	if !sawRoot {
		return Document{}, fmt.Errorf("xmltv: missing <tv> root element")
	}
	return doc, nil
}

func decodeError(err error, counter *limitedReader, limit int64) error {
	if counter != nil && counter.exceeded {
		return fmt.Errorf("%w (%d bytes)", ErrDocumentTooLarge, limit)
	}
	return err
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func normalizedChannelFilter(ids map[string]struct{}) map[string]bool {
	if ids == nil {
		return nil
	}
	filter := make(map[string]bool, len(ids))
	for id := range ids {
		if key := normalizeChannelID(id); key != "" {
			filter[key] = true
		}
	}
	return filter
}

func normalizeChannelID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// limitedReader behaves like io.LimitReader but remembers whether the source
// had more data than allowed, so oversize input is reported instead of being
// silently truncated into a parse error.
type limitedReader struct {
	reader    io.Reader
	remaining int64
	exceeded  bool
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		var probe [1]byte
		n, _ := l.reader.Read(probe[:])
		if n > 0 {
			l.exceeded = true
			return 0, ErrDocumentTooLarge
		}
		return 0, io.EOF
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.reader.Read(p)
	l.remaining -= int64(n)
	return n, err
}
