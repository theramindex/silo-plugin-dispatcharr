package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

type sportsEventEnricher interface {
	EnrichEvents(context.Context, []SportsEvent, int) []SportsEvent
}

const (
	sportsChannelMinimumScore  = 28
	sportsProviderFetchTimeout = 4 * time.Second
)

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
	SportName     string `json:"sportName,omitempty"`
	LogoURL       string `json:"logoUrl,omitempty"`
	Description   string `json:"description,omitempty"`
	LiveCount     int    `json:"liveCount"`
	UpcomingCount int    `json:"upcomingCount"`
}

type SportsTeam struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Abbreviation   string `json:"abbreviation,omitempty"`
	LogoURL        string `json:"logoUrl,omitempty"`
	PrimaryColor   string `json:"primaryColor,omitempty"`
	SecondaryColor string `json:"secondaryColor,omitempty"`
	Favorite       bool   `json:"favorite,omitempty"`
}

type SportsEvent struct {
	ID                string               `json:"id"`
	ProviderID        string               `json:"providerId,omitempty"`
	LeagueID          string               `json:"leagueId"`
	LeagueName        string               `json:"leagueName"`
	LeagueLogoURL     string               `json:"leagueLogoUrl,omitempty"`
	LeagueDescription string               `json:"leagueDescription,omitempty"`
	SportName         string               `json:"sportName,omitempty"`
	Name              string               `json:"name"`
	ShortName         string               `json:"shortName,omitempty"`
	EventType         string               `json:"eventType,omitempty"`
	Season            string               `json:"season,omitempty"`
	Round             string               `json:"round,omitempty"`
	Venue             string               `json:"venue,omitempty"`
	BroadcastTimezone string               `json:"broadcastTimezone,omitempty"`
	ImageURL          string               `json:"imageUrl,omitempty"`
	Description       string               `json:"description,omitempty"`
	Status            string               `json:"status"`
	StatusText        string               `json:"statusText,omitempty"`
	StartUnix         int64                `json:"startUnix"`
	EndUnix           int64                `json:"endUnix,omitempty"`
	Home              SportsTeam           `json:"home"`
	Away              SportsTeam           `json:"away"`
	HomeScore         string               `json:"homeScore,omitempty"`
	AwayScore         string               `json:"awayScore,omitempty"`
	Live              bool                 `json:"live"`
	Completed         bool                 `json:"completed"`
	Channels          []SportsChannelMatch `json:"channels"`
}

