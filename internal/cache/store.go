package cache

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

type Snapshot struct {
	Catalog                model.CatalogState
	Health                 model.SyncHealth
	PlaybackResolvedAtUnix int64
	ConfigKey              string
}

type Store struct {
	mu            sync.RWMutex
	snapshot      Snapshot
	preferences   Preferences
	adminSettings json.RawMessage
	sessions      map[string]WatchSession
}

type WatchSession struct {
	ID                string `json:"id"`
	ItemKind          string `json:"itemKind"`
	ItemID            string `json:"itemId"`
	ItemName          string `json:"itemName,omitempty"`
	StartedAtUnix     int64  `json:"startedAtUnix"`
	LastHeartbeatUnix int64  `json:"lastHeartbeatUnix"`
	EndedAtUnix       int64  `json:"endedAtUnix,omitempty"`
	EndReason         string `json:"endReason,omitempty"`
}

func NewStore() *Store {
	return &Store{preferences: defaultPreferences(), sessions: map[string]WatchSession{}}
}

func (s *Store) Current() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func (s *Store) Replace(snapshot Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if shouldPreserveGuide(s.snapshot, snapshot) {
		snapshot.Catalog.Programs = append([]model.Program(nil), s.snapshot.Catalog.Programs...)
		snapshot.Catalog.Health.EPGStatus = s.snapshot.Health.EPGStatus
		snapshot.Catalog.Health.EPGProgramCount = s.snapshot.Health.EPGProgramCount
		snapshot.Catalog.Health.EPGLastSuccessUnix = s.snapshot.Health.EPGLastSuccessUnix
		snapshot.Catalog.Health.EPGLastFailureUnix = s.snapshot.Health.EPGLastFailureUnix
		snapshot.Catalog.Health.EPGLastError = s.snapshot.Health.EPGLastError
	}
	snapshot.Health.LastFailureUnix = 0
	snapshot.Health.LastError = ""
	if snapshot.Health.EPGStatus == "" && snapshot.Health.LastSuccessUnix != 0 && len(snapshot.Catalog.Programs) > 0 {
		snapshot.Catalog.Health.EPGStatus = "ok"
		snapshot.Catalog.Health.EPGProgramCount = len(snapshot.Catalog.Programs)
		snapshot.Catalog.Health.EPGLastSuccessUnix = snapshot.Health.LastSuccessUnix
		snapshot.Health.EPGStatus = "ok"
		snapshot.Health.EPGProgramCount = len(snapshot.Catalog.Programs)
		snapshot.Health.EPGLastSuccessUnix = snapshot.Health.LastSuccessUnix
	}
	if snapshot.Health.EPGStatus == "" {
		snapshot.Health.EPGStatus = s.snapshot.Health.EPGStatus
		snapshot.Health.EPGProgramCount = s.snapshot.Health.EPGProgramCount
		snapshot.Health.EPGLastSuccessUnix = s.snapshot.Health.EPGLastSuccessUnix
		snapshot.Health.EPGLastFailureUnix = s.snapshot.Health.EPGLastFailureUnix
		snapshot.Health.EPGLastError = s.snapshot.Health.EPGLastError
	}
	s.snapshot = snapshot
}

func shouldPreserveGuide(current, next Snapshot) bool {
	if !sameCatalogSource(current.Catalog.Source, next.Catalog.Source) {
		return false
	}
	if current.Health.EPGStatus != "ok" || len(current.Catalog.Programs) == 0 {
		return false
	}
	if len(next.Catalog.Programs) >= len(current.Catalog.Programs) {
		return false
	}
	return haveProgramChannels(next.Catalog.Channels, current.Catalog.Programs)
}

func sameCatalogSource(current, next model.Source) bool {
	if current.ID != next.ID || current.Name != next.Name || current.Mode != next.Mode {
		return false
	}
	if current.ChannelProfile == nil || next.ChannelProfile == nil {
		return current.ChannelProfile == nil && next.ChannelProfile == nil
	}
	return current.ChannelProfile.ID == next.ChannelProfile.ID && current.ChannelProfile.Name == next.ChannelProfile.Name
}

