package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

type sportsProvider interface {
	Events(context.Context, time.Time) ([]SportsEvent, error)
	Source() string
}

type sportsEventCache struct {
	Events       []SportsEvent
	UpdatedUnix  int64
	Source       string
	ExpiresAfter time.Time
}

type SportsPayload struct {
	UpdatedAtUnix int64          `json:"updatedAtUnix"`
	Source        string         `json:"source"`
	Leagues       []SportsLeague `json:"leagues"`
	Events        []SportsEvent  `json:"events"`
	FavoriteTeams []string       `json:"favoriteTeams"`
	Error         string         `json:"error,omitempty"`
}

type SportsLeague struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	LiveCount     int    `json:"liveCount"`
	UpcomingCount int    `json:"upcomingCount"`
}

type SportsTeam struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation,omitempty"`
	LogoURL      string `json:"logoUrl,omitempty"`
	Favorite     bool   `json:"favorite,omitempty"`
}

type SportsEvent struct {
	ID         string               `json:"id"`
	LeagueID   string               `json:"leagueId"`
	LeagueName string               `json:"leagueName"`
	Name       string               `json:"name"`
	ShortName  string               `json:"shortName,omitempty"`
	Status     string               `json:"status"`
	StatusText string               `json:"statusText,omitempty"`
	StartUnix  int64                `json:"startUnix"`
	Home       SportsTeam           `json:"home"`
	Away       SportsTeam           `json:"away"`
	HomeScore  string               `json:"homeScore,omitempty"`
	AwayScore  string               `json:"awayScore,omitempty"`
	Live       bool                 `json:"live"`
	Completed  bool                 `json:"completed"`
	Channels   []SportsChannelMatch `json:"channels"`
}

