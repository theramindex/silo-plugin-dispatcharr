package plugin

import (
	"reflect"
	"strings"
	"testing"
)

func TestGameThumbsBroadLeagueMappings(t *testing.T) {
	cases := []struct{ title, slug string }{
		{"Women's College Field Hockey: Michigan at Duke", "ncaawfh"},
		{"Women's College Volleyball: Stanford at Oregon", "ncaawvb"},
		{"Men's College Soccer: Michigan at Notre Dame", "ncaas"},
		{"Women's College Gymnastics: UCLA at Utah", "ncaa"},
		{"Canadian Premier League Soccer: Inter Toronto at Forge", "can.1"},
		{"Scottish Premier League: Celtic at Rangers", "spfl"},
		{"NWSL Soccer: Washington Spirit at Portland Thorns", "usa.nwsl"},
		{"FIBA Women's Basketball World Cup: USA vs Italy", "fiba-women"},
		{"Rugby World Cup: France vs Italy", "rwc"},
		{"Women's Rugby World Cup: France vs Italy", "wrwc"},
		{"Formula 1: Italian Grand Prix", "F1"},
		{"Ontario Hockey League: London Knights at Ottawa 67s", "ohl"},
		{"Korean Baseball Organization: Samsung Lions at Kia Tigers", "kbo"},
		{"ATP Tour: Djokovic vs Alcaraz", "atp"},
		{"UEFA Women's Champions League: Chelsea at Barcelona", "uefa.wchampions"},
		{"Caribbean Premier League: Guyana at St Kitts", "cpl"},
		{"Mystery Sporting Event: College Park at New York", ""},
	}
	for _, tc := range cases {
		if got := gameThumbsLeagueSlugForEvent(SportsEvent{Name: tc.title}); got != tc.slug {
			t.Errorf("%q: got %q, want %q", tc.title, got, tc.slug)
		}
	}
}

func TestGameThumbsArtworkPreservesFallbacksAndIdentity(t *testing.T) {
	input := SportsEvent{ID: "original-id", StableID: "stable-id", LeagueID: "nba", LeagueName: "NBA", Name: "Lakers at Celtics", LeagueLogoURL: "https://example.com/nba.png", Home: SportsTeam{ID: "home", Name: "Celtics", LogoURL: "https://example.com/celtics.png"}, Away: SportsTeam{ID: "away", Name: "Lakers"}}
	got := applySportsIdentityFallbacks(input)
	if got.ID != input.ID || got.StableID != input.StableID || got.Home.ID != input.Home.ID || got.LeagueID != input.LeagueID {
		t.Fatal("artwork changes must preserve event, team, and league identities")
	}
	if got.Home.LogoFallbackURL != input.Home.LogoURL || got.LeagueLogoFallbackURL != input.LeagueLogoURL || got.Home.LogoURL != gameThumbsTeamLogoURL("nba", "Celtics") {
		t.Fatalf("missing preferred/fallback logo: %+v", got)
	}
	if !strings.Contains(got.GameThumbsBackgroundURL, "/nba/lakers/celtics/thumb.png?") {
		t.Fatal("expected generated matchup background")
	}
	if again := applySportsIdentityFallbacks(got); !reflect.DeepEqual(got, again) {
		t.Fatal("repeated normalization must preserve preferred and fallback artwork")
	}
	program := applySportsIdentityFallbacks(SportsEvent{LeagueID: "boxing", EventType: "event", Name: "Two boxing bouts", Home: SportsTeam{Name: "Not a fighter"}})
	if strings.Contains(program.GameThumbsBackgroundURL, "not-a-fighter") {
		t.Fatal("programs must use league backgrounds rather than invented matchups")
	}
}