func haveProgramChannels(channels []model.Channel, programs []model.Program) bool {
	channelIDs := make(map[string]bool, len(channels))
	for _, channel := range channels {
		channelIDs[channel.ID] = true
	}
	for _, program := range programs {
		if program.ChannelID != "" && !channelIDs[program.ChannelID] {
			return false
		}
	}
	return true
}

func (s *Store) Preferences() Preferences {
	s.mu.RLock()
	defer s.mu.RUnlock()

	preferences := s.preferences
	preferences.Favorites = cloneBoolMap(s.preferences.Favorites)
	preferences.FavoriteOrder = append([]string(nil), s.preferences.FavoriteOrder...)
	preferences.AutoFavorites = cloneBoolMap(s.preferences.AutoFavorites)
	preferences.HiddenCategories = cloneBoolMap(s.preferences.HiddenCategories)
	preferences.SportsFavoriteTeams = cloneBoolMap(s.preferences.SportsFavoriteTeams)
	preferences.RecentChannels = append([]string(nil), s.preferences.RecentChannels...)
	preferences.ContinueWatching = cloneAnyMap(s.preferences.ContinueWatching)
	preferences.CustomGroups = cloneCustomGroups(s.preferences.CustomGroups)
	preferences.CustomGroupMemberships = cloneStringSliceMap(s.preferences.CustomGroupMemberships)
	return preferences
}

func (s *Store) AdminSettings() json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.adminSettings) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), s.adminSettings...)
}

func (s *Store) HasAdminSettings() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.adminSettings) > 0
}

func (s *Store) SetAdminSettings(settings json.RawMessage) json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.adminSettings = append(json.RawMessage(nil), settings...)
	return append(json.RawMessage(nil), s.adminSettings...)
}

func (s *Store) StartWatch(kind, id, name string) (WatchSession, Preferences) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensurePreferences()
	s.ensureSessions()
	now := time.Now().Unix()
	session := WatchSession{
		ID:                newSessionID(),
		ItemKind:          kind,
		ItemID:            id,
		ItemName:          name,
		StartedAtUnix:     now,
		LastHeartbeatUnix: now,
	}
	s.sessions[session.ID] = session

	if kind == "channel" && id != "" {
		s.preferences.RecentChannels = prependUnique(s.preferences.RecentChannels, id, 24)
		plays := 1
		if previous, ok := s.preferences.ContinueWatching[id].(map[string]any); ok {
			if value, ok := previous["plays"].(float64); ok {
				plays = int(value) + 1
			}
		}
		s.preferences.ContinueWatching[id] = map[string]any{
			"kind":     kind,
			"name":     name,
			"playedAt": now,
			"plays":    plays,
		}
		if plays >= 3 && !s.preferences.Favorites[id] {
			s.preferences.AutoFavorites[id] = true
		}
	}

	return session, s.preferencesSnapshotLocked()
}

func (s *Store) HeartbeatWatch(id string) (WatchSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureSessions()
	session, ok := s.sessions[id]
	if !ok || session.EndedAtUnix != 0 {
		return WatchSession{}, false
	}
	session.LastHeartbeatUnix = time.Now().Unix()
	s.sessions[id] = session
	return session, true
}

func (s *Store) StopWatch(id, reason string) (WatchSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureSessions()
	session, ok := s.sessions[id]
	if !ok {
		return WatchSession{}, false
	}
	if session.EndedAtUnix == 0 {
		session.EndedAtUnix = time.Now().Unix()
		if reason == "" {
			reason = "stopped"
		}
		session.EndReason = reason
		s.sessions[id] = session
	}
	return session, true
}

func (s *Store) ActiveSessions() []WatchSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]WatchSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session.EndedAtUnix == 0 {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func (s *Store) SetFavorite(id string, enabled bool) Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensurePreferences()
	if enabled {
		s.preferences.Favorites[id] = true
		s.preferences.FavoriteOrder = appendUnique(s.preferences.FavoriteOrder, id, 256)
	} else {
		delete(s.preferences.Favorites, id)
		s.preferences.FavoriteOrder = removeString(s.preferences.FavoriteOrder, id)
	}
	return s.preferencesSnapshotLocked()
}

