package main

import (
	"context"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	configsdk "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestRuntimeConfigureReadsObjectShapedConfigEntries(t *testing.T) {
	t.Parallel()

	state := &settingsState{settings: config.Settings{SourceMode: config.SourceModeXtream, LiveTVEnabled: true, ChannelRefreshH: config.DefaultChannelRefreshHours, EPGRefreshH: config.DefaultEPGRefreshHours}}
	server := &runtimeServer{settings: state}

	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{
		{Key: "general", Value: mustStruct(t, map[string]any{"source_mode": "m3u_xmltv", "live_tv_enabled": true})},
		{Key: "m3u_xmltv", Value: mustStruct(t, map[string]any{"m3u_url": "https://dispatcharr.example.com/playlist.m3u", "epg_xml_url": "https://dispatcharr.example.com/guide.xml"})},
	}}

	if _, err := server.Configure(context.Background(), req); err != nil {
		t.Fatalf("configure: %v", err)
	}

	settings := state.Get()
	if settings.SourceMode != config.SourceModeM3UXMLTV {
		t.Fatalf("expected source mode to update, got %q", settings.SourceMode)
	}
	if settings.M3UURL == "" || settings.EPGXMLURL == "" {
		t.Fatalf("expected m3u/xmltv urls to be loaded, got %+v", settings)
	}
}

func TestManifestGlobalConfigSchemasValidateExpectedObjects(t *testing.T) {
	t.Parallel()

	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if err := configsdk.ValidateManifestGlobalValue(manifest, "general", map[string]any{"source_mode": "xtream", "live_tv_enabled": true}); err != nil {
		t.Fatalf("validate general schema: %v", err)
	}
	if err := configsdk.ValidateManifestGlobalValue(manifest, "xtream", map[string]any{"base_url": "https://dispatcharr.example.com", "username": "demo", "password": "secret"}); err != nil {
		t.Fatalf("validate xtream schema: %v", err)
	}
	if err := configsdk.ValidateManifestGlobalValue(manifest, "m3u_xmltv", map[string]any{"m3u_url": "https://dispatcharr.example.com/playlist.m3u", "epg_xml_url": "https://dispatcharr.example.com/guide.xml"}); err != nil {
		t.Fatalf("validate m3u/xmltv schema: %v", err)
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
