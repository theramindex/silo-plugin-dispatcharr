package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	goruntime "runtime"
	"sync"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/manifest"
	sdkruntime "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/app"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	pluginimpl "github.com/theramindex/silo-plugin-dispatcharr/internal/plugin"
)

//go:embed manifest.json
var manifestJSON []byte

// buildVersion is injected by CI via -ldflags:
//
//	-X main.buildVersion=<semver>
var buildVersion string

type runtimeServer struct {
	pluginv1.UnimplementedRuntimeServer
	manifest *pluginv1.PluginManifest
	settings *settingsState
}

type settingsState struct {
	mu       sync.RWMutex
	settings config.Settings
}

func (s *runtimeServer) GetManifest(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

func (s *runtimeServer) Configure(_ context.Context, request *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	if s.settings == nil {
		return &pluginv1.ConfigureResponse{}, nil
	}

	current := s.settings.Get()
	for _, entry := range request.GetConfig() {
		values := entry.GetValue().AsMap()
		switch entry.GetKey() {
		case "connection":
			current.DispatcharrURL = asString(values["base_url"])
			current.DispatcharrUser = asString(values["username"])
			current.DispatcharrPass = asString(values["password"])
			current.DispatcharrAPIKey = asString(values["api_key"])
			if stringValue, ok := values["source_mode"].(string); ok {
				current.SourceMode = config.SourceMode(stringValue)
			}
			if current.DispatcharrAPIKey != "" {
				current.SourceMode = config.SourceModeAPIKey
			} else if current.SourceMode == "" {
				current.SourceMode = config.SourceModeDirectLogin
			}
			if boolValue, ok := values["live_tv_enabled"].(bool); ok {
				current.LiveTVEnabled = boolValue
			}
			if numberValue, ok := values["channel_refresh_hours"].(float64); ok {
				current.ChannelRefreshH = int(numberValue)
			}
			if numberValue, ok := values["epg_refresh_hours"].(float64); ok {
				current.EPGRefreshH = int(numberValue)
			}
		case "general":
			if stringValue, ok := values["source_mode"].(string); ok {
				current.SourceMode = config.SourceMode(stringValue)
			}
			if boolValue, ok := values["live_tv_enabled"].(bool); ok {
				current.LiveTVEnabled = boolValue
			}
			if numberValue, ok := values["channel_refresh_hours"].(float64); ok {
				current.ChannelRefreshH = int(numberValue)
			}
			if numberValue, ok := values["epg_refresh_hours"].(float64); ok {
				current.EPGRefreshH = int(numberValue)
			}
		case "dispatcharr":
			current.DispatcharrURL = asString(values["base_url"])
			current.DispatcharrUser = asString(values["username"])
			current.DispatcharrPass = asString(values["password"])
			current.DispatcharrAPIKey = asString(values["api_key"])
		case "xtream":
			current.XtreamBaseURL = asString(values["base_url"])
			current.XtreamUsername = asString(values["username"])
			current.XtreamPassword = asString(values["password"])
		case "m3u_xmltv":
			current.M3UURL = asString(values["m3u_url"])
			current.EPGXMLURL = asString(values["epg_xml_url"])
		}
	}
	if current.ChannelRefreshH == 0 {
		current.ChannelRefreshH = config.DefaultChannelRefreshHours
	}
	if current.EPGRefreshH == 0 {
		current.EPGRefreshH = config.DefaultEPGRefreshHours
	}
	s.settings.Set(current)
	return &pluginv1.ConfigureResponse{}, nil
}

func (s *settingsState) Get() config.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *settingsState) Set(settings config.Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings
}

func main() {
	manifest, err := loadManifest()
	if err != nil {
		panic(err)
	}
	store := cache.NewStore()
	settings := &settingsState{settings: config.Settings{SourceMode: config.SourceModeDirectLogin, LiveTVEnabled: true, ChannelRefreshH: config.DefaultChannelRefreshHours, EPGRefreshH: config.DefaultEPGRefreshHours}}
	service := app.NewService(app.Dependencies{Store: store})

	sdkruntime.Serve(sdkruntime.ServeConfig{
		Servers: sdkruntime.CapabilityServers{
			Runtime:       &runtimeServer{manifest: manifest, settings: settings},
			ScheduledTask: pluginimpl.NewScheduledTaskServerWithProvider(service, settings.Get),
			HttpRoutes:    pluginimpl.NewHTTPRoutesServerWithSyncer(store, settings.Get, service),
		},
	})
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestJSON)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}

	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}

	binaryData, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}

	checksum := sha256.Sum256(binaryData)
	manifest.Checksum = hex.EncodeToString(checksum[:])
	if buildVersion != "" {
		manifest.Version = buildVersion
	}
	if len(manifest.GetSupportedPlatforms()) == 0 {
		manifest.SupportedPlatforms = []*pluginv1.SupportedPlatform{{
			Os:   goruntime.GOOS,
			Arch: goruntime.GOARCH,
		}}
	}
	manifest.GlobalConfigSchema = config.GlobalConfigSchema()
	manifest.UserConfigSchema = config.UserConfigSchema()

	return manifest, nil
}

func asString(value any) string {
	if stringValue, ok := value.(string); ok {
		return stringValue
	}
	return ""
}
