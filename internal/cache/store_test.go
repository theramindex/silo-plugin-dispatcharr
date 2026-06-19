package cache

import (
	"testing"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

func TestStorePreservesLastSuccessfulSnapshotOnFailure(t *testing.T) {
	t.Parallel()

	store := NewStore()
	snapshot := Snapshot{Catalog: model.CatalogState{Source: model.LiveTVSource(model.SourceModeXtream), Channels: []model.Channel{{ID: "xtream:1", Name: "News"}}}}
	store.Replace(snapshot)
	store.RecordFailure(100, "upstream unavailable")

	current := store.Current()
	if len(current.Catalog.Channels) != 1 {
		t.Fatalf("expected stale channels to remain available, got %d", len(current.Catalog.Channels))
	}
	if current.Health.LastError != "upstream unavailable" {
		t.Fatalf("expected failure to be tracked, got %q", current.Health.LastError)
	}
}

func TestStoreReplaceClearsPreviousFailureOnSuccess(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.RecordFailure(100, "timeout")
	store.Replace(Snapshot{Catalog: model.CatalogState{Source: model.LiveTVSource(model.SourceModeXtream)}, Health: model.SyncHealth{LastSuccessUnix: 200}})

	current := store.Current()
	if current.Health.LastFailureUnix != 0 {
		t.Fatalf("expected failure timestamp to clear, got %d", current.Health.LastFailureUnix)
	}
	if current.Health.LastError != "" {
		t.Fatalf("expected failure message to clear, got %q", current.Health.LastError)
	}
	if current.Health.LastSuccessUnix != 200 {
		t.Fatalf("expected success timestamp to persist, got %d", current.Health.LastSuccessUnix)
	}
}

func TestStoreDoesNotPersistPlaybackURLStateSeparately(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.Replace(Snapshot{Catalog: model.CatalogState{Channels: []model.Channel{{ID: "xtream:1", StreamURL: "https://example.com/live.m3u8"}}}})

	current := store.Current()
	if current.Catalog.Channels[0].StreamURL != "https://example.com/live.m3u8" {
		t.Fatalf("expected catalog snapshot to preserve stream url field, got %q", current.Catalog.Channels[0].StreamURL)
	}
	if current.PlaybackResolvedAtUnix != 0 {
		t.Fatalf("expected no cached playback resolution timestamp, got %d", current.PlaybackResolvedAtUnix)
	}
}
