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

func TestInferGuideSportsLeaguesUsesCityPrefixWhenOtherTeamIsUnique(t *testing.T) {
	t.Parallel()
	events := inferGuideSportsLeagues([]SportsEvent{
		{ID: "mlb-1", LeagueID: "mlb", LeagueName: "MLB", SportName: "Baseball", Away: SportsTeam{Name: "Tampa Bay Rays"}, Home: SportsTeam{Name: "New York Yankees"}},
		{ID: "nhl-1", LeagueID: "nhl", LeagueName: "NHL", SportName: "Hockey", Away: SportsTeam{Name: "Tampa Bay Lightning"}, Home: SportsTeam{Name: "Florida Panthers"}},
		{ID: "epg:1", LeagueID: "sports", LeagueName: "Sports", Away: SportsTeam{Name: "Tampa Bay"}, Home: SportsTeam{Name: "New York Yankees"}},
		{ID: "epg:2", LeagueID: "sports", LeagueName: "Sports", Away: SportsTeam{Name: "Tampa Bay"}, Home: SportsTeam{Name: "New York"}},
	})
	byID := map[string]SportsEvent{}
	for _, event := range events {
		byID[event.ID] = event
	}
	if got := byID["epg:1"]; got.LeagueID != "mlb" || got.LeagueName != "MLB" || got.SportName != "Baseball" {
		t.Fatalf("Tampa Bay at Yankees must move to MLB, got %+v", got)
	}
	if got := byID["epg:2"].LeagueID; got != "sports" {
		t.Fatalf("city-only Tampa Bay at New York must stay unresolved, got %q", got)
	}
}

func TestInferGuideSportsLeaguesSeparatesGLeagueExhibitions(t *testing.T) {
	t.Parallel()
	events := inferGuideSportsLeagues([]SportsEvent{{ID: "sportarr:1", LeagueID: "nba", LeagueName: "NBA", ProviderLeagueID: "nba", Away: SportsTeam{Name: "NBA G League United"}, Home: SportsTeam{Name: "Boca Juniors"}}})
	if events[0].LeagueID != "nba-g-league" || events[0].LeagueName != "NBA G League" || events[0].ProviderLeagueID != "nba" {
		t.Fatalf("G League exhibitions must list under NBA G League, got %+v", events[0])
	}
}
