package config

import (
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
	if len(schema) != 1 {
		t.Fatalf("expected one config schema entry, got %d", len(schema))
	}

	byKey := map[string]bool{}
	for _, item := range schema {
		byKey[item.GetKey()] = true
	}

	for _, key := range []string{"connection"} {
		if !byKey[key] {
			t.Fatalf("expected schema key %q", key)
		}
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
	if !strings.Contains(connection.GetJsonSchema(), `"type":"object"`) {
		t.Fatalf("expected connection schema to be object-shaped, got %q", connection.GetJsonSchema())
	}
}

func TestGlobalConfigSchema_ProvidesAdminFormsForSiloUI(t *testing.T) {
	t.Parallel()

	connection := mustFindSchema(t, GlobalConfigSchema(), "connection")

	if connection.GetAdminForm() == nil || len(connection.GetAdminForm().GetFields()) != 11 {
		t.Fatalf("expected connection admin form fields, got %+v", connection.GetAdminForm())
	}

	if connection.GetAdminForm().GetFields()[0].GetControl().String() != "ADMIN_FORM_CONTROL_SELECT" {
		t.Fatalf("expected source mode field control, got %s", connection.GetAdminForm().GetFields()[0].GetControl().String())
	}
	if len(connection.GetAdminForm().GetFields()[0].GetOptions()) != 3 {
		t.Fatalf("expected direct/api-key/xtream options, got %+v", connection.GetAdminForm().GetFields()[0].GetOptions())
	}
	if connection.GetAdminForm().GetFields()[2].GetControl().String() != "ADMIN_FORM_CONTROL_PASSWORD" {
		t.Fatalf("expected api key field control, got %s", connection.GetAdminForm().GetFields()[2].GetControl().String())
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
