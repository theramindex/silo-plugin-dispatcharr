package mapping

import (
	"regexp"
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
	title, categories := guideDisplayTitle(programme.Title, programme.SubTitle, programme.Categories)
	return model.Program{
		ID:         model.StableProgramID(model.ProgramIdentity{ChannelID: channelID, Title: title, StartUnix: startUnix}),
		ChannelID:  channelID,
		Title:      title,
		Summary:    programme.Desc,
		ImageURL:   programme.Icon.Src,
		Categories: categories,
		StartUnix:  startUnix,
		EndUnix:    endUnix,
	}
}

var guideMatchupSeparator = regexp.MustCompile(`(?i)\s+(?:vs\.?|v\.?|at|@)\s+`)
var guidePhaseLabelPrefix = regexp.MustCompile(`(?i)^(?:pre[\s-]?game|post[\s-]?game|live)\s*[:|\-]\s*`)

// Generic league and phase titles hide the matchup in the subtitle. Episode
// subtitles stay put because the programme title is not one of these labels.
var genericSportsGuideTitles = map[string]struct{}{
	"sports": {}, "sport": {}, "game": {}, "live": {},
	"college football": {}, "nfl": {}, "nfl football": {},
	"nba": {}, "nba basketball": {}, "wnba": {},
	"mlb": {}, "mlb baseball": {}, "nhl": {}, "nhl hockey": {},
	"mls": {}, "premier league": {}, "uefa champions league": {}, "champions league": {},
	"ufc": {}, "mma": {}, "boxing": {}, "golf": {}, "pga": {}, "tennis": {},
	"nascar": {}, "nascar cup series": {}, "formula 1": {}, "f1": {}, "cricket": {},
	"pregame": {}, "pre-game": {}, "pre game": {},
	"postgame": {}, "post-game": {}, "post game": {},
}

func guideDisplayTitle(title, subtitle string, categories []string) (string, []string) {
	title = GuideProgramTitle(title, subtitle)
	display, leagueHint := guideSportsMatchupTitle(title, subtitle)
	if leagueHint != "" {
		categories = append(categories, leagueHint)
	}
	return display, categories
}

func guideSportsMatchupTitle(title, subtitle string) (string, string) {
	title = strings.TrimSpace(title)
	subtitle = strings.TrimSpace(subtitle)
	if subtitle == "" || !guideMatchupSeparator.MatchString(subtitle) || guideMatchupSeparator.MatchString(title) || !genericSportsGuideLabel(title) {
		return title, ""
	}
	return subtitle, title
}

func genericSportsGuideLabel(title string) bool {
	label := normalizeGuideLabel(title)
	if label == "" {
		return false
	}
	if _, ok := genericSportsGuideTitles[label]; ok {
		return true
	}
	stripped := strings.TrimSpace(guidePhaseLabelPrefix.ReplaceAllString(title, ""))
	if stripped == "" || stripped == title {
		return stripped == ""
	}
	_, ok := genericSportsGuideTitles[normalizeGuideLabel(stripped)]
	return ok
}

func normalizeGuideLabel(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
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
