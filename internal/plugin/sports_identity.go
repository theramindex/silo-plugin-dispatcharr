package plugin

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const gameThumbsPublicBaseURL = "https://game-thumbs.swvn.io"

const (
	aflLeagueLogoURL = "https://r2.thesportsdb.com/images/media/league/badge/wvx4721525519372.png"
	aflTeamLogoBase  = "https://squiggle.com.au/wp-content/themes/squiggle/assets/images/"
)

type sportsIdentityRoute struct {
	pattern *regexp.Regexp
	slug    string
}

var gameThumbsLeagueRoutes = []sportsIdentityRoute{
	{regexp.MustCompile(`(?i)\bnascar\s+cup\s+series\b|\bncs\s+race\b`), "NASCAR"},
	{regexp.MustCompile(`(?i)\bindian premier league\b|\bipl\b`), "ipl"},
	{regexp.MustCompile(`(?i)\bbangladesh premier league\b|\bbpl\b`), "bpl"},
	{regexp.MustCompile(`(?i)\bcaribbean premier league\b|\bcpl\b`), "cpl"},
	{regexp.MustCompile(`(?i)\bmajor league cricket\b|\bmlc\b`), "mlc"},
	{regexp.MustCompile(`(?i)\befl championship\b|\benglish championship\b|\beng\.2\b`), "championship"},
	{regexp.MustCompile(`(?i)\befl league (?:one|1)\b|\benglish league (?:one|1)\b|\beng\.3\b`), "league-one"},
	{regexp.MustCompile(`(?i)\befl league (?:two|2)\b|\benglish league (?:two|2)\b|\beng\.4\b`), "league-two"},
	{regexp.MustCompile(`(?i)\benglish premier league\b|\bpremier league\b|\bepl\b`), "epl"},
	{regexp.MustCompile(`(?i)\bnational women'?s soccer league\b|\bnwsl\b`), "usa.nwsl"},
	{regexp.MustCompile(`(?i)\bmajor league soccer\b|\bmls\b`), "mls"},
	{regexp.MustCompile(`(?i)\buefa champions league\b|\bchampions league\b`), "uefa"},
	{regexp.MustCompile(`(?i)\bla ?liga\b`), "laliga"},
	{regexp.MustCompile(`(?i)\bbundesliga\b`), "bundesliga"},
	{regexp.MustCompile(`(?i)\bserie a\b`), "seriea"},
	{regexp.MustCompile(`(?i)\bligue 1\b`), "ligue1"},
	{regexp.MustCompile(`(?i)\bbrazil(?:ian)? (?:serie a|série a)\b|\bbra\.1\b`), "bra.1"},
	{regexp.MustCompile(`(?i)\bcollege football\b|\bncaa football\b|\bncaaf\b`), "ncaaf"},
	{regexp.MustCompile(`(?i)\bnational football league\b|\bnfl\b`), "nfl"},
	{regexp.MustCompile(`(?i)\bnational hockey league\b|\bnhl\b`), "nhl"},
	{regexp.MustCompile(`(?i)\bwomen'?s national basketball association\b|\bwnba\b`), "wnba"},
	{regexp.MustCompile(`(?i)\bnational basketball association\b|\bnba\b`), "nba"},
	{regexp.MustCompile(`(?i)\bmajor league baseball\b|\bmlb\b`), "mlb"},
	{regexp.MustCompile(`(?i)\bultimate fighting championship\b|\bufc\b`), "ufc"},
	{regexp.MustCompile(`(?i)\bprofessional fighters league\b|\bpfl\b`), "pfl"},
	{regexp.MustCompile(`(?i)\bbellator\b`), "bellator"},
	{regexp.MustCompile(`(?i)\bboxing\b`), "boxing"},
}

var sportsCountryNames = map[string]string{
	"afghanistan":          "Afghanistan",
	"australia":            "Australia",
	"bangladesh":           "Bangladesh",
	"canada":               "Canada",
	"england":              "England",
	"india":                "India",
	"ireland":              "Ireland",
	"namibia":              "Namibia",
	"nepal":                "Nepal",
	"netherlands":          "Netherlands",
	"new zealand":          "New Zealand",
	"pakistan":             "Pakistan",
	"scotland":             "Scotland",
	"south africa":         "South Africa",
	"sri lanka":            "Sri Lanka",
	"united arab emirates": "United Arab Emirates",
	"united states":        "United States",
	"usa":                  "USA",
	"zimbabwe":             "Zimbabwe",
}

