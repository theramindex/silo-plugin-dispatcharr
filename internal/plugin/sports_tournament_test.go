package plugin

import (
	"strings"
	"testing"
	"time"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

func tournamentGuideEvents(t *testing.T, now time.Time, channelName, categoryName string, programs ...model.Program) map[string]SportsEvent {
	t.Helper()
	for index := range programs {
		programs[index].ChannelID = "channel:tournament"
		if programs[index].ID == "" {
			programs[index].ID = "program:" + programs[index].Title
		}
		if programs[index].StartUnix == 0 {
			programs[index].StartUnix = now.Add(-time.Hour).Unix()
			programs[index].EndUnix = now.Add(2 * time.Hour).Unix()
		}
	}
	events := sportsEventsFromGuide(cache.Snapshot{Catalog: model.CatalogState{
		Channels: []model.Channel{{ID: "channel:tournament", Name: channelName, CategoryID: "category", CategoryName: categoryName}},
		Programs: programs,
		Content:  model.ContentState{LiveCategories: []model.Category{{ID: "category", Name: categoryName, Kind: "live"}}},
	}}, now)
	byName := map[string]SportsEvent{}
	for _, event := range events {
		byName[event.Name] = event
	}
	return byName
}

func TestGuideAdmitsOneSidedTennisMajorListings(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 18, 0, 0, 0, time.UTC)
	events := tournamentGuideEvents(t, now, "ESPN", "Entertainment",
		model.Program{Title: "2026 US Open Tennis Championships"},
		model.Program{Title: "U.S. Open Tennis"},
		model.Program{Title: "2026 US Open: Quarterfinals"},
		model.Program{Title: "Wimbledon: Gentlemen's Final"},
		model.Program{Title: "Roland-Garros: Day 14"},
		model.Program{Title: "Australian Open: Women's Semifinals"},
	)
	want := map[string]string{
		"2026 US Open Tennis Championships":   "us-open-tennis",
		"U.S. Open Tennis":                    "us-open-tennis",
		"2026 US Open: Quarterfinals":         "us-open-tennis",
		"Wimbledon: Gentlemen's Final":        "wimbledon",
		"Roland-Garros: Day 14":               "roland-garros",
		"Australian Open: Women's Semifinals": "australian-open",
	}
	for title, leagueID := range want {
		event, ok := events[title]
		if !ok {
			t.Fatalf("one-sided tournament listing %q was dropped; got %v", title, sportsEventNames(events))
		}
		if event.LeagueID != leagueID || event.SportName != "Tennis" || event.EventType != "event" {
			t.Fatalf("%q classified as %q/%q/%q", title, event.LeagueID, event.SportName, event.EventType)
		}
		if event.Home.ID != "" || event.Away.ID != "" {
			t.Fatalf("one-sided listing %q must not create team identities: %+v %+v", title, event.Home, event.Away)
		}
	}
}

func TestGuideKeepsGenericOneSidedListingsOut(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 5, 18, 0, 0, 0, time.UTC)
	events := tournamentGuideEvents(t, now, "ESPN", "Sports",
		model.Program{Title: "SportsCenter"},
		model.Program{Title: "NBA Today"},
		model.Program{Title: "US Open Preview Show"},
		model.Program{Title: "Wimbledon Highlights"},
		model.Program{Title: "Tennis Tonight"},
		model.Program{Title: "Lamar Hunt U.S. Open Cup Final"},
	)
	if len(events) != 0 {
		t.Fatalf("generic or studio listings must stay out of sports events, got %v", sportsEventNames(events))
	}
}

func TestGuideAdmitsGolfMajorsAndMotorsportWithContext(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.June, 20, 18, 0, 0, 0, time.UTC)
	golf := tournamentGuideEvents(t, now, "NBC", "Golf",
		model.Program{Title: "2026 U.S. Open: Third Round"},
		model.Program{Title: "Masters Tournament: Final Round"},
		model.Program{Title: "PGA Championship: Second Round"},
		model.Program{Title: "The Open Championship: Final Round"},
	)
	for title, leagueID := range map[string]string{
		"2026 U.S. Open: Third Round":        "us-open-golf",
		"Masters Tournament: Final Round":    "masters-tournament",
		"PGA Championship: Second Round":     "pga-championship",
		"The Open Championship: Final Round": "the-open",
	} {
		if event, ok := golf[title]; !ok || event.LeagueID != leagueID || event.SportName != "Golf" {
			t.Fatalf("golf major %q = %+v (present %v)", title, event, ok)
		}
	}
	motorsport := tournamentGuideEvents(t, now, "ESPN2", "Entertainment",
		model.Program{Title: "Formula 1: Italian Grand Prix"},
	)
	if event, ok := motorsport["Formula 1: Italian Grand Prix"]; !ok || event.LeagueID != "formula-1" {
		t.Fatalf("one-sided Grand Prix listing must be admitted, got %v", sportsEventNames(motorsport))
	}
}

