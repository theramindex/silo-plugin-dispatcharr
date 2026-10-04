package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
)

const (
	sportsNewsTTL        = 10 * time.Minute
	sportsTeamSummaryTTL = 30 * time.Minute
	sportsTeamListTTL    = 24 * time.Hour
	sportsNewsTimeout    = 8 * time.Second
	sportsNewsCacheLimit = 256
)

type espnSportsLeague struct {
	Path string
	Name string
}

// espnSportsLeagues is derived from sportsLeagueRegistry (sports_leagues.go).
func espnLeagueFor(leagueID string) (espnSportsLeague, bool) {
	league, ok := espnSportsLeagues[strings.ToLower(strings.TrimSpace(leagueID))]
	return league, ok
}

type SportsNewsArticle struct {
	ID          string   `json:"id"`
	Headline    string   `json:"headline"`
	Description string   `json:"description,omitempty"`
	Published   string   `json:"published,omitempty"`
	ImageURL    string   `json:"imageUrl,omitempty"`
	URL         string   `json:"url,omitempty"`
	LeagueID    string   `json:"leagueId"`
	LeagueName  string   `json:"leagueName"`
	Teams       []string `json:"teams,omitempty"`
	Premium     bool     `json:"premium,omitempty"`
}

type SportsNewsPayload struct {
	LeagueID   string              `json:"leagueId"`
	LeagueName string              `json:"leagueName,omitempty"`
	Team       string              `json:"team,omitempty"`
	Articles   []SportsNewsArticle `json:"articles"`
	Message    string              `json:"message,omitempty"`
}

type SportsTeamSummary struct {
	Available     bool                `json:"available"`
	LeagueID      string              `json:"leagueId"`
	LeagueName    string              `json:"leagueName,omitempty"`
	Name          string              `json:"name"`
	Abbreviation  string              `json:"abbreviation,omitempty"`
	LogoURL       string              `json:"logoUrl,omitempty"`
	Color         string              `json:"color,omitempty"`
	Record        string              `json:"record,omitempty"`
	Standing      string              `json:"standing,omitempty"`
	NextEventName string              `json:"nextEventName,omitempty"`
	NextEventUnix int64               `json:"nextEventUnix,omitempty"`
	Upcoming      []SportsTeamGame    `json:"upcoming"`
	Recent        []SportsTeamGame    `json:"recent,omitempty"`
	Stats         []SportsTeamStat    `json:"stats,omitempty"`
	Articles      []SportsNewsArticle `json:"articles"`
	Message       string              `json:"message,omitempty"`
	espnTeamID    string              `json:"-"`
}

type SportsTeamStat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type SportsTeamGame struct {
	Name      string `json:"name"`
	StartUnix int64  `json:"startUnix"`
	Opponent  string `json:"opponent"`
	Home      bool   `json:"home"`
	Broadcast string `json:"broadcast,omitempty"`
	Score     string `json:"score,omitempty"`
	Result    string `json:"result,omitempty"`
}

type SportsStandingsPayload struct {
	LeagueID   string                 `json:"leagueId"`
	LeagueName string                 `json:"leagueName,omitempty"`
	Columns    []string               `json:"columns"`
	Groups     []SportsStandingsGroup `json:"groups"`
	Message    string                 `json:"message,omitempty"`
}

type SportsStandingsGroup struct {
	Name string               `json:"name"`
	Rows []SportsStandingsRow `json:"rows"`
}

type SportsStandingsRow struct {
	Team         string   `json:"team"`
	Abbreviation string   `json:"abbreviation,omitempty"`
	LogoURL      string   `json:"logoUrl,omitempty"`
	Clinch       string   `json:"clinch,omitempty"`
	Values       []string `json:"values"`
}

type espnStandingsNode struct {
	Name      string `json:"name"`
	Standings *struct {
		Entries []struct {
			Team struct {
				DisplayName  string `json:"displayName"`
				Abbreviation string `json:"abbreviation"`
				Logos        []struct {
					Href string `json:"href"`
				} `json:"logos"`
			} `json:"team"`
			Stats []struct {
				Name         string   `json:"name"`
				Abbreviation string   `json:"abbreviation"`
				DisplayValue string   `json:"displayValue"`
				Value        *float64 `json:"value"`
			} `json:"stats"`
		} `json:"entries"`
	} `json:"standings"`
	Children []espnStandingsNode `json:"children"`
}