var aflTeamLogoFiles = map[string]string{
	"adelaide":               "Adelaide.png",
	"brisbane":               "Brisbane.png",
	"brisbane lions":         "Brisbane.png",
	"carlton":                "Carlton.png",
	"collingwood":            "Collingwood.png",
	"essendon":               "Essendon.png",
	"fremantle":              "Fremantle.png",
	"geelong":                "Geelong.png",
	"geelong cats":           "Geelong.png",
	"gold coast":             "GoldCoast.png",
	"gold coast suns":        "GoldCoast.png",
	"greater western sydney": "Giants.png",
	"gws":                    "Giants.png",
	"gws giants":             "Giants.png",
	"hawthorn":               "Hawthorn.png",
	"melbourne":              "Melbourne.png",
	"north melbourne":        "NorthMelbourne.png",
	"port adelaide":          "PortAdelaide.png",
	"richmond":               "Richmond.png",
	"st kilda":               "StKilda.png",
	"sydney":                 "Sydney.png",
	"sydney swans":           "Sydney.png",
	"west coast":             "WestCoast.png",
	"west coast eagles":      "WestCoast.png",
	"western bulldogs":       "Bulldogs.png",
}

var gameThumbsTeamLeagueRoutes = []sportsIdentityRoute{
	{regexp.MustCompile(`(?i)\b(?:atlanta hawks|boston celtics|brooklyn nets|charlotte hornets|chicago bulls|cleveland cavaliers|dallas mavericks|denver nuggets|detroit pistons|golden state warriors|houston rockets|indiana pacers|(?:la|los angeles) clippers|(?:la|los angeles) lakers|memphis grizzlies|miami heat|milwaukee bucks|minnesota timberwolves|new orleans pelicans|new york knicks|oklahoma city thunder|orlando magic|philadelphia 76ers|phoenix suns|portland trail blazers|sacramento kings|san antonio spurs|toronto raptors|utah jazz|washington wizards)\b`), "nba"},
	{regexp.MustCompile(`(?i)\b(?:anaheim ducks|boston bruins|buffalo sabres|calgary flames|carolina hurricanes|chicago blackhawks|colorado avalanche|columbus blue jackets|dallas stars|detroit red wings|edmonton oilers|florida panthers|los angeles kings|minnesota wild|montreal canadiens|nashville predators|new jersey devils|new york islanders|new york rangers|ottawa senators|philadelphia flyers|pittsburgh penguins|san jose sharks|seattle kraken|st louis blues|st\. louis blues|tampa bay lightning|toronto maple leafs|utah mammoth|utah hockey club|vancouver canucks|vegas golden knights|washington capitals|winnipeg jets)\b`), "nhl"},
	// Game Thumbs' epl namespace resolves the English pyramid through its configured feeder leagues.
	{regexp.MustCompile(`(?i)\b(?:chelsea(?:\s+(?:u21|under[ -]?21s?))?|bristol rovers)\b`), "epl"},
	{regexp.MustCompile(`(?i)\b(?:palmeiras|flamengo|fluminense|corinthians|santos|botafogo|vasco da gama|sao paulo|são paulo|gremio|grêmio|internacional|cruzeiro|atletico mineiro|atlético mineiro)\b`), "bra.1"},
}

var gameThumbsChelseaYouthSuffix = regexp.MustCompile(`(?i)\bchelsea\s+(?:u21|under[ -]?21s?)\b`)
var compoundBoxingParticipant = regexp.MustCompile(`(?i)\s+(?:&|and)\s+`)

func applySportsIdentityFallbacks(event SportsEvent) SportsEvent {
	event = applySpecialSportsIdentityFallbacks(event)
	leagueSlug := gameThumbsLeagueSlugForEvent(event)
	if leagueSlug == "" {
		awayLeagueSlug := gameThumbsLeagueSlugForTeam(event.Away, "")
		homeLeagueSlug := gameThumbsLeagueSlugForTeam(event.Home, "")
		if awayLeagueSlug != "" && awayLeagueSlug == homeLeagueSlug {
			leagueSlug = awayLeagueSlug
		}
	}
	if event.LeagueLogoURL == "" && leagueSlug != "" {
		event.LeagueLogoURL = gameThumbsLeagueLogoURL(leagueSlug)
	}
	event.Away = applySportsTeamIdentityFallback(event.Away, leagueSlug)
	event.Home = applySportsTeamIdentityFallback(event.Home, leagueSlug)
	return event
}

