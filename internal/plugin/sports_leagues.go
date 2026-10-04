package plugin

import (
	"regexp"
	"strings"
)

// sportsLeagueDefinition is the one place a competition's identity lives:
// guide recognition terms, ESPN news/standings/box-score paths and the Game
// Thumbs namespace. Other lookups are derived from sportsLeagueRegistry.
type sportsLeagueDefinition struct {
	ID    string
	Name  string
	Sport string
	// Terms are normalized guide phrases that identify the competition. Entries
	// without terms are reached through dedicated parsers (college, World Cup,
	// tennis and golf majors).
	Terms []string
	// Tournament competitions are listed one-sided ("2026 US Open Tennis
	// Championships", "Formula 1: Italian Grand Prix"), so a guide listing that
	// names them is an event even without an "A vs B" title.
	Tournament bool
	// ESPN site API path and display name for news and standings.
	ESPNPath string
	ESPNName string
	// Stats marks team-vs-team leagues with ESPN box scores and play-by-play.
	Stats bool
	// GameThumbs is the Game Thumbs namespace when it differs from automatic
	// registry matching. "-" suppresses artwork (youth competitions must not
	// borrow the senior tournament's identity).
	GameThumbs string
}

// Guide recognition runs in this order; earlier entries win.
var sportsLeagueRegistry = []sportsLeagueDefinition{
	{ID: "wnba", Name: "WNBA", Sport: "Basketball", Terms: []string{"wnba"}, ESPNPath: "basketball/wnba", ESPNName: "WNBA", Stats: true},
	{ID: "nba", Name: "NBA", Sport: "Basketball", Terms: []string{"nba"}, ESPNPath: "basketball/nba", ESPNName: "NBA", Stats: true},
	{ID: "nfl", Name: "NFL", Sport: "Football", Terms: []string{"nfl"}, ESPNPath: "football/nfl", ESPNName: "NFL", Stats: true},
	{ID: "college-football", Name: "College Football", Sport: "Football", Terms: []string{"cfp", "college football"}, ESPNPath: "football/college-football", ESPNName: "College Football", Stats: true, GameThumbs: "ncaaf"},
	{ID: "mlb", Name: "MLB", Sport: "Baseball", Terms: []string{"mlb", "cubs classics"}, ESPNPath: "baseball/mlb", ESPNName: "MLB", Stats: true},
	{ID: "cebl", Name: "CEBL", Sport: "Basketball", Terms: []string{"cebl", "canadian elite basketball league"}},
	{ID: "nhl", Name: "NHL", Sport: "Hockey", Terms: []string{"nhl"}, ESPNPath: "hockey/nhl", ESPNName: "NHL", Stats: true},
	{ID: "mls", Name: "MLS", Sport: "Soccer", Terms: []string{"mls"}, ESPNPath: "soccer/usa.1", ESPNName: "MLS", Stats: true},
	{ID: "uefa-champions-league", Name: "UEFA Champions League", Sport: "Soccer", Terms: []string{"uefa champions league", "champions league"}, ESPNPath: "soccer/uefa.champions", ESPNName: "Champions League", Stats: true},
	{ID: "premier-league", Name: "Premier League", Sport: "Soccer", Terms: []string{"premier league"}, ESPNPath: "soccer/eng.1", ESPNName: "Premier League", Stats: true},
	{ID: "world-cup", Name: "World Cup", Sport: "Soccer", Terms: []string{"world cup", "fifa"}},
	{ID: "mma", Name: "MMA", Sport: "Combat Sports", Terms: []string{"ufc", "mma"}, ESPNPath: "mma/ufc", ESPNName: "UFC"},
	{ID: "boxing", Name: "Boxing", Sport: "Combat Sports", Terms: []string{"boxing", "boxeo"}},
	{ID: "formula-e", Name: "Formula E", Sport: "Motorsport", Terms: []string{"formula e", "formule e"}, Tournament: true},
	{ID: "formula-1", Name: "Formula 1", Sport: "Motorsport", Terms: []string{"formula 1", "f1"}, Tournament: true, ESPNPath: "racing/f1", ESPNName: "Formula 1"},
	{ID: "nascar-cup-series", Name: "NASCAR Cup Series", Sport: "Motorsport", Terms: []string{"nascar cup series", "ncs race"}, Tournament: true},
	{ID: "indycar", Name: "IndyCar", Sport: "Motorsport", Terms: []string{"indycar", "indy 500", "indianapolis 500"}, Tournament: true},
	{ID: "motogp", Name: "MotoGP", Sport: "Motorsport", Terms: []string{"motogp"}, Tournament: true},
	{ID: "afl", Name: "AFL", Sport: "Australian Football", Terms: []string{"afl premiership", "australian football league"}},
	// Majors are recognized by guideSportsMajorTournament, which separates
	// the tennis and golf events that share a name.
	{ID: "us-open-tennis", Name: "US Open", Sport: "Tennis", Tournament: true},
	{ID: "wimbledon", Name: "Wimbledon", Sport: "Tennis", Tournament: true},
	{ID: "roland-garros", Name: "Roland Garros", Sport: "Tennis", Tournament: true},
	{ID: "australian-open", Name: "Australian Open", Sport: "Tennis", Tournament: true},
	{ID: "masters-tournament", Name: "The Masters", Sport: "Golf", Tournament: true, ESPNPath: "golf/pga", ESPNName: "PGA Tour"},
	{ID: "pga-championship", Name: "PGA Championship", Sport: "Golf", Tournament: true, ESPNPath: "golf/pga", ESPNName: "PGA Tour"},
	{ID: "us-open-golf", Name: "U.S. Open", Sport: "Golf", Tournament: true, ESPNPath: "golf/pga", ESPNName: "PGA Tour"},
	{ID: "the-open", Name: "The Open", Sport: "Golf", Tournament: true, ESPNPath: "golf/pga", ESPNName: "PGA Tour"},
	{ID: "tennis", Name: "Tennis", Sport: "Tennis", Terms: []string{"tennis"}},
	{ID: "golf", Name: "Golf", Sport: "Golf", Terms: []string{"golf", "pga"}, ESPNPath: "golf/pga", ESPNName: "PGA Tour"},
	{ID: "cricket", Name: "Cricket", Sport: "Cricket", Terms: []string{"cricket"}},

	// Reached through dedicated parsers or provider IDs only.
	{ID: "mens-college-basketball", Name: "Men's College Basketball", Sport: "Basketball", ESPNPath: "basketball/mens-college-basketball", ESPNName: "Men's College Basketball", Stats: true},
	{ID: "womens-college-basketball", Name: "Women's College Basketball", Sport: "Basketball", ESPNPath: "basketball/womens-college-basketball", ESPNName: "Women's College Basketball", Stats: true},
	{ID: "college-mens-football", Name: "Men's College Football", Sport: "Football", GameThumbs: "ncaaf"},
	{ID: "college-basketball", Name: "College Basketball", Sport: "Basketball", GameThumbs: "ncaam"},
	{ID: "college-mens-basketball", Name: "Men's College Basketball", Sport: "Basketball", GameThumbs: "ncaam"},
	{ID: "college-womens-basketball", Name: "Women's College Basketball", Sport: "Basketball", GameThumbs: "ncaaw"},
	{ID: "college-baseball", Name: "College Baseball", Sport: "Baseball", GameThumbs: "ncaabb"},
	{ID: "college-mens-baseball", Name: "Men's College Baseball", Sport: "Baseball", GameThumbs: "ncaabb"},
	{ID: "college-softball", Name: "College Softball", Sport: "Softball", GameThumbs: "ncaasbw"},
	{ID: "college-womens-softball", Name: "Women's College Softball", Sport: "Softball", GameThumbs: "ncaasbw"},
	{ID: "college-soccer", Name: "College Soccer", Sport: "Soccer", GameThumbs: "ncaas"},
	{ID: "college-mens-soccer", Name: "Men's College Soccer", Sport: "Soccer", GameThumbs: "ncaas"},
	{ID: "college-womens-soccer", Name: "Women's College Soccer", Sport: "Soccer", GameThumbs: "ncaaws"},
	{ID: "college-hockey", Name: "College Hockey", Sport: "Hockey", GameThumbs: "ncaah"},
	{ID: "college-mens-hockey", Name: "Men's College Hockey", Sport: "Hockey", GameThumbs: "ncaah"},
	{ID: "college-womens-hockey", Name: "Women's College Hockey", Sport: "Hockey", GameThumbs: "ncaawh"},
	{ID: "college-volleyball", Name: "College Volleyball", Sport: "Volleyball", GameThumbs: "ncaavb"},
	{ID: "college-mens-volleyball", Name: "Men's College Volleyball", Sport: "Volleyball", GameThumbs: "ncaavb"},
	{ID: "college-womens-volleyball", Name: "Women's College Volleyball", Sport: "Volleyball", GameThumbs: "ncaawvb"},
	{ID: "college-lacrosse", Name: "College Lacrosse", Sport: "Lacrosse", GameThumbs: "ncaalax"},
	{ID: "college-mens-lacrosse", Name: "Men's College Lacrosse", Sport: "Lacrosse", GameThumbs: "ncaalax"},
	{ID: "college-womens-lacrosse", Name: "Women's College Lacrosse", Sport: "Lacrosse", GameThumbs: "ncaawlax"},
	{ID: "college-field-hockey", Name: "College Field Hockey", Sport: "Field Hockey", GameThumbs: "ncaawfh"},
	{ID: "college-womens-field-hockey", Name: "Women's College Field Hockey", Sport: "Field Hockey", GameThumbs: "ncaawfh"},
	{ID: "fiba-world-cup", Name: "FIBA Basketball World Cup", Sport: "Basketball", GameThumbs: "fiba"},
	{ID: "fiba-womens-world-cup", Name: "FIBA Women's Basketball World Cup", Sport: "Basketball", GameThumbs: "fiba-women"},
	{ID: "fifa-womens-world-cup", Name: "FIFA Women's World Cup", Sport: "Soccer", GameThumbs: "fifa.wwc"},
	{ID: "fifa-u20-world-cup", Name: "FIFA U-20 World Cup", Sport: "Soccer", GameThumbs: "-"},
	{ID: "fifa-womens-u20-world-cup", Name: "FIFA Women's U-20 World Cup", Sport: "Soccer", GameThumbs: "-"},
}

