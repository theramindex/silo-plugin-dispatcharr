package plugin

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The quadratic implementations the bucketed merge replaced; kept as the
// behavioral reference.
func referenceCollapseSportsEvents(events []SportsEvent) []SportsEvent {
	collapsed := make([]SportsEvent, 0, len(events))
	for _, event := range events {
		merged := false
		for index := range collapsed {
			if !sportsEventsSameMatchup(collapsed[index], event) {
				continue
			}
			overlaySportsGuideEvent(&collapsed[index], event)
			merged = true
			break
		}
		if !merged {
			collapsed = append(collapsed, event)
		}
	}
	return collapsed
}

func referenceMergeSportsGuideEvents(events, guideEvents []SportsEvent) []SportsEvent {
	merged := cloneSportsEvents(events)
	for _, guideEvent := range guideEvents {
		matched := false
		for index := range merged {
			if !sportsEventsSameMatchup(merged[index], guideEvent) {
				continue
			}
			overlaySportsGuideEvent(&merged[index], guideEvent)
			matched = true
			break
		}
		if !matched {
			if !guideEvent.Completed {
				for _, providerEvent := range merged {
					recent := guideEvent.Live || guideEvent.StartUnix-providerEvent.StartUnix < 36*3600
					if providerEvent.Completed && providerEvent.StartUnix < guideEvent.StartUnix && recent && sportsEventsSameIdentity(providerEvent, guideEvent) {
						guideEvent.Status = "replay"
						guideEvent.StatusText = "Replay"
						break
					}
				}
			}
			merged = append(merged, guideEvent)
		}
	}
	return referenceCollapseSportsEvents(merged)
}

func randomMergeEvents(random *rand.Rand, count int, base time.Time, prefix string) []SportsEvent {
	teams := []string{"Boston Bruins", "Montreal Canadiens", "Toronto Maple Leafs", "Bruins", "Canadiens", "New York Rangers", "Texas Rangers", "Rangers", "Seattle Kraken", "Kraken"}
	offsets := []time.Duration{0, 10 * time.Minute, 2 * time.Hour, 5 * time.Hour, 7 * time.Hour, 20 * time.Hour, 29 * time.Hour, 35 * time.Hour, 37 * time.Hour, 50 * time.Hour}
	events := make([]SportsEvent, 0, count)
	for index := 0; index < count; index++ {
		home := teams[random.Intn(len(teams))]
		away := teams[random.Intn(len(teams))]
		event := SportsEvent{
			ID: fmt.Sprintf("%s:%d", prefix, index), LeagueID: []string{"nhl", "sports", ""}[random.Intn(3)],
			Name: away + " at " + home, Home: SportsTeam{Name: home}, Away: SportsTeam{Name: away},
			StartUnix: base.Add(offsets[random.Intn(len(offsets))]).Unix(), Status: "scheduled",
		}
		switch random.Intn(12) {
		case 0:
			event.StartUnix = 0
		case 1:
			event.guidePhase = "pregame"
		case 2:
			event.Completed = true
			event.Status = "final"
		case 3:
			event.Live = true
		case 4:
			event.Venue = "Arena"
		case 5:
			event.ShortName = away + " vs " + home
			event.Home.Name = ""
		case 6:
			event.Home.Abbreviation = sportsTeamInitials(home)
			event.Away.Abbreviation = sportsTeamInitials(away)
		case 7:
			event.LeagueName = "NHL"
			event.Name = "NHL " + event.Name
		case 8:
			event.Home.Name += " Hockey Club"
		case 9:
			event.Home.Name = strings.ToUpper(home)
		case 10:
			event.EventType = "race"
			event.Name, event.ShortName = "British Grand Prix", "Formula One"
			event.Home.Name, event.Away.Name = "Silverstone", "Formula One"
		}
		events = append(events, event)
	}
	return events
}

func TestBucketedSportsMergeMatchesQuadraticReference(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, time.October, 3, 19, 0, 0, 0, time.UTC)
	for seed := int64(1); seed <= 40; seed++ {
		random := rand.New(rand.NewSource(seed))
		events := randomMergeEvents(random, 60, base, "provider")
		guide := randomMergeEvents(random, 60, base, "guide")
		if got, want := collapseEquivalentSportsEvents(cloneSportsEvents(events)), referenceCollapseSportsEvents(cloneSportsEvents(events)); !reflect.DeepEqual(got, want) {
			t.Fatalf("seed %d: bucketed collapse diverged\n got %d events\nwant %d events", seed, len(got), len(want))
		}
		if got, want := mergeSportsGuideEvents(events, guide), referenceMergeSportsGuideEvents(events, guide); !reflect.DeepEqual(got, want) {
			t.Fatalf("seed %d: bucketed guide merge diverged\n got %d events\nwant %d events", seed, len(got), len(want))
		}
	}
}

func TestSportsEventStartIndexTracksMovedStarts(t *testing.T) {
	t.Parallel()
	index := newSportsEventStartIndex()
	start := time.Date(2026, time.October, 3, 19, 0, 0, 0, time.UTC).Unix()
	index.add(0, start)
	index.add(1, 0)
	index.add(2, start+100*3600)
	if got := index.candidates(start+3600, nil); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("candidates = %v", got)
	}
	index.move(2, start+100*3600, start+2*3600)
	index.move(1, 0, start+200*3600)
	if got := index.candidates(start+3600, nil); !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("moved candidates = %v", got)
	}
	if got := index.candidates(0, nil); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("undated events compare against everything, got %v", got)
	}
}

func referenceNormalizeMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	space := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			space = false
			continue
		}
		if !space {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func TestNormalizeMatchTextMatchesReference(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", " ", "abc", "a b", "a  b", " a", "a ", "A.B. C", "Women's College Hockey", "NHL 05: Montreal Canadiens | Toronto",
		"İstanbul Başakşehir", "KELVIN K", "Ñandú FC", "\xff\xfeinvalid", "ᴸᶦᵛᵉ Yankees vs Red Sox ᴺᵉʷ", "  2026-09-05  ", "already normalized text",
	} {
		if got, want := normalizeMatchText(value), referenceNormalizeMatchText(value); got != want {
			t.Fatalf("normalizeMatchText(%q) = %q, want %q", value, got, want)
		}
	}
}

func BenchmarkCollapseEquivalentSportsEvents(b *testing.B) {
	base := time.Date(2026, time.October, 3, 19, 0, 0, 0, time.UTC)
	events := make([]SportsEvent, 0, 2000)
	for index := 0; index < 2000; index++ {
		events = append(events, SportsEvent{
			ID: fmt.Sprintf("event:%d", index), Name: fmt.Sprintf("Away %d at Home %d", index, index),
			Home: SportsTeam{Name: fmt.Sprintf("Home Club %d", index)}, Away: SportsTeam{Name: fmt.Sprintf("Away Club %d", index)},
			StartUnix: base.Add(time.Duration(index%96) * time.Hour).Unix(),
		})
	}
	b.Run("bucketed", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			collapseEquivalentSportsEvents(cloneSportsEvents(events))
		}
	})
	b.Run("quadratic-reference", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			referenceCollapseSportsEvents(cloneSportsEvents(events))
		}
	})
}
