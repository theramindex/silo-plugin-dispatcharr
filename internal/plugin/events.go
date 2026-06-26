package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

type EventsPayload struct {
	UpdatedAtUnix int64            `json:"updatedAtUnix"`
	Source        string           `json:"source"`
	Categories    []EventCategory  `json:"categories"`
	Events        []BroadcastEvent `json:"events"`
}

type EventCategory struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	LiveCount     int    `json:"liveCount"`
	UpcomingCount int    `json:"upcomingCount"`
}

type BroadcastEvent struct {
	ID           string               `json:"id"`
	CategoryID   string               `json:"categoryId"`
	CategoryName string               `json:"categoryName"`
	Name         string               `json:"name"`
	ShortName    string               `json:"shortName,omitempty"`
	Description  string               `json:"description,omitempty"`
	Keyword      string               `json:"keyword,omitempty"`
	StartUnix    int64                `json:"startUnix"`
	EndUnix      int64                `json:"endUnix,omitempty"`
	Live         bool                 `json:"live"`
	Completed    bool                 `json:"completed"`
	Channels     []SportsChannelMatch `json:"channels"`
}

type EventKeywordRule struct {
	CategoryID   string   `json:"categoryId"`
	CategoryName string   `json:"categoryName"`
	Keywords     []string `json:"keywords"`
}

func (s *HTTPRoutesServer) handleEvents(ctx context.Context, request *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	if request.GetMethod() != "" && request.GetMethod() != http.MethodGet {
		return textResponse(http.StatusMethodNotAllowed, "method not allowed"), nil
	}
	return s.respondJSON(http.StatusOK, s.eventsPayload(time.Now()))
}

func (s *HTTPRoutesServer) eventsPayload(now time.Time) EventsPayload {
	snapshot := s.store.Current()
	rules := s.eventKeywordRules()
	events := detectGuideBroadcastEvents(snapshot, now, rules)
	sort.Slice(events, func(i, j int) bool {
		if events[i].Live != events[j].Live {
			return events[i].Live
		}
		leftStart := broadcastEventSortStartUnix(events[i])
		rightStart := broadcastEventSortStartUnix(events[j])
		if leftStart != rightStart {
			return leftStart < rightStart
		}
		return events[i].Name < events[j].Name
	})
	return EventsPayload{
		UpdatedAtUnix: now.Unix(),
		Source:        "epg",
		Categories:    eventCategories(events),
		Events:        events,
	}
}

func (s *HTTPRoutesServer) eventKeywordRules() []EventKeywordRule {
	if s.store != nil && s.store.HasAdminSettings() {
		return eventKeywordRulesFromAdminSettings(s.store.AdminSettings())
	}
	if s.settingsProvider != nil {
		settings := s.settingsProvider()
		if len(settings.AdminSettings) > 0 {
			return eventKeywordRulesFromAdminSettings(settings.AdminSettings)
		}
	}
	return defaultEventKeywordRules()
}

func eventKeywordRulesFromAdminSettings(raw json.RawMessage) []EventKeywordRule {
	var payload map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return defaultEventKeywordRules()
	}
	rules := normalizeEventKeywordRules(payload["eventKeywords"])
	if len(rules) == 0 {
		return defaultEventKeywordRules()
	}
	return rules
}

func defaultEventKeywordRules() []EventKeywordRule {
	return []EventKeywordRule{
		{CategoryID: "awards", CategoryName: "Awards", Keywords: []string{"Academy Awards", "The Oscars", "Oscars", "Tony Awards", "The Tonys", "Golden Globes", "Grammy Awards", "Grammys", "Emmy Awards", "Emmys", "CMA Awards", "ACM Awards", "Billboard Music Awards", "American Music Awards", "BET Awards", "MTV Video Music Awards", "Critics Choice Awards", "SAG Awards"}},
		{CategoryID: "civic", CategoryName: "Civic", Keywords: []string{"State of the Union", "Presidential Address", "Joint Session", "Inauguration", "Election Night", "Presidential Debate"}},
		{CategoryID: "parades", CategoryName: "Parades", Keywords: []string{"Thanksgiving Day Parade", "Macy's Thanksgiving Day Parade", "Rose Parade", "Christmas Parade"}},
		{CategoryID: "entertainment", CategoryName: "Entertainment", Keywords: []string{"Live Special", "Special Presentation", "Red Carpet", "Ceremony", "Tribute Concert", "Benefit Concert", "Festival"}},
	}
}