func TestGuideTennisMajorMatchupsResolveLeagueAndCompetitors(t *testing.T) {
	t.Parallel()
	for _, title := range []string{"US Open: Sinner vs. Alcaraz", "US Open Sinner vs Alcaraz", "2026 US Open Tennis Sinner vs Alcaraz", "Wimbledon Sinner vs. Alcaraz"} {
		away, home, ok := guideSportsMatchup(title)
		if !ok || away != "Sinner" || home != "Alcaraz" {
			t.Fatalf("%q parsed competitors %q / %q (%v)", title, away, home, ok)
		}
		match := guideSportsLeagueMatch(title)
		if !match.OK || match.Sport != "Tennis" || match.ID == "sports" {
			t.Fatalf("%q must resolve to a tennis major, got %+v", title, match)
		}
	}
	if id, _, _, _ := guideSportsLeague("US Open: Sinner vs. Alcaraz"); id != "us-open-tennis" {
		t.Fatalf("US Open matchup landed in %q", id)
	}
	if id, _, _, _ := guideSportsLeague("U.S. Open Golf: Scheffler vs McIlroy"); id != "us-open-golf" {
		t.Fatalf("golf context must select the golf U.S. Open, got %q", id)
	}
	if match := guideSportsLeagueMatch("Lamar Hunt U.S. Open Cup: Sounders vs Galaxy"); match.Tournament {
		t.Fatalf("the U.S. Open Cup is soccer, got %+v", match)
	}
	if match := guideSportsLeagueMatch("NFL Football: Jets at Giants"); match.Tournament {
		t.Fatalf("team leagues must not admit one-sided listings, got %+v", match)
	}
	if match := guideSportsLeagueMatch("Sports Tonight"); match.Tournament || match.ID != "sports" {
		t.Fatalf("generic fallback must never be a tournament, got %+v", match)
	}
}

func sportsEventNames(events map[string]SportsEvent) []string {
	names := make([]string, 0, len(events))
	for name := range events {
		names = append(names, name)
	}
	return names
}

func TestNamelessTeamsCarryNoIdentity(t *testing.T) {
	t.Parallel()
	if got := stableSportsTeamID(SportsTeam{}); got != legacyEmptySportsTeamID {
		t.Fatalf("legacy phantom team ID changed: %q", got)
	}
	team := normalizeSportsTeam(SportsTeam{ID: "provider:1", Abbreviation: "X"})
	if team.ID != "" {
		t.Fatalf("nameless team kept ID %q", team.ID)
	}
	finalized := finalizeSportsTeamIdentity(SportsTeam{})
	if finalized.ID != "" || finalized.FollowIDs != nil || finalized.LegacyIDs != nil {
		t.Fatalf("nameless team must not be followable: %+v", finalized)
	}
	events := normalizeSportsEvents([]SportsEvent{
		{Name: "2026 US Open Tennis Championships", EventType: "event", StartUnix: 1},
		{Name: "Formula 1: Italian Grand Prix", EventType: "event", StartUnix: 2},
	})
	for _, event := range events {
		if event.Home.ID != "" || event.Away.ID != "" {
			t.Fatalf("one-sided event emitted team IDs: %+v", event)
		}
	}
	// Event identities keep the key earlier builds derived from the shared
	// empty-name hash, so saved event reminders still resolve.
	legacy := "sports-event:" + sportsHash(strings.Join([]string{"formula-1", "", "", "event", legacyEmptySportsTeamID, legacyEmptySportsTeamID, "formula 1: italian grand prix"}, "|"))
	if events[1].StableID != legacy {
		t.Fatalf("stable event identity changed: %q != %q", events[1].StableID, legacy)
	}
}

func TestSportsTeamLegacyIDsCoverMergedAndShortNames(t *testing.T) {
	t.Parallel()
	guide := normalizeSportsTeam(SportsTeam{Name: "Yankees", Abbreviation: sportsTeamInitials("Yankees")})
	provider := normalizeSportsTeam(SportsTeam{ID: "sportarr-team:10", Name: "New York Yankees", Abbreviation: "NYY"})
	primary, supplemental := preferSportsTeamIdentity(guide, provider)
	merged := finalizeSportsTeamIdentity(mergeSportsTeamIdentity(primary, supplemental))
	if merged.ID != "sportarr-team:10" || merged.Name != "New York Yankees" {
		t.Fatalf("merge kept the wrong identity: %+v", merged)
	}
	legacy := strings.Join(merged.LegacyIDs, ",")
	if !strings.Contains(legacy, guide.ID) {
		t.Fatalf("guide copy's ID %q missing from legacyIds %v", guide.ID, merged.LegacyIDs)
	}
	for _, id := range append([]string{merged.ID}, merged.FollowIDs...) {
		if strings.Contains(legacy, id) {
			t.Fatalf("legacyIds must not repeat id/followIds (%q): %v", id, merged.LegacyIDs)
		}
	}
	if strings.Contains(legacy, legacyEmptySportsTeamID) {
		t.Fatal("legacyIds must never include the phantom empty-name ID")
	}

	// A Sportarr-only team still resolves follows saved on guide short names.
	alone := finalizeSportsTeamIdentity(normalizeSportsTeam(SportsTeam{ID: "sportarr-team:20", Name: "Michigan Wolverines"}))
	shortID := stableSportsTeamID(SportsTeam{Name: "Michigan", Abbreviation: "M"})
	nicknameID := stableSportsTeamID(SportsTeam{Name: "Wolverines", Abbreviation: "W"})
	ids := strings.Join(alone.LegacyIDs, ",")
	if !strings.Contains(ids, shortID) || !strings.Contains(ids, nicknameID) {
		t.Fatalf("short-name legacy IDs missing: %v", alone.LegacyIDs)
	}
	if len(alone.LegacyIDs) > sportsTeamLegacyIDLimit {
		t.Fatalf("legacyIds must stay bounded, got %d", len(alone.LegacyIDs))
	}
	if generic := sportsTeamShortNames("Manchester United"); len(generic) != 1 || generic[0] != "Manchester" {
		t.Fatalf("generic nicknames must not become legacy names: %v", generic)
	}
}