// Standings columns by sport: ESPN stat abbreviation and the header shown.
func sportsStandingsColumns(path string) [][2]string {
	switch {
	case strings.HasPrefix(path, "baseball/"):
		return [][2]string{{"W", "W"}, {"L", "L"}, {"PCT", "PCT"}, {"GB", "GB"}, {"STRK", "STRK"}, {"Last Ten", "L10"}}
	case strings.HasPrefix(path, "basketball/"):
		return [][2]string{{"W", "W"}, {"L", "L"}, {"PCT", "PCT"}, {"GB", "GB"}, {"STRK", "STRK"}}
	case strings.HasPrefix(path, "hockey/"):
		return [][2]string{{"GP", "GP"}, {"W", "W"}, {"L", "L"}, {"OTL", "OTL"}, {"PTS", "PTS"}}
	case strings.HasPrefix(path, "football/"):
		return [][2]string{{"W", "W"}, {"L", "L"}, {"T", "T"}, {"PCT", "PCT"}, {"STRK", "STRK"}}
	case strings.HasPrefix(path, "soccer/"):
		return [][2]string{{"GP", "GP"}, {"W", "W"}, {"D", "D"}, {"L", "L"}, {"GD", "GD"}, {"P", "PTS"}}
	default:
		return nil
	}
}

func (cache *sportsNewsCache) standings(ctx context.Context, leagueID string, league espnSportsLeague, divisions bool) SportsStandingsPayload {
	payload := SportsStandingsPayload{LeagueID: leagueID, LeagueName: league.Name, Columns: []string{}, Groups: []SportsStandingsGroup{}}
	columns := sportsStandingsColumns(league.Path)
	if columns == nil {
		payload.Message = "Standings aren't available for this league."
		return payload
	}
	path := league.Path + "/standings"
	if divisions {
		path += "?level=3"
	}
	value, err := cache.load(ctx, "standings|"+path, func(ctx context.Context) (any, time.Duration, error) {
		var root espnStandingsNode
		if err := cache.getStandings(ctx, path, &root); err != nil {
			return nil, 0, err
		}
		return buildSportsStandings(payload, league, columns, root), sportsTeamSummaryTTL, nil
	})
	if err != nil {
		payload.Message = "Standings are unavailable right now."
		return payload
	}
	return value.(SportsStandingsPayload)
}

