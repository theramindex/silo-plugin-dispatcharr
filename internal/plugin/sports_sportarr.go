package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultSportarrAPIBaseURL = "https://sportarr.net/api/public/v1"
	sportarrPageSize          = 100
	sportarrMaxPages          = 20
	sportarrPageConcurrency   = 4
	sportarrDetailConcurrency = 6
	sportarrTeamCacheLimit    = 256
	sportarrLeagueCacheLimit  = 64
	sportarrImageCacheLimit   = 256
	sportarrMaxRetries        = 2
)

type sportarrSportsProvider struct {
	client  *http.Client
	baseURL string

	metadataMu   sync.Mutex
	teams        map[string]sportarrTeamCacheEntry
	leagues      map[string]sportarrLeagueCacheEntry
	images       map[string]sportarrImageCacheEntry
	teamFlight   map[string]*sportarrTeamFlight
	leagueFlight map[string]*sportarrLeagueFlight
	imageFlight  map[string]*sportarrImageFlight
}

type sportarrTeamFlight struct {
	done chan struct{}
	team sportarrTeam
	err  error
}

type sportarrLeagueFlight struct {
	done   chan struct{}
	league sportarrLeague
	err    error
}

type sportarrImageFlight struct {
	done  chan struct{}
	image string
	err   error
}

type sportarrTeamCacheEntry struct {
	Team      sportarrTeam
	ExpiresAt time.Time
}

type sportarrLeagueCacheEntry struct {
	League    sportarrLeague
	ExpiresAt time.Time
}

type sportarrImageCacheEntry struct {
	ImageURL  string
	ExpiresAt time.Time
}

type sportarrDetailRequest struct {
	kind string
	id   string
}

