package plugin

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
)

func (s *HTTPRoutesServer) sportsPayload(ctx context.Context, refresh bool) SportsPayload {
	return s.sportsPayloadWithBudget(ctx, refresh, sportsBuildBudget{})
}

func (s *HTTPRoutesServer) sportsPayloadWithBudget(ctx context.Context, refresh bool, budget sportsBuildBudget) SportsPayload {
	budget = budget.withDefaults()
	now := time.Now()
	snapshot := s.store.Current()
	guideEvents, refreshScores := sportsEventsFromGuideWithScoreHints(snapshot, now)
	return s.buildSportsPayload(ctx, refresh, budget, snapshot, now, guideEvents, refreshScores)
}

// buildSportsPayload reuses the guide extraction already prepared by the
// background rebuild, instead of parsing it again for the provider merge.
func (s *HTTPRoutesServer) buildSportsPayload(ctx context.Context, refresh bool, budget sportsBuildBudget, snapshot cache.Snapshot, now time.Time, guideEvents []SportsEvent, refreshScores bool) SportsPayload {
	providerCtx, cancelProvider := context.WithTimeout(ctx, budget.Provider)
	events, updatedUnix, source, err := s.cachedSportsEvents(providerCtx, now, refresh || refreshScores)
	cancelProvider()
	if len(guideEvents) > 0 {
		if len(events) == 0 {
			events = collapseEquivalentSportsEvents(guideEvents)
			source = "EPG fallback"
			err = nil
		} else {
			events = mergeSportsGuideEvents(events, guideEvents)
			source = firstNonEmpty(source, "Sports provider") + " + EPG"
		}
		if updatedUnix <= 0 {
			updatedUnix = snapshot.Health.EPGLastSuccessUnix
			if updatedUnix <= 0 {
				updatedUnix = now.Unix()
			}
		}
	}
	events = collapseEquivalentSportsEvents(inferGuideSportsLeagues(events))
	// The match deadline starts after the provider fetch, independent of it.
	matchCtx, cancelMatch := context.WithTimeout(ctx, budget.Match)
	defer cancelMatch()
	channelIndex := newSportsChannelIndex(snapshot)
	incomplete := false
	for index := range events {
		events[index] = normalizeSportsEventFreshness(events[index], now)
		events[index].Home = finalizeSportsTeamIdentity(events[index].Home)
		events[index].Away = finalizeSportsTeamIdentity(events[index].Away)
		if len(events[index].Channels) > 0 {
			events[index].Channels = mergeSportsChannelMatches(events[index].Channels)
			events[index].MatchDiagnostics = make([]SportsMatchDiagnostic, 0, len(events[index].Channels))
			for _, match := range events[index].Channels {
				events[index].MatchDiagnostics = append(events[index].MatchDiagnostics, SportsMatchDiagnostic{
					ChannelID: match.ID, ChannelName: match.Name, Accepted: true,
					Evidence: "epg", Confidence: "high", Reason: firstNonEmpty(match.Reason, "EPG-derived event feed"), Score: match.Score,
				})
			}
			continue
		}
		if matchCtx.Err() != nil {
			incomplete = true
			continue
		}
		matches, diagnostics := channelIndex.MatchDetailedContext(matchCtx, events[index])
		if matchCtx.Err() != nil {
			// The channel scan stopped part way; this event's matches are partial.
			incomplete = true
		}
		events[index].Channels = mergeSportsChannelMatches(events[index].Channels, matches)
		events[index].MatchDiagnostics = diagnostics
	}
	cancelMatch()
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
	events = applySportsTeamChannels(events, snapshot, s.sportsTeamChannelPins(), now)
	events = selectSportsEvents(events, now)
	if enricher, ok := s.sportsProvider.(sportsEventEnricher); ok {
		enrichCtx, cancelEnrich := context.WithTimeout(ctx, budget.Enrich)
		events = enricher.EnrichEvents(enrichCtx, events, 8)
		cancelEnrich()
	}
	events = rankSportsEvents(events, now)
	events = s.proxySportsEventImages(events)
	payload := SportsPayload{
		UpdatedAtUnix: updatedUnix,
		Source:        source,
		Leagues:       sportsLeagues(events),
		Events:        events,
		FavoriteTeams: []string{},
		Incomplete:    incomplete,
	}
	if err != nil {
		payload.Error = err.Error()
	}
	return payload
}

