package main

import (
	"context"
	"encoding/json"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	configsdk "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestRuntimeConfigureReadsObjectShapedConfigEntries(t *testing.T) {
	t.Parallel()

	state := &settingsState{settings: config.Settings{SourceMode: config.SourceModeDirectLogin, LiveTVEnabled: true, ChannelRefreshH: config.DefaultChannelRefreshHours, EPGRefreshH: config.DefaultEPGRefreshHours}}
	server := &runtimeServer{settings: state}

	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{
		{Key: "connection", Value: mustStruct(t, map[string]any{"source_mode": "api_key", "base_url": "https://dispatcharr.example.com", "api_key": "secret", "live_tv_enabled": true})},
	}}

	if _, err := server.Configure(context.Background(), req); err != nil {
		t.Fatalf("configure: %v", err)
	}

	settings := state.Get()
	if settings.SourceMode != config.SourceModeAPIKey {
		t.Fatalf("expected source mode to update, got %q", settings.SourceMode)
	}
	if settings.DispatcharrURL == "" || settings.DispatcharrAPIKey == "" {
		t.Fatalf("expected dispatcharr connection to be loaded, got %+v", settings)
	}
}

func TestRuntimeConfigureMapsXtreamSharedConnectionFields(t *testing.T) {
	t.Parallel()

	state := &settingsState{settings: config.Settings{SourceMode: config.SourceModeDirectLogin, LiveTVEnabled: true, ChannelRefreshH: config.DefaultChannelRefreshHours, EPGRefreshH: config.DefaultEPGRefreshHours}}
	server := &runtimeServer{settings: state}

	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{
		{Key: "connection", Value: mustStruct(t, map[string]any{
			"source_mode":     "xtream",
			"base_url":        "https://dispatcharr.example.com",
			"username":        "xc-user",
			"password":        "xc-pass",
			"epg_xml_url":     "https://dispatcharr.example.com/xmltv.php?username=xc-user&password=xc-pass",
			"live_tv_enabled": true,
		})},
	}}

	if _, err := server.Configure(context.Background(), req); err != nil {
		t.Fatalf("configure: %v", err)
	}

	settings := state.Get()
	if settings.SourceMode != config.SourceModeXtream {
		t.Fatalf("expected xtream source mode, got %q", settings.SourceMode)
	}
	if settings.XtreamBaseURL != "https://dispatcharr.example.com" || settings.XtreamUsername != "xc-user" || settings.XtreamPassword != "xc-pass" {
		t.Fatalf("expected xtream connection to be loaded, got %+v", settings)
	}
	if settings.EPGXMLURL == "" {
		t.Fatalf("expected custom xmltv url to be saved, got %+v", settings)
	}
}

func TestRuntimeConfigureMapsM3UXMLTVFromConnectionEntry(t *testing.T) {
	t.Parallel()

	state := &settingsState{settings: config.Settings{SourceMode: config.SourceModeDirectLogin, LiveTVEnabled: true, ChannelRefreshH: config.DefaultChannelRefreshHours, EPGRefreshH: config.DefaultEPGRefreshHours}}
	server := &runtimeServer{settings: state}

	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{
		{Key: "connection", Value: mustStruct(t, map[string]any{
			"source_mode": "m3u_xmltv",
			"m3u_url":     "https://provider.example.com/playlist.m3u",
			"epg_xml_url": "https://provider.example.com/guide.xml",
		})},
	}}

	if _, err := server.Configure(context.Background(), req); err != nil {
		t.Fatalf("configure: %v", err)
	}

	settings := state.Get()
	if settings.SourceMode != config.SourceModeM3UXMLTV || settings.M3UURL == "" || settings.EPGXMLURL == "" {
		t.Fatalf("expected m3u/xmltv connection to be loaded, got %+v", settings)
	}
}

func TestRuntimeConfigureReadsCategorySettings(t *testing.T) {
	t.Parallel()

	state := &settingsState{settings: config.Settings{SourceMode: config.SourceModeDirectLogin, LiveTVEnabled: true, ChannelRefreshH: config.DefaultChannelRefreshHours, EPGRefreshH: config.DefaultEPGRefreshHours}}
	server := &runtimeServer{settings: state}

	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{
		{Key: "category_settings", Value: mustStruct(t, map[string]any{
			"mode":      "admin_delimiter",
			"delimiter": "pipe",
			"adminGroups": []any{
				map[string]any{"id": "admin:sports", "name": "Sports | Argentina", "order": 1},
			},
		})},
	}}

	if _, err := server.Configure(context.Background(), req); err != nil {
		t.Fatalf("configure: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(state.Get().AdminSettings, &settings); err != nil {
		t.Fatalf("decode admin settings: %v", err)
	}
	if settings["mode"] != "admin_delimiter" || settings["delimiter"] != "pipe" {
		t.Fatalf("expected category settings to be loaded, got %+v", settings)
	}
}