type sportarrEventsPage struct {
	Items      []sportarrEvent `json:"items"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	TotalPages int             `json:"totalPages"`
}

type sportarrEvent struct {
	ID                  string         `json:"id"`
	ShortID             string         `json:"shortId"`
	Name                string         `json:"name"`
	ShortName           string         `json:"shortName"`
	EventType           string         `json:"eventType"`
	LeagueID            string         `json:"leagueId"`
	LeagueName          string         `json:"leagueName"`
	SeasonName          string         `json:"seasonName"`
	Round               string         `json:"round"`
	VenueName           string         `json:"venueName"`
	ScheduledStart      string         `json:"scheduledStart"`
	ScheduledStartLocal string         `json:"scheduledStartLocal"`
	ScheduledEnd        string         `json:"scheduledEnd"`
	BroadcastTimezone   string         `json:"broadcastTimezone"`
	Status              string         `json:"status"`
	HomeTeamID          string         `json:"homeTeamId"`
	HomeTeamName        string         `json:"homeTeamName"`
	AwayTeamID          string         `json:"awayTeamId"`
	AwayTeamName        string         `json:"awayTeamName"`
	HomeScore           sportarrString `json:"homeScore"`
	AwayScore           sportarrString `json:"awayScore"`
	Parts               []any          `json:"parts"`
}

type sportarrString string

func (value *sportarrString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*value = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = sportarrString(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*value = sportarrString(number.String())
	return nil
}

type sportarrTeam struct {
	ID             string   `json:"id"`
	ShortID        string   `json:"shortId"`
	Name           string   `json:"name"`
	Abbreviation   string   `json:"abbreviation"`
	LogoURL        string   `json:"logoUrl"`
	PrimaryColor   string   `json:"primaryColor"`
	SecondaryColor string   `json:"secondaryColor"`
	AlternateNames []string `json:"alternateNames"`
}

type sportarrLeague struct {
	ID             string   `json:"id"`
	ShortID        string   `json:"shortId"`
	Name           string   `json:"name"`
	Slug           string   `json:"slug"`
	Abbreviation   string   `json:"abbreviation"`
	Description    string   `json:"description"`
	SportName      string   `json:"sportName"`
	LogoURL        string   `json:"logoUrl"`
	AlternateNames []string `json:"alternateNames"`
}

type sportarrEntityImageResponse struct {
	Images []sportarrEntityImage `json:"images"`
}

type sportarrEntityImage struct {
	ImageType string `json:"image_type"`
	URL       string `json:"url"`
	IsPrimary bool   `json:"is_primary"`
	Priority  int    `json:"priority"`
}

func newSportarrSportsProvider(client *http.Client) *sportarrSportsProvider {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &sportarrSportsProvider{
		client:       client,
		baseURL:      defaultSportarrAPIBaseURL,
		teams:        map[string]sportarrTeamCacheEntry{},
		leagues:      map[string]sportarrLeagueCacheEntry{},
		images:       map[string]sportarrImageCacheEntry{},
		teamFlight:   map[string]*sportarrTeamFlight{},
		leagueFlight: map[string]*sportarrLeagueFlight{},
		imageFlight:  map[string]*sportarrImageFlight{},
	}
}

func (p *sportarrSportsProvider) Source() string {
	return "sportarr"
}

func (p *sportarrSportsProvider) Events(ctx context.Context, now time.Time) ([]SportsEvent, error) {
	from := now.Add(-24 * time.Hour).UTC().Format("2006-01-02")
	to := now.Add(72 * time.Hour).UTC().Format("2006-01-02")
	first, err := p.eventsPage(ctx, from, to, 1)
	if err != nil {
		return nil, err
	}
	pageCount := first.TotalPages
	if pageCount < 1 {
		pageCount = 1
	}
	if pageCount > sportarrMaxPages {
		pageCount = sportarrMaxPages
	}
	pages := make([][]sportarrEvent, pageCount)
	pages[0] = limitSportarrEvents(first.Items)
	if pageCount > 1 {
		if err := p.fetchRemainingEventPages(ctx, from, to, pages); err != nil {
			return nil, err
		}
	}

	capacity := first.Total
	if capacity < 0 {
		capacity = 0
	}
	maximumEvents := sportarrMaxPages * sportarrPageSize
	if capacity > maximumEvents {
		capacity = maximumEvents
	}
	seen := make(map[string]bool, capacity)
	events := make([]SportsEvent, 0, capacity)
	for _, page := range pages {
		for _, event := range page {
			converted := event.sportsEvent()
			if converted.ID == "" || seen[converted.ID] {
				continue
			}
			seen[converted.ID] = true
			events = append(events, converted)
		}
	}
	return events, nil
}

func limitSportarrEvents(events []sportarrEvent) []sportarrEvent {
	if len(events) > sportarrPageSize {
		return events[:sportarrPageSize]
	}
	return events
}

func (p *sportarrSportsProvider) fetchRemainingEventPages(ctx context.Context, from, to string, pages [][]sportarrEvent) error {
	jobs := make(chan int)
	errs := make(chan error, len(pages)-1)
	var wg sync.WaitGroup
	workerCount := sportarrPageConcurrency
	if workerCount > len(pages)-1 {
		workerCount = len(pages) - 1
	}
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				page, err := p.eventsPage(ctx, from, to, index+1)
				if err != nil {
					errs <- err
					continue
				}
				pages[index] = limitSportarrEvents(page.Items)
			}
		}()
	}
	for index := 1; index < len(pages); index++ {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

func (p *sportarrSportsProvider) eventsPage(ctx context.Context, from, to string, page int) (sportarrEventsPage, error) {
	endpoint, err := url.Parse(strings.TrimRight(p.baseURL, "/") + "/events")
	if err != nil {
		return sportarrEventsPage{}, err
	}
	query := endpoint.Query()
	query.Set("from", from)
	query.Set("to", to)
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("page_size", fmt.Sprintf("%d", sportarrPageSize))
	endpoint.RawQuery = query.Encode()
	var payload sportarrEventsPage
	if err := p.getJSON(ctx, endpoint.String(), &payload); err != nil {
		return sportarrEventsPage{}, fmt.Errorf("sportarr events page %d: %w", page, err)
	}
	return payload, nil
}

func (p *sportarrSportsProvider) getJSON(ctx context.Context, endpoint string, target any) error {
	for attempt := 0; attempt <= sportarrMaxRetries; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "Silo-Dispatcharr/1.0")
		request.Header.Set("Cache-Control", "no-cache, no-store")
		request.Header.Set("Pragma", "no-cache")
		response, err := p.client.Do(request)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if err := json.Unmarshal(body, target); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			return nil
		}
		if attempt < sportarrMaxRetries && (response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500) {
			if err := waitForSportarrRetry(ctx, response.Header.Get("Retry-After"), attempt); err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("returned %d", response.StatusCode)
	}
	return fmt.Errorf("retry limit exceeded")
}

func waitForSportarrRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := time.Duration(1<<attempt) * 250 * time.Millisecond
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
		delay = time.Duration(seconds) * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (event sportarrEvent) sportsEvent() SportsEvent {
	id := firstNonEmpty(event.ShortID, event.ID)
	if id != "" {
		id = "sportarr:" + id
	}
	startUnix := parseSportarrTime(event.ScheduledStart)
	if startUnix == 0 {
		startUnix = parseSportarrTime(event.ScheduledStartLocal)
	}
	endUnix := parseSportarrTime(event.ScheduledEnd)
	status := normalizeSportarrStatus(event.Status)
	live := status == "live"
	completed := status == "completed"
	statusText := ""
	if live {
		statusText = "Live"
	} else if completed {
		statusText = "Final"
	}
	return SportsEvent{
		ID:                id,
		ProviderID:        event.ID,
		LeagueID:          event.LeagueID,
		LeagueName:        event.LeagueName,
		Name:              event.Name,
		ShortName:         event.ShortName,
		EventType:         event.EventType,
		Season:            event.SeasonName,
		Round:             event.Round,
		Venue:             event.VenueName,
		BroadcastTimezone: event.BroadcastTimezone,
		Status:            status,
		StatusText:        statusText,
		StartUnix:         startUnix,
		EndUnix:           endUnix,
		Home: SportsTeam{
			ID:   event.HomeTeamID,
			Name: event.HomeTeamName,
		},
		Away: SportsTeam{
			ID:   event.AwayTeamID,
			Name: event.AwayTeamName,
		},
		HomeScore: string(event.HomeScore),
		AwayScore: string(event.AwayScore),
		Live:      live,
		Completed: completed,
	}
}

func parseSportarrTime(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z07:00"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Unix()
		}
	}
	return 0
}

func normalizeSportarrStatus(value string) string {
	status := strings.ToLower(strings.TrimSpace(value))
	switch status {
	case "live", "in_progress", "in progress", "in", "ongoing", "halftime":
		return "live"
	case "completed", "complete", "final", "finished", "post":
		return "completed"
	case "postponed", "cancelled", "canceled", "suspended", "delayed":
		return status
	case "", "scheduled", "pre", "not_started", "not started":
		return "scheduled"
	default:
		return status
	}
}

func (p *sportarrSportsProvider) EnrichEvents(ctx context.Context, events []SportsEvent, limit int) []SportsEvent {
	if limit <= 0 || len(events) == 0 {
		return events
	}
	p.purgeExpiredMetadata(time.Now())
	requests := make([]sportarrDetailRequest, 0, limit*3)
	seen := map[string]bool{}
	selected := 0
	for _, event := range events {
		if len(event.Channels) == 0 || selected >= limit {
			continue
		}
		selected++
		for _, request := range []sportarrDetailRequest{{kind: "league", id: event.LeagueID}, {kind: "team", id: event.Home.ID}, {kind: "team", id: event.Away.ID}, {kind: "event", id: event.ProviderID}} {
			key := request.kind + ":" + request.id
			if request.id == "" || seen[key] {
				continue
			}
			seen[key] = true
			requests = append(requests, request)
		}
	}
	if len(requests) > 0 {
		p.fetchDetails(ctx, requests)
	}
	for index := range events {
		events[index] = p.applyCachedDetails(events[index])
	}
	return events
}

func (p *sportarrSportsProvider) purgeExpiredMetadata(now time.Time) {
	p.metadataMu.Lock()
	defer p.metadataMu.Unlock()
	for id, entry := range p.teams {
		if !now.Before(entry.ExpiresAt) {
			delete(p.teams, id)
		}
	}
	for id, entry := range p.leagues {
		if !now.Before(entry.ExpiresAt) {
			delete(p.leagues, id)
		}
	}
	for id, entry := range p.images {
		if !now.Before(entry.ExpiresAt) {
			delete(p.images, id)
		}
	}
}

func (p *sportarrSportsProvider) fetchDetails(ctx context.Context, requests []sportarrDetailRequest) {
	jobs := make(chan sportarrDetailRequest)
	var wg sync.WaitGroup
	workers := sportarrDetailConcurrency
	if workers > len(requests) {
		workers = len(requests)
	}
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for request := range jobs {
				if request.kind == "team" {
					_, _ = p.team(ctx, request.id)
				} else if request.kind == "league" {
					_, _ = p.league(ctx, request.id)
				} else {
					_, _ = p.eventImage(ctx, request.id)
				}
			}
		}()
	}
	for _, request := range requests {
		jobs <- request
	}
	close(jobs)
	wg.Wait()
}

func (p *sportarrSportsProvider) eventImage(ctx context.Context, id string) (string, error) {
	now := time.Now()
	p.metadataMu.Lock()
	cached, ok := p.images[id]
	if ok && now.Before(cached.ExpiresAt) {
		p.metadataMu.Unlock()
		return cached.ImageURL, nil
	}
	if flight := p.imageFlight[id]; flight != nil {
		p.metadataMu.Unlock()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-flight.done:
			return flight.image, flight.err
		}
	}
	flight := &sportarrImageFlight{done: make(chan struct{})}
	p.imageFlight[id] = flight
	p.metadataMu.Unlock()

	endpoint := sportarrRootURL(p.baseURL) + "/api/v1/images/entity/event/" + url.PathEscape(id) + "?completed_only=true"
	var payload sportarrEntityImageResponse
	err := p.getJSON(ctx, endpoint, &payload)
	imageURL := pickSportarrEventImage(payload.Images)
	p.metadataMu.Lock()
	if err == nil {
		ttl := 24 * time.Hour
		if imageURL == "" {
			ttl = 15 * time.Minute
		}
		p.storeImageLocked(id, sportarrImageCacheEntry{ImageURL: imageURL, ExpiresAt: now.Add(ttl)})
	}
	flight.image = imageURL
	flight.err = err
	delete(p.imageFlight, id)
	close(flight.done)
	p.metadataMu.Unlock()
	return imageURL, err
}

func sportarrRootURL(baseURL string) string {
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/api/public/v1")
}

func pickSportarrEventImage(images []sportarrEntityImage) string {
	bestURL := ""
	bestTypeRank := -1
	bestPrimary := false
	bestPriority := 0
	for _, image := range images {
		typeRank := map[string]int{"backdrop": 3, "thumbnail": 2, "banner": 2, "poster": 1}[strings.ToLower(strings.TrimSpace(image.ImageType))]
		if typeRank == 0 {
			continue
		}
		imageURL := safeSportsImageURL(image.URL)
		if imageURL == "" {
			continue
		}
		better := typeRank > bestTypeRank
		if typeRank == bestTypeRank {
			better = image.IsPrimary && !bestPrimary
			if image.IsPrimary == bestPrimary {
				better = image.Priority > bestPriority
			}
		}
		if better {
			bestURL = imageURL
			bestTypeRank = typeRank
			bestPrimary = image.IsPrimary
			bestPriority = image.Priority
		}
	}
	return bestURL
}

func (p *sportarrSportsProvider) team(ctx context.Context, id string) (sportarrTeam, error) {
	now := time.Now()
	p.metadataMu.Lock()
	cached, ok := p.teams[id]
	if ok && now.Before(cached.ExpiresAt) {
		p.metadataMu.Unlock()
		return cached.Team, nil
	}
	if flight := p.teamFlight[id]; flight != nil {
		p.metadataMu.Unlock()
		select {
		case <-ctx.Done():
			return sportarrTeam{}, ctx.Err()
		case <-flight.done:
			return flight.team, flight.err
		}
	}
	flight := &sportarrTeamFlight{done: make(chan struct{})}
	p.teamFlight[id] = flight
	p.metadataMu.Unlock()

	var team sportarrTeam
	endpoint := strings.TrimRight(p.baseURL, "/") + "/teams/" + url.PathEscape(id)
	err := p.getJSON(ctx, endpoint, &team)
	p.metadataMu.Lock()
	if err == nil {
		p.storeTeamLocked(id, sportarrTeamCacheEntry{Team: team, ExpiresAt: now.Add(24 * time.Hour)})
	}
	flight.team = team
	flight.err = err
	delete(p.teamFlight, id)
	close(flight.done)
	p.metadataMu.Unlock()
	return team, err
}

func (p *sportarrSportsProvider) league(ctx context.Context, id string) (sportarrLeague, error) {
	now := time.Now()
	p.metadataMu.Lock()
	cached, ok := p.leagues[id]
	if ok && now.Before(cached.ExpiresAt) {
		p.metadataMu.Unlock()
		return cached.League, nil
	}
	if flight := p.leagueFlight[id]; flight != nil {
		p.metadataMu.Unlock()
		select {
		case <-ctx.Done():
			return sportarrLeague{}, ctx.Err()
		case <-flight.done:
			return flight.league, flight.err
		}
	}
	flight := &sportarrLeagueFlight{done: make(chan struct{})}
	p.leagueFlight[id] = flight
	p.metadataMu.Unlock()

	var league sportarrLeague
	endpoint := strings.TrimRight(p.baseURL, "/") + "/leagues/" + url.PathEscape(id)
	err := p.getJSON(ctx, endpoint, &league)
	p.metadataMu.Lock()
	if err == nil {
		p.storeLeagueLocked(id, sportarrLeagueCacheEntry{League: league, ExpiresAt: now.Add(24 * time.Hour)})
	}
	flight.league = league
	flight.err = err
	delete(p.leagueFlight, id)
	close(flight.done)
	p.metadataMu.Unlock()
	return league, err
}

func (p *sportarrSportsProvider) storeTeamLocked(id string, entry sportarrTeamCacheEntry) {
	if len(p.teams) >= sportarrTeamCacheLimit {
		p.evictOldestTeamLocked()
	}
	p.teams[id] = entry
}

func (p *sportarrSportsProvider) storeLeagueLocked(id string, entry sportarrLeagueCacheEntry) {
	if len(p.leagues) >= sportarrLeagueCacheLimit {
		p.evictOldestLeagueLocked()
	}
	p.leagues[id] = entry
}

func (p *sportarrSportsProvider) storeImageLocked(id string, entry sportarrImageCacheEntry) {
	if len(p.images) >= sportarrImageCacheLimit {
		p.evictOldestImageLocked()
	}
	p.images[id] = entry
}

func (p *sportarrSportsProvider) evictOldestTeamLocked() {
	oldestID := ""
	var oldest time.Time
	for id, entry := range p.teams {
		if oldestID == "" || entry.ExpiresAt.Before(oldest) {
			oldestID, oldest = id, entry.ExpiresAt
		}
	}
	delete(p.teams, oldestID)
}

func (p *sportarrSportsProvider) evictOldestLeagueLocked() {
	oldestID := ""
	var oldest time.Time
	for id, entry := range p.leagues {
		if oldestID == "" || entry.ExpiresAt.Before(oldest) {
			oldestID, oldest = id, entry.ExpiresAt
		}
	}
	delete(p.leagues, oldestID)
}

func (p *sportarrSportsProvider) evictOldestImageLocked() {
	oldestID := ""
	var oldest time.Time
	for id, entry := range p.images {
		if oldestID == "" || entry.ExpiresAt.Before(oldest) {
			oldestID, oldest = id, entry.ExpiresAt
		}
	}
	delete(p.images, oldestID)
}

func (p *sportarrSportsProvider) applyCachedDetails(event SportsEvent) SportsEvent {
	p.metadataMu.Lock()
	defer p.metadataMu.Unlock()
	if league, ok := p.leagues[event.LeagueID]; ok {
		event.LeagueName = firstNonEmpty(league.League.Name, event.LeagueName)
		event.LeagueLogoURL = safeSportsImageURL(league.League.LogoURL)
		event.LeagueDescription = strings.TrimSpace(league.League.Description)
		event.SportName = strings.TrimSpace(league.League.SportName)
	}
	if image, ok := p.images[event.ProviderID]; ok {
		event.ImageURL = safeSportsImageURL(image.ImageURL)
	}
	event.Home = applySportarrTeam(event.Home, p.teams[event.Home.ID].Team)
	event.Away = applySportarrTeam(event.Away, p.teams[event.Away.ID].Team)
	return event
}

func applySportarrTeam(team SportsTeam, metadata sportarrTeam) SportsTeam {
	team.Name = firstNonEmpty(metadata.Name, team.Name)
	team.Abbreviation = firstNonEmpty(metadata.Abbreviation, team.Abbreviation)
	team.LogoURL = safeSportsImageURL(metadata.LogoURL)
	team.PrimaryColor = strings.TrimSpace(metadata.PrimaryColor)
	team.SecondaryColor = strings.TrimSpace(metadata.SecondaryColor)
	return team
}
