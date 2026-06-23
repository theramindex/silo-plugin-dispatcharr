package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidate_XtreamRequiresCredentials(t *testing.T) {
	t.Parallel()

	cfg := Settings{SourceMode: SourceModeXtream}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing xtream credentials")
	}
}

func TestValidate_DirectLoginRequiresDispatcharrCredentials(t *testing.T) {
	t.Parallel()

	cfg := Settings{SourceMode: SourceModeDirectLogin}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing Dispatcharr credentials")
	}
}

func TestValidate_M3UXMLTVRequiresURLs(t *testing.T) {
	t.Parallel()

	cfg := Settings{SourceMode: SourceModeM3UXMLTV}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for missing playlist and epg urls")
	}
}

func TestValidate_EPGRequiredForV1(t *testing.T) {
	t.Parallel()

	cfg := Settings{
		SourceMode: SourceModeM3UXMLTV,
		M3UURL:     "https://example.com/playlist.m3u",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when epg url is missing")
	}
}

func TestValidate_XtreamConfigPasses(t *testing.T) {
	t.Parallel()

	cfg := Settings{
		SourceMode:      SourceModeXtream,
		XtreamBaseURL:   "https://dispatcharr.example.com",
		XtreamUsername:  "demo",
		XtreamPassword:  "secret",
		LiveTVEnabled:   true,
		ChannelRefreshH: DefaultChannelRefreshHours,
		EPGRefreshH:     DefaultEPGRefreshHours,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid settings, got %v", err)
	}
}

func TestValidate_ExplicitSourceModeWinsOverLegacyAPIKey(t *testing.T) {
	t.Parallel()

	cfg := Settings{
		SourceMode:        SourceModeXtream,
		DispatcharrAPIKey: "legacy-key",
		XtreamBaseURL:     "https://provider.example.com",
		XtreamUsername:    "demo",
		XtreamPassword:    "secret",
		ChannelRefreshH:   DefaultChannelRefreshHours,
		EPGRefreshH:       DefaultEPGRefreshHours,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid settings, got %v", err)
	}
}

func TestValidate_DirectLoginConfigPasses(t *testing.T) {
	t.Parallel()

	cfg := Settings{
		SourceMode:      SourceModeDirectLogin,
		DispatcharrURL:  "https://dispatcharr.example.com",
		DispatcharrUser: "demo",
		DispatcharrPass: "secret",
		LiveTVEnabled:   true,
		ChannelRefreshH: DefaultChannelRefreshHours,
		EPGRefreshH:     DefaultEPGRefreshHours,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid settings, got %v", err)
	}
}

func TestGlobalConfigSchema_ContainsExpectedFields(t *testing.T) {
	t.Parallel()

	schema := GlobalConfigSchema()
	if len(schema) != 2 {
		t.Fatalf("expected two config schema entries, got %d", len(schema))
	}

	byKey := map[string]bool{}
	for _, item := range schema {
		byKey[item.GetKey()] = true
	}

	for _, key := range []string{"connection", "category_settings"} {
		if !byKey[key] {
			t.Fatalf("expected schema key %q", key)
		}
	}
}

func TestUserConfigSchema_DeclaresCurrentPreferenceShape(t *testing.T) {
	t.Parallel()

	userSchema := UserConfigSchema()
	if len(userSchema) != 2 {
		t.Fatalf("expected two user config schema entries, got %d", len(userSchema))
	}

	byKey := map[string]bool{}
	for _, item := range userSchema {
		byKey[item.GetKey()] = true
	}
	for _, key := range []string{"preferences", "adminCategorySettings"} {
		if !byKey[key] {
			t.Fatalf("expected user schema key %q", key)
		}
	}

	preferences := mustFindSchema(t, UserConfigSchema(), "preferences")
	var schema map[string]any
	if err := json.Unmarshal([]byte(preferences.GetJsonSchema()), &schema); err != nil {
		t.Fatalf("decode preferences schema: %v", err)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected preferences schema properties, got %q", preferences.GetJsonSchema())
	}
	for _, key := range []string{"favorites", "autoFavorites", "hiddenCategories", "recentChannels", "continueWatching", "playback", "categoryParsing", "customGroups", "customGroupMemberships"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("expected preferences schema to declare %q", key)
		}
	}
	if _, ok := properties["auto_favorites"]; ok {
		t.Fatal("preferences schema should use the camelCase frontend preference keys")
	}
}

