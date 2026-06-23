package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	normalized, err := normalizeAdminSettingsJSON(settings)
	if err != nil {
		return fmt.Errorf("encode admin settings file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	data := append([]byte(nil), normalized...)
	data = append(data, '\n')
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
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

	ecmEnabled := true
	if enabled, ok := payload["ecmEnabled"].(bool); ok {
		ecmEnabled = enabled
	}
	ecmURL, _ := payload["ecmURL"].(string)
	ecmURL = normalizeAdminECMURL(ecmURL)

	return map[string]any{
		"mode":       mode,
		"delimiter":  delimiter,
		"ecmEnabled": ecmEnabled,
		"ecmURL":     ecmURL,
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