func applySpecialSportsIdentityFallbacks(event SportsEvent) SportsEvent {
	identityText := normalizeSportsIdentityText(strings.Join([]string{event.LeagueID, event.LeagueName, event.SportName, event.Name}, " "))
	if strings.Contains(identityText, "afl") || strings.Contains(identityText, "australian football") || strings.Contains(identityText, "afl premiership") {
		if event.LeagueLogoURL == "" {
			event.LeagueLogoURL = aflLeagueLogoURL
		}
		event.Away = applyAFLTeamIdentity(event.Away)
		event.Home = applyAFLTeamIdentity(event.Home)
	}
	if strings.Contains(identityText, "cricket") {
		event.Away = applyCountryTeamIdentity(event.Away)
		event.Home = applyCountryTeamIdentity(event.Home)
	}
	return event
}

func applyAFLTeamIdentity(team SportsTeam) SportsTeam {
	if team.LogoURL != "" {
		return team
	}
	if filename := aflTeamLogoFiles[normalizeSportsIdentityText(team.Name)]; filename != "" {
		team.LogoURL = aflTeamLogoBase + filename
	}
	return team
}

func applyCountryTeamIdentity(team SportsTeam) SportsTeam {
	if team.LogoURL != "" {
		return team
	}
	if country := sportsCountryNames[normalizeSportsIdentityText(team.Name)]; country != "" {
		team.LogoURL = gameThumbsTeamLogoURL("country", country)
	}
	return team
}

func normalizeSportsIdentityText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func applySportsTeamIdentityFallback(team SportsTeam, eventLeagueSlug string) SportsTeam {
	if team.LogoURL != "" || !usableSportsIdentityName(team.Name) {
		return team
	}
	if eventLeagueSlug == "boxing" && compoundBoxingParticipant.MatchString(team.Name) {
		team.LogoURL = gameThumbsLeagueLogoURL(eventLeagueSlug)
		return team
	}
	leagueSlug := gameThumbsLeagueSlugForTeam(team, eventLeagueSlug)
	if leagueSlug != "" {
		team.LogoURL = gameThumbsTeamLogoURL(leagueSlug, team.Name)
	}
	return team
}

func gameThumbsLeagueSlugForEvent(event SportsEvent) string {
	value := strings.Join([]string{event.LeagueID, event.LeagueName, event.SportName, event.Name, event.ShortName}, " ")
	return firstSportsIdentityRoute(value, gameThumbsLeagueRoutes)
}

func gameThumbsLeagueSlugForTeam(team SportsTeam, eventLeagueSlug string) string {
	if slug := firstSportsIdentityRoute(strings.Join([]string{team.ID, team.Name, team.Abbreviation}, " "), gameThumbsTeamLeagueRoutes); slug != "" {
		return slug
	}
	return eventLeagueSlug
}

func firstSportsIdentityRoute(value string, routes []sportsIdentityRoute) string {
	for _, route := range routes {
		if route.pattern.MatchString(value) {
			return route.slug
		}
	}
	return ""
}

func usableSportsIdentityName(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value != "" && value != "team" && value != "tbd" && value != "unknown"
}

func gameThumbsLeagueLogoURL(leagueSlug string) string {
	if leagueSlug == "" {
		return ""
	}
	return gameThumbsPublicBaseURL + "/" + url.PathEscape(leagueSlug) + "/leaguelogo.png"
}

func gameThumbsTeamLogoURL(leagueSlug, teamName string) string {
	teamKey := gameThumbsTeamKey(gameThumbsCanonicalTeamName(teamName))
	if leagueSlug == "" || teamKey == "" {
		return ""
	}
	return gameThumbsPublicBaseURL + "/" + url.PathEscape(leagueSlug) + "/" + url.PathEscape(teamKey) + "/teamlogo.png"
}

func gameThumbsCanonicalTeamName(value string) string {
	value = strings.TrimSpace(value)
	if gameThumbsChelseaYouthSuffix.MatchString(value) {
		return "Chelsea"
	}
	return value
}

func gameThumbsTeamKey(value string) string {
	var result strings.Builder
	separator := false
	for _, character := range norm.NFD.String(strings.ToLower(strings.TrimSpace(value))) {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			separator = false
			continue
		}
		if result.Len() > 0 && !separator {
			result.WriteByte('-')
			separator = true
		}
	}
	return strings.Trim(result.String(), "-")
}
