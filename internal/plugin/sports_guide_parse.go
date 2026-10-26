package plugin

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

var sportsMatchupSeparator = regexp.MustCompile(`(?i)\s+(?:vs\.?|v\.?|at|@)\s+`)
var formulaERacePattern = regexp.MustCompile(`(?i)\bformul[ae]\s+e\b`)
var nascarCupRacePattern = regexp.MustCompile(`(?i)\b(?:nascar\s+cup\s+series|ncs\s+race)\b`)
var raceLocationPrefix = regexp.MustCompile(`(?i)^\s*(?:v(?:s\.)?|at|@|:|-)\s*`)
var guideSportsTimestampSuffix = regexp.MustCompile(`(?i)\s*\(\d{4}-\d{2}-\d{2}(?:[ t]\d{1,2}:\d{2}(?::\d{2})?)?\)\s*$`)
var guideSportsVenueSuffix = regexp.MustCompile(`\s+_\s+([^_]+?)\s*$`)
var guideSportsNextGameSuffix = regexp.MustCompile(`(?i)\s+on\s+\d{4}-\d{2}-\d{2}\s+at\s+\d{1,2}:\d{2}\s*(?:am|pm)?(?:\s+[a-z]{2,5})?\s*$`)
var guideSportsNextGamePrefix = regexp.MustCompile(`(?i)^\s*next\s+game\s*:\s*`)
var guideSportsNextGameWhen = regexp.MustCompile(`(?i)\bon\s+(\d{4}-\d{2}-\d{2})\s+at\s+(\d{1,2}):(\d{2})\s*(am|pm)?\s*([a-z]{2,5})?\s*$`)

var guideSportsZoneOffsets = map[string]int{"UTC": 0, "GMT": 0, "EDT": -4, "EST": -5, "ET": -4, "CDT": -5, "CST": -6, "CT": -5, "MDT": -6, "MST": -7, "MT": -6, "PDT": -7, "PST": -8, "PT": -7, "BST": 1}

func guideSportsNextGameStart(title string) (int64, bool) {
	match := guideSportsNextGameWhen.FindStringSubmatch(title)
	if match == nil {
		return 0, false
	}
	day, err := time.Parse("2006-01-02", match[1])
	if err != nil {
		return 0, false
	}
	hour, _ := strconv.Atoi(match[2])
	minute, _ := strconv.Atoi(match[3])
	switch strings.ToLower(match[4]) {
	case "pm":
		if hour < 12 {
			hour += 12
		}
	case "am":
		if hour == 12 {
			hour = 0
		}
	}
	offset, ok := guideSportsZoneOffsets[strings.ToUpper(firstNonEmpty(match[5], "ET"))]
	if !ok || hour > 23 || minute > 59 {
		return 0, false
	}
	zone := time.FixedZone(match[5], offset*3600)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, zone).Unix(), true
}

var sportsISODatePattern = regexp.MustCompile(`\b(20\d{2})[-_/](\d{1,2})[-_/](\d{1,2})\b`)
var sportsUSDatePattern = regexp.MustCompile(`\b(\d{1,2})[-_/](\d{1,2})[-_/](20\d{2})\b`)
var guideSportsMatchNumberSuffix = regexp.MustCompile(`(?i)\s*(?:,\s*match\s+\d+|[-,]?\s*\d+(?:st|nd|rd|th)\s+match)\s*$`)
var guideSportsStageSuffix = regexp.MustCompile(`(?i)\s+[-–—]\s+(?:qualifier|eliminator|play[- ]?offs?|quarter[- ]?final|semi[- ]?final|final)(?:\s+\d+)?\s*$`)

// Country codes such as "(NOR)" stay on the team; only network tags are dropped.
var guideSportsNetworkSuffix = regexp.MustCompile(`(?i)\s*\((?:accnx|accn|secn\+?|secplus|sec\s*\+|sec\s+network\s*\+?|espn[2u+]?|espnews|btn\+?|b1g\+|fs[12]|cbssn|nbcsn|peacock|paramount\+|dazn|tsn\+?\d*|sn\d*|flo\w*|nfhs)\)\s*$`)