type SportsChannelMatch struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CategoryName string `json:"categoryName,omitempty"`
	LogoURL      string `json:"logoUrl,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Score        int    `json:"score"`
}

type sportsFavoriteRequest struct {
	TeamID  string `json:"teamId"`
	Enabled bool   `json:"enabled"`
}

func (s *HTTPRoutesServer) handleSports(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	payload := s.sportsPayload(ctx, queryValue(request, "refresh") == "1")
	return s.respondJSON(http.StatusOK, payload)
}

func (s *HTTPRoutesServer) handleSportsFavorite(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != http.MethodPost {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	var payload sportsFavoriteRequest
	if err := json.Unmarshal(request.GetBody(), &payload); err != nil {
		return textResponse(http.StatusBadRequest, "invalid sports favorite payload"), nil
	}
	teamID := strings.TrimSpace(payload.TeamID)
	if teamID == "" {
		return textResponse(http.StatusBadRequest, "missing teamId"), nil
	}
	return s.respondJSON(http.StatusOK, s.store.SetSportsFavoriteTeam(teamID, payload.Enabled))
}

func (s *HTTPRoutesServer) sportsPayload(ctx context.Context, refresh bool) SportsPayload {
	now := time.Now()
	events, updatedUnix, source, err := s.cachedSportsEvents(ctx, now, refresh)
	preferences := s.store.Preferences()
	favorites := preferences.SportsFavoriteTeams
	if favorites == nil {
		favorites = map[string]bool{}
	}
	snapshot := s.store.Current()
	for index := range events {
		events[index].Home.Favorite = favorites[events[index].Home.ID]
		events[index].Away.Favorite = favorites[events[index].Away.ID]
		events[index].Channels = matchSportsChannels(events[index], snapshot)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Live != events[j].Live {
			return events[i].Live
		}
		if events[i].StartUnix != events[j].StartUnix {
			return events[i].StartUnix < events[j].StartUnix
		}
		return events[i].Name < events[j].Name
	})
	payload := SportsPayload{
		UpdatedAtUnix: updatedUnix,
		Source:        source,
		Leagues:       sportsLeagues(events),
		Events:        events,
		FavoriteTeams: sortedBoolKeys(favorites),
	}
	if err != nil {
		payload.Error = err.Error()
	}
	return payload
}

func (s *HTTPRoutesServer) cachedSportsEvents(ctx context.Context, now time.Time, refresh bool) ([]SportsEvent, int64, string, error) {
	s.sportsMu.Lock()
	defer s.sportsMu.Unlock()

	if !refresh && now.Before(s.sportsCache.ExpiresAfter) {
		return cloneSportsEvents(s.sportsCache.Events), s.sportsCache.UpdatedUnix, s.sportsCache.Source, nil
	}
	provider := s.sportsProvider
	if provider == nil {
		provider = noopSportsProvider{}
	}
	events, err := provider.Events(ctx, now)
	source := provider.Source()
	if err != nil {
		if len(s.sportsCache.Events) > 0 {
			return cloneSportsEvents(s.sportsCache.Events), s.sportsCache.UpdatedUnix, s.sportsCache.Source, err
		}
		return []SportsEvent{}, now.Unix(), source, err
	}
	events = normalizeSportsEvents(events)
	updatedUnix := now.Unix()
	s.sportsCache = sportsEventCache{
		Events:       cloneSportsEvents(events),
		UpdatedUnix:  updatedUnix,
		Source:       source,
		ExpiresAfter: now.Add(sportsCacheTTL(events)),
	}
	return cloneSportsEvents(events), updatedUnix, source, nil
}

func sportsCacheTTL(events []SportsEvent) time.Duration {
	for _, event := range events {
		if event.Live {
			return 30 * time.Second
		}
	}
	return 5 * time.Minute
}

func sportsLeagues(events []SportsEvent) []SportsLeague {
	byID := map[string]*SportsLeague{}
	for _, event := range events {
		id := strings.TrimSpace(event.LeagueID)
		if id == "" {
			id = "sports"
		}
		league := byID[id]
		if league == nil {
			league = &SportsLeague{ID: id, Name: firstNonEmpty(event.LeagueName, id)}
			byID[id] = league
		}
		if event.Live {
			league.LiveCount++
		} else if !event.Completed {
			league.UpcomingCount++
		}
	}
	leagues := make([]SportsLeague, 0, len(byID))
	for _, league := range byID {
		leagues = append(leagues, *league)
	}
	sort.Slice(leagues, func(i, j int) bool {
		return leagues[i].Name < leagues[j].Name
	})
	return leagues
}

func normalizeSportsEvents(events []SportsEvent) []SportsEvent {
	normalized := make([]SportsEvent, 0, len(events))
	for _, event := range events {
		event.ID = strings.TrimSpace(event.ID)
		event.LeagueID = strings.TrimSpace(event.LeagueID)
		event.LeagueName = strings.TrimSpace(event.LeagueName)
		event.Name = strings.TrimSpace(event.Name)
		event.ShortName = strings.TrimSpace(event.ShortName)
		event.Status = strings.TrimSpace(event.Status)
		event.StatusText = strings.TrimSpace(event.StatusText)
		event.Home = normalizeSportsTeam(event.Home)
		event.Away = normalizeSportsTeam(event.Away)
		if event.ID == "" {
			event.ID = stableSportsID(event)
		}
		if event.Name == "" {
			event.Name = strings.TrimSpace(event.Away.Name + " at " + event.Home.Name)
		}
		if event.ShortName == "" {
			event.ShortName = event.Name
		}
		if event.Status == "" {
			event.Status = "scheduled"
		}
		normalized = append(normalized, event)
	}
	return normalized
}

func normalizeSportsTeam(team SportsTeam) SportsTeam {
	team.ID = strings.TrimSpace(team.ID)
	team.Name = strings.TrimSpace(team.Name)
	team.Abbreviation = strings.TrimSpace(team.Abbreviation)
	team.LogoURL = strings.TrimSpace(team.LogoURL)
	if team.ID == "" {
		team.ID = stableSportsTeamID(team)
	}
	return team
}

func stableSportsID(event SportsEvent) string {
	parts := []string{event.LeagueID, event.Name, event.Home.Name, event.Away.Name, fmt.Sprintf("%d", event.StartUnix)}
	return "sports:" + sportsHash(strings.Join(parts, "|"))
}

func stableSportsTeamID(team SportsTeam) string {
	return "sports-team:" + sportsHash(strings.ToLower(strings.TrimSpace(team.Name+"|"+team.Abbreviation)))
}

func sportsHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func cloneSportsEvents(events []SportsEvent) []SportsEvent {
	clone := make([]SportsEvent, len(events))
	for index, event := range events {
		event.Channels = append([]SportsChannelMatch(nil), event.Channels...)
		clone[index] = event
	}
	return clone
}

func sortedBoolKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if value {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

type sportsTerm struct {
	Text   string
	Reason string
	Weight int
}

func matchSportsChannels(event SportsEvent, snapshot cache.Snapshot) []SportsChannelMatch {
	terms := sportsMatchTerms(event)
	if len(terms) == 0 {
		return []SportsChannelMatch{}
	}
	categoryNames := map[string]string{}
	for _, category := range liveCategories(snapshot) {
		categoryNames[category.ID] = category.Name
	}
	programsByChannel := map[string][]model.Program{}
	for _, program := range snapshot.Catalog.Programs {
		programsByChannel[program.ChannelID] = append(programsByChannel[program.ChannelID], program)
	}
	matches := make([]SportsChannelMatch, 0)
	for _, channel := range snapshot.Catalog.Channels {
		score, reason := scoreSportsChannel(channel, categoryNames[channel.CategoryID], programsByChannel[channel.ID], event, terms)
		if score <= 0 {
			continue
		}
		matches = append(matches, SportsChannelMatch{
			ID:           channel.ID,
			Name:         channel.Name,
			CategoryName: firstNonEmpty(categoryNames[channel.CategoryID], channel.CategoryName),
			LogoURL:      channel.LogoURL,
			Reason:       reason,
			Score:        score,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Name < matches[j].Name
	})
	if len(matches) > 8 {
		matches = matches[:8]
	}
	return matches
}

func sportsMatchTerms(event SportsEvent) []sportsTerm {
	var terms []sportsTerm
	add := func(text, reason string, weight int) {
		text = strings.TrimSpace(text)
		if text == "" || len([]rune(text)) < 3 {
			return
		}
		normalized := normalizeMatchText(text)
		for _, existing := range terms {
			if normalizeMatchText(existing.Text) == normalized {
				return
			}
		}
		terms = append(terms, sportsTerm{Text: text, Reason: reason, Weight: weight})
	}
	add(event.Home.Name, event.Home.Name, 60)
	add(event.Away.Name, event.Away.Name, 60)
	add(event.Home.Abbreviation, event.Home.Abbreviation, 28)
	add(event.Away.Abbreviation, event.Away.Abbreviation, 28)
	add(event.LeagueName, event.LeagueName, 16)
	add(event.Name, "event title", 22)
	add(event.ShortName, "event title", 22)
	return terms
}

func scoreSportsChannel(channel model.Channel, categoryName string, programs []model.Program, event SportsEvent, terms []sportsTerm) (int, string) {
	score := 0
	reasons := map[string]bool{}
	channelText := normalizeMatchText(strings.Join([]string{channel.Name, channel.Number}, " "))
	categoryText := normalizeMatchText(strings.Join([]string{categoryName, channel.CategoryName}, " "))
	for _, term := range terms {
		if containsMatchTerm(channelText, term.Text) {
			score += term.Weight
			reasons["channel: "+term.Reason] = true
		}
		if containsMatchTerm(categoryText, term.Text) {
			score += term.Weight / 2
			reasons["group: "+term.Reason] = true
		}
	}
	for _, program := range programs {
		if !programNearSportsEvent(program, event) {
			continue
		}
		programText := normalizeMatchText(strings.Join([]string{program.Title, program.Summary}, " "))
		for _, term := range terms {
			if containsMatchTerm(programText, term.Text) {
				score += term.Weight + 20
				reasons["guide: "+term.Reason] = true
			}
		}
	}
	if score == 0 {
		return 0, ""
	}
	return score, joinMatchReasons(reasons)
}

func programNearSportsEvent(program model.Program, event SportsEvent) bool {
	if event.StartUnix == 0 {
		return true
	}
	start := event.StartUnix - 6*3600
	end := event.StartUnix + 8*3600
	programStart := program.StartUnix
	programEnd := program.EndUnix
	if programEnd == 0 {
		programEnd = programStart + 2*3600
	}
	return programEnd >= start && programStart <= end
}

func joinMatchReasons(reasons map[string]bool) string {
	values := make([]string, 0, len(reasons))
	for reason := range reasons {
		values = append(values, reason)
	}
	sort.Strings(values)
	if len(values) > 3 {
		values = values[:3]
	}
	return strings.Join(values, ", ")
}

func normalizeMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	space := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			space = false
			continue
		}
		if !space {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func containsMatchTerm(text, term string) bool {
	term = normalizeMatchText(term)
	if term == "" {
		return false
	}
	return strings.Contains(" "+text+" ", " "+term+" ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type noopSportsProvider struct{}

func (noopSportsProvider) Events(context.Context, time.Time) ([]SportsEvent, error) {
	return []SportsEvent{}, nil
}

func (noopSportsProvider) Source() string {
	return "none"
}

type espnSportsProvider struct {
	client *http.Client
}

type espnLeagueConfig struct {
	ID     string
	Sport  string
	League string
	Name   string
}

var espnSportsLeagues = []espnLeagueConfig{
	{ID: "nfl", Sport: "football", League: "nfl", Name: "NFL"},
	{ID: "nba", Sport: "basketball", League: "nba", Name: "NBA"},
	{ID: "mlb", Sport: "baseball", League: "mlb", Name: "MLB"},
	{ID: "nhl", Sport: "hockey", League: "nhl", Name: "NHL"},
	{ID: "wnba", Sport: "basketball", League: "wnba", Name: "WNBA"},
	{ID: "mls", Sport: "soccer", League: "usa.1", Name: "MLS"},
	{ID: "epl", Sport: "soccer", League: "eng.1", Name: "Premier League"},
	{ID: "world-cup", Sport: "soccer", League: "fifa.world", Name: "World Cup"},
}

func newESPNSportsProvider(client *http.Client) espnSportsProvider {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return espnSportsProvider{client: client}
}

func (p espnSportsProvider) Source() string {
	return "espn"
}

func (p espnSportsProvider) Events(ctx context.Context, now time.Time) ([]SportsEvent, error) {
	events := make([]SportsEvent, 0)
	var firstErr error
	for _, league := range espnSportsLeagues {
		leagueEvents, err := p.leagueEvents(ctx, league, now)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		events = append(events, leagueEvents...)
	}
	if len(events) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return events, nil
}

func (p espnSportsProvider) leagueEvents(ctx context.Context, league espnLeagueConfig, now time.Time) ([]SportsEvent, error) {
	endpoint := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/%s/%s/scoreboard?limit=100", url.PathEscape(league.Sport), url.PathEscape(league.League))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("espn %s returned %d", league.Name, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var payload espnScoreboard
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	events := make([]SportsEvent, 0, len(payload.Events))
	for _, event := range payload.Events {
		converted := event.sportsEvent(league)
		if converted.ID == "" {
			continue
		}
		if converted.StartUnix == 0 {
			converted.StartUnix = now.Unix()
		}
		events = append(events, converted)
	}
	return events, nil
}

type espnScoreboard struct {
	Events []espnEvent `json:"events"`
}

type espnEvent struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ShortName    string            `json:"shortName"`
	Date         string            `json:"date"`
	Status       espnStatus        `json:"status"`
	Competitions []espnCompetition `json:"competitions"`
}

type espnStatus struct {
	Type espnStatusType `json:"type"`
}

type espnStatusType struct {
	State     string `json:"state"`
	Detail    string `json:"detail"`
	Completed bool   `json:"completed"`
}

type espnCompetition struct {
	Competitors []espnCompetitor `json:"competitors"`
}

type espnCompetitor struct {
	HomeAway string   `json:"homeAway"`
	Score    string   `json:"score"`
	Team     espnTeam `json:"team"`
}

type espnTeam struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	ShortName    string `json:"shortDisplayName"`
	Abbreviation string `json:"abbreviation"`
	Logo         string `json:"logo"`
}

func (event espnEvent) sportsEvent(league espnLeagueConfig) SportsEvent {
	var home, away SportsTeam
	var homeScore, awayScore string
	if len(event.Competitions) > 0 {
		for _, competitor := range event.Competitions[0].Competitors {
			team := SportsTeam{
				ID:           "espn:" + league.ID + ":" + strings.TrimSpace(competitor.Team.ID),
				Name:         firstNonEmpty(competitor.Team.DisplayName, competitor.Team.ShortName),
				Abbreviation: competitor.Team.Abbreviation,
				LogoURL:      competitor.Team.Logo,
			}
			if competitor.HomeAway == "home" {
				home = team
				homeScore = competitor.Score
			} else {
				away = team
				awayScore = competitor.Score
			}
		}
	}
	startUnix := int64(0)
	if start, err := time.Parse(time.RFC3339, event.Date); err == nil {
		startUnix = start.Unix()
	}
	state := strings.ToLower(strings.TrimSpace(event.Status.Type.State))
	return SportsEvent{
		ID:         "espn:" + league.ID + ":" + strings.TrimSpace(event.ID),
		LeagueID:   league.ID,
		LeagueName: league.Name,
		Name:       event.Name,
		ShortName:  event.ShortName,
		Status:     firstNonEmpty(state, "scheduled"),
		StatusText: event.Status.Type.Detail,
		StartUnix:  startUnix,
		Home:       home,
		Away:       away,
		HomeScore:  homeScore,
		AwayScore:  awayScore,
		Live:       state == "in",
		Completed:  event.Status.Type.Completed || state == "post",
	}
}