func (s *Store) SetHiddenCategory(id string, hidden bool) Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensurePreferences()
	if hidden {
		s.preferences.HiddenCategories[id] = true
	} else {
		delete(s.preferences.HiddenCategories, id)
	}
	return s.preferencesSnapshotLocked()
}

func (s *Store) SetPlaybackSettings(settings PlaybackSettings) Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensurePreferences()
	settings.BackendProxySupported = false
	if settings.StreamMode == "" || settings.StreamMode == "proxy" {
		settings.StreamMode = "redirect"
	}
	if settings.OutputFormat == "" {
		settings.OutputFormat = "ts"
	}
	s.preferences.Playback = settings
	return s.preferencesSnapshotLocked()
}

func (s *Store) SetPreferences(preferences Preferences) Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.preferences = preferences
	s.ensurePreferences()
	return s.preferencesSnapshotLocked()
}

func (s *Store) SetSportsFavoriteTeam(teamID string, enabled bool) Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensurePreferences()
	if enabled {
		s.preferences.SportsFavoriteTeams[teamID] = true
	} else {
		delete(s.preferences.SportsFavoriteTeams, teamID)
	}
	return s.preferencesSnapshotLocked()
}

func (s *Store) RecordFailure(atUnix int64, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Health.LastFailureUnix = atUnix
	s.snapshot.Health.LastError = message
	if s.snapshot.Catalog.Health.LastSuccessUnix != 0 && s.snapshot.Health.LastSuccessUnix == 0 {
		s.snapshot.Health.LastSuccessUnix = s.snapshot.Catalog.Health.LastSuccessUnix
	}
}

func (s *Store) MarkEPGLoading() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Health.EPGStatus = "loading"
	s.snapshot.Health.EPGLastError = ""
}

func (s *Store) ClearGuidePrograms(atUnix int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Catalog.Programs = nil
	s.snapshot.Catalog.Health.EPGStatus = "loading"
	s.snapshot.Catalog.Health.EPGProgramCount = 0
	s.snapshot.Catalog.Health.EPGLastSuccessUnix = 0
	s.snapshot.Catalog.Health.EPGLastFailureUnix = 0
	s.snapshot.Catalog.Health.EPGLastError = ""
	s.snapshot.Health.EPGStatus = "loading"
	s.snapshot.Health.EPGProgramCount = 0
	s.snapshot.Health.EPGLastSuccessUnix = 0
	s.snapshot.Health.EPGLastFailureUnix = 0
	s.snapshot.Health.EPGLastError = ""
	_ = atUnix
}

func (s *Store) ReplacePrograms(programs []model.Program, atUnix int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Catalog.Programs = append([]model.Program(nil), programs...)
	s.snapshot.Catalog.Health.EPGStatus = "ok"
	s.snapshot.Catalog.Health.EPGProgramCount = len(programs)
	s.snapshot.Catalog.Health.EPGLastSuccessUnix = atUnix
	s.snapshot.Health.EPGStatus = "ok"
	s.snapshot.Health.EPGProgramCount = len(programs)
	s.snapshot.Health.EPGLastSuccessUnix = atUnix
	s.snapshot.Health.EPGLastFailureUnix = 0
	s.snapshot.Health.EPGLastError = ""
}

func (s *Store) RecordEPGFailure(atUnix int64, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Health.EPGStatus = "failed"
	s.snapshot.Health.EPGLastFailureUnix = atUnix
	s.snapshot.Health.EPGLastError = message
	s.snapshot.Catalog.Health.EPGStatus = "failed"
	s.snapshot.Catalog.Health.EPGLastFailureUnix = atUnix
	s.snapshot.Catalog.Health.EPGLastError = message
}