type SportsChannelMatch struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CategoryName string `json:"categoryName,omitempty"`
	LogoURL      string `json:"logoUrl,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Score        int    `json:"score"`
}

func (s *HTTPRoutesServer) handleSports(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	payload := s.sportsPayload(ctx, queryValue(request, "refresh") == "1")
	return s.respondJSON(http.StatusOK, payload)
}

func (s *HTTPRoutesServer) handleSportsFavorite(request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	return userStateUnavailableResponse(), nil
}

func (s *HTTPRoutesServer) sportsPayload(ctx context.Context, refresh bool) SportsPayload {
	now := time.Now()
	providerCtx, cancelProvider := context.WithTimeout(ctx, sportsProviderFetchTimeout)
	events, updatedUnix, source, err := s.cachedSportsEvents(providerCtx, now, refresh)
	cancelProvider()
	snapshot := s.store.Current()
	channelIndex := newSportsChannelIndex(snapshot)
	for index := range events {
		events[index].Home.Favorite = false
		events[index].Away.Favorite = false
		events[index].Channels = channelIndex.Match(events[index])
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Live != events[j].Live {
			return events[i].Live
		}
		leftStart := sportsSortStartUnix(events[i])
		rightStart := sportsSortStartUnix(events[j])
		if leftStart != rightStart {
			return leftStart < rightStart
		}
		return events[i].Name < events[j].Name
	})
	if enricher, ok := s.sportsProvider.(sportsEventEnricher); ok {
		events = enricher.EnrichEvents(ctx, events, 8)
	}
	payload := SportsPayload{
		UpdatedAtUnix: updatedUnix,
		Source:        source,
		Leagues:       sportsLeagues(events),
		Events:        events,
		FavoriteTeams: []string{},
	}
	if err != nil {
		payload.Error = err.Error()
	}
	return payload
}

func sportsSortStartUnix(event SportsEvent) int64 {
	if event.StartUnix > 0 {
		return event.StartUnix
	}
	return 1<<62 - 1
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
			league = &SportsLeague{
				ID:          id,
				Name:        firstNonEmpty(event.LeagueName, id),
				SportName:   event.SportName,
				LogoURL:     event.LeagueLogoURL,
				Description: event.LeagueDescription,
			}
			byID[id] = league
		}
		if league.SportName == "" {
			league.SportName = event.SportName
		}
		if league.LogoURL == "" {
			league.LogoURL = event.LeagueLogoURL
		}
		if league.Description == "" {
			league.Description = event.LeagueDescription
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
		event.ProviderID = strings.TrimSpace(event.ProviderID)
		event.LeagueID = strings.TrimSpace(event.LeagueID)
		event.LeagueName = strings.TrimSpace(event.LeagueName)
		event.LeagueLogoURL = safeSportsImageURL(event.LeagueLogoURL)
		event.LeagueDescription = strings.TrimSpace(event.LeagueDescription)
		event.SportName = strings.TrimSpace(event.SportName)
		event.Name = strings.TrimSpace(event.Name)
		event.ShortName = strings.TrimSpace(event.ShortName)
		event.EventType = strings.TrimSpace(event.EventType)
		event.Season = strings.TrimSpace(event.Season)
		event.Round = strings.TrimSpace(event.Round)
		event.Venue = strings.TrimSpace(event.Venue)
		event.BroadcastTimezone = strings.TrimSpace(event.BroadcastTimezone)
		event.ImageURL = safeSportsImageURL(event.ImageURL)
		event.Description = strings.TrimSpace(event.Description)
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
	team.LogoURL = safeSportsImageURL(team.LogoURL)
	team.PrimaryColor = strings.TrimSpace(team.PrimaryColor)
	team.SecondaryColor = strings.TrimSpace(team.SecondaryColor)
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
	Text         string
	Reason       string
	Weight       int
	TeamName     bool
	Abbreviation bool
}

type sportsIndexedChannel struct {
	Channel      model.Channel
	CategoryName string
	ChannelText  string
	CategoryText string
	Programs     []sportsIndexedProgram
}

type sportsIndexedProgram struct {
	Program model.Program
	Text    string
}

type sportsChannelIndex struct {
	Channels []sportsIndexedChannel
}

func newSportsChannelIndex(snapshot cache.Snapshot) sportsChannelIndex {
	categoryNames := map[string]string{}
	for _, category := range liveCategories(snapshot) {
		categoryNames[category.ID] = category.Name
	}
	programsByChannel := map[string][]sportsIndexedProgram{}
	for _, program := range snapshot.Catalog.Programs {
		programsByChannel[program.ChannelID] = append(programsByChannel[program.ChannelID], sportsIndexedProgram{
			Program: program,
			Text:    normalizeMatchText(strings.Join([]string{program.Title, program.Summary}, " ")),
		})
	}
	channels := make([]sportsIndexedChannel, 0, len(snapshot.Catalog.Channels))
	seenChannels := map[string]bool{}
	for _, channel := range snapshot.Catalog.Channels {
		if channel.ID == "" || seenChannels[channel.ID] {
			continue
		}
		seenChannels[channel.ID] = true
		categoryName := firstNonEmpty(categoryNames[channel.CategoryID], channel.CategoryName)
		channels = append(channels, sportsIndexedChannel{
			Channel:      channel,
			CategoryName: categoryName,
			ChannelText:  normalizeMatchText(strings.Join([]string{channel.Name, channel.Number}, " ")),
			CategoryText: normalizeMatchText(strings.Join([]string{categoryName, channel.CategoryName}, " ")),
			Programs:     programsByChannel[channel.ID],
		})
	}
	return sportsChannelIndex{Channels: channels}
}

func matchSportsChannels(event SportsEvent, snapshot cache.Snapshot) []SportsChannelMatch {
	return newSportsChannelIndex(snapshot).Match(event)
}

func (index sportsChannelIndex) Match(event SportsEvent) []SportsChannelMatch {
	terms := sportsMatchTerms(event)
	if len(terms) == 0 {
		return []SportsChannelMatch{}
	}
	matches := make([]SportsChannelMatch, 0)
	for _, indexed := range index.Channels {
		score, reason := scoreIndexedSportsChannel(indexed, event, terms)
		if score < sportsChannelMinimumScore {
			continue
		}
		matches = append(matches, SportsChannelMatch{
			ID:           indexed.Channel.ID,
			Name:         indexed.Channel.Name,
			CategoryName: indexed.CategoryName,
			LogoURL:      indexed.Channel.LogoURL,
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
	if len(matches) > 6 {
		matches = matches[:6]
	}
	return matches
}

func sportsMatchTerms(event SportsEvent) []sportsTerm {
	var terms []sportsTerm
	add := func(text, reason string, weight int, teamName, abbreviation bool) {
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
		terms = append(terms, sportsTerm{Text: text, Reason: reason, Weight: weight, TeamName: teamName, Abbreviation: abbreviation})
	}
	add(event.Home.Name, event.Home.Name, 60, true, false)
	add(event.Away.Name, event.Away.Name, 60, true, false)
	add(event.Home.Abbreviation, event.Home.Abbreviation, 28, false, true)
	add(event.Away.Abbreviation, event.Away.Abbreviation, 28, false, true)
	// League names are too broad for channel matching; "NFL" or "MLB" would pull in every team group.
	add(event.Name, "event title", 22, false, false)
	add(event.ShortName, "event title", 22, false, false)
	return terms
}

func scoreSportsChannel(channel model.Channel, categoryName string, programs []model.Program, event SportsEvent, terms []sportsTerm) (int, string) {
	indexedPrograms := make([]sportsIndexedProgram, 0, len(programs))
	for _, program := range programs {
		indexedPrograms = append(indexedPrograms, sportsIndexedProgram{
			Program: program,
			Text:    normalizeMatchText(strings.Join([]string{program.Title, program.Summary}, " ")),
		})
	}
	return scoreIndexedSportsChannel(sportsIndexedChannel{
		Channel:      channel,
		CategoryName: firstNonEmpty(categoryName, channel.CategoryName),
		ChannelText:  normalizeMatchText(strings.Join([]string{channel.Name, channel.Number}, " ")),
		CategoryText: normalizeMatchText(strings.Join([]string{categoryName, channel.CategoryName}, " ")),
		Programs:     indexedPrograms,
	}, event, terms)
}

func scoreIndexedSportsChannel(channel sportsIndexedChannel, event SportsEvent, terms []sportsTerm) (int, string) {
	score := 0
	structuralMatch := false
	strongGuideMatch := false
	reasons := map[string]bool{}
	channelText := channel.ChannelText
	categoryText := channel.CategoryText
	hasAbbreviationContext := sportsChannelAbbreviationContext(channelText, categoryText, event)
	for _, term := range terms {
		if term.Abbreviation && !hasAbbreviationContext {
			continue
		}
		if containsSportsStructuralTerm(channelText, term) {
			score += term.Weight
			structuralMatch = true
			reasons["channel: "+term.Reason] = true
		}
		if containsSportsStructuralTerm(categoryText, term) {
			score += term.Weight / 2
			structuralMatch = true
			reasons["group: "+term.Reason] = true
		}
	}
	for _, program := range channel.Programs {
		if !programNearSportsEvent(program.Program, event) {
			continue
		}
		programText := program.Text
		if strongSportsGuideMatch(programText, event) {
			strongGuideMatch = true
		}
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
	if !structuralMatch && !strongGuideMatch {
		return 0, ""
	}
	return score, joinMatchReasons(reasons)
}

// Abbreviations such as TEN and EDM are too ambiguous on their own. A channel
// needs to identify the event's league unless its guide explicitly confirms it.
func sportsChannelAbbreviationContext(channelText, categoryText string, event SportsEvent) bool {
	text := channelText + " " + categoryText
	return containsMatchTerm(text, event.LeagueName) || containsMatchTerm(text, event.LeagueID)
}

// Single-word national teams should not match a longer club name merely because
// the country is one word inside it (for example, England vs New England Revolution).
func containsSportsStructuralTerm(text string, term sportsTerm) bool {
	if !containsMatchTerm(text, term.Text) {
		return false
	}
	termText := normalizeMatchText(term.Text)
	if !term.TeamName || strings.Contains(termText, " ") {
		return true
	}
	return text == termText || strings.HasPrefix(text, termText+" ") || strings.HasSuffix(text, " "+termText)
}

func strongSportsGuideMatch(programText string, event SportsEvent) bool {
	if containsMatchTerm(programText, event.Name) || containsMatchTerm(programText, event.ShortName) {
		return true
	}
	homeName := containsMatchTerm(programText, event.Home.Name)
	awayName := containsMatchTerm(programText, event.Away.Name)
	if homeName && awayName {
		return true
	}
	homeAbbr := containsMatchTerm(programText, event.Home.Abbreviation)
	awayAbbr := containsMatchTerm(programText, event.Away.Abbreviation)
	if (homeName || homeAbbr) && (awayName || awayAbbr) {
		return true
	}
	leagueName := strings.TrimSpace(event.LeagueName)
	if leagueName != "" && containsMatchTerm(programText, leagueName) && (homeName || awayName || homeAbbr || awayAbbr) {
		return true
	}
	return false
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

func safeSportsImageURL(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	return value
}

type noopSportsProvider struct{}

func (noopSportsProvider) Events(context.Context, time.Time) ([]SportsEvent, error) {
	return []SportsEvent{}, nil
}

func (noopSportsProvider) Source() string {
	return "none"
}