// Provider event channels append the air time ("@ 24 Sep 07:50 AM ET",
// "SEP 25 06:00 PM"); it is not the second half of a matchup.
var guideSportsAirTimeSuffix = regexp.MustCompile(`(?i)\s*(?:@\s*)?(?:\d{1,2}\s+(?:jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*|(?:jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*\s+\d{1,2})\s+\d{1,2}:\d{2}(?:\s*[ap]\.?m\.?)?(?:\s+[a-z]{2,12})?\s*$`)

var guideSportsDottedDateSuffix = regexp.MustCompile(`(?i)\s*\(\d{1,2}[./]\d{1,2}(?:[./]\d{2,4})?\s+\d{1,2}:\d{2}(?:\s*[ap]\.?m\.?)?(?:\s+[a-z]{2,12})?\)\s*$`)

var guideSportsGameNumberSuffix = regexp.MustCompile(`\s+[-–—]\s+#\d+\s*$`)

var guideSportsClockAtSuffix = regexp.MustCompile(`(?i)\s*@\s*\d{1,2}:\d{2}(?:\s*[ap]\.?m\.?)?(?:\s+[a-z]{2,12})?\s*$`)

var guideSportsFeedSuffix = regexp.MustCompile(`(?i)\s*\(\s*[^()]*\bfeed\s*\)\s*$`)
var guideSportsPhasePrefix = regexp.MustCompile(`(?i)^\s*(?:pre[\s-]?game|post[\s-]?game|live)\s*[:|\-]\s*`)
var guideSportsSideFeedSuffix = regexp.MustCompile(`(?i)\s+(home|away|national)\s*$`)
var guideSportsFeedToken = regexp.MustCompile(`\b(HOME|AWAY|NATIONAL)\b`)
var guideSportsPhaseWord = regexp.MustCompile(`(?i)(?:^|[^a-z])(pre[\s-]?game|post[\s-]?game)(?:[^a-z]|$)`)
var guideSportsLiveLabel = regexp.MustCompile(`(?i)^live$|^live\s*[:|\-]|LIVE`)
var guideSportsNoteLine = regexp.MustCompile(`(?i)(?:^|\n)\s*(venue|broadcast|network|records?|final|score)\s*:\s*([^\n]+)`)
var guideSportsUpperLive = regexp.MustCompile(`\bLIVE\b`)

var guideSportsStagePrefix = regexp.MustCompile(`(?i)^(?:grand\s+|quarter[- ]?|semi[- ]?)?(?:finals?|play[- ]?offs?|qualifiers?|eliminators?)\s*[-–—:]\s+`)

// Tennis listings put the tournament before the first player without a colon.
var guideSportsTournamentPrefix = regexp.MustCompile(`^(?:(?i:atp|wta|itf)\b|.*\s(?i:at\s+the)\s).*\b(?:Open|Cup|Championships?|Masters|Classic|Finals)\s+`)
var guideSportsCompetitionSuffix = regexp.MustCompile(`(?i)\s+[-–—]\s+(?:.*?\b(?:league|cup|championships?|tournament|trophy|qualif\w*|series|games|olympics?)\b|(?:match\s*day|matchday|round|week|leg|group)\s+\w+).*$`)
var guideSportsClockName = regexp.MustCompile(`(?i)^\d{1,2}(?::\d{2})?\s*(?:am|pm)(?:\s+[a-z]{2,5})?$`)
var guideSportsNonMatchTitle = regexp.MustCompile(`(?i)\b(?:good morning|outdoor magazine|the verdict|the case for)\b`)

func guideSportsTitleHasScoreLookupHint(title string) bool {
	return guideSportsTitleIsLive(title) || strings.Contains(title, "ᴺᵉʷ")
}

func guideSportsTitleIsLive(title string) bool {
	return strings.Contains(title, "ᴸᶦᵛᵉ") || containsMatchTerm(normalizeMatchText(title), "live")
}

