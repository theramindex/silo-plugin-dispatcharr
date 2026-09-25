package plugin

import "testing"

func TestInferGuideSportsLeaguesUsesProviderTeamNicknames(t *testing.T) {
	t.Parallel()
	events := inferGuideSportsLeagues([]SportsEvent{
		{ID: "mlb-1", LeagueID: "mlb", LeagueName: "MLB", SportName: "Baseball", Away: SportsTeam{Name: "San Diego Padres"}, Home: SportsTeam{Name: "Los Angeles Dodgers"}},
		{ID: "nfl-1", LeagueID: "nfl", LeagueName: "NFL", Away: SportsTeam{Name: "Arizona Cardinals"}, Home: SportsTeam{Name: "Seattle Seahawks"}},
		{ID: "mlb-2", LeagueID: "mlb", LeagueName: "MLB", Away: SportsTeam{Name: "St. Louis Cardinals"}, Home: SportsTeam{Name: "Chicago Cubs"}},
		{ID: "epg:1", LeagueID: "sports", LeagueName: "Sports", Away: SportsTeam{Name: "Padres"}, Home: SportsTeam{Name: "Dodgers"}},
		{ID: "epg:2", LeagueID: "sports", LeagueName: "Sports", Away: SportsTeam{Name: "Cardinals"}, Home: SportsTeam{Name: "Cubs"}},
		{ID: "epg:3", LeagueID: "sports", LeagueName: "Sports", Away: SportsTeam{Name: "Padres"}, Home: SportsTeam{Name: "Seahawks"}},
		{ID: "epg:4", LeagueID: "sports", LeagueName: "Sports", EventType: "event", Away: SportsTeam{Name: "Padres"}, Home: SportsTeam{Name: "Dodgers"}},
	})
	byID := map[string]SportsEvent{}
	for _, event := range events {
		byID[event.ID] = event
	}
	if got := byID["epg:1"]; got.LeagueID != "mlb" || got.LeagueName != "MLB" || got.SportName != "Baseball" {
		t.Fatalf("Padres vs Dodgers must move to MLB, got %+v", got)
	}
	if got := byID["epg:2"].LeagueID; got != "sports" {
		t.Fatalf("a nickname shared across leagues must stay unresolved, got %q", got)
	}
	if got := byID["epg:3"].LeagueID; got != "sports" {
		t.Fatalf("teams from different leagues must not be reassigned, got %q", got)
	}
	if got := byID["epg:4"].LeagueID; got != "sports" {
		t.Fatalf("studio programs must not be reassigned, got %q", got)
	}
}