func normalizeEventKeywordRules(value any) []EventKeywordRule {
	rows, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]EventKeywordRule); ok {
			return normalizeTypedEventKeywordRules(typed)
		}
		return []EventKeywordRule{}
	}
	rules := make([]EventKeywordRule, 0, len(rows))
	for _, row := range rows {
		object, ok := row.(map[string]any)
		if !ok {
			continue
		}
		categoryID, _ := object["categoryId"].(string)
		categoryName, _ := object["categoryName"].(string)
		keywords := normalizeKeywordValues(object["keywords"])
		rule := EventKeywordRule{
			CategoryID:   normalizeEventCategoryID(categoryID, categoryName),
			CategoryName: strings.TrimSpace(categoryName),
			Keywords:     keywords,
		}
		if rule.CategoryName == "" {
			rule.CategoryName = eventCategoryName(rule.CategoryID)
		}
		if rule.CategoryID != "" && len(rule.Keywords) > 0 {
			rules = append(rules, rule)
		}
	}
	return rules
}

func normalizeTypedEventKeywordRules(values []EventKeywordRule) []EventKeywordRule {
	normalized := make([]EventKeywordRule, 0, len(values))
	for _, rule := range values {
		rule.CategoryID = normalizeEventCategoryID(rule.CategoryID, rule.CategoryName)
		rule.CategoryName = strings.TrimSpace(rule.CategoryName)
		if rule.CategoryName == "" {
			rule.CategoryName = eventCategoryName(rule.CategoryID)
		}
		rule.Keywords = normalizeKeywordStrings(rule.Keywords)
		if rule.CategoryID != "" && len(rule.Keywords) > 0 {
			normalized = append(normalized, rule)
		}
	}
	return normalized
}

func normalizeKeywordValues(value any) []string {
	if values, ok := value.([]string); ok {
		return normalizeKeywordStrings(values)
	}
	rows, ok := value.([]any)
	if !ok {
		if text, ok := value.(string); ok {
			return normalizeKeywordText(text)
		}
		return []string{}
	}
	keywords := make([]string, 0, len(rows))
	for _, row := range rows {
		if text, ok := row.(string); ok {
			keywords = append(keywords, text)
		}
	}
	return normalizeKeywordStrings(keywords)
}

func normalizeKeywordText(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ','
	})
	return normalizeKeywordStrings(parts)
}

func normalizeKeywordStrings(values []string) []string {
	seen := map[string]bool{}
	keywords := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeMatchText(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keywords = append(keywords, value)
	}
	return keywords
}

func normalizeEventCategoryID(categoryID, categoryName string) string {
	value := normalizeMatchText(firstNonEmpty(categoryID, categoryName))
	switch value {
	case "awards", "award":
		return "awards"
	case "civic", "politics", "political":
		return "civic"
	case "parades", "parade":
		return "parades"
	case "entertainment", "specials", "special":
		return "entertainment"
	default:
		return strings.ReplaceAll(value, " ", "-")
	}
}

func eventCategoryName(categoryID string) string {
	switch categoryID {
	case "awards":
		return "Awards"
	case "civic":
		return "Civic"
	case "parades":
		return "Parades"
	case "entertainment":
		return "Entertainment"
	default:
		return strings.TrimSpace(categoryID)
	}
}