func sportsEventsFromGuideWithScoreHints(snapshot cache.Snapshot, now time.Time) ([]SportsEvent, bool) {
	categoryNames := map[string]string{}
	for _, category := range liveCategories(snapshot) {
		categoryNames[category.ID] = category.Name
	}
	channels := map[string]model.Channel{}
	for _, channel := range snapshot.Catalog.Channels {
		if channel.ID != "" {
			channels[channel.ID] = channel
		}
	}

	fromUnix := now.Add(-24 * time.Hour).Unix()
	toUnix := now.Add(72 * time.Hour).Unix()
	byKey := map[string]*SportsEvent{}
	refreshScores := false
	for _, program := range snapshot.Catalog.Programs {
		if program.EndUnix < fromUnix || program.StartUnix > toUnix {
			continue
		}
		channel, ok := channels[program.ChannelID]
		if !ok {
			continue
		}
		categoryName := firstNonEmpty(categoryNames[channel.CategoryID], channel.CategoryName)
		phase := guideSportsListingPhase(program.Title, program.Categories)
		displayTitle := stripGuideSportsPhasePrefix(cleanGuideSportsAnnotations(program.Title))
		notes := parseGuideSportsNotes(program.Summary)
		metadataSports, excludeProgram := guideSportsMetadata(program.Categories)
		if excludeProgram {
			continue
		}
		league := guideSportsLeagueMatch(strings.Join([]string{displayTitle, strings.Join(program.Categories, " "), channel.Name, categoryName}, " "))
		leagueID, leagueName, sportName := league.ID, league.Name, league.Sport
		sportsContext := league.OK || metadataSports
		awayName, homeName, matchup := guideSportsMatchup(displayTitle)
		eventType := ""
		if series, location, race := guideSportsRace(displayTitle); race {
			awayName, homeName, matchup = series, location, true
			eventType = "race"
		}
		boutCard := leagueID == "boxing" && guideSportsMultipleBouts(displayTitle)
		// Tournaments and races are listed one-sided; a specific registry
		// competition admits them, the generic "Sports" fallback does not.
		tournament := !matchup && league.Tournament && guideSportsTournamentListing(displayTitle, phase)
		if !sportsContext || (!matchup && !metadataSports && !boutCard && !tournament) {
			continue
		}
		if !matchup {
			eventType = "event"
		}
		if program.StartUnix <= now.Unix() && program.EndUnix > now.Unix() && guideSportsTitleHasScoreLookupHint(program.Title) {
			refreshScores = true
		}

		nextGame := guideSportsNextGamePrefix.MatchString(displayTitle)
		if nextGame {
			// Team channels list "Next Game: A @ B on <date> at <time>" as filler; the
			// game itself starts at the embedded time, not the listing time.
			gameStart, ok := guideSportsNextGameStart(displayTitle)
			if !ok {
				continue
			}
			program.StartUnix = gameStart
			program.EndUnix = gameStart + 3*3600
		} else if phase == "pregame" && program.EndUnix > program.StartUnix {
			// Pregame runs until kickoff. The game starts when that block ends.
			program.StartUnix = program.EndUnix
			program.EndUnix = program.StartUnix + 3*3600
		}
		startBucket := program.StartUnix / (15 * 60)
		key := normalizeMatchText(displayTitle) + "|" + fmt.Sprintf("%d", startBucket)
		if matchup && eventType == "" {
			key = normalizeMatchText(awayName) + "|" + normalizeMatchText(homeName) + "|" + fmt.Sprintf("%d", startBucket)
		}
		endUnix := program.EndUnix
		if endUnix <= program.StartUnix {
			endUnix = program.StartUnix + 3*3600
		}
		live, completed, status, statusText := guideSportsBroadcastStatus(program, endUnix, now)
		if nextGame {
			displayTitle = strings.TrimSpace(guideSportsNextGameSuffix.ReplaceAllString(guideSportsNextGamePrefix.ReplaceAllString(displayTitle, ""), ""))
			live, completed, status, statusText = false, false, "scheduled", "Upcoming"
		}
		shortName := strings.TrimSpace(awayName + " vs " + homeName)
		if eventType == "event" {
			shortName = displayTitle
		}
		if eventType == "race" {
			shortName = strings.Trim(strings.Join([]string{awayName, homeName}, " · "), " ·")
		}
		awayScore, homeScore := guideSportsNamedScore(awayName, homeName, notes.Score)
		reason := "guide: exact program"
		if feed := guideSportsFeedLabel(program.Title, channel.Name); feed != "" {
			reason = feed + " feed"
		}
		incoming := SportsEvent{
			ID:          "epg:" + sportsHash(key),
			LeagueID:    leagueID,
			LeagueName:  leagueName,
			SportName:   sportName,
			Name:        displayTitle,
			ShortName:   shortName,
			EventType:   eventType,
			Venue:       firstNonEmpty(guideSportsVenue(displayTitle), notes.Venue),
			Description: guideSportsDetailLine(notes.Broadcast, notes.Records),
			StartUnix:   program.StartUnix,
			EndUnix:     endUnix,
			Live:        live,
			Completed:   completed,
			Status:      status,
			StatusText:  statusText,
			Away:        SportsTeam{Name: awayName, Abbreviation: sportsTeamInitials(awayName)},
			Home:        SportsTeam{Name: homeName, Abbreviation: sportsTeamInitials(homeName)},
			AwayScore:   awayScore,
			HomeScore:   homeScore,
			guidePhase:  phase,
			Channels: []SportsChannelMatch{{
				ID:           channel.ID,
				Name:         channel.Name,
				CategoryName: categoryName,
				LogoURL:      channel.LogoURL,
				Reason:       reason,
				Score:        100,
			}},
		}
		if event := byKey[key]; event != nil {
			overlaySportsGuideEvent(event, incoming)
			continue
		}
		byKey[key] = &incoming
	}

	events := make([]SportsEvent, 0, len(byKey))
	for _, event := range byKey {
		events = append(events, *event)
	}
	events = collapseEquivalentSportsEvents(normalizeSportsEvents(events))
	sort.Slice(events, func(i, j int) bool {
		if events[i].Live != events[j].Live {
			return events[i].Live
		}
		leftFuture := events[i].StartUnix >= now.Unix()
		rightFuture := events[j].StartUnix >= now.Unix()
		if leftFuture != rightFuture {
			return leftFuture
		}
		if leftFuture {
			return events[i].StartUnix < events[j].StartUnix
		}
		return events[i].StartUnix > events[j].StartUnix
	})
	return capSportsEventsByLeague(events, sportsGuideEventLeagueLimit, sportsGuideEventLimit), refreshScores
}

