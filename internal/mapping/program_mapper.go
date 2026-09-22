package mapping

import (
	"strconv"
	"strings"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xmltv"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/upstream/xtream"
)

func MapXtreamProgram(channelID string, listing xtream.EPGListing) model.Program {
	startUnix, _ := strconv.ParseInt(listing.StartTimestamp, 10, 64)
	endUnix, _ := strconv.ParseInt(listing.StopTimestamp, 10, 64)
	title := GuideProgramTitle(listing.Title)

	return model.Program{
		ID: model.StableProgramID(model.ProgramIdentity{
			UpstreamID: listing.ID,
			ChannelID:  channelID,
			Title:      title,
			StartUnix:  startUnix,
		}),
		ChannelID: channelID,
		Title:     title,
		Summary:   listing.Description,
		StartUnix: startUnix,
		EndUnix:   endUnix,
	}
}

func MapXMLTVProgramme(channelID string, programme xmltv.Programme) model.Program {
	startUnix := parseXMLTVTime(programme.Start)
	endUnix := parseXMLTVTime(programme.Stop)
	title := GuideProgramTitle(programme.Title)
	return model.Program{
		ID:         model.StableProgramID(model.ProgramIdentity{ChannelID: channelID, Title: title, StartUnix: startUnix}),
		ChannelID:  channelID,
		Title:      title,
		Summary:    programme.Desc,
		ImageURL:   programme.Icon.Src,
		Categories: append([]string(nil), programme.Categories...),
		StartUnix:  startUnix,
		EndUnix:    endUnix,
	}
}

func GuideProgramTitle(values ...string) string {
	for _, value := range values {
		title := strings.TrimSpace(value)
		if title == "" || IsPlaceholderProgramTitle(title) {
			continue
		}
		return title
	}
	return ""
}

func KeepGuideProgram(program model.Program) bool {
	return strings.TrimSpace(program.Title) != "" && !IsPlaceholderProgramTitle(program.Title)
}

func IsPlaceholderProgramTitle(title string) bool {
	normalized := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(title))), " ")
	switch normalized {
	case "", "tba", "to be announced", "sendepause", "sign off", "off air", "no games today", "no game today", "data not available", "no data available", "no guide data available", "no information available", "no programming available", "programming unavailable", "information not available":
		return true
	default:
		return false
	}
}

func parseXMLTVTime(value string) int64 {
	parsed, err := time.Parse("20060102150405 -0700", value)
	if err != nil {
		return 0
	}
	return parsed.Unix()
}