var sportsLeaguesByID = func() map[string]sportsLeagueDefinition {
	byID := make(map[string]sportsLeagueDefinition, len(sportsLeagueRegistry))
	for _, league := range sportsLeagueRegistry {
		if _, duplicate := byID[league.ID]; duplicate {
			panic("duplicate sports league registry ID: " + league.ID)
		}
		byID[league.ID] = league
	}
	return byID
}()

// ESPN league paths for the site API. Leagues outside this list still get
// scores and schedules from the sports provider, just without news.
var espnSportsLeagues = func() map[string]espnSportsLeague {
	leagues := map[string]espnSportsLeague{}
	for _, league := range sportsLeagueRegistry {
		if league.ESPNPath != "" {
			leagues[league.ID] = espnSportsLeague{Path: league.ESPNPath, Name: firstNonEmpty(league.ESPNName, league.Name)}
		}
	}
	return leagues
}()

// Team-vs-team leagues where ESPN box scores and play-by-play are available.
var sportsStatsLeagueIDs = func() map[string]bool {
	ids := map[string]bool{}
	for _, league := range sportsLeagueRegistry {
		if league.Stats && league.ESPNPath != "" {
			ids[league.ID] = true
		}
	}
	return ids
}()

func sportsLeagueByID(id string) (sportsLeagueDefinition, bool) {
	league, ok := sportsLeaguesByID[strings.ToLower(strings.TrimSpace(id))]
	return league, ok
}