const (
	sportsGuideEventLimit       = 250
	sportsGuideEventLeagueLimit = 60
)

// capSportsEventsByLeague keeps at most total events from a list already in
// priority order. While the list is over the cap, no league may take more than
// perLeague slots until every league has had its share; live games are always
// kept first. Order is preserved.
func capSportsEventsByLeague(events []SportsEvent, perLeague, total int) []SportsEvent {
	if len(events) <= total {
		return events
	}
	keep := make([]bool, len(events))
	kept := 0
	perLeagueCount := map[string]int{}
	leagueKey := func(event SportsEvent) string {
		return firstNonEmpty(event.LeagueID, "sports")
	}
	for index, event := range events {
		if kept >= total {
			break
		}
		if event.Live {
			keep[index] = true
			kept++
			perLeagueCount[leagueKey(event)]++
		}
	}
	for index, event := range events {
		if kept >= total {
			break
		}
		key := leagueKey(event)
		if keep[index] || perLeagueCount[key] >= perLeague {
			continue
		}
		keep[index] = true
		kept++
		perLeagueCount[key]++
	}
	// Leftover room goes to the over-share leagues in priority order.
	for index := range events {
		if kept >= total {
			break
		}
		if !keep[index] {
			keep[index] = true
			kept++
		}
	}
	selected := make([]SportsEvent, 0, kept)
	for index, event := range events {
		if keep[index] {
			selected = append(selected, event)
		}
	}
	return selected
}