func normalizeSportsEventFreshness(event SportsEvent, now time.Time) SportsEvent {
	if !event.Live {
		return event
	}
	stale := event.StartUnix <= 0 && event.EndUnix <= 0
	if !stale && event.EndUnix > 0 && event.EndUnix < now.Add(-2*time.Hour).Unix() {
		stale = true
	}
	if !stale && event.StartUnix > 0 && event.StartUnix < now.Add(-18*time.Hour).Unix() {
		stale = true
	}
	if !stale {
		return event
	}
	event.Live = false
	event.Completed = true
	event.Status = "final"
	event.StatusText = "Final"
	return event
}

func sportsPlayableChannelMatch(match SportsChannelMatch) bool {
	switch strings.ToLower(strings.TrimSpace(match.Confidence)) {
	case "low":
		return false
	case "high", "medium":
		return true
	default:
		return match.Score >= sportsChannelMinimumScore
	}
}

type sportsTeamChannelPin struct {
	LeagueID  string
	TeamName  string
	ChannelID string
}

func (s *HTTPRoutesServer) sportsTeamChannelPins() []sportsTeamChannelPin {
	rows, _ := s.normalizedAdminSettings()["sportsTeamChannels"].([]map[string]any)
	pins := make([]sportsTeamChannelPin, 0, len(rows))
	for _, row := range rows {
		pins = append(pins, sportsTeamChannelPin{LeagueID: asStringValue(row["leagueId"]), TeamName: normalizeMatchText(asStringValue(row["teamName"])), ChannelID: asStringValue(row["channelId"])})
	}
	return pins
}

// applySportsTeamChannels puts dedicated team feeds ahead of guide-text matches
// without depending on EPG freshness. Explicit admin assignments take priority.
func applySportsTeamChannels(events []SportsEvent, snapshot cache.Snapshot, pins []sportsTeamChannelPin, now time.Time) []SportsEvent {
	teamChannels := dedicatedSportsTeamChannels(snapshot)
	for index := range events {
		event := &events[index]
		if event.Completed || (event.StartUnix > 0 && event.StartUnix > now.Add(36*time.Hour).Unix()) {
			continue
		}
		home, away := normalizeMatchText(event.Home.Name), normalizeMatchText(event.Away.Name)
		var matches []SportsChannelMatch
		if event.Status != "replay" && sportsEventInScoreWindow(*event, now) {
			matches = append(matches, teamChannels[event.LeagueID+"|"+home]...)
			matches = append(matches, teamChannels[event.LeagueID+"|"+away]...)
		}
		if len(matches) > 0 {
			matches = append(matches, event.Channels...)
			seen := map[string]bool{}
			event.Channels = nil
			for _, match := range matches {
				if !seen[match.ID] {
					seen[match.ID] = true
					event.Channels = append(event.Channels, match)
				}
			}
		}
		for _, pin := range pins {
			if pin.LeagueID != event.LeagueID || (pin.TeamName != home && pin.TeamName != away) {
				continue
			}
			channel, ok := channelByIDFromSnapshot(snapshot, pin.ChannelID)
			if !ok {
				continue
			}
			pinned := SportsChannelMatch{ID: channel.ID, Name: channel.Name, CategoryName: channel.CategoryName, LogoURL: channel.LogoURL, Reason: "Team channel", Evidence: "admin", Confidence: "high", Score: 100}
			rest := make([]SportsChannelMatch, 0, len(event.Channels))
			for _, match := range event.Channels {
				if match.ID != channel.ID {
					rest = append(rest, match)
				}
			}
			event.Channels = append([]SportsChannelMatch{pinned}, rest...)
		}
	}
	return events
}

func dedicatedSportsTeamChannels(snapshot cache.Snapshot) map[string][]SportsChannelMatch {
	categoryNames := map[string]string{}
	for _, category := range liveCategories(snapshot) {
		categoryNames[category.ID] = category.Name
	}
	channels := map[string][]SportsChannelMatch{}
	leagues := map[string]string{}
	for _, channel := range snapshot.Catalog.Channels {
		if channel.ID == "" {
			continue
		}
		category := firstNonEmpty(channel.CategoryName, categoryNames[channel.CategoryID])
		league, known := leagues[category]
		if !known {
			league, _, _, _ = guideSportsLeague(category)
			leagues[category] = league
		}
		if league == "" || league == "sports" || containsMatchTerm(normalizeMatchText(category), "radio") || sportsAncillaryBroadcast(category) {
			continue
		}
		// Full names only: nicknames, dated matchup labels and numbered event
		// feeds must still earn their place through the normal guide matcher.
		name := strings.Fields(normalizeMatchText(channel.Name))
		for len(name) > 0 {
			last := name[len(name)-1]
			if last != "hd" && last != "fhd" && last != "uhd" && last != "4k" {
				break
			}
			name = name[:len(name)-1]
		}
		if len(name) == 0 {
			continue
		}
		key := league + "|" + strings.Join(name, " ")
		channels[key] = append(channels[key], SportsChannelMatch{ID: channel.ID, Name: channel.Name, CategoryName: category, LogoURL: channel.LogoURL, Reason: "Team channel", Evidence: "team", Confidence: "high", Score: 100})
	}
	return channels
}

