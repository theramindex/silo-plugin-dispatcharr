package plugin

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

func guideRecoveryServer(t *testing.T, programs []model.Program) (*HTTPRoutesServer, *controlledRefreshTarget) {
	t.Helper()
	settings := config.Settings{SourceMode: config.SourceModeAPIKey, DispatcharrURL: "https://dispatcharr.example", DispatcharrAPIKey: "secret"}
	source := model.LiveTVSource(modelSourceModeForSettings(settings))
	source.ProfileAccess = &model.ProfileAccess{Status: "available"}
	store := cache.NewStore()
	store.Replace(cache.Snapshot{
		Catalog: model.CatalogState{
			Source:   source,
			Channels: []model.Channel{{ID: "m3u:news.hd", Name: "News HD"}},
			Programs: programs,
		},
		ConfigKey: config.CatalogCacheKey(settings),
	})
	target := &controlledRefreshTarget{started: make(chan RefreshOperation, 2), release: make(chan struct{})}
	coordinator := NewRefreshCoordinator(target)
	t.Cleanup(func() {
		close(target.release)
		coordinator.Close()
	})
	dir := t.TempDir()
	server := NewHTTPRoutesServerWithCoordinatorAndSettingsFiles(store, func() config.Settings { return settings }, coordinator,
		filepath.Join(dir, "admin.json"), config.NewConnectionStore(filepath.Join(dir, "connection.json")))
	return server, target
}

// A catalog with channels but no guide, such as one rebuilt by a channels-only
// refresh after the saved snapshot was lost, loads the guide on its next
// request instead of waiting for the scheduled guide refresh.
func TestEnsureCatalogHydratedLoadsMissingGuide(t *testing.T) {
	t.Parallel()

	server, target := guideRecoveryServer(t, nil)
	server.ensureCatalogHydrated(context.Background())
	if operation := waitForRefreshOperation(t, target.started); operation != RefreshGuide {
		t.Fatalf("started %s, want a guide refresh", operation)
	}
	if status := server.store.Current().Health.EPGStatus; status != "loading" {
		t.Fatalf("EPG status = %q, want loading", status)
	}
}

func TestEnsureCatalogHydratedLeavesLoadedGuide(t *testing.T) {
	t.Parallel()

	server, target := guideRecoveryServer(t, []model.Program{{ID: "p1", ChannelID: "m3u:news.hd", Title: "News"}})
	server.ensureCatalogHydrated(context.Background())
	select {
	case operation := <-target.started:
		t.Fatalf("started %s with a guide already loaded", operation)
	case <-time.After(100 * time.Millisecond):
	}
}