func guideSportsMetadata(categories []string) (bool, bool) {
	sportsMetadata := false
	sportsTalk := false
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}
		text := normalizeMatchText(category)
		if text == "sports talk" {
			sportsTalk = true
			continue
		}
		if text == "sports event" || text == "sporting event" {
			sportsMetadata = true
			continue
		}
		if _, _, _, sportsContext := guideSportsLeague(category); sportsContext {
			sportsMetadata = true
		}
	}
	return sportsMetadata, sportsTalk
}

func stripGuideSportsPhasePrefix(title string) string {
	stripped := strings.TrimSpace(guideSportsPhasePrefix.ReplaceAllString(title, ""))
	if stripped == "" {
		return strings.TrimSpace(title)
	}
	return stripped
}

func guideSportsListingPhase(title string, categories []string) string {
	for _, value := range append([]string{title}, categories...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if match := guideSportsPhaseWord.FindStringSubmatch(value); match != nil {
			word := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(match[1], " ", ""), "-", ""))
			if strings.HasPrefix(word, "pre") {
				return "pregame"
			}
			return "postgame"
		}
		if guideSportsLivePhase(value) {
			return "live"
		}
	}
	return ""
}

func guideSportsLivePhase(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "live" || strings.HasPrefix(lower, "live:") || strings.HasPrefix(lower, "live-") || strings.HasPrefix(lower, "live -") || strings.HasPrefix(lower, "live|") {
		return true
	}
	return guideSportsUpperLive.MatchString(value)
}

func guideSportsFeedLabel(values ...string) string {
	for _, value := range values {
		value = cleanGuideSportsAnnotations(value)
		if value == "" {
			continue
		}
		if match := guideSportsFeedToken.FindStringSubmatch(value); match != nil {
			return sportsFeedWord(match[1])
		}
		cleaned := guideSportsAirTimeSuffix.ReplaceAllString(value, "")
		cleaned = guideSportsClockAtSuffix.ReplaceAllString(cleaned, "")
		cleaned = guideSportsTimestampSuffix.ReplaceAllString(cleaned, "")
		if match := guideSportsSideFeedSuffix.FindStringSubmatch(cleaned); match != nil {
			return sportsFeedWord(match[1])
		}
	}
	return ""
}