func detectGuideBroadcastEvents(snapshot cache.Snapshot, now time.Time, rules []EventKeywordRule) []BroadcastEvent {
	nowUnix := now.Unix()
	maxUnix := now.AddDate(0, 0, 60).Unix()
	categoryNames := map[string]string{}
	for _, category := range liveCategories(snapshot) {
		categoryNames[category.ID] = category.Name
	}
	byKey := map[string]*BroadcastEvent{}
	for _, program := range snapshot.Catalog.Programs {
		if program.EndUnix > 0 && program.EndUnix < nowUnix-6*3600 {
			continue
		}
		if program.StartUnix > maxUnix {
			continue
		}
		rule, keyword, ok := matchEventKeyword(program, rules)
		if !ok {
			continue
		}
		channel, ok := channelByIDFromSnapshot(snapshot, program.ChannelID)
		if !ok {
			continue
		}
		key := broadcastProgramMergeKey(program, rule.CategoryID)
		event := byKey[key]
		if event == nil {
			event = &BroadcastEvent{
				ID:           "guide:" + firstNonEmpty(program.ID, shortHash(program.Title+"|"+fmt.Sprintf("%d", program.StartUnix))),
				CategoryID:   rule.CategoryID,
				CategoryName: rule.CategoryName,
				Name:         program.Title,
				ShortName:    program.Title,
				Description:  program.Summary,
				Keyword:      keyword,
				StartUnix:    program.StartUnix,
				EndUnix:      program.EndUnix,
			}
			event.Live = broadcastEventIsLive(*event, now)
			event.Completed = event.EndUnix > 0 && event.EndUnix < nowUnix
			byKey[key] = event
		}
		if event.Description == "" {
			event.Description = program.Summary
		}
		if program.EndUnix > event.EndUnix {
			event.EndUnix = program.EndUnix
		}
		event.Channels = appendEventChannelMatch(event.Channels, SportsChannelMatch{
			ID:           channel.ID,
			Name:         channel.Name,
			CategoryName: firstNonEmpty(categoryNames[channel.CategoryID], channel.CategoryName),
			LogoURL:      channel.LogoURL,
			Reason:       "guide: " + keyword,
			Score:        100,
		})
	}
	events := make([]BroadcastEvent, 0, len(byKey))
	for _, event := range byKey {
		sort.Slice(event.Channels, func(i, j int) bool {
			if event.Channels[i].Score != event.Channels[j].Score {
				return event.Channels[i].Score > event.Channels[j].Score
			}
			return event.Channels[i].Name < event.Channels[j].Name
		})
		events = append(events, *event)
	}
	return events
}

func matchEventKeyword(program model.Program, rules []EventKeywordRule) (EventKeywordRule, string, bool) {
	text := normalizeMatchText(strings.Join([]string{program.Title, program.Summary}, " "))
	for _, rule := range rules {
		for _, keyword := range rule.Keywords {
			normalizedKeyword := normalizeMatchText(keyword)
			if normalizedKeyword != "" && strings.Contains(" "+text+" ", " "+normalizedKeyword+" ") {
				return rule, keyword, true
			}
		}
	}
	return EventKeywordRule{}, "", false
}

func appendEventChannelMatch(matches []SportsChannelMatch, match SportsChannelMatch) []SportsChannelMatch {
	for index, existing := range matches {
		if existing.ID == match.ID {
			if match.Score > existing.Score {
				matches[index] = match
			}
			return matches
		}
	}
	return append(matches, match)
}

func channelByIDFromSnapshot(snapshot cache.Snapshot, channelID string) (model.Channel, bool) {
	for _, channel := range snapshot.Catalog.Channels {
		if channel.ID == channelID {
			return channel, true
		}
	}
	return model.Channel{}, false
}

func broadcastProgramMergeKey(program model.Program, categoryID string) string {
	day := ""
	if program.StartUnix > 0 {
		day = time.Unix(program.StartUnix, 0).UTC().Format("2006-01-02")
	}
	return categoryID + "|" + normalizeMatchText(program.Title) + "|" + day
}

func broadcastEventSortStartUnix(event BroadcastEvent) int64 {
	if event.StartUnix > 0 {
		return event.StartUnix
	}
	return 1<<62 - 1
}

func shortHash(value string) string {
	sum := sportsHash(value)
	if len(sum) > 16 {
		return sum[:16]
	}
	return sum
}

func eventCategories(events []BroadcastEvent) []EventCategory {
	byID := map[string]*EventCategory{}
	for _, event := range events {
		id := firstNonEmpty(event.CategoryID, "events")
		category := byID[id]
		if category == nil {
			category = &EventCategory{ID: id, Name: firstNonEmpty(event.CategoryName, eventCategoryName(id))}
			byID[id] = category
		}
		if event.Live {
			category.LiveCount++
		} else if !event.Completed {
			category.UpcomingCount++
		}
	}
	categories := make([]EventCategory, 0, len(byID))
	for _, category := range byID {
		categories = append(categories, *category)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})
	return categories
}

func broadcastEventIsLive(event BroadcastEvent, now time.Time) bool {
	nowUnix := now.Unix()
	if event.StartUnix == 0 {
		return false
	}
	end := event.EndUnix
	if end == 0 {
		end = event.StartUnix + 3*3600
	}
	return event.StartUnix <= nowUnix && end >= nowUnix
}
