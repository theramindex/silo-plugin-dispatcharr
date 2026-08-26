package plugin

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const gameThumbsPublicBaseURL = "https://game-thumbs.swvn.io"

type sportsIdentityRoute struct {
	pattern *regexp.Regexp
	slug    string
}

var gameThumbsLeagueRoutes = []sportsIdentityRoute{
	{regexp.MustCompile(`(?i)\bindian premier league\b|\bipl\b`), "ipl"},
	{regexp.MustCompile(`(?i)\bbangladesh premier league\b|\bbpl\b`), "bpl"},
	{regexp.MustCompile(`(?i)\bcaribbean premier league\b|\bcpl\b`), "cpl"},
	{regexp.MustCompile(`(?i)\bmajor league cricket\b|\bmlc\b`), "mlc"},
	{regexp.MustCompile(`(?i)\benglish premier league\b|\bpremier league\b|\bepl\b`), "epl"},
	{regexp.MustCompile(`(?i)\bnational women'?s soccer league\b|\bnwsl\b`), "usa.nwsl"},
	{regexp.MustCompile(`(?i)\bmajor league soccer\b|\bmls\b`), "mls"},
	{regexp.MustCompile(`(?i)\buefa champions league\b|\bchampions league\b`), "uefa"},
	{regexp.MustCompile(`(?i)\bla ?liga\b`), "laliga"},
	{regexp.MustCompile(`(?i)\bbundesliga\b`), "bundesliga"},
	{regexp.MustCompile(`(?i)\bserie a\b`), "seriea"},
	{regexp.MustCompile(`(?i)\bligue 1\b`), "ligue1"},
	{regexp.MustCompile(`(?i)\bbrazil(?:ian)? (?:serie a|série a)\b|\bbra\.1\b`), "bra.1"},
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

var gameThumbsTeamLeagueRoutes = []sportsIdentityRoute{
	// Game Thumbs' epl namespace resolves the English pyramid through its configured feeder leagues.
	{regexp.MustCompile(`(?i)\b(?:chelsea(?:\s+(?:u21|under[ -]?21s?))?|bristol rovers)\b`), "epl"},
	{regexp.MustCompile(`(?i)\b(?:palmeiras|flamengo|fluminense|corinthians|santos|botafogo|vasco da gama|sao paulo|são paulo|gremio|grêmio|internacional|cruzeiro|atletico mineiro|atlético mineiro)\b`), "bra.1"},
}

var gameThumbsChelseaYouthSuffix = regexp.MustCompile(`(?i)\bchelsea\s+(?:u21|under[ -]?21s?)\b`)

func applySportsIdentityFallbacks(event SportsEvent) SportsEvent {
	leagueSlug := gameThumbsLeagueSlugForEvent(event)
	if event.LeagueLogoURL == "" && leagueSlug != "" {
		event.LeagueLogoURL = gameThumbsLeagueLogoURL(leagueSlug)
	}
	event.Away = applySportsTeamIdentityFallback(event.Away, leagueSlug)
	event.Home = applySportsTeamIdentityFallback(event.Home, leagueSlug)
	return event
}

func applySportsTeamIdentityFallback(team SportsTeam, eventLeagueSlug string) SportsTeam {
	if team.LogoURL != "" || !usableSportsIdentityName(team.Name) {
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