func sportsFeedWord(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

type guideSportsNotes struct {
	Venue, Broadcast, Records, Score string
}

func parseGuideSportsNotes(summary string) guideSportsNotes {
	var notes guideSportsNotes
	for _, match := range guideSportsNoteLine.FindAllStringSubmatch(summary, -1) {
		if len(match) != 3 {
			continue
		}
		value := strings.Trim(strings.TrimSpace(match[2]), " .")
		switch strings.ToLower(match[1]) {
		case "venue":
			if notes.Venue == "" {
				notes.Venue = value
			}
		case "broadcast", "network":
			if notes.Broadcast == "" {
				notes.Broadcast = value
			}
		case "record", "records":
			if notes.Records == "" {
				notes.Records = value
			}
		case "final", "score":
			if notes.Score == "" {
				notes.Score = value
			}
		}
	}
	return notes
}

func guideSportsDetailLine(broadcast, records string) string {
	parts := make([]string, 0, 2)
	if broadcast != "" {
		parts = append(parts, broadcast)
	}
	if records != "" {
		parts = append(parts, records)
	}
	return strings.Join(parts, " · ")
}

var guideSportsScorePair = regexp.MustCompile(`(?i)([a-z0-9][a-z0-9 .'-]*?)\s+(\d{1,3})`)

func guideSportsNamedScore(away, home, line string) (string, string) {
	awayNorm := normalizeMatchText(away)
	homeNorm := normalizeMatchText(home)
	if awayNorm == "" || homeNorm == "" || strings.TrimSpace(line) == "" {
		return "", ""
	}
	found := map[string]string{}
	for _, match := range guideSportsScorePair.FindAllStringSubmatch(line, -1) {
		name := normalizeMatchText(match[1])
		if name == "" {
			continue
		}
		if strings.Contains(awayNorm, name) || strings.Contains(name, awayNorm) {
			found["away"] = match[2]
		}
		if strings.Contains(homeNorm, name) || strings.Contains(name, homeNorm) {
			found["home"] = match[2]
		}
	}
	if found["away"] == "" || found["home"] == "" {
		return "", ""
	}
	return found["away"], found["home"]
}

func guideSportsMatchup(title string) (string, string, bool) {
	title = cleanGuideSportsAnnotations(title)
	title = stripGuideSportsPhasePrefix(title)
	if guideSportsNonMatchTitle.MatchString(title) || guideSportsMultipleBouts(title) {
		return "", "", false
	}
	title = guideSportsNextGameSuffix.ReplaceAllString(title, "")
	title = guideSportsTimestampSuffix.ReplaceAllString(title, "")
	title = guideSportsAirTimeSuffix.ReplaceAllString(title, "")
	title = guideSportsDottedDateSuffix.ReplaceAllString(title, "")
	title = guideSportsClockAtSuffix.ReplaceAllString(title, "")
	title = guideSportsFeedSuffix.ReplaceAllString(title, "")
	title = guideSportsSideFeedSuffix.ReplaceAllString(title, "")
	locations := sportsMatchupSeparator.FindAllStringIndex(title, -1)
	if len(locations) == 0 {
		return "", "", false
	}
	location := locations[len(locations)-1]
	left := strings.TrimSpace(title[:location[0]])
	right := strings.TrimSpace(title[location[1]:])
	if colon := strings.LastIndex(left, ":"); colon >= 0 {
		left = strings.TrimSpace(left[colon+1:])
	}
	if pipe := strings.LastIndex(left, "|"); pipe >= 0 {
		left = strings.TrimSpace(left[pipe+1:])
	}
	if trimmed := strings.TrimSpace(guideSportsKnownTournamentPrefix.ReplaceAllString(left, "")); trimmed != "" {
		left = trimmed
	}
	if trimmed := strings.TrimSpace(guideSportsTournamentPrefix.ReplaceAllString(left, "")); trimmed != "" {
		left = trimmed
	}
	if trimmed := strings.TrimSpace(guideSportsStagePrefix.ReplaceAllString(left, "")); trimmed != "" {
		left = trimmed
	}
	if colon := strings.Index(right, ":"); colon >= 0 {
		right = strings.TrimSpace(right[:colon])
	}
	if pipe := strings.Index(right, "|"); pipe >= 0 {
		right = strings.TrimSpace(right[:pipe])
	}
	left = cleanGuideSportsTeamName(strings.Trim(left, " -:|,."))
	right = cleanGuideSportsTeamName(strings.Trim(right, " -:|,."))
	if guideSportsClockName.MatchString(left) || guideSportsClockName.MatchString(right) {
		return "", "", false
	}
	if len([]rune(normalizeMatchText(left))) < 2 || len([]rune(normalizeMatchText(right))) < 2 {
		return "", "", false
	}
	return left, right, true
}

type sportsLeagueRef struct {
	ID, Name, Sport string
}

// Guide listings like "Padres vs Dodgers" never name their league. Both teams
// must resolve to the same league through names the provider already knows.
func inferGuideSportsLeagues(events []SportsEvent) []SportsEvent {
	for index, event := range events {
		if event.LeagueID == "nba" && (strings.HasPrefix(event.Home.Name, "NBA G League") || strings.HasPrefix(event.Away.Name, "NBA G League")) {
			events[index].LeagueID = "nba-g-league"
			events[index].LeagueName = "NBA G League"
		}
	}
	known := map[string]sportsLeagueRef{}
	ambiguous := map[string]bool{}
	leagueTeams := map[string][]string{}
	for _, event := range events {
		if event.LeagueID == "" || event.LeagueID == "sports" || event.EventType != "" {
			continue
		}
		ref := sportsLeagueRef{ID: event.LeagueID, Name: event.LeagueName, Sport: event.SportName}
		leagueTeams[event.LeagueID] = append(leagueTeams[event.LeagueID], event.Home.Name, event.Away.Name)
		for _, team := range []SportsTeam{event.Home, event.Away} {
			for _, key := range sportsTeamNameKeys(team.Name) {
				if existing, ok := known[key]; ok && existing.ID != ref.ID {
					ambiguous[key] = true
					continue
				}
				known[key] = ref
			}
		}
	}
	resolve := func(name string) (sportsLeagueRef, bool) {
		for _, key := range sportsTeamNameKeys(name) {
			if ref, ok := known[key]; ok && !ambiguous[key] {
				return ref, true
			}
		}
		return sportsLeagueRef{}, false
	}
	assign := func(index int, ref sportsLeagueRef) {
		events[index].LeagueID = ref.ID
		events[index].LeagueName = ref.Name
		events[index].SportName = firstNonEmpty(ref.Sport, events[index].SportName)
	}
	for index, event := range events {
		if (event.LeagueID != "" && event.LeagueID != "sports") || event.EventType != "" {
			continue
		}
		away, awayOK := resolve(event.Away.Name)
		home, homeOK := resolve(event.Home.Name)
		if awayOK && homeOK && away.ID == home.ID {
			assign(index, home)
			continue
		}
		if homeOK && uniqueCompatibleTeamInLeague(event.Away.Name, home.ID, leagueTeams, ambiguous) {
			assign(index, home)
			continue
		}
		if awayOK && uniqueCompatibleTeamInLeague(event.Home.Name, away.ID, leagueTeams, ambiguous) {
			assign(index, away)
		}
	}
	return events
}

func uniqueCompatibleTeamInLeague(name, leagueID string, leagueTeams map[string][]string, ambiguous map[string]bool) bool {
	for _, key := range sportsTeamNameKeys(name) {
		if ambiguous[key] {
			return false
		}
	}
	canonical := map[string]bool{}
	for _, known := range leagueTeams[leagueID] {
		if !sportsTeamNameMatch(name, known).ok {
			continue
		}
		canonical[normalizeMatchText(known)] = true
	}
	return len(canonical) == 1
}

func sportsTeamNameKeys(name string) []string {
	words := strings.Fields(normalizeMatchText(name))
	if len(words) == 0 {
		return nil
	}
	keys := []string{strings.Join(words, " ")}
	if len(words) > 1 {
		keys = append(keys, words[len(words)-1], strings.Join(words[len(words)-2:], " "))
	}
	filtered := keys[:0]
	for _, key := range keys {
		if len([]rune(key)) >= 4 {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

// A semicolon-separated fight card is one broadcast, not one pair of fighters.
func guideSportsMultipleBouts(title string) bool {
	parts := strings.Split(title, ";")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if len(sportsMatchupSeparator.FindAllStringIndex(part, -1)) != 1 {
			return false
		}
	}
	return true
}

func cleanGuideSportsAnnotations(value string) string {
	value = strings.NewReplacer("ᴸᶦᵛᵉ", "", "ᴺᵉʷ", "").Replace(value)
	return strings.TrimSpace(value)
}

func cleanGuideSportsTeamName(value string) string {
	value = cleanGuideSportsAnnotations(value)
	value = guideSportsTimestampSuffix.ReplaceAllString(value, "")
	value = guideSportsVenueSuffix.ReplaceAllString(value, "")
	value = guideSportsMatchNumberSuffix.ReplaceAllString(value, "")
	value = guideSportsStageSuffix.ReplaceAllString(value, "")
	value = guideSportsCompetitionSuffix.ReplaceAllString(value, "")
	value = guideSportsNetworkSuffix.ReplaceAllString(value, "")
	value = guideSportsGameNumberSuffix.ReplaceAllString(value, "")
	value = guideSportsSeasonPrefix.ReplaceAllString(value, "")
	value = strings.TrimSpace(value)
	if open := strings.LastIndex(value, " ("); open > 0 && strings.HasSuffix(value, ")") {
		base := strings.TrimSpace(value[:open])
		parenthetical := strings.TrimSpace(value[open+2 : len(value)-1])
		if normalizeMatchText(base) == normalizeMatchText(parenthetical) {
			return titleCaseShoutedName(base)
		}
	}
	return titleCaseShoutedName(strings.TrimSpace(value))
}

var guideSportsSeasonPrefix = regexp.MustCompile(`^(?:19|20)\d{2}(?:[-/]\d{2,4})?\s+`)

// Some providers shout team names ("NORTH BAY BATTALION"); short tokens such
// as "LSU" or "NYM" are abbreviations and stay uppercase.
func titleCaseShoutedName(value string) string {
	if value == "" || strings.ToUpper(value) != value || strings.ToLower(value) == value {
		return value
	}
	words := strings.Fields(value)
	for index, word := range words {
		letters := 0
		for _, r := range word {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters <= 3 {
			continue
		}
		lower := []rune(strings.ToLower(word))
		capitalize := true
		for position, r := range lower {
			if capitalize && unicode.IsLetter(r) {
				lower[position] = unicode.ToUpper(r)
				capitalize = false
				continue
			}
			if r == '-' || r == '\'' || r == '/' {
				capitalize = true
			}
		}
		words[index] = string(lower)
	}
	return strings.Join(words, " ")
}

func guideSportsVenue(title string) string {
	title = cleanGuideSportsAnnotations(title)
	title = guideSportsTimestampSuffix.ReplaceAllString(title, "")
	match := guideSportsVenueSuffix.FindStringSubmatch(title)
	if len(match) != 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(match[1]), " -:|,.")
}

func guideSportsRace(title string) (string, string, bool) {
	if nascarCupRacePattern.MatchString(title) {
		series, location, race := guideSportsMatchup(title)
		if race {
			return series, location, true
		}
	}
	location := formulaERacePattern.FindStringIndex(title)
	if location == nil {
		return "", "", false
	}
	venue := raceLocationPrefix.ReplaceAllString(strings.TrimSpace(title[location[1]:]), "")
	venue = strings.Trim(venue, " -:|,.")
	if venue == "" {
		venue = "Race"
	}
	return "Formula E", venue, true
}

func guideSportsBroadcastStatus(program model.Program, endUnix int64, now time.Time) (bool, bool, string, string) {
	live := program.StartUnix <= now.Unix() && endUnix > now.Unix()
	completed := endUnix <= now.Unix()
	text := normalizeMatchText(cleanGuideSportsAnnotations(program.Title) + " " + program.Summary)
	if strings.HasPrefix(text, "next game ") {
		return false, false, "scheduled", "Upcoming"
	}
	if containsMatchTerm(text, "highlight") || containsMatchTerm(text, "highlights") {
		return live, completed, "highlights", "Highlights"
	}
	for _, term := range []string{"rebroadcast", "replay", "re-air", "reair", "encore", "previously recorded", "tape delayed", "tape-delayed"} {
		if containsMatchTerm(text, term) {
			return live, completed, "replay", "Replay"
		}
	}
	if live {
		if guideSportsTitleIsLive(program.Title) {
			return true, false, "live", "Live"
		}
		return true, false, "airing", "On now"
	}
	if completed {
		return false, true, "ended", "Ended"
	}
	return false, false, "scheduled", ""
}

func sportsTeamInitials(name string) string {
	parts := strings.Fields(name)
	var builder strings.Builder
	for _, part := range parts {
		for _, r := range part {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				builder.WriteRune(r)
				break
			}
		}
		if builder.Len() == 3 {
			break
		}
	}
	return strings.ToUpper(builder.String())
}