// sportsLeagueMatch is a guide classification. Tournament is set only when a
// specific registry competition matched, never for the generic fallback.
type sportsLeagueMatch struct {
	ID, Name, Sport string
	OK              bool
	Tournament      bool
}

func (league sportsLeagueDefinition) match() sportsLeagueMatch {
	return sportsLeagueMatch{ID: league.ID, Name: league.Name, Sport: league.Sport, OK: true, Tournament: league.Tournament}
}

func registrySportsLeague(id string) sportsLeagueMatch {
	league, ok := sportsLeagueByID(id)
	if !ok {
		return sportsLeagueMatch{}
	}
	return league.match()
}

// Words that mark a non-tennis "Open" (the U.S. Open Cup is soccer).
var sportsNonTennisOpenTerms = []string{"cup", "squash", "badminton", "surfing", "bowling", "pool", "snooker", "darts", "racquetball", "table tennis", "polo", "chess", "volleyball"}

// guideSportsMajorTournament recognizes tennis and golf majors. "US Open"
// means tennis unless the listing, channel or category says golf.
func guideSportsMajorTournament(text string) (sportsLeagueMatch, bool) {
	has := func(terms ...string) bool {
		for _, term := range terms {
			if containsMatchTerm(text, term) {
				return true
			}
		}
		return false
	}
	golf := has("golf", "usga", "pga", "lpga", "augusta")
	tennis := has("tennis", "atp", "wta")
	otherOpen := has(sportsNonTennisOpenTerms...)
	tennisMajor := func(id string) (sportsLeagueMatch, bool) {
		if otherOpen || (golf && !tennis) {
			return sportsLeagueMatch{}, false
		}
		return registrySportsLeague(id), true
	}
	switch {
	case has("us open", "u s open"):
		if golf && !tennis {
			return registrySportsLeague("us-open-golf"), true
		}
		return tennisMajor("us-open-tennis")
	case has("wimbledon"):
		return tennisMajor("wimbledon")
	case has("roland garros", "french open"):
		return tennisMajor("roland-garros")
	case has("australian open"):
		return tennisMajor("australian-open")
	case has("masters tournament", "augusta national") || (golf && has("the masters")):
		return registrySportsLeague("masters-tournament"), true
	case has("pga championship"):
		return registrySportsLeague("pga-championship"), true
	case has("open championship", "british open") || (golf && has("the open")):
		return registrySportsLeague("the-open"), true
	}
	return sportsLeagueMatch{}, false
}