func (s *Store) ensurePreferences() {
	if s.preferences.Favorites == nil {
		s.preferences.Favorites = map[string]bool{}
	}
	if s.preferences.FavoriteOrder == nil {
		s.preferences.FavoriteOrder = []string{}
	}
	if s.preferences.AutoFavorites == nil {
		s.preferences.AutoFavorites = map[string]bool{}
	}
	if s.preferences.HiddenCategories == nil {
		s.preferences.HiddenCategories = map[string]bool{}
	}
	if s.preferences.SportsFavoriteTeams == nil {
		s.preferences.SportsFavoriteTeams = map[string]bool{}
	}
	if s.preferences.KeywordPasses == nil {
		s.preferences.KeywordPasses = []KeywordPass{}
	}
	if s.preferences.RecentChannels == nil {
		s.preferences.RecentChannels = []string{}
	}
	if s.preferences.ContinueWatching == nil {
		s.preferences.ContinueWatching = map[string]any{}
	}
	if s.preferences.CategoryParsing.Mode == "" {
		s.preferences.CategoryParsing.Mode = "off"
	}
	if s.preferences.CategoryParsing.Delimiter == "" {
		s.preferences.CategoryParsing.Delimiter = "dash"
	}
	if s.preferences.CustomGroups == nil {
		s.preferences.CustomGroups = []CustomGroup{}
	}
	if s.preferences.CustomGroupMemberships == nil {
		s.preferences.CustomGroupMemberships = map[string][]string{}
	}
	if s.preferences.Playback.StreamMode == "" {
		s.preferences.Playback.StreamMode = "redirect"
	}
	if s.preferences.Playback.OutputFormat == "" {
		s.preferences.Playback.OutputFormat = "ts"
	}
}

func (s *Store) ensureSessions() {
	if s.sessions == nil {
		s.sessions = map[string]WatchSession{}
	}
}

func (s *Store) preferencesSnapshotLocked() Preferences {
	preferences := s.preferences
	preferences.Favorites = cloneBoolMap(s.preferences.Favorites)
	preferences.FavoriteOrder = append([]string(nil), s.preferences.FavoriteOrder...)
	preferences.AutoFavorites = cloneBoolMap(s.preferences.AutoFavorites)
	preferences.HiddenCategories = cloneBoolMap(s.preferences.HiddenCategories)
	preferences.SportsFavoriteTeams = cloneBoolMap(s.preferences.SportsFavoriteTeams)
	preferences.KeywordPasses = append([]KeywordPass(nil), s.preferences.KeywordPasses...)
	preferences.RecentChannels = append([]string(nil), s.preferences.RecentChannels...)
	preferences.ContinueWatching = cloneAnyMap(s.preferences.ContinueWatching)
	preferences.CustomGroups = cloneCustomGroups(s.preferences.CustomGroups)
	preferences.CustomGroupMemberships = cloneStringSliceMap(s.preferences.CustomGroupMemberships)
	return preferences
}

func cloneBoolMap(values map[string]bool) map[string]bool {
	clone := make(map[string]bool, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func cloneAnyMap(values map[string]any) map[string]any {
	clone := make(map[string]any, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func cloneCustomGroups(values []CustomGroup) []CustomGroup {
	return append([]CustomGroup(nil), values...)
}

func cloneStringSliceMap(values map[string][]string) map[string][]string {
	clone := make(map[string][]string, len(values))
	for key, value := range values {
		clone[key] = append([]string(nil), value...)
	}
	return clone
}

func appendUnique(values []string, value string, limit int) []string {
	result := append([]string(nil), values...)
	if value == "" {
		return result
	}
	for _, existing := range result {
		if existing == value {
			return result
		}
	}
	result = append(result, value)
	if limit > 0 && len(result) > limit {
		return result[len(result)-limit:]
	}
	return result
}

func removeString(values []string, value string) []string {
	result := make([]string, 0, len(values))
	for _, existing := range values {
		if existing != value {
			result = append(result, existing)
		}
	}
	return result
}

func prependUnique(values []string, value string, limit int) []string {
	result := make([]string, 0, len(values)+1)
	if value != "" {
		result = append(result, value)
	}
	for _, existing := range values {
		if existing == "" || existing == value {
			continue
		}
		result = append(result, existing)
		if limit > 0 && len(result) >= limit {
			return result
		}
	}
	return result
}

func newSessionID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}
