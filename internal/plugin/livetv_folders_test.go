package plugin

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

func liveTVFolderServer(t *testing.T, adminSettings string, source model.Source, channels []model.Channel) *HTTPRoutesServer {
	t.Helper()
	store := cache.NewStore()
	store.Replace(cache.Snapshot{Catalog: model.CatalogState{Source: source, Channels: channels}})
	store.SetAdminSettings(json.RawMessage(adminSettings))
	return NewHTTPRoutesServerWithSettings(store, func() config.Settings { return config.Settings{} })
}

func liveTVFolders(t *testing.T, server *HTTPRoutesServer) map[string][]string {
	t.Helper()
	response := liveTVContractGet(t, server, "/silo-compat/livetv/v1/channels", nil)
	if response.GetStatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.GetStatusCode(), response.GetBody())
	}
	var lineup []livetvChannel
	if err := json.Unmarshal(response.GetBody(), &lineup); err != nil {
		t.Fatalf("decode channels: %v", err)
	}
	out := map[string][]string{}
	for _, channel := range lineup {
		out[channel.ID] = channel.Folders
	}
	return out
}

// Profile folders nest each profile's groups under it, drop the group name the
// profile already implies, and list a channel only in its deepest folders.
func TestLiveTVChannelFoldersFromProfilesAndGroups(t *testing.T) {
	t.Parallel()

	source := model.LiveTVSource(model.SourceModeDirectLogin)
	source.Profiles = []model.ChannelProfile{{ID: "1", Name: "US TV | NY"}, {ID: "2", Name: "US TV | NJ"}, {ID: "3", Name: "International Sports"}}
	server := liveTVFolderServer(t, `{"virtualGroupSource":"profile_group"}`, source, []model.Channel{
		{ID: "abc", Name: "NY | WABC", CategoryID: "10", CategoryName: "US TV | Locals | ABC", ProfileIDs: []string{"1", "2"}},
		{ID: "hbo", Name: "HBO", CategoryID: "11", CategoryName: "US TV | Premium", ProfileIDs: []string{"1"}},
		{ID: "dazn", Name: "DAZN 1", CategoryID: "12", CategoryName: "International Sports | DAZN", ProfileIDs: []string{"3"}},
		{ID: "beIN", Name: "beIN 1", CategoryID: "13", CategoryName: "International Sports | beIN", ProfileIDs: []string{"3"}},
		{ID: "bluey", Name: "Bluey", CategoryID: "14", CategoryName: "*24/7 | Kids"},
	})

	want := map[string][]string{
		"abc":   {"US TV / NY / Locals / ABC", "US TV / NJ / Locals / ABC"},
		"hbo":   {"US TV / NY / Premium"},
		"dazn":  {"International Sports / DAZN"},
		"beIN":  {"International Sports / beIN"},
		"bluey": {"24/7 / Kids"},
	}
	if got := liveTVFolders(t, server); !reflect.DeepEqual(got, want) {
		t.Fatalf("folders = %v, want %v", got, want)
	}
}

// Without delimiter parsing a channel's folder is its whole group name.
func TestLiveTVChannelFoldersFallBackToGroupName(t *testing.T) {
	t.Parallel()

	server := liveTVFolderServer(t, `{"mode":"normal"}`, model.LiveTVSource(model.SourceModeM3UXMLTV), []model.Channel{
		{ID: "news", Name: "News HD", CategoryID: "7", CategoryName: "*USA | News"},
		{ID: "loose", Name: "No Group"},
	})

	got := liveTVFolders(t, server)
	if want := []string{"USA | News"}; !reflect.DeepEqual(got["news"], want) {
		t.Fatalf("news folders = %v, want %v", got["news"], want)
	}
	if got["loose"] != nil {
		t.Fatalf("a channel without a group has no folders, got %v", got["loose"])
	}
}

// Channel-name folders file "NY | WABC" under US TV / Locals / NY and other
// prefixes under an International root picked from the name and group.
func TestLiveTVChannelFoldersFromChannelNames(t *testing.T) {
	t.Parallel()

	server := liveTVFolderServer(t, `{"mode":"delimiter","virtualGroupSource":"channel"}`, model.LiveTVSource(model.SourceModeM3UXMLTV), []model.Channel{
		{ID: "wabc", Name: "NY | ABC | WABC", CategoryID: "1", CategoryName: "Locals"},
		{ID: "sky", Name: "UK | Sky Sports | Main Event", CategoryID: "2", CategoryName: "UK"},
		{ID: "bbc", Name: "UK | BBC One", CategoryID: "2", CategoryName: "UK"},
	})

	want := map[string][]string{
		"wabc": {"US TV / Locals / NY / ABC"},
		"sky":  {"International Sports / UK / Sky Sports"},
		"bbc":  {"International TV / UK / BBC One"},
	}
	if got := liveTVFolders(t, server); !reflect.DeepEqual(got, want) {
		t.Fatalf("folders = %v, want %v", got, want)
	}
}