func buildSportsStandings(payload SportsStandingsPayload, league espnSportsLeague, columns [][2]string, root espnStandingsNode) SportsStandingsPayload {
	present := map[string]bool{}
	var leaves []espnStandingsNode
	var walk func(node espnStandingsNode)
	walk = func(node espnStandingsNode) {
		if node.Standings != nil && len(node.Standings.Entries) > 0 {
			leaves = append(leaves, node)
			for _, stat := range node.Standings.Entries[0].Stats {
				present[stat.Abbreviation] = true
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	used := [][2]string{}
	for _, column := range columns {
		if present[column[0]] {
			used = append(used, column)
			payload.Columns = append(payload.Columns, column[1])
		}
	}
	for _, leaf := range leaves {
		group := SportsStandingsGroup{Name: firstNonEmpty(leaf.Name, league.Name)}
		type ranked struct {
			row  SportsStandingsRow
			sort float64
		}
		rows := []ranked{}
		for _, entry := range leaf.Standings.Entries {
			stats := map[string]string{}
			sortValue := 0.0
			for _, stat := range entry.Stats {
				stats[stat.Abbreviation] = stat.DisplayValue
				if stat.Value != nil && (stat.Abbreviation == "PCT" || stat.Abbreviation == "PTS" || stat.Abbreviation == "P") {
					sortValue = *stat.Value
				}
			}
			row := SportsStandingsRow{Team: entry.Team.DisplayName, Abbreviation: entry.Team.Abbreviation, Clinch: strings.TrimSpace(stats["CLINCH"])}
			if len(entry.Team.Logos) > 0 && strings.HasPrefix(entry.Team.Logos[0].Href, "https://") {
				row.LogoURL = entry.Team.Logos[0].Href
			}
			for _, column := range used {
				row.Values = append(row.Values, stats[column[0]])
			}
			rows = append(rows, ranked{row: row, sort: sortValue})
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].sort > rows[j].sort })
		for _, item := range rows {
			group.Rows = append(group.Rows, item.row)
		}
		payload.Groups = append(payload.Groups, group)
	}
	if len(payload.Groups) == 0 {
		payload.Message = "Standings aren't published yet."
	}
	return payload
}

func (cache *sportsNewsCache) teamSchedule(ctx context.Context, league espnSportsLeague, espnTeamID string, now time.Time) []SportsTeamGame {
	var payload struct {
		Events []struct {
			Name         string `json:"name"`
			Date         string `json:"date"`
			Competitions []struct {
				Competitors []struct {
					HomeAway string `json:"homeAway"`
					Team     struct {
						ID          string `json:"id"`
						DisplayName string `json:"displayName"`
					} `json:"team"`
				} `json:"competitors"`
				Broadcasts []struct {
					Media struct {
						ShortName string `json:"shortName"`
					} `json:"media"`
				} `json:"broadcasts"`
			} `json:"competitions"`
		} `json:"events"`
	}
	games := []SportsTeamGame{}
	if err := cache.get(ctx, league.Path+"/teams/"+url.PathEscape(espnTeamID)+"/schedule", &payload); err != nil {
		return games
	}
	for _, event := range payload.Events {
		start, err := time.Parse("2006-01-02T15:04Z", event.Date)
		if err != nil || start.Before(now.Add(-3*time.Hour)) || len(event.Competitions) == 0 {
			continue
		}
		game := SportsTeamGame{Name: event.Name, StartUnix: start.Unix()}
		for _, side := range event.Competitions[0].Competitors {
			if side.Team.ID == espnTeamID {
				game.Home = side.HomeAway == "home"
			} else {
				game.Opponent = side.Team.DisplayName
			}
		}
		networks := []string{}
		for _, broadcast := range event.Competitions[0].Broadcasts {
			if name := strings.TrimSpace(broadcast.Media.ShortName); name != "" {
				networks = append(networks, name)
			}
		}
		game.Broadcast = strings.Join(networks, ", ")
		games = append(games, game)
		if len(games) >= 10 {
			break
		}
	}
	return games
}

func (s *HTTPRoutesServer) handleSportsStandings(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	leagueID := strings.TrimSpace(queryValue(request, "league"))
	league, ok := espnLeagueFor(leagueID)
	if !s.sportsFeatureEnabled() || !ok {
		return s.respondJSON(http.StatusOK, SportsStandingsPayload{LeagueID: leagueID, Columns: []string{}, Groups: []SportsStandingsGroup{}, Message: "Standings aren't available for this league."})
	}
	divisions := queryValue(request, "level") == "division"
	payload := s.sportsNews.standings(ctx, leagueID, league, divisions)
	if divisions && len(payload.Groups) == 0 {
		payload = s.sportsNews.standings(ctx, leagueID, league, false)
	}
	return s.respondJSON(http.StatusOK, payload)
}

type espnTeam struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	ShortDisplayName string `json:"shortDisplayName"`
	Name             string `json:"name"`
	Nickname         string `json:"nickname"`
	Location         string `json:"location"`
	Abbreviation     string `json:"abbreviation"`
	Color            string `json:"color"`
	Logos            []struct {
		Href string `json:"href"`
	} `json:"logos"`
}

type sportsNewsCacheEntry struct {
	value   any
	err     error
	expires time.Time
}

type sportsNewsCache struct {
	mu               sync.Mutex
	entries          map[string]sportsNewsCacheEntry
	flights          sportsFlightGroup
	baseURL          string
	standingsBaseURL string
	client           *http.Client
}

const (
	// A failed ESPN lookup is remembered this long so a page of team cards
	// does not retry it once per card.
	sportsNewsNegativeTTL = 60 * time.Second
	// Bound on one shared load; a team summary makes several requests.
	sportsNewsLoadTimeout = 30 * time.Second
)

func (cache *sportsNewsCache) cached(key string) (any, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry, ok := cache.entries[key]
	if !ok || entry.err != nil || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.value, true
}

