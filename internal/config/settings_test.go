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
	if len(schema) == 0 {
		t.Fatal("expected config schema entries")
	}

	byKey := map[string]bool{}
	for _, item := range schema {
		byKey[item.GetKey()] = true
	}

	for _, key := range []string{"general", "dispatcharr", "xtream", "m3u_xmltv"} {
		if !byKey[key] {
			t.Fatalf("expected schema key %q", key)
		}
	}
}

func TestGlobalConfigSchema_SecretsAndStatusFields(t *testing.T) {
	t.Parallel()

	schema := GlobalConfigSchema()
	dispatcharr := mustFindSchema(t, schema, "dispatcharr")
	xtream := mustFindSchema(t, schema, "xtream")

	if !strings.Contains(dispatcharr.GetJsonSchema(), "writeOnly") {
		t.Fatalf("expected dispatcharr schema to declare writeOnly secret fields, got %q", dispatcharr.GetJsonSchema())
	}

	if !strings.Contains(xtream.GetJsonSchema(), "writeOnly") {
		t.Fatalf("expected xtream schema to declare writeOnly password field, got %q", xtream.GetJsonSchema())
	}

	if !dispatcharr.GetRequired() {
		t.Fatal("expected dispatcharr schema to be required")
	}
}

func TestGlobalConfigSchema_UsesObjectSchemasForConfigurePayloads(t *testing.T) {
	t.Parallel()

	general := mustFindSchema(t, GlobalConfigSchema(), "general")
	if !strings.Contains(general.GetJsonSchema(), `"type":"object"`) {
		t.Fatalf("expected general schema to be object-shaped, got %q", general.GetJsonSchema())
	}
}

func TestGlobalConfigSchema_ProvidesAdminFormsForSiloUI(t *testing.T) {
	t.Parallel()

	general := mustFindSchema(t, GlobalConfigSchema(), "general")
	dispatcharr := mustFindSchema(t, GlobalConfigSchema(), "dispatcharr")
	xtream := mustFindSchema(t, GlobalConfigSchema(), "xtream")
	m3u := mustFindSchema(t, GlobalConfigSchema(), "m3u_xmltv")

	if general.GetAdminForm() == nil || len(general.GetAdminForm().GetFields()) == 0 {
		t.Fatal("expected general schema to include admin form fields")
	}
	if dispatcharr.GetAdminForm() == nil || len(dispatcharr.GetAdminForm().GetFields()) != 5 {
		t.Fatalf("expected dispatcharr admin form fields, got %+v", dispatcharr.GetAdminForm())
	}
	if xtream.GetAdminForm() == nil || len(xtream.GetAdminForm().GetFields()) != 3 {
		t.Fatalf("expected xtream admin form fields, got %+v", xtream.GetAdminForm())
	}
	if m3u.GetAdminForm() == nil || len(m3u.GetAdminForm().GetFields()) != 2 {
		t.Fatalf("expected m3u/xmltv admin form fields, got %+v", m3u.GetAdminForm())
	}

	if dispatcharr.GetAdminForm().GetFields()[2].GetControl().String() != "ADMIN_FORM_CONTROL_PASSWORD" {
		t.Fatalf("expected dispatcharr password field control, got %s", dispatcharr.GetAdminForm().GetFields()[2].GetControl().String())
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