const sportsUnwatchableEventLimit = 250

// selectSportsEvents keeps every game a channel carries. Games without a
// channel are kept for scores and schedules when they are close in time and in
// a league the lineup covers or that has news coverage.
func selectSportsEvents(events []SportsEvent, now time.Time) []SportsEvent {
	coveredLeagues := map[string]bool{}
	trimmed := make([]SportsEvent, len(events))
	for index, event := range events {
		channels := make([]SportsChannelMatch, 0, len(event.Channels))
		for _, match := range event.Channels {
			if sportsPlayableChannelMatch(match) {
				channels = append(channels, match)
			}
		}
		event.Channels = channels
		if len(channels) > 0 && event.LeagueID != "" {
			coveredLeagues[event.LeagueID] = true
		}
		trimmed[index] = event
	}
	selected := make([]SportsEvent, 0, len(trimmed))
	unwatchable := 0
	for _, event := range trimmed {
		if len(event.Channels) > 0 {
			selected = append(selected, event)
			continue
		}
		if unwatchable >= sportsUnwatchableEventLimit || event.LeagueID == "" || event.LeagueID == "sports" {
			continue
		}
		if _, hasNews := espnLeagueFor(event.LeagueID); !coveredLeagues[event.LeagueID] && !hasNews {
			continue
		}
		if !sportsEventInScoreWindow(event, now) {
			continue
		}
		event.MatchDiagnostics = nil
		selected = append(selected, event)
		unwatchable++
	}
	return selected
}

func sportsEventInScoreWindow(event SportsEvent, now time.Time) bool {
	if event.Live {
		return true
	}
	if event.StartUnix <= 0 {
		return false
	}
	if event.Completed {
		return event.StartUnix >= now.Add(-30*time.Hour).Unix()
	}
	return event.StartUnix >= now.Add(-6*time.Hour).Unix() && event.StartUnix <= now.Add(36*time.Hour).Unix()
}

func (s *HTTPRoutesServer) preparedSportsPayload(refresh bool) SportsPayload {
	now := time.Now()
	snapshot := s.store.Current()
	s.sportsPreparedMu.Lock()
	s.scheduleSportsRebuildLocked(now, snapshot.Health.EPGLastSuccessUnix, refresh)
	payload := cloneSportsPayload(s.sportsPrepared.Payload)
	payload.Refreshing = s.sportsPrepared.Refreshing
	if !s.sportsPrepared.Ready && payload.Source == "" {
		provider := s.sportsProvider
		if provider == nil {
			provider = noopSportsProvider{}
		}
		payload.Source = provider.Source()
	}
	s.sportsPreparedMu.Unlock()
	return payload
}

// scheduleSportsRebuildLocked starts a background rebuild when the prepared
// payload is missing, stale or behind the guide. sportsPreparedMu must be held.
func (s *HTTPRoutesServer) scheduleSportsRebuildLocked(now time.Time, guideUpdatedUnix int64, refresh bool) {
	guideChanged := guideUpdatedUnix > 0 && guideUpdatedUnix != s.sportsPrepared.GuideUpdatedUnix
	needsRefresh := !s.sportsPrepared.Ready || refresh || guideChanged || now.After(s.sportsPrepared.ExpiresAfter)
	if needsRefresh && !s.sportsPrepared.Refreshing {
		s.sportsPrepared.Refreshing = true
		go s.rebuildSportsPayload(refresh)
	}
}