func TestManifestGlobalConfigSchemasValidateExpectedObjects(t *testing.T) {
	t.Parallel()

	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if err := configsdk.ValidateManifestGlobalValue(manifest, "connection", map[string]any{"source_mode": "api_key", "base_url": "https://dispatcharr.example.com", "api_key": "secret", "live_tv_enabled": true}); err != nil {
		t.Fatalf("validate connection schema: %v", err)
	}
	if err := configsdk.ValidateManifestGlobalValue(manifest, "connection", map[string]any{"source_mode": "xtream", "base_url": "https://provider.example.com", "username": "demo", "password": "secret", "epg_xml_url": "https://provider.example.com/guide.xml", "live_tv_enabled": true}); err != nil {
		t.Fatalf("validate xtream connection schema: %v", err)
	}
	if err := configsdk.ValidateManifestGlobalValue(manifest, "connection", map[string]any{"source_mode": "m3u_xmltv", "m3u_url": "https://provider.example.com/playlist.m3u", "epg_xml_url": "https://provider.example.com/guide.xml", "live_tv_enabled": true}); err != nil {
		t.Fatalf("validate m3u/xmltv connection schema: %v", err)
	}
	if err := configsdk.ValidateManifestGlobalValue(manifest, "connection", map[string]any{"source_mode": "m3u_xmltv", "m3u_url": "https://provider.example.com/playlist.m3u"}); err == nil {
		t.Fatalf("expected incomplete m3u/xmltv connection to fail validation")
	}
	if err := configsdk.ValidateManifestGlobalValue(manifest, "category_settings", map[string]any{
		"mode":      "admin_delimiter",
		"delimiter": "pipe",
		"adminGroups": []any{
			map[string]any{"id": "admin:sports", "name": "Sports | Argentina", "order": 1},
		},
		"adminGroupMemberships": map[string]any{"admin:sports": []any{"channel:1"}},
	}); err != nil {
		t.Fatalf("validate category settings schema: %v", err)
	}
}

func TestManifestExposesAdminNavigationRoute(t *testing.T) {
	t.Parallel()

	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	for _, route := range manifest.GetHttpRoutes() {
		if route.GetPath() != "/dispatcharr/admin" {
			continue
		}
		if !route.GetNavigable() || route.GetNavigationKind() != "admin" || route.GetNavigationLabel() != "Live TV Admin" || route.GetAccess() != "admin" {
			t.Fatalf("unexpected admin route metadata: %+v", route)
		}
		return
	}
	t.Fatalf("expected manifest to expose /dispatcharr/admin as a navigable admin route")
}

func TestManifestExposesAdminSettingsAPIRoutes(t *testing.T) {
	t.Parallel()

	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	found := map[string]bool{}
	for _, route := range manifest.GetHttpRoutes() {
		if route.GetPath() != "/dispatcharr/api/admin-settings" {
			continue
		}
		if route.GetAccess() != "admin" || route.GetNavigable() {
			t.Fatalf("unexpected admin settings route metadata: %+v", route)
		}
		found[route.GetMethod()] = true
	}
	if !found["GET"] || !found["POST"] {
		t.Fatalf("expected manifest to expose admin settings GET and POST routes, got %+v", found)
	}
}

func TestManifestKeepsInternalRefreshTasksOffPluginCard(t *testing.T) {
	t.Parallel()

	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	scheduledTaskIDs := make([]string, 0)
	for _, capability := range manifest.GetCapabilities() {
		if capability.GetType() == "scheduled_task.v1" {
			scheduledTaskIDs = append(scheduledTaskIDs, capability.GetId())
		}
	}
	if len(scheduledTaskIDs) != 1 || scheduledTaskIDs[0] != "dispatcharr-sync" {
		t.Fatalf("expected only the public scheduled task capability, got %+v", scheduledTaskIDs)
	}
}

func mustStruct(t *testing.T, value map[string]any) *structpb.Struct {
	t.Helper()
	result, err := structpb.NewStruct(value)
	if err != nil {
		t.Fatalf("new struct: %v", err)
	}
	return result
}