// Studio and magazine shows about a tournament are not the event itself.
var guideSportsTournamentShowPattern = regexp.MustCompile(`(?i)\b(?:preview|review|recap|magazine|countdown|show|notebook|press\s+conference|tonight|today|daily|classics?|documentary|weekly|insider|uncut|analysis|studio|debrief|reaction|draw|highlights?|talk|podcast|central|report)\b`)

func guideSportsTournamentListing(title, phase string) bool {
	if phase == "pregame" || phase == "postgame" {
		return false
	}
	return !guideSportsTournamentShowPattern.MatchString(title) && !guideSportsNonMatchTitle.MatchString(title)
}

// Known tournament names that precede the first competitor without a colon
// ("US Open Sinner vs Alcaraz").
var guideSportsKnownTournamentPrefix = regexp.MustCompile(`(?i)^(?:(?:19|20)\d{2}\s+)?(?:the\s+)?(?:u\.?\s?s\.?\s+open|wimbledon|roland[\s-]+garros|french\s+open|australian\s+open|masters\s+tournament|pga\s+championship|open\s+championship|british\s+open)(?:\s+(?:tennis|golf))?(?:\s+championships?)?(?:\s+(?:19|20)\d{2})?\s+`)

// gameThumbsRegistryLeague maps a classified league to its Game Thumbs
// namespace when the registry pins one. ok with an empty slug means artwork
// is deliberately suppressed.
func gameThumbsRegistryLeague(id string) (string, bool) {
	league, ok := sportsLeagueByID(id)
	if !ok || league.GameThumbs == "" {
		return "", false
	}
	if league.GameThumbs == "-" {
		return "", true
	}
	return league.GameThumbs, true
}

var collegeCompetitionPattern = regexp.MustCompile(`\b(?:(womens|mens|women|men) )?(?:college|ncaa) (?:(womens|mens|women|men) )?(football|basketball|soccer|baseball|softball|volleyball|field hockey|hockey|lacrosse|gymnastics|wrestling)\b`)

func guideCollegeCompetition(value string) (string, string, string) {
	text := strings.NewReplacer("women s", "womens", "men s", "mens").Replace(normalizeMatchText(value))
	var chosen []string
	for _, match := range collegeCompetitionPattern.FindAllStringSubmatch(text, -1) {
		if chosen == nil || match[1] != "" || match[2] != "" {
			chosen = match
		}
		if match[1] != "" || match[2] != "" {
			break
		}
	}
	if chosen == nil {
		return "", "", ""
	}
	sport := chosen[3]
	sportName := strings.ToUpper(sport[:1]) + sport[1:]
	if sport == "field hockey" {
		sport = "field-hockey"
		sportName = "Field Hockey"
	}
	gender := firstNonEmpty(chosen[1], chosen[2])
	if gender != "" {
		if strings.HasPrefix(gender, "women") {
			return "college-womens-" + sport, "Women's College " + sportName, sportName
		}
		return "college-mens-" + sport, "Men's College " + sportName, sportName
	}
	return "college-" + sport, "College " + sportName, sportName
}