// preparedSportsEvent finds one event by ID or stable ID, copying only that
// event rather than the whole payload.
func (s *HTTPRoutesServer) preparedSportsEvent(id string) (SportsEvent, bool) {
	if id == "" {
		return SportsEvent{}, false
	}
	now := time.Now()
	guideUpdatedUnix := s.store.Current().Health.EPGLastSuccessUnix
	s.sportsPreparedMu.Lock()
	if s.sportsPrepared.Ready {
		s.scheduleSportsRebuildLocked(now, guideUpdatedUnix, false)
		for _, event := range s.sportsPrepared.Payload.Events {
			if event.ID == id || event.StableID == id {
				found := cloneSportsEvents([]SportsEvent{event})[0]
				s.sportsPreparedMu.Unlock()
				return found, true
			}
		}
		s.sportsPreparedMu.Unlock()
		return SportsEvent{}, false
	}
	s.sportsPreparedMu.Unlock()
	// The background build may have published a guide fallback already.
	for _, event := range s.preparedSportsPayload(false).Events {
		if event.ID == id || event.StableID == id {
			return event, true
		}
	}
	return SportsEvent{}, false
}

func (s *HTTPRoutesServer) rebuildSportsPayload(refresh bool) {
	snapshot := s.store.Current()
	guideUpdatedUnix := snapshot.Health.EPGLastSuccessUnix
	s.sportsPreparedMu.Lock()
	budget := s.sportsPrepared.budget.withDefaults()
	s.sportsPreparedMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), budget.Provider+budget.Match+budget.Enrich+6*time.Second)
	startedAt := time.Now()
	guideEvents, refreshScores := sportsEventsFromGuideWithScoreHints(snapshot, startedAt)
	// Publish the first guide fallback before waiting for Sportarr. This work
	// happens once per rebuild and never holds up an HTTP request or the lock.
	if len(guideEvents) > 0 {
		updated := guideUpdatedUnix
		if updated <= 0 {
			updated = startedAt.Unix()
		}
		s.sportsPreparedMu.Lock()
		if !s.sportsPrepared.Ready {
			s.sportsPrepared.Payload = SportsPayload{Source: "EPG fallback", Events: cloneSportsEvents(guideEvents), Leagues: sportsLeagues(guideEvents), UpdatedAtUnix: updated, FavoriteTeams: []string{}}
		}
		s.sportsPreparedMu.Unlock()
	}
	payload := s.buildSportsPayload(ctx, refresh, budget, snapshot, startedAt, guideEvents, refreshScores)
	cancel()

	now := time.Now()
	s.sportsPreparedMu.Lock()
	previousComplete := s.sportsPrepared.Ready && !s.sportsPrepared.Incomplete && len(s.sportsPrepared.Payload.Events) > 0
	keepPrevious := false
	switch {
	case payload.Error != "" && s.sportsPrepared.Ready && len(s.sportsPrepared.Payload.Events) > 0:
		stale := cloneSportsPayload(s.sportsPrepared.Payload)
		stale.Error = payload.Error
		payload = stale
	case payload.Incomplete && previousComplete:
		// Keep serving the last complete payload rather than one with
		// unmatched channels, and retry soon.
		keepPrevious = true
	}
	if keepPrevious {
		s.sportsPrepared.Refreshing = false
		// Record the guide revision so the retry waits for the short deadline
		// instead of restarting on the next request.
		s.sportsPrepared.GuideUpdatedUnix = guideUpdatedUnix
		s.sportsPrepared.ExpiresAfter = now.Add(sportsIncompleteRetry)
		s.sportsPreparedMu.Unlock()
		return
	}
	payload.Refreshing = false
	s.sportsPrepared.Payload = cloneSportsPayload(payload)
	s.sportsPrepared.Ready = true
	s.sportsPrepared.Refreshing = false
	s.sportsPrepared.Incomplete = payload.Incomplete
	s.sportsPrepared.GuideUpdatedUnix = guideUpdatedUnix
	s.sportsPrepared.ExpiresAfter = now.Add(sportsPreparedTTL(payload))
	s.sportsPreparedMu.Unlock()
}

func sportsPreparedTTL(payload SportsPayload) time.Duration {
	if payload.Incomplete {
		return sportsIncompleteRetry
	}
	if payload.Error != "" || len(payload.Events) == 0 {
		return 30 * time.Second
	}
	return sportsCacheTTL(payload.Events)
}

func cloneSportsPayload(payload SportsPayload) SportsPayload {
	clone := payload
	clone.Events = cloneSportsEvents(payload.Events)
	clone.Leagues = append([]SportsLeague(nil), payload.Leagues...)
	clone.FavoriteTeams = append([]string(nil), payload.FavoriteTeams...)
	return clone
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
	for index := range events {
		events[index].ProviderSource = strings.TrimSpace(source)
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
