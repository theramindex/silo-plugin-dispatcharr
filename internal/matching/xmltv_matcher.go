package matching

import (
	"strings"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/m3u"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xmltv"
)

type Index struct {
	byID   map[string]xmltv.Channel
	byName map[string]xmltv.Channel
}

func NewIndex(doc xmltv.Document) *Index {
	index := &Index{
		byID:   make(map[string]xmltv.Channel, len(doc.Channels)),
		byName: map[string]xmltv.Channel{},
	}
	for _, channel := range doc.Channels {
		for _, key := range matchKeys(channel.ID) {
			index.byID[key] = channel
		}
		for _, displayName := range channel.DisplayNames {
			for _, key := range matchKeys(displayName) {
				if _, exists := index.byName[key]; !exists {
					index.byName[key] = channel
				}
			}
		}
	}
	return index
}

func (i *Index) Match(entry m3u.Entry) (xmltv.Channel, bool) {
	if i == nil {
		return xmltv.Channel{}, false
	}
	for _, key := range matchKeys(entry.GuideID, entry.TvgName, entry.Name) {
		if channel, ok := i.byID[key]; ok {
			return channel, true
		}
	}
	for _, key := range matchKeys(entry.TvgName, entry.Name) {
		if channel, ok := i.byName[key]; ok {
			return channel, true
		}
	}
	return xmltv.Channel{}, false
}

func Match(entry m3u.Entry, doc xmltv.Document) (xmltv.Channel, bool) {
	return NewIndex(doc).Match(entry)
}

func matchKeys(values ...string) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(values)*2)
	for _, value := range values {
		normalized := normalize(value)
		stripped := stripQualitySuffix(normalized)
		for _, key := range []string{normalized, stripped} {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func stripQualitySuffix(value string) string {
	for {
		trimmed := value
		for _, suffix := range []string{" uhd", " fhd", " hd", " sd", " 4k", " 1080p", " 720p", " hevc", " hdr"} {
			if strings.HasSuffix(value, suffix) {
				value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
			}
		}
		if value == trimmed {
			return value
		}
	}
}
