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

	state := &settingsState{settings: config.Settings{SourceMode: config.SourceModeDirectLogin, LiveTVEnabled: true, ChannelRefreshH: config.DefaultChannelRefreshHours, EPGRefreshH: config.DefaultEPGRefreshHours}}
	server := &runtimeServer{settings: state}

	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{
		{Key: "connection", Value: mustStruct(t, map[string]any{"base_url": "https://dispatcharr.example.com", "api_key": "secret", "live_tv_enabled": true})},
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

func TestManifestGlobalConfigSchemasValidateExpectedObjects(t *testing.T) {
	t.Parallel()

	manifest, err := loadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	if err := configsdk.ValidateManifestGlobalValue(manifest, "connection", map[string]any{"base_url": "https://dispatcharr.example.com", "api_key": "secret", "live_tv_enabled": true}); err != nil {
		t.Fatalf("validate connection schema: %v", err)
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
