package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const DefaultConnectionFile = "/var/lib/continuum/plugins/silo.ramindex.dispatcharr/connection.json"

type ConnectionSettings struct {
	SourceMode        SourceMode `json:"sourceMode"`
	DispatcharrURL    string     `json:"dispatcharrUrl,omitempty"`
	DispatcharrUser   string     `json:"dispatcharrUser,omitempty"`
	DispatcharrPass   string     `json:"dispatcharrPass,omitempty"`
	DispatcharrAPIKey string     `json:"dispatcharrApiKey,omitempty"`
	ChannelProfile    string     `json:"channelProfile,omitempty"`
	ChannelProfiles   []string   `json:"channelProfiles,omitempty"`
	ChannelGroups     []string   `json:"channelGroups,omitempty"`
	XtreamBaseURL     string     `json:"xtreamBaseUrl,omitempty"`
	XtreamUsername    string     `json:"xtreamUsername,omitempty"`
	XtreamPassword    string     `json:"xtreamPassword,omitempty"`
	M3UURL            string     `json:"m3uUrl,omitempty"`
	EPGXMLURL         string     `json:"epgXmlUrl,omitempty"`
}

func ConnectionFromSettings(settings Settings) ConnectionSettings {
	return ConnectionSettings{
		SourceMode:        settings.EffectiveSourceMode(),
		DispatcharrURL:    settings.DispatcharrURL,
		DispatcharrUser:   settings.DispatcharrUser,
		DispatcharrPass:   settings.DispatcharrPass,
		DispatcharrAPIKey: settings.DispatcharrAPIKey,
		ChannelProfile:    settings.ChannelProfile,
		ChannelProfiles:   append([]string(nil), settings.ChannelProfiles...),
		ChannelGroups:     append([]string(nil), settings.ChannelGroups...),
		XtreamBaseURL:     settings.XtreamBaseURL,
		XtreamUsername:    settings.XtreamUsername,
		XtreamPassword:    settings.XtreamPassword,
		M3UURL:            settings.M3UURL,
		EPGXMLURL:         settings.EPGXMLURL,
	}
}

func (connection ConnectionSettings) Apply(settings *Settings) {
	settings.SourceMode = connection.SourceMode
	settings.DispatcharrURL = connection.DispatcharrURL
	settings.DispatcharrUser = connection.DispatcharrUser
	settings.DispatcharrPass = connection.DispatcharrPass
	settings.DispatcharrAPIKey = connection.DispatcharrAPIKey
	settings.ChannelProfile = connection.ChannelProfile
	settings.ChannelProfiles = append([]string(nil), connection.ChannelProfiles...)
	settings.ChannelGroups = append([]string(nil), connection.ChannelGroups...)
	settings.XtreamBaseURL = connection.XtreamBaseURL
	settings.XtreamUsername = connection.XtreamUsername
	settings.XtreamPassword = connection.XtreamPassword
	settings.M3UURL = connection.M3UURL
	settings.EPGXMLURL = connection.EPGXMLURL
}

func NormalizeConnection(connection ConnectionSettings) (ConnectionSettings, error) {
	connection.DispatcharrURL = strings.TrimRight(strings.TrimSpace(connection.DispatcharrURL), "/")
	connection.DispatcharrUser = strings.TrimSpace(connection.DispatcharrUser)
	connection.ChannelProfile = strings.TrimSpace(connection.ChannelProfile)
	connection.ChannelProfiles = normalizeStringSelection(connection.ChannelProfiles)
	connection.ChannelGroups = normalizeStringSelection(connection.ChannelGroups)
	if len(connection.ChannelProfiles) == 0 && connection.ChannelProfile != "" {
		connection.ChannelProfiles = []string{connection.ChannelProfile}
	}
	if len(connection.ChannelProfiles) == 1 {
		connection.ChannelProfile = connection.ChannelProfiles[0]
	} else if len(connection.ChannelProfiles) > 1 {
		connection.ChannelProfile = ""
	}
	connection.XtreamBaseURL = strings.TrimRight(strings.TrimSpace(connection.XtreamBaseURL), "/")
	connection.XtreamUsername = strings.TrimSpace(connection.XtreamUsername)
	connection.M3UURL = strings.TrimSpace(connection.M3UURL)
	connection.EPGXMLURL = strings.TrimSpace(connection.EPGXMLURL)

	settings := Settings{ChannelRefreshH: DefaultChannelRefreshHours, EPGRefreshH: DefaultEPGRefreshHours}
	connection.Apply(&settings)
	if err := settings.Validate(); err != nil {
		return ConnectionSettings{}, err
	}
	return connection, nil
}

type ConnectionStore struct {
	mu   sync.RWMutex
	path string
}

func NewConnectionStore(path string) *ConnectionStore {
	if strings.TrimSpace(path) == "" {
		path = DefaultConnectionFile
	}
	return &ConnectionStore{path: path}
}

func (store *ConnectionStore) Load() (ConnectionSettings, bool, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return ConnectionSettings{}, false, nil
	}
	if err != nil {
		return ConnectionSettings{}, false, fmt.Errorf("read connection settings: %w", err)
	}
	var connection ConnectionSettings
	if err := json.Unmarshal(data, &connection); err != nil {
		return ConnectionSettings{}, false, fmt.Errorf("decode connection settings: %w", err)
	}
	normalized, err := NormalizeConnection(connection)
	if err != nil {
		return ConnectionSettings{}, false, fmt.Errorf("validate connection settings: %w", err)
	}
	return normalized, true, nil
}

func (store *ConnectionStore) Save(connection ConnectionSettings) error {
	normalized, err := NormalizeConnection(connection)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode connection settings: %w", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("create connection settings directory: %w", err)
	}
	temporary := store.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write connection settings: %w", err)
	}
	if err := os.Rename(temporary, store.path); err != nil {
		return fmt.Errorf("replace connection settings: %w", err)
	}
	return nil
}
