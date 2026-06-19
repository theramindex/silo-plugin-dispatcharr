package cache

import (
	"sync"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

type Snapshot struct {
	Catalog                model.CatalogState
	Health                 model.SyncHealth
	PlaybackResolvedAtUnix int64
}

type Store struct {
	mu          sync.RWMutex
	snapshot    Snapshot
	preferences Preferences
}

func NewStore() *Store {
	return &Store{preferences: defaultPreferences()}
}

func (s *Store) Current() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func (s *Store) Replace(snapshot Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot.Health.LastFailureUnix = 0
	snapshot.Health.LastError = ""
	s.snapshot = snapshot
}

func (s *Store) Preferences() Preferences {
	s.mu.RLock()
	defer s.mu.RUnlock()

	preferences := s.preferences
	preferences.Favorites = cloneBoolMap(s.preferences.Favorites)
	preferences.AutoFavorites = cloneBoolMap(s.preferences.AutoFavorites)
	preferences.HiddenCategories = cloneBoolMap(s.preferences.HiddenCategories)
	preferences.RecentChannels = append([]string(nil), s.preferences.RecentChannels...)
	preferences.ContinueWatching = cloneAnyMap(s.preferences.ContinueWatching)
	return preferences
}

func (s *Store) SetFavorite(id string, enabled bool) Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensurePreferences()
	if enabled {
		s.preferences.Favorites[id] = true
	} else {
		delete(s.preferences.Favorites, id)
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
		settings.OutputFormat = "hls"
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

func (s *Store) RecordFailure(atUnix int64, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Health.LastFailureUnix = atUnix
	s.snapshot.Health.LastError = message
	if s.snapshot.Catalog.Health.LastSuccessUnix != 0 && s.snapshot.Health.LastSuccessUnix == 0 {
		s.snapshot.Health.LastSuccessUnix = s.snapshot.Catalog.Health.LastSuccessUnix
	}
}

func (s *Store) ensurePreferences() {
	if s.preferences.Favorites == nil {
		s.preferences.Favorites = map[string]bool{}
	}
	if s.preferences.AutoFavorites == nil {
		s.preferences.AutoFavorites = map[string]bool{}
	}
	if s.preferences.HiddenCategories == nil {
		s.preferences.HiddenCategories = map[string]bool{}
	}
	if s.preferences.RecentChannels == nil {
		s.preferences.RecentChannels = []string{}
	}
	if s.preferences.ContinueWatching == nil {
		s.preferences.ContinueWatching = map[string]any{}
	}
	if s.preferences.Playback.StreamMode == "" {
		s.preferences.Playback.StreamMode = "redirect"
	}
	if s.preferences.Playback.OutputFormat == "" {
		s.preferences.Playback.OutputFormat = "hls"
	}
}

func (s *Store) preferencesSnapshotLocked() Preferences {
	preferences := s.preferences
	preferences.Favorites = cloneBoolMap(s.preferences.Favorites)
	preferences.AutoFavorites = cloneBoolMap(s.preferences.AutoFavorites)
	preferences.HiddenCategories = cloneBoolMap(s.preferences.HiddenCategories)
	preferences.RecentChannels = append([]string(nil), s.preferences.RecentChannels...)
	preferences.ContinueWatching = cloneAnyMap(s.preferences.ContinueWatching)
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