func (cache *sportsNewsCache) store(key string, value any, ttl time.Duration) {
	cache.storeEntry(key, sportsNewsCacheEntry{value: value, expires: time.Now().Add(ttl)})
}

func (cache *sportsNewsCache) storeEntry(key string, entry sportsNewsCacheEntry) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries == nil || len(cache.entries) >= sportsNewsCacheLimit {
		cache.entries = make(map[string]sportsNewsCacheEntry)
	}
	cache.entries[key] = entry
}

// load returns the cached value for key, or runs fetch once for all
// concurrent callers. Successes are kept for the TTL fetch returns; failures
// are remembered for sportsNewsNegativeTTL.
func (cache *sportsNewsCache) load(ctx context.Context, key string, fetch func(context.Context) (any, time.Duration, error)) (any, error) {
	cache.mu.Lock()
	if entry, ok := cache.entries[key]; ok && time.Now().Before(entry.expires) {
		cache.mu.Unlock()
		return entry.value, entry.err
	}
	cache.mu.Unlock()
	return cache.flights.do(key, func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sportsNewsLoadTimeout)
		defer cancel()
		value, ttl, err := fetch(loadCtx)
		if err != nil {
			cache.storeEntry(key, sportsNewsCacheEntry{err: err, expires: time.Now().Add(sportsNewsNegativeTTL)})
			return nil, err
		}
		cache.store(key, value, ttl)
		return value, nil
	})
}

func (cache *sportsNewsCache) get(ctx context.Context, path string, result any) error {
	base := cache.baseURL
	if base == "" {
		base = "https://site.api.espn.com/apis/site/v2/sports/"
	}
	return cache.fetch(ctx, base+path, result)
}

func (cache *sportsNewsCache) getStandings(ctx context.Context, path string, result any) error {
	base := cache.standingsBaseURL
	if base == "" {
		base = "https://site.api.espn.com/apis/v2/sports/"
	}
	return cache.fetch(ctx, base+path, result)
}