var under20CompetitionPattern = regexp.MustCompile(`\b(?:u|under|sub) ?20\b`)

func guideWorldCupCompetition(value string) (string, string, string) {
	text := normalizeMatchText(value)
	if !containsMatchTerm(text, "world cup") && !containsMatchTerm(text, "copa mundial") {
		return "", "", ""
	}
	womens := false
	for _, term := range []string{"women", "womens", "femenina", "femenino"} {
		womens = womens || containsMatchTerm(text, term)
	}
	if containsMatchTerm(text, "fiba") || containsMatchTerm(text, "basketball") {
		if womens {
			return "fiba-womens-world-cup", "FIBA Women's Basketball World Cup", "Basketball"
		}
		return "fiba-world-cup", "FIBA Basketball World Cup", "Basketball"
	}
	if containsMatchTerm(text, "fifa") || containsMatchTerm(text, "soccer") {
		if under20CompetitionPattern.MatchString(text) {
			if womens {
				return "fifa-womens-u20-world-cup", "FIFA Women's U-20 World Cup", "Soccer"
			}
			return "fifa-u20-world-cup", "FIFA U-20 World Cup", "Soccer"
		}
		if womens {
			return "fifa-womens-world-cup", "FIFA Women's World Cup", "Soccer"
		}
	}
	return "", "", ""
}

func guideSportsLeague(value string) (string, string, string, bool) {
	match := guideSportsLeagueMatch(value)
	return match.ID, match.Name, match.Sport, match.OK
}

func guideSportsLeagueMatch(value string) sportsLeagueMatch {
	competitionText := normalizeMatchText(value)
	if containsMatchTerm(competitionText, "lanka premier league") || (containsMatchTerm(competitionText, "lpl") && containsMatchTerm(competitionText, "cricket")) {
		return sportsLeagueMatch{ID: "lanka-premier-league", Name: "Lanka Premier League", Sport: "Cricket", OK: true}
	}
	if containsMatchTerm(competitionText, "hoopqueens") || containsMatchTerm(competitionText, "hoop queens") {
		return sportsLeagueMatch{ID: "hoopqueens", Name: "HoopQueens Basketball", Sport: "Basketball", OK: true}
	}
	if containsMatchTerm(competitionText, "canada cup") && containsMatchTerm(competitionText, "softball") {
		return sportsLeagueMatch{ID: "canada-cup-softball", Name: "Canada Cup Softball", Sport: "Softball", OK: true}
	}
	if text := gameThumbsMatchText(value); containsMatchTerm(text, "volleyball nations league") {
		if containsMatchTerm(text, "womens") || containsMatchTerm(text, "women") {
			return sportsLeagueMatch{ID: "womens-volleyball-nations-league", Name: "Women's Volleyball Nations League", Sport: "Volleyball", OK: true}
		}
		if containsMatchTerm(text, "mens") || containsMatchTerm(text, "men") {
			return sportsLeagueMatch{ID: "mens-volleyball-nations-league", Name: "Men's Volleyball Nations League", Sport: "Volleyball", OK: true}
		}
		return sportsLeagueMatch{ID: "volleyball-nations-league", Name: "Volleyball Nations League", Sport: "Volleyball", OK: true}
	}
	if id, name, sport := guideWorldCupCompetition(value); id != "" {
		return sportsLeagueMatch{ID: id, Name: name, Sport: sport, OK: true}
	}
	if id, name, sport := guideCollegeCompetition(value); id != "" {
		return sportsLeagueMatch{ID: id, Name: name, Sport: sport, OK: true}
	}
	text := competitionText
	major, majorOK := guideSportsMajorTournament(text)
	for _, league := range sportsLeagueRegistry {
		// Majors outrank the generic tennis and golf entries that follow them.
		if majorOK && (league.ID == "tennis" || league.ID == "golf") {
			return major
		}
		for _, term := range league.Terms {
			if containsMatchTerm(text, term) {
				return league.match()
			}
		}
	}
	if majorOK {
		return major
	}
	if containsMatchTerm(text, "sports") || containsMatchTerm(text, "soccer") || containsMatchTerm(text, "football") || containsMatchTerm(text, "basketball") || containsMatchTerm(text, "baseball") || containsMatchTerm(text, "hockey") {
		return sportsLeagueMatch{ID: "sports", Name: "Sports", Sport: "Sports", OK: true}
	}
	return sportsLeagueMatch{}
}
