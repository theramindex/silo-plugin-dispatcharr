package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const DefaultAdminSettingsFile = "/var/lib/continuum/plugins/silo.ramindex.dispatcharr/category-settings.json"
const defaultAdminECMURL = ""

type adminSettingsStorage interface {
	Load() (json.RawMessage, bool, error)
	Save(json.RawMessage) error
	Path() string
}

type FileAdminSettingsStorage struct {
	path string
	mu   sync.Mutex
}

func NewFileAdminSettingsStorage(path string) *FileAdminSettingsStorage {
	if strings.TrimSpace(path) == "" {
		path = os.Getenv("DISPATCHARR_ADMIN_SETTINGS_FILE")
	}
	if strings.TrimSpace(path) == "" {
		path = DefaultAdminSettingsFile
	}
	return &FileAdminSettingsStorage{path: path}
}

func (s *FileAdminSettingsStorage) Path() string {
	return s.path
}

func (s *FileAdminSettingsStorage) Load() (json.RawMessage, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, false, nil
	}
	normalized, err := normalizeAdminSettingsJSON(data)
	if err != nil {
		return nil, false, fmt.Errorf("decode admin settings file: %w", err)
	}
	return normalized, true, nil
}

func (s *FileAdminSettingsStorage) Save(settings json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized, err := normalizeAdminSettingsJSON(settings)
	if err != nil {
		return fmt.Errorf("encode admin settings file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".dispatcharr-category-settings-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	data := append([]byte(nil), normalized...)
	data = append(data, '\n')
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func normalizeAdminSettingsJSON(data []byte) (json.RawMessage, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	normalized := normalizeAdminSettingsPayload(payload)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func normalizeAdminSettingsPayload(payload map[string]any) map[string]any {
	mode, _ := payload["mode"].(string)
	mode = strings.TrimSpace(mode)
	if mode == "custom" || mode == "admin_delimiter" {
		mode = "delimiter"
	}
	if mode != "normal" && mode != "delimiter" {
		mode = "normal"
	}

	delimiter, _ := payload["delimiter"].(string)
	delimiter = strings.TrimSpace(delimiter)
	if delimiter != "pipe" && delimiter != "dash" {
		delimiter = "pipe"
	}
	virtualGroupLabel := strings.TrimSpace(asStringValue(payload["virtualGroupLabel"]))
	if virtualGroupLabel == "" {
		virtualGroupLabel = "Virtual Groups"
	}

	allowRecordingsByDefault := true
	if enabled, ok := payload["allowRecordingsByDefault"].(bool); ok {
		allowRecordingsByDefault = enabled
	}
	collapseDuplicateVirtualGroups := true
	if enabled, ok := payload["collapseDuplicateVirtualGroups"].(bool); ok {
		collapseDuplicateVirtualGroups = enabled
	} else if enabled, ok := payload["collapseDuplicateProfileGroups"].(bool); ok {
		collapseDuplicateVirtualGroups = enabled
	}
	inferChannelNameGroups := false
	if enabled, ok := payload["inferChannelNameGroups"].(bool); ok {
		inferChannelNameGroups = enabled
	}
	virtualGroupSource := strings.TrimSpace(asStringValue(payload["virtualGroupSource"]))
	switch virtualGroupSource {
	case "group", "group_channel", "profile_group", "channel":
	default:
		if inferChannelNameGroups {
			virtualGroupSource = "group_channel"
		} else {
			virtualGroupSource = "group"
		}
	}
	if virtualGroupSource == "profile_group" {
		mode = "delimiter"
	}
	inferChannelNameGroups = virtualGroupSource != "group"
	ecmURL, _ := payload["ecmURL"].(string)
	ecmURL = normalizeAdminECMURL(ecmURL)
	ecmEnabled := ecmURL != ""
	categoryRenames := normalizeCategoryRenames(payload["categoryRenames"])
	categoryAliases := normalizeCategoryAliases(payload["categoryAliases"])
	eventKeywords := normalizeEventKeywordRules(payload["eventKeywords"])
	if len(eventKeywords) == 0 {
		eventKeywords = defaultEventKeywordRules()
	}

	return map[string]any{
		"mode":                           mode,
		"delimiter":                      delimiter,
		"virtualGroupLabel":              virtualGroupLabel,
		"virtualGroupSource":             virtualGroupSource,
		"ecmEnabled":                     ecmEnabled,
		"ecmURL":                         ecmURL,
		"allowRecordingsByDefault":       allowRecordingsByDefault,
		"collapseDuplicateVirtualGroups": collapseDuplicateVirtualGroups,
		"inferChannelNameGroups":         inferChannelNameGroups,
		"categoryRenames":                categoryRenames,
		"categoryAliases":                categoryAliases,
		"eventKeywords":                  eventKeywords,
	}
}

func normalizeAdminECMURL(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return trimmed
	}
	return defaultAdminECMURL
}

func normalizeCategoryAliases(value any) []map[string]string {
	items, ok := value.([]any)
	if !ok {
		return []map[string]string{}
	}
	seen := map[string]bool{}
	aliases := make([]map[string]string, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		sourcePath := strings.TrimSpace(asStringValue(row["sourcePath"]))
		aliasPath := strings.TrimSpace(asStringValue(row["aliasPath"]))
		if sourcePath == "" || aliasPath == "" {
			continue
		}
		key := sourcePath + "\x00" + aliasPath
		if seen[key] {
			continue
		}
		seen[key] = true
		aliases = append(aliases, map[string]string{
			"sourcePath": sourcePath,
			"aliasPath":  aliasPath,
		})
	}
	return aliases
}

func normalizeCategoryRenames(value any) []map[string]string {
	items, ok := value.([]any)
	if !ok {
		return []map[string]string{}
	}
	seen := map[string]bool{}
	renames := make([]map[string]string, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		sourcePath := strings.TrimSpace(asStringValue(row["sourcePath"]))
		displayName := strings.TrimSpace(asStringValue(row["displayName"]))
		if displayName == "" {
			displayName = strings.TrimSpace(asStringValue(row["aliasPath"]))
		}
		key := strings.ToLower(sourcePath)
		if sourcePath == "" || displayName == "" || seen[key] {
			continue
		}
		seen[key] = true
		renames = append(renames, map[string]string{
			"sourcePath":  sourcePath,
			"displayName": displayName,
		})
	}
	return renames
}

func asStringValue(value any) string {
	text, _ := value.(string)
	return text
}