func TestUserConfigSchema_DeclaresAdminCategorySettingsShape(t *testing.T) {
	t.Parallel()

	adminSettings := mustFindSchema(t, UserConfigSchema(), "adminCategorySettings")
	var schema map[string]any
	if err := json.Unmarshal([]byte(adminSettings.GetJsonSchema()), &schema); err != nil {
		t.Fatalf("decode admin category settings schema: %v", err)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected admin category settings schema properties, got %q", adminSettings.GetJsonSchema())
	}
	for _, key := range []string{"mode", "delimiter"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("expected admin category settings schema to declare %q", key)
		}
	}
	if additionalProperties, ok := schema["additionalProperties"].(bool); !ok || additionalProperties {
		t.Fatalf("expected admin category settings schema to reject unknown keys, got %+v", schema["additionalProperties"])
	}
}

func TestGlobalConfigSchema_SecretsAndStatusFields(t *testing.T) {
	t.Parallel()

	schema := GlobalConfigSchema()
	connection := mustFindSchema(t, schema, "connection")

	if !strings.Contains(connection.GetJsonSchema(), "writeOnly") {
		t.Fatalf("expected connection schema to declare writeOnly secret fields, got %q", connection.GetJsonSchema())
	}

	if !connection.GetRequired() {
		t.Fatal("expected connection schema to be required")
	}
}

func TestGlobalConfigSchema_UsesObjectSchemasForConfigurePayloads(t *testing.T) {
	t.Parallel()

	connection := mustFindSchema(t, GlobalConfigSchema(), "connection")
	var schema map[string]any
	if err := json.Unmarshal([]byte(connection.GetJsonSchema()), &schema); err != nil {
		t.Fatalf("decode connection schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("expected connection schema to be object-shaped, got %q", connection.GetJsonSchema())
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected connection schema properties, got %q", connection.GetJsonSchema())
	}
	for _, key := range []string{"source_mode", "base_url", "api_key", "username", "password", "m3u_url", "epg_xml_url"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("expected connection schema property %q", key)
		}
	}
}

func TestGlobalConfigSchema_ProvidesAdminFormsForSiloUI(t *testing.T) {
	t.Parallel()

	connection := mustFindSchema(t, GlobalConfigSchema(), "connection")

	if connection.GetAdminForm() == nil || len(connection.GetAdminForm().GetFields()) != 10 {
		t.Fatalf("expected connection admin form fields, got %+v", connection.GetAdminForm())
	}

	if connection.GetAdminForm().GetFields()[0].GetControl().String() != "ADMIN_FORM_CONTROL_SELECT" {
		t.Fatalf("expected source mode field control, got %s", connection.GetAdminForm().GetFields()[0].GetControl().String())
	}
	if len(connection.GetAdminForm().GetFields()[0].GetOptions()) != 4 {
		t.Fatalf("expected direct/api-key/xtream/m3u options, got %+v", connection.GetAdminForm().GetFields()[0].GetOptions())
	}
	if connection.GetAdminForm().GetFields()[2].GetControl().String() != "ADMIN_FORM_CONTROL_PASSWORD" {
		t.Fatalf("expected api key field control, got %s", connection.GetAdminForm().GetFields()[2].GetControl().String())
	}
	fieldKeys := map[string]bool{}
	for _, field := range connection.GetAdminForm().GetFields() {
		fieldKeys[field.GetKey()] = true
	}
	for _, key := range []string{"base_url", "username", "password", "m3u_url", "epg_xml_url"} {
		if !fieldKeys[key] {
			t.Fatalf("expected admin form field %q", key)
		}
	}
}

func mustFindSchema(t *testing.T, schema []*ConfigSchema, key string) *ConfigSchema {
	t.Helper()
	for _, item := range schema {
		if item.GetKey() == key {
			return item
		}
	}
	t.Fatalf("missing schema %q", key)
	return nil
}