func (cache *sportsNewsCache) fetch(ctx context.Context, target string, result any) error {
	ctx, cancel := context.WithTimeout(ctx, sportsNewsTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	client := cache.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("news source returned %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(result)
}

type espnNewsResponse struct {
	Articles []struct {
		ID          json.Number `json:"id"`
		Headline    string      `json:"headline"`
		Description string      `json:"description"`
		Published   string      `json:"published"`
		Premium     bool        `json:"premium"`
		Images      []struct {
			URL string `json:"url"`
		} `json:"images"`
		Links struct {
			Web struct {
				Href string `json:"href"`
			} `json:"web"`
		} `json:"links"`
		Categories []struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"categories"`
	} `json:"articles"`
}

func (cache *sportsNewsCache) news(ctx context.Context, leagueID string, league espnSportsLeague, espnTeamID string, limit int) ([]SportsNewsArticle, error) {
	value, err := cache.load(ctx, "news|"+league.Path+"|"+espnTeamID, func(ctx context.Context) (any, time.Duration, error) {
		query := url.Values{"limit": {"25"}}
		if espnTeamID != "" {
			query.Set("team", espnTeamID)
		}
		var payload espnNewsResponse
		if err := cache.get(ctx, league.Path+"/news?"+query.Encode(), &payload); err != nil {
			return nil, 0, err
		}
		return sportsNewsArticles(payload, leagueID, league), sportsNewsTTL, nil
	})
	if err != nil {
		return nil, err
	}
	return trimSportsNews(value.([]SportsNewsArticle), limit), nil
}

func sportsNewsArticles(payload espnNewsResponse, leagueID string, league espnSportsLeague) []SportsNewsArticle {
	articles := make([]SportsNewsArticle, 0, len(payload.Articles))
	for _, item := range payload.Articles {
		headline := strings.TrimSpace(item.Headline)
		link := strings.TrimSpace(item.Links.Web.Href)
		if headline == "" || !strings.HasPrefix(link, "https://") {
			continue
		}
		article := SportsNewsArticle{
			ID:          item.ID.String(),
			Headline:    headline,
			Description: strings.TrimSpace(item.Description),
			Published:   item.Published,
			URL:         link,
			LeagueID:    leagueID,
			LeagueName:  league.Name,
			Premium:     item.Premium,
		}
		if len(item.Images) > 0 && strings.HasPrefix(item.Images[0].URL, "https://") {
			article.ImageURL = item.Images[0].URL
		}
		for _, category := range item.Categories {
			if category.Type == "team" && strings.TrimSpace(category.Description) != "" {
				article.Teams = append(article.Teams, strings.TrimSpace(category.Description))
			}
		}
		articles = append(articles, article)
	}
	return articles
}

func trimSportsNews(articles []SportsNewsArticle, limit int) []SportsNewsArticle {
	if limit <= 0 || len(articles) <= limit {
		return append([]SportsNewsArticle(nil), articles...)
	}
	return append([]SportsNewsArticle(nil), articles[:limit]...)
}

func (cache *sportsNewsCache) teams(ctx context.Context, league espnSportsLeague) ([]espnTeam, error) {
	value, err := cache.load(ctx, "teams|"+league.Path, func(ctx context.Context) (any, time.Duration, error) {
		var payload struct {
			Sports []struct {
				Leagues []struct {
					Teams []struct {
						Team espnTeam `json:"team"`
					} `json:"teams"`
				} `json:"leagues"`
			} `json:"sports"`
		}
		if err := cache.get(ctx, league.Path+"/teams?limit=1000", &payload); err != nil {
			return nil, 0, err
		}
		teams := []espnTeam{}
		for _, sport := range payload.Sports {
			for _, entry := range sport.Leagues {
				for _, team := range entry.Teams {
					teams = append(teams, team.Team)
				}
			}
		}
		return teams, sportsTeamListTTL, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]espnTeam), nil
}

// matchESPNTeam prefers exact display names, then school or city names, so
// "Michigan" finds "Michigan Wolverines" without matching "Michigan State".
func matchESPNTeam(teams []espnTeam, name string) (espnTeam, bool) {
	target := normalizeMatchText(name)
	if target == "" {
		return espnTeam{}, false
	}
	for _, pass := range []func(espnTeam) []string{
		func(team espnTeam) []string { return []string{team.DisplayName} },
		func(team espnTeam) []string { return []string{team.ShortDisplayName, team.Location, team.Nickname} },
		func(team espnTeam) []string {
			return []string{team.Location + " " + team.Name, team.Abbreviation, team.Name}
		},
	} {
		for _, team := range teams {
			for _, candidate := range pass(team) {
				if candidate != "" && normalizeMatchText(candidate) == target {
					return team, true
				}
			}
		}
	}
	return espnTeam{}, false
}

func (cache *sportsNewsCache) teamSummary(ctx context.Context, leagueID string, league espnSportsLeague, name string) SportsTeamSummary {
	key := "summary|" + league.Path + "|" + normalizeMatchText(name)
	value, err := cache.load(ctx, key, func(ctx context.Context) (any, time.Duration, error) {
		summary, err := cache.fetchTeamSummary(ctx, leagueID, league, name)
		return summary, sportsTeamSummaryTTL, err
	})
	if err != nil {
		summary := SportsTeamSummary{LeagueID: leagueID, LeagueName: league.Name, Name: name, Upcoming: []SportsTeamGame{}, Articles: []SportsNewsArticle{}}
		summary.Message = "Team details are unavailable right now."
		return summary
	}
	summary := value.(SportsTeamSummary)
	summary.Stats = cache.teamStats(ctx, league, summary.espnTeamID)
	return summary
}

func (cache *sportsNewsCache) fetchTeamSummary(ctx context.Context, leagueID string, league espnSportsLeague, name string) (SportsTeamSummary, error) {
	summary := SportsTeamSummary{LeagueID: leagueID, LeagueName: league.Name, Name: name, Upcoming: []SportsTeamGame{}, Articles: []SportsNewsArticle{}}
	teams, err := cache.teams(ctx, league)
	if err != nil {
		return summary, err
	}
	team, ok := matchESPNTeam(teams, name)
	if !ok {
		summary.Message = "No team details found for " + name + "."
		return summary, nil
	}
	var detail struct {
		Team struct {
			espnTeam
			StandingSummary string `json:"standingSummary"`
			Record          struct {
				Items []struct {
					Summary string `json:"summary"`
				} `json:"items"`
			} `json:"record"`
			NextEvent []struct {
				Name string `json:"name"`
				Date string `json:"date"`
			} `json:"nextEvent"`
		} `json:"team"`
	}
	if err := cache.get(ctx, league.Path+"/teams/"+url.PathEscape(team.ID), &detail); err != nil {
		detail.Team.espnTeam = team
	}
	summary.Available = true
	summary.Name = firstNonEmpty(detail.Team.DisplayName, team.DisplayName, name)
	summary.Abbreviation = firstNonEmpty(detail.Team.Abbreviation, team.Abbreviation)
	summary.Color = firstNonEmpty(detail.Team.Color, team.Color)
	summary.Standing = strings.TrimSpace(detail.Team.StandingSummary)
	if len(detail.Team.Record.Items) > 0 {
		summary.Record = strings.TrimSpace(detail.Team.Record.Items[0].Summary)
	}
	for _, logos := range [][]struct {
		Href string `json:"href"`
	}{detail.Team.Logos, team.Logos} {
		if len(logos) > 0 && strings.HasPrefix(logos[0].Href, "https://") {
			summary.LogoURL = logos[0].Href
			break
		}
	}
	if len(detail.Team.NextEvent) > 0 {
		summary.NextEventName = detail.Team.NextEvent[0].Name
		if start, err := time.Parse("2006-01-02T15:04Z", detail.Team.NextEvent[0].Date); err == nil {
			summary.NextEventUnix = start.Unix()
		}
	}
	if articles, err := cache.news(ctx, leagueID, league, team.ID, 12); err == nil {
		summary.Articles = articles
	}
	summary.Upcoming = cache.teamSchedule(ctx, league, team.ID, time.Now())
	summary.espnTeamID = team.ID
	return summary, nil
}

// teamStats keeps the counting sheet on its own cache entry. A failed lookup
// expires in sportsNewsNegativeTTL. It must not ride inside the 30-minute summary.
func (cache *sportsNewsCache) teamStats(ctx context.Context, league espnSportsLeague, espnTeamID string) []SportsTeamStat {
	if strings.TrimSpace(espnTeamID) == "" {
		return nil
	}
	value, err := cache.load(ctx, "stats|"+league.Path+"|"+espnTeamID, func(ctx context.Context) (any, time.Duration, error) {
		sheet, err := cache.teamStatSheet(ctx, league, espnTeamID)
		if err != nil {
			return nil, 0, err
		}
		if sheet == nil {
			sheet = []SportsTeamStat{}
		}
		return sheet, sportsTeamSummaryTTL, nil
	})
	if err != nil {
		return nil
	}
	sheet, _ := value.([]SportsTeamStat)
	return sheet
}

// seasontype=2 is the regular season of the year ESPN already selected.
// A season year is never sent, so this cannot ask for a past year.
func (cache *sportsNewsCache) teamStatSheet(ctx context.Context, league espnSportsLeague, espnTeamID string) ([]SportsTeamStat, error) {
	var payload struct {
		Results struct {
			Stats struct {
				Categories []sportsStatCategory `json:"categories"`
			} `json:"stats"`
		} `json:"results"`
	}
	if err := cache.get(ctx, league.Path+"/teams/"+url.PathEscape(espnTeamID)+"/statistics?seasontype=2", &payload); err != nil {
		return nil, err
	}
	return sportsTeamStatSheet(league.Path, payload.Results.Stats.Categories), nil
}

type sportsStatCategory struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Stats       []struct {
		Abbreviation string `json:"abbreviation"`
		DisplayValue string `json:"displayValue"`
	} `json:"stats"`
}

type sportsStatPick struct {
	category     string
	abbreviation string
	label        string
}

func sportsTeamStatPicks(path string) []sportsStatPick {
	switch {
	case strings.HasPrefix(path, "baseball/"):
		return []sportsStatPick{{"batting", "AVG", "AVG"}, {"batting", "HR", "HR"}, {"batting", "RBI", "RBI"}, {"pitching", "W", "W"}, {"pitching", "L", "L"}, {"pitching", "WHIP", "WHIP"}}
	case strings.HasPrefix(path, "football/"):
		return []sportsStatPick{{"passing", "RTG", "RTG"}, {"passing", "TD", "Pass TD"}, {"rushing", "YDS", "Rush"}, {"rushing", "TD", "Rush TD"}, {"defense", "SACK", "Sacks"}, {"scoring", "PTS", "PTS"}}
	case strings.HasPrefix(path, "basketball/"):
		return []sportsStatPick{{"offensive", "PTS", "PTS"}, {"offensive", "FG%", "FG%"}, {"offensive", "3P%", "3P%"}, {"offensive", "AST", "AST"}, {"general", "REB", "REB"}, {"defensive", "STL", "STL"}}
	case strings.HasPrefix(path, "hockey/"):
		return []sportsStatPick{{"general", "WINS", "W"}, {"offensive", "G", "G"}, {"offensive", "A", "A"}, {"offensive", "PTS", "PTS"}, {"defensive", "SV%", "SV%"}, {"defensive", "GAA", "GAA"}}
	case strings.HasPrefix(path, "soccer/"):
		return []sportsStatPick{{"offensive", "G", "G"}, {"offensive", "A", "A"}, {"general", "GP", "GP"}, {"defensive", "GA", "GA"}}
	default:
		return nil
	}
}

func sportsTeamStatSheet(path string, categories []sportsStatCategory) []SportsTeamStat {
	picks := sportsTeamStatPicks(path)
	stats := make([]SportsTeamStat, 0, len(picks))
	for _, pick := range picks {
	categories:
		for _, category := range categories {
			if !strings.EqualFold(category.Name, pick.category) && !strings.EqualFold(category.DisplayName, pick.category) {
				continue
			}
			for _, stat := range category.Stats {
				if strings.EqualFold(stat.Abbreviation, pick.abbreviation) && strings.TrimSpace(stat.DisplayValue) != "" {
					stats = append(stats, SportsTeamStat{Label: pick.label, Value: strings.TrimSpace(stat.DisplayValue)})
					break categories
				}
			}
		}
	}
	if len(stats) == 0 {
		return nil
	}
	return stats
}

func (s *HTTPRoutesServer) handleSportsNews(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	leagueID := strings.TrimSpace(queryValue(request, "league"))
	teamName := strings.TrimSpace(queryValue(request, "team"))
	payload := SportsNewsPayload{LeagueID: leagueID, Team: teamName, Articles: []SportsNewsArticle{}}
	league, ok := espnLeagueFor(leagueID)
	if !s.sportsFeatureEnabled() || !ok {
		payload.Message = "News isn't available for this league yet."
		return s.respondJSON(http.StatusOK, payload)
	}
	payload.LeagueName = league.Name
	espnTeamID := ""
	if teamName != "" {
		teams, err := s.sportsNews.teams(ctx, league)
		team, found := matchESPNTeam(teams, teamName)
		if err != nil || !found {
			payload.Message = "No news found for " + teamName + "."
			return s.respondJSON(http.StatusOK, payload)
		}
		espnTeamID = team.ID
	}
	articles, err := s.sportsNews.news(ctx, leagueID, league, espnTeamID, 20)
	if err != nil {
		payload.Message = "News is unavailable right now."
		return s.respondJSON(http.StatusOK, payload)
	}
	payload.Articles = articles
	return s.respondJSON(http.StatusOK, payload)
}

func (s *HTTPRoutesServer) handleSportsTeam(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	leagueID := strings.TrimSpace(queryValue(request, "league"))
	name := strings.TrimSpace(queryValue(request, "name"))
	league, ok := espnLeagueFor(leagueID)
	if !s.sportsFeatureEnabled() || !ok || name == "" {
		return s.respondJSON(http.StatusOK, SportsTeamSummary{LeagueID: leagueID, Name: name, Upcoming: []SportsTeamGame{}, Articles: []SportsNewsArticle{}, Message: "Team details aren't available for this league yet."})
	}
	summary := s.sportsNews.teamSummary(ctx, leagueID, league, name)
	if recent, ok := s.sportsProvider.(sportsRecentResultsProvider); ok {
		summary.Recent = recent.RecentResults(ctx, queryValue(request, "id"), name)
	}
	return s.respondJSON(http.StatusOK, summary)
}
