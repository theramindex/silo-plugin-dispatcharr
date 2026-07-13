package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "connection.json")
	store := NewConnectionStore(path)
	want := ConnectionSettings{
		SourceMode:      SourceModeDirectLogin,
		DispatcharrURL:  " https://dispatcharr.example.com/ ",
		DispatcharrUser: " admin ",
		DispatcharrPass: "secret",
		ChannelProfile:  " All Profiles ",
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	got, ok, err := store.Load()
	if err != nil || !ok {
		t.Fatalf("load connection: ok=%v err=%v", ok, err)
	}
	if got.DispatcharrURL != "https://dispatcharr.example.com" || got.DispatcharrUser != "admin" || got.ChannelProfile != "All Profiles" || got.DispatcharrPass != "secret" {
		t.Fatalf("unexpected normalized connection: %+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat connection file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("connection file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestConnectionStoreRejectsIncompleteSource(t *testing.T) {
	store := NewConnectionStore(filepath.Join(t.TempDir(), "connection.json"))
	err := store.Save(ConnectionSettings{SourceMode: SourceModeAPIKey, DispatcharrURL: "https://dispatcharr.example.com"})
	if err == nil || err.Error() != "dispatcharr api key is required" {
		t.Fatalf("expected API key validation error, got %v", err)
	}
}

func TestConnectionSettingsApplyOverridesOnlyConnectionFields(t *testing.T) {
	settings := Settings{LiveTVEnabled: true, ChannelRefreshH: 12, EPGRefreshH: 6, AdminSettings: []byte(`{"mode":"delimiter"}`)}
	ConnectionSettings{SourceMode: SourceModeM3UXMLTV, M3UURL: "https://example.com/live.m3u", EPGXMLURL: "https://example.com/guide.xml"}.Apply(&settings)
	if settings.EffectiveSourceMode() != SourceModeM3UXMLTV || settings.M3UURL == "" || settings.EPGXMLURL == "" {
		t.Fatalf("connection was not applied: %+v", settings)
	}
	if !settings.LiveTVEnabled || settings.ChannelRefreshH != 12 || settings.EPGRefreshH != 6 || len(settings.AdminSettings) == 0 {
		t.Fatalf("non-connection settings changed: %+v", settings)
	}
}
